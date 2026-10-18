#!/usr/bin/env python3
"""Prepare or start one interactive role session in the application project root."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import string
import sys
import uuid
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent / "scripts"))
from factorylib import factory_definition, agent_directory, factory_recipe, ROOT, actor, event, installed_project, print_json, read_task, Client, remote_task
from adapters.pi import profile as pi_profile


def configuration(role: str, factory: str | None = None) -> tuple[Path, dict]:
    actor(role)
    directory = agent_directory(role, factory)
    config = json.loads((directory / "config.json").read_text(encoding="utf-8"))
    if not isinstance(config, dict):
        raise ValueError("agent config.json must be an object")
    overlay = directory / "config.local.json"
    if overlay.exists():
        config.update(pi_profile.object_file(overlay))
    misplaced = [name for name in ("provider", "model", "thinking", "tools") if name in config]
    if misplaced:
        raise ValueError("configure " + ", ".join(misplaced) + " in native Pi settings.json/models.json, not config.json")
    if factory is not None:
        member = next(item for item in factory_definition(factory)["agents"] if item["name"] == role)
        overrides = dict(member.get("config", {}))
        if "skills" in overrides:
            base = ROOT / "factories" / factory
            overrides["skills"] = [str((base / value).resolve()) for value in overrides["skills"]]
        config = pi_profile.merge(config, overrides)
    return directory, config


def text_list(config: dict, key: str) -> list[str]:
    value = config.get(key, [])
    if not isinstance(value, list) or any(not isinstance(item, str) or not item for item in value):
        raise ValueError(f"{key} must be a list of nonempty strings")
    return value


def approved_documents(task: Path) -> None:
    """Require current artifact decisions before starting autonomous iterations.

    Decision records declare authority; this does not authenticate their author.
    The coordinator remains responsible for the user's actual authorization.
    """
    _, metadata = read_task(str(task))
    client = Client()
    remote_task(metadata, client)
    latest = {}
    for record in client.events(metadata["comms_id"]):
        if record["kind"] != "decision":
            continue
        decision = record.get("data") or {}
        outcome = decision.get("outcome")
        if outcome not in ("approve", "request_changes", "reject"):
            continue
        artifacts = decision.get("artifacts", [])
        if not isinstance(artifacts, list):
            raise ValueError("invalid decision artifact evidence in comms")
        if not artifacts and outcome != "approve":
            latest.update({name: (outcome, None) for name in ("spec.html", "plan.html")})
        for artifact in artifacts:
            if not isinstance(artifact, dict) or not isinstance(artifact.get("path"), str):
                raise ValueError("invalid decision artifact evidence in comms")
            latest[artifact["path"]] = (outcome, artifact.get("sha256"))

    for name in ("spec.html", "plan.html"):
        current = hashlib.sha256((task / name).read_bytes()).hexdigest()
        if latest.get(name) != ("approve", current):
            raise ValueError(f"Ralph requires approval of the current {name}; ask the coordinator to record the actual decision")


def prepare(role: str, task: Path | None, metadata: dict | None, runtime: Path, loop_iterations: int | None = None, factory: str | None = None) -> dict:
    project = installed_project()
    if metadata and factory and metadata["factory"] != factory:
        raise ValueError("--factory conflicts with the task's recorded factory")
    selected_factory = metadata["factory"] if metadata else factory
    directory, config = configuration(role, selected_factory)
    instruction = directory / ("SKILL.md" if role == "coordinator" else "agent.md")
    skill_files = []
    for value in text_list(config, "skills"):
        path = (directory / value).resolve()
        if not path.is_relative_to(ROOT):
            raise ValueError(f"skill must be inside the portable toolbox: {value}")
        skill_files.append(path / "SKILL.md" if path.is_dir() else path)
    project_key = hashlib.sha256(str(project).encode()).hexdigest()[:16]
    session = runtime / project_key / (task.name if task else "unassigned") / role / uuid.uuid4().hex
    context = (f"Role: {role}\nProject root: {project}\nToolkit root: {ROOT}\nInstance ID: {session.name}\n"
               f"Role instruction source: {instruction}\n")
    recipe_text = ""
    if task is not None:
        context += (
            f"Task directory: {task}\nTask owner: {metadata['owner']}\nTask name: {metadata['name']}\n"
            f"Comms project: {metadata['project']}\nComms task ID: {metadata['comms_id']}\n"
            f"Task brief (JSON string):\n{json.dumps(metadata['brief'], ensure_ascii=False)}\n"
        )
    else:
        context += (
            f"Comms project: {project}\nNo task is selected. Do not assume a previous task or choose the newest.\n"
            "The coordinator or pair writer can establish a task from the user's request through comms "
            "and attach its local artifacts. Other workers wait for an explicit assignment containing "
            "the task ID, artifact directory, and recipe before beginning work.\n"
        )
    if selected_factory is not None:
        recipe = factory_recipe(selected_factory)
        context += f"Selected factory configuration: {recipe.parent / 'factory.json'}\n"
        recipe_text = (f"\n## Factory configuration\n\nSource: {recipe.parent / 'factory.json'}\n"
                       f"{json.dumps(factory_definition(selected_factory), indent=2)}\n"
                       f"\n## Factory recipe\n\nSource file: {recipe}\n"
                       f"Source directory for relative references: {recipe.parent}\n\n"
                       f"{recipe.read_text(encoding='utf-8')}\n")
    else:
        available = sorted((ROOT / "factories").glob("*/factory.json"))
        context += "Available factory configuration files:\n" + "".join(f"- {path}\n" for path in available)
    instructions = (
        f"# Factory session\n\n{context}\n"
        "Status, messages, decisions, and events are authoritative in comms, not local files.\n\n"
        "This is a fresh interactive session. For an assigned task, read its status and your inbox. "
        "Starting this session does not authorize implementation or approval. "
        "Workers wait for an assignment; the pair writer can collaborate directly with the user. "
        "The coordinator can begin by clarifying scope and authority. Stay available for follow-up.\n\n"
        f"## Role instructions\n\nSource file: {instruction}\n"
        f"Source directory for relative references: {instruction.parent}\n\n"
        "Resolve relative document links from each section's stated source directory. "
        "Resolve factory member paths from their factory.json directory.\n\n"
        f"{instruction.read_text(encoding='utf-8')}\n"
        f"{recipe_text}"
    )
    for path in skill_files:
        instructions += (
            f"\n## Skill: {path.relative_to(ROOT)}\n\n"
            f"Source directory for relative references: {path.parent}\n\n"
            f"{path.read_text(encoding='utf-8')}\n"
        )
    mapping = config.get("env", {})
    if not isinstance(mapping, dict):
        raise ValueError("env must map child environment names to existing environment variable names")
    child_env = os.environ.copy()
    child_env.setdefault("FACTORY_COMMS_URL", "http://127.0.0.1:8090/mcp")
    child_env["FACTORY_PROJECT"] = str(project)
    child_env.pop("FACTORY_TASK_ID", None)
    child_env.pop("FACTORY_TASK_DIR", None)
    if task is not None:
        child_env["FACTORY_TASK_ID"] = metadata["comms_id"]
        child_env["FACTORY_TASK_DIR"] = str(task)
    child_env["FACTORY_ROLE"] = role
    child_env["FACTORY_INSTANCE_ID"] = session.name
    for child_name, source_name in mapping.items():
        if not isinstance(child_name, str) or not isinstance(source_name, str) or not source_name:
            raise ValueError("env names and references must be nonempty strings")
        if source_name not in os.environ:
            raise ValueError(f"environment variable required by {role}: {source_name}")
        if child_name == "PI_CODING_AGENT_DIR" or child_name.startswith("FACTORY_") or child_name == "COMMS_AUTH_TOKEN":
            raise ValueError("session identity and comms variables are managed by the launcher; set the shared comms environment before launch")
        child_env[child_name] = os.environ[source_name]
    instruction_file = session / "instructions.md"
    runner = config.get("runner", "pi")
    profile = None
    if loop_iterations is not None:
        if task is None:
            raise ValueError("--loop requires an explicit task artifact directory and current document approvals")
        if role != "coder_golang" or runner != "pi" or metadata["factory"] != "golang":
            raise ValueError("--loop is available for the native Pi coder_golang assignment in the Go factory")
        if loop_iterations < 1:
            raise ValueError("--loop must specify a positive iteration limit")
        approved_documents(task)
    if runner == "pi":
        profile = pi_profile.load(directory, config, project, session, child_env)
        executable = config.get("executable", "pi")
        if not isinstance(executable, str) or not executable:
            raise ValueError("executable must be a nonempty string")
        extra = text_list(config, "args")
        reserved = {"--provider", "--model", "--thinking", "--tools", "-t", "--no-tools", "-nt",
                    "--no-builtin-tools", "-nbt", "--session", "--session-dir", "--session-id",
                    "--continue", "-c", "--resume", "-r", "--fork", "--print", "-p", "--mode",
                    "--approve", "-a", "--no-approve", "-na", "--api-key"}
        if any(value.split("=", 1)[0] in reserved for value in extra):
            raise ValueError("args must not override native model/tools settings, credentials, project scope, or interactive session ownership")
        command = [executable, "--append-system-prompt", str(instruction_file), "--no-skills", "--no-approve", *extra]
        initial_prompt = (
            "Read your loaded Factory session instructions, then check task status and your inbox." if task else
            "Read your loaded Factory role and skills. No task is selected; follow the startup guidance and stay interactive."
        )
        if loop_iterations is not None:
            # A native Pi command, not shell code or a nested agent process.
            initial_prompt = (
                f"/ralph-loop --max-iterations={loop_iterations} Follow the loaded bounded-iteration skill. Task directory: {task}. "
                f"Project root: {project}. Read your coder role instructions, the canonical assignment "
                "through get_assignment, get_task, read_inbox, list_events approvals, local spec.html and plan.html, and progress.md "
                "at every iteration. Without an active coordinator assignment or unchanged current "
                "approvals, report the blocker and end with <promise>STOP</promise>. "
                "Implement one dependency-ready plan item, verify it, append progress, and notify the "
                "coordinator. Never edit approved documents or commit without authorization. "
                "Use <promise>NEXT</promise> for verified progress with more work available, "
                "<promise>COMPLETE</promise> after publishing the finished assignment for independent "
                "review, or <promise>STOP</promise> for a blocker or decision. "
                "Put the control tag on the last nonempty line."
            )
        command.extend([
            "--session", str(session / "session.jsonl"), "--name", f"{metadata['name'] if metadata else 'Factory'} / {role}",
            initial_prompt,
        ])
    elif runner == "command":
        if any(key in config for key in ("provider", "model", "thinking", "tools")) or config.get("pi"):
            raise ValueError("Pi provider/model/thinking/tools and pi overrides require runner=pi; configure command runners through command argv and env")
        if any(config.get(key) for key in ("config_dir", "auth_file_env", "mcp_auth_file_env")):
            raise ValueError("config_dir/auth_file_env/mcp_auth_file_env are Pi adapter options; remove them for runner=command")
        if config.get("args"):
            raise ValueError("use command argv for runner=command, not Pi args")
        raw = text_list(config, "command")
        if not raw or not any("{instructions_file}" in item for item in raw):
            raise ValueError("command argv must explicitly load {instructions_file}")
        replacements = {
            "instructions_file": str(instruction_file), "project_root": str(project),
            "task_dir": str(task) if task else "", "session_dir": str(session),
        }
        command = []
        for item in raw:
            fields = [field for _, field, _, _ in string.Formatter().parse(item) if field is not None]
            if any(field not in replacements for field in fields):
                raise ValueError("unknown command placeholder; use only the documented path placeholders")
            command.append(item.format_map(replacements))
    else:
        raise ValueError(f"unsupported runner: {runner}; use pi or an explicit command adapter")
    return {
        "role": role, "task": str(task) if task else None, "cwd": str(project), "command": command,
        "skills": [str(path) for path in skill_files],
        "instructions_file": str(instruction_file), "instructions": instructions,
        "environment_references": mapping, "child_env": child_env,
        "pi_profile": profile, "runner_configuration": profile["summary"] if profile else None,
        "check_timeout_seconds": config.get("check_timeout_seconds", 30),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("role")
    parser.add_argument("task", nargs="?", help="explicit local task artifact directory; omit to start an unassigned interactive role")
    parser.add_argument("--factory", help="explicit factory configuration for an unassigned session; must match a supplied task")
    parser.add_argument("--loop", type=int, metavar="ITERATIONS", help="start an approved Go coder assignment in a bounded native Ralph loop")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--dry-run", action="store_true", help="print the launch plan without writes; --loop reads comms approvals")
    mode.add_argument("--check", action="store_true", help="prepare private configuration and check native startup without starting chat")
    args = parser.parse_args()
    try:
        installed_project()
        task, metadata = read_task(args.task) if args.task else (None, None)
        override = os.environ.get("FACTORY_RUNTIME_DIR")
        runtime = Path(override).expanduser() if override else Path.home() / ".factory" / "runtime"
        if not runtime.is_absolute():
            raise ValueError("FACTORY_RUNTIME_DIR must be absolute")
        if not args.dry_run and task is not None:
            record = remote_task(metadata)
            for field in ("name", "brief", "owner"):
                metadata[field] = record[field]
        plan = prepare(args.role, task, metadata, runtime, args.loop, args.factory)
        if args.dry_run:
            print_json({key: value for key, value in plan.items() if key not in ("child_env", "pi_profile")})
            return 0
        if shutil.which(plan["command"][0], path=plan["child_env"].get("PATH")) is None:
            raise ValueError(f"runner executable not found: {plan['command'][0]}")
        instructions = Path(plan["instructions_file"])
        os.umask(0o077)
        instructions.parent.mkdir(parents=True, mode=0o700)
        instructions.write_text(plan["instructions"], encoding="utf-8")
        instructions.chmod(0o600)
        if plan["pi_profile"]:
            timeout = plan["check_timeout_seconds"]
            if not isinstance(timeout, (int, float)) or isinstance(timeout, bool) or not 0 < timeout <= 300:
                raise ValueError("check_timeout_seconds must be greater than zero and at most 300")
            pi_profile.materialize(plan["pi_profile"])
            try:
                print("Factory: checking Pi credentials and MCP connections...", file=sys.stderr)
                required_commands = ("ralph-loop", "ralph-status", "ralph-stop", "ralph-resume") if args.loop is not None else ()
                checked = pi_profile.check(plan["pi_profile"], plan["command"][0], plan["child_env"], Path(plan["cwd"]), timeout,
                                           required_commands=required_commands)
            except (ValueError, OSError) as error:
                try:
                    if task is not None:
                        event(task, "session_startup_failed", args.role, error=str(error), runtime_dir=str(instructions.parent))
                except (ValueError, OSError):
                    print("Factory: could not record startup failure in comms; private diagnostics remain available.", file=sys.stderr)
                raise
            if task is not None:
                event(task, "session_checked", args.role, **checked)
            if args.check:
                print_json(checked)
                return 0
        elif args.check:
            raise ValueError("--check is available for native Pi bundles; command adapters use their runner's checks")
        if task is not None:
            event(task, "session_prepared", args.role, instructions_file=str(instructions), runner=plan["command"][0])
        os.chdir(plan["cwd"])
        os.execvpe(plan["command"][0], plan["command"], plan["child_env"])
        return 0
    except (OSError, ValueError) as error:
        print(f"factory session: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
