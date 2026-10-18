"""Prepare native Pi configuration and use Pi itself for startup checks."""

from __future__ import annotations

import copy
import contextlib
import json
import os
import re
import shutil
import signal
import subprocess
from pathlib import Path

from factorylib import ROOT, write_json

FILES = ("settings", "models", "mcp")
REFERENCE = re.compile(r"(?<!\$)\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))")


def object_file(path: Path) -> dict:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"expected a JSON object: {path}")
    return value


def merge(base: dict, overlay: dict) -> dict:
    result = copy.deepcopy(base)
    for key, value in overlay.items():
        result[key] = merge(result[key], value) if isinstance(result.get(key), dict) and isinstance(value, dict) else copy.deepcopy(value)
    return result


def environment_references(value: object) -> set[str]:
    if isinstance(value, dict):
        return set().union(*(environment_references(item) for item in value.values())) if value else set()
    if isinstance(value, list):
        return set().union(*(environment_references(item) for item in value)) if value else set()
    if isinstance(value, str) and not value.startswith("!"):
        return {first or second for first, second in REFERENCE.findall(value)}
    return set()


def private_auth_file(config: dict, field: str, environment: dict) -> Path | None:
    reference = config.get(field)
    if reference is None:
        return None
    if not isinstance(reference, str) or not reference:
        raise ValueError(f"{field} must name an existing environment variable")
    if not environment.get(reference):
        raise ValueError(f"missing environment variable for private Pi authentication file: {reference}")
    path = Path(environment[reference]).expanduser()
    if not path.is_absolute():
        raise ValueError(f"{field} must reference an absolute file path")
    path = path.resolve()
    if not path.is_file() or path.is_relative_to(ROOT):
        raise ValueError(f"{field} must identify an existing private auth file outside .factory")
    return path


def load(directory: Path, config: dict, project: Path, session: Path, environment: dict) -> dict:
    configured_path = config.get("config_dir", "pi")
    if not isinstance(configured_path, str) or not configured_path:
        raise ValueError("Pi config_dir must be a nonempty directory path")
    source = (directory / configured_path).resolve()
    if not source.is_relative_to(ROOT) or not source.is_dir():
        raise ValueError("Pi config_dir must be a directory inside the portable toolbox")
    files = {}
    for name in FILES:
        base = object_file(source / f"{name}.json")
        local = source / f"{name}.local.json"
        files[name] = merge(base, object_file(local)) if local.exists() else base
    settings, models, mcp = (files[name] for name in FILES)
    provider, model = settings.get("defaultProvider"), settings.get("defaultModel")
    for name, value in (("defaultProvider", provider), ("defaultModel", model)):
        if value is not None and (not isinstance(value, str) or not value.strip()):
            raise ValueError(f"{source}/settings.json: {name} must be a nonempty string when configured")
    providers = models.get("providers", {})
    servers = mcp.get("mcpServers", {})
    if not isinstance(providers, dict) or not isinstance(servers, dict):
        raise ValueError("models.providers and mcp.mcpServers must be JSON objects")
    # Helpers and native agents must use the same service, including local port
    # overrides and optional shared-token authentication. Preserve other MCPs.
    if "comms" not in servers or servers["comms"].get("enabled", True) is False:
        raise ValueError("Factory sessions require the comms MCP connection")
    servers["comms"]["url"] = environment["FACTORY_COMMS_URL"]
    servers["comms"].pop("command", None)
    servers["comms"].pop("args", None)
    if environment.get("COMMS_AUTH_TOKEN"):
        servers["comms"].setdefault("headers", {})["Authorization"] = "Bearer ${COMMS_AUTH_TOKEN}"
    # Pi resolves model credentials itself, including native auth-file precedence.
    required = set()
    enabled = []
    for name, server in servers.items():
        if not isinstance(server, dict):
            raise ValueError(f"MCP server {name}: expected an object")
        if server.get("enabled", True) is False:
            continue
        enabled.append(name)
        required |= environment_references(server)
        if "command" in server:
            cwd = Path(server.get("cwd", str(project))).expanduser()
            server["cwd"] = str((project / cwd).resolve() if not cwd.is_absolute() else cwd.resolve())
    auth_files = {}
    for name, field in (("auth.json", "auth_file_env"), ("mcp-auth.json", "mcp_auth_file_env")):
        path = private_auth_file(config, field, environment)
        if path is not None:
            auth_files[name] = path
    target = session / "pi"
    environment["PI_CODING_AGENT_DIR"] = str(target)
    # Parent/session environment must not redirect these explicit runtime choices.
    environment.pop("PI_CODING_AGENT_SESSION_DIR", None)
    return {
        "source": source, "target": target, "files": files, "auth_files": auth_files,
        "required_env": sorted(required), "enabled_mcps": enabled,
        "summary": {
            "source_dir": str(source), "runtime_dir": str(target),
            "provider": provider, "model": model, "thinking": settings.get("defaultThinkingLevel"),
            "configured": bool(provider and model), "enabled_mcps": enabled,
            "required_environment": sorted(required), "shared_auth_files": sorted(auth_files),
        },
    }


def materialize(profile: dict) -> None:
    # Keep native relative resource paths valid without rewriting Pi's schema.
    # Credential/trust state is never imported from the authored profile.
    ignored = shutil.ignore_patterns("*.local.json", "auth.json", "mcp-auth.json", "trust.json", "*.log", "__pycache__")
    shutil.copytree(profile["source"], profile["target"], ignore=ignored)
    # copytree preserves source modes, including a read-only checkout's modes.
    # Runtime files must remain writable and private regardless of their source.
    for path in [profile["target"], *profile["target"].rglob("*")]:
        path.chmod(0o700 if path.is_dir() else 0o600 | (path.stat().st_mode & 0o100))
    for name, value in profile["files"].items():
        write_json(profile["target"] / f"{name}.json", value)
    for name, source in profile["auth_files"].items():
        (profile["target"] / name).symlink_to(source)


def run_check(command: list[str], environment: dict, cwd: Path, timeout: float, input_text: str | None = None) -> dict:
    """Bound startup checks and terminate their children if a server hangs."""
    process = subprocess.Popen(command, cwd=cwd, env=environment, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               text=True, start_new_session=True)
    try:
        stdout, stderr = process.communicate(input=input_text, timeout=timeout)
    except subprocess.TimeoutExpired:
        if os.name == "posix":
            with contextlib.suppress(ProcessLookupError):
                os.killpg(process.pid, signal.SIGTERM)
        else:
            process.terminate()
        try:
            stdout, stderr = process.communicate(timeout=2)
        except subprocess.TimeoutExpired:
            if os.name == "posix":
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(process.pid, signal.SIGKILL)
            else:
                process.kill()
            stdout, stderr = process.communicate()
        return {"exit_code": process.returncode, "timed_out": True, "stdout": stdout, "stderr": stderr}
    return {"exit_code": process.returncode, "timed_out": False, "stdout": stdout, "stderr": stderr}


def check(profile: dict, executable: str, environment: dict, cwd: Path, timeout: float,
          required_commands: tuple[str, ...] = ()) -> dict:
    settings = profile["files"]["settings"]
    provider, model = settings.get("defaultProvider"), settings.get("defaultModel")
    if not provider or not model:
        raise ValueError(f"set defaultProvider and defaultModel in {profile['source']}/settings.json (or settings.local.json)")
    missing = [name for name in profile["required_env"] if not environment.get(name)]
    if missing:
        raise ValueError("missing configuration environment variable(s): " + ", ".join(missing))
    report_path = profile["target"].parent / "startup-checks.json"
    report = {"provider": provider, "model": model, "checks": {}}
    # These native commands do not invoke a chat model. They may execute configured
    # credential commands or contact explicitly enabled MCP servers.
    commands = [
        ("credentials", [executable, "auth", "check", "--provider", provider, "--model", model, "--json", "--no-refresh"]),
        ("mcp", [executable, "mcp", "list", "--json"]),
    ]
    check_environment = {**environment, "PI_OFFLINE": "1"}
    for name, command in commands:
        result = run_check(command, check_environment, cwd, timeout)
        report["checks"][name] = result
        write_json(report_path, report)
        if result["timed_out"] or result["exit_code"]:
            reason = "timed out" if result["timed_out"] else "failed"
            raise ValueError(f"Pi {name} startup check {reason}; private diagnostic output: {report_path}")
        try:
            data = json.loads(result["stdout"])
        except json.JSONDecodeError as error:
            raise ValueError(f"Pi {name} returned invalid JSON; check the installed Pi version; details: {report_path}") from error
        if name == "credentials" and (not isinstance(data, dict) or data.get("status") != "ready"):
            raise ValueError(f"Pi credentials are not ready; details: {report_path}")
        if name == "mcp":
            if not isinstance(data, dict) or not isinstance(data.get("servers"), list) or data.get("errors"):
                raise ValueError(f"Pi MCP configuration is invalid; details: {report_path}")
            if any(not isinstance(entry, dict) or not isinstance(entry.get("name"), str) for entry in data["servers"]):
                raise ValueError(f"Pi MCP returned an unsupported server report; details: {report_path}")
            connections = {entry["name"]: entry for entry in data["servers"]}
            unavailable = [server for server in profile["enabled_mcps"]
                           if connections.get(server, {}).get("state") != "connected"]
            if unavailable:
                raise ValueError(f"MCP server(s) not connected: {', '.join(unavailable)}; details: {report_path}")
            report["mcps"] = [{"name": server, "tools": connections[server].get("tools", []),
                               "exposure": connections[server].get("exposure")} for server in profile["enabled_mcps"]]
    if required_commands:
        # Ask the real interactive runtime to load its extensions and list commands.
        # This never sends a model prompt. Pi manages declared package installation.
        command = [executable, "--mode", "rpc", "--no-session", "--no-skills", "--no-approve"]
        request = json.dumps({"type": "get_commands", "id": "factory-startup"}) + "\n"
        result = run_check(command, environment, cwd, timeout, request)
        report["checks"]["extensions"] = result
        write_json(report_path, report)
        if result["timed_out"] or result["exit_code"]:
            reason = "timed out" if result["timed_out"] else "failed"
            raise ValueError(f"Pi extension startup check {reason}; private diagnostic output: {report_path}")
        responses = []
        for line in result["stdout"].splitlines():
            try:
                responses.append(json.loads(line))
            except json.JSONDecodeError:
                continue  # Package progress and native notices are not RPC responses.
        response = next((item for item in responses if isinstance(item, dict) and item.get("id") == "factory-startup"), {})
        commands = response.get("data", {}).get("commands", []) if response.get("success") else []
        available = {entry.get("name") for entry in commands if isinstance(entry, dict)}
        missing = set(required_commands) - available
        if missing:
            raise ValueError(f"Pi command(s) unavailable: {', '.join(sorted(missing))}; check declared packages; details: {report_path}")
        report["commands"] = sorted(available)
    report["ready"] = True
    write_json(report_path, report)
    return {"ready": True, "provider": provider, "model": model, "mcps": report.get("mcps", []),
            "commands": report.get("commands", []), "report": str(report_path)}
