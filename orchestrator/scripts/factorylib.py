"""Artifact references and shared comms operations for task/session helpers."""

from __future__ import annotations

import json
import os
import re
import tempfile
import uuid
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "orchestrator" / "comms"))
from client import Client


def identifier() -> str:
    return datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ") + "-" + uuid.uuid4().hex[:8]


def actor(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,63}", value):
        raise ValueError("actor must be 1–64 letters, digits, underscores, dots, or hyphens")
    return value


def installed_project() -> Path:
    if ROOT.name != ".factory":
        raise ValueError("copy this toolbox into the project's .factory directory before creating or running tasks")
    return ROOT.parent


def write_json(path: Path, data: dict) -> None:
    """Publish a complete local artifact reference or private configuration atomically."""
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".writing-", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(data, stream, ensure_ascii=False, indent=2)
            stream.write("\n")
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def read_task(value: str) -> tuple[Path, dict]:
    installed_project()
    path = Path(value).expanduser().resolve()
    if path.parent != ROOT / "tasks":
        raise ValueError("task must be a direct child of this project's .factory/tasks")
    data = json.loads((path / "task.json").read_text(encoding="utf-8"))
    if not isinstance(data, dict) or any(not isinstance(data.get(key), str) or not data[key]
                                         for key in ("id", "comms_id", "project", "name", "factory", "owner")):
        raise ValueError("task.json must be an object with nonempty id, comms_id, project, name, factory, and owner strings")
    if not isinstance(data.get("brief"), str):
        raise ValueError("task.json brief must be a string")
    if data.get("schema_version") != 2 or data.get("id") != path.name or data.get("project") != str(installed_project()):
        raise ValueError("invalid task.json identity or schema_version")
    return path, data


def remote_task(metadata: dict, client: Client | None = None) -> dict:
    record = (client or Client()).call("get_task", task_id=metadata["comms_id"])
    if record["project"] != metadata["project"] or record["owner"] != metadata["owner"]:
        raise ValueError("comms task identity does not match the local artifact reference")
    return record


def event(task: Path, kind: str, by: str, **details: object) -> dict:
    _, metadata = read_task(str(task))
    client = Client()
    remote_task(metadata, client)
    public_kind = kind if kind in ("decision", "note", "check", "finding", "failure", "progress") else (
        "failure" if kind == "session_startup_failed" else "note")
    return client.call("record_event", task_id=metadata["comms_id"], actor=actor(by),
                       kind=public_kind, message=str(details.get("text", kind)),
                       data={"event_type": kind, **details}, request_key=identifier())


def print_json(data: object) -> None:
    print(json.dumps(data, ensure_ascii=False, indent=2))


def factory_definition(name: str) -> dict:
    """Read explicit factory membership and validate references before use."""
    if not isinstance(name, str) or not re.fullmatch(r"[a-z][a-z0-9_-]*", name):
        raise ValueError("factory must name factories/<name>/factory.json")
    directory = ROOT / "factories" / name
    path = directory / "factory.json"
    if not path.is_file():
        raise ValueError(f"factory configuration not found: {name}")
    definition = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(definition, dict) or definition.get("name") != name:
        raise ValueError(f"{path}: name must match its factory directory")
    members = definition.get("agents")
    if not isinstance(members, list) or not members:
        raise ValueError(f"{path}: agents must be a nonempty list of names and paths")
    names = set()
    for member in members:
        if not isinstance(member, dict) or not isinstance(member.get("name"), str):
            raise ValueError(f"{path}: each agent needs a name and path")
        role = member["name"]
        actor(role)
        if role in names:
            raise ValueError(f"{path}: duplicate agent name: {role}")
        names.add(role)
        handoff = member.get("handoff", {})
        if not isinstance(handoff, dict) or set(handoff) - {"require_artifacts", "require_checks", "require_limitations", "artifacts", "checks"}:
            raise ValueError(f"{path}: agent {role} has invalid handoff requirements")
        for key in ("require_artifacts", "require_checks", "require_limitations"):
            if key in handoff and type(handoff[key]) is not bool:
                raise ValueError(f"{path}: {key} must be boolean")
        for key in ("artifacts", "checks"):
            if key in handoff and (not isinstance(handoff[key], list) or any(not isinstance(v, str) or not v.strip() for v in handoff[key])):
                raise ValueError(f"{path}: {key} must list nonempty references")
        overrides = member.get("config", {})
        if not isinstance(overrides, dict):
            raise ValueError(f"{path}: agent {role} config must be an object")
        supported = {"provider", "model", "thinking", "tools", "pi", "runner", "config_dir", "skills", "args", "env", "executable", "command", "auth_file_env", "mcp_auth_file_env", "check_timeout_seconds"}
        unknown = set(overrides) - supported
        if unknown:
            raise ValueError(f"{path}: unsupported agent config fields: {', '.join(sorted(unknown))}")
        for field in ("provider", "model", "thinking"):
            if field in overrides and (not isinstance(overrides[field], str) or not overrides[field].strip()):
                raise ValueError(f"{path}: agent {role} config.{field} must be a nonempty string")
        for field in ("skills", "tools"):
            if field in overrides and (not isinstance(overrides[field], list) or any(not isinstance(item, str) or not item for item in overrides[field])):
                raise ValueError(f"{path}: agent {role} config.{field} must be a list of nonempty strings")
        native = overrides.get("pi", {})
        if not isinstance(native, dict) or set(native) - {"settings", "models", "mcp"} or any(not isinstance(value, dict) for value in native.values()):
            raise ValueError(f"{path}: agent {role} config.pi accepts settings, models, and mcp objects")
        reference = member.get("path")
        if not isinstance(reference, str) or not reference.strip() or Path(reference).is_absolute():
            raise ValueError(f"{path}: agent {role} needs a relative bundle path")
        bundle = (directory / reference).resolve()
        if not bundle.is_relative_to(ROOT.resolve()):
            raise ValueError(f"{path}: agent {role} path must stay inside the toolkit")
        instruction = "SKILL.md" if role == "coordinator" else "agent.md"
        if not (bundle / "config.json").is_file() or not (bundle / instruction).is_file():
            raise ValueError(f"{path}: agent {role} bundle is missing config.json or {instruction}: {reference}")
    if not isinstance(definition.get("owner"), str) or definition["owner"] not in names:
        raise ValueError(f"{path}: owner must name a member agent")
    if not (directory / "README.md").is_file():
        raise ValueError(f"factory recipe not found: {name}")
    return definition


def agent_directory(role: str, factory: str | None = None) -> Path:
    """Use declared paths, never infer membership from directory placement."""
    actor(role)
    if factory is None and role == "coordinator":
        return ROOT / "orchestrator"  # Bootstrap before the user selects a factory.
    configurations = [ROOT / "factories" / factory / "factory.json"] if factory else sorted((ROOT / "factories").glob("*/factory.json"))
    matches = set()
    for path in configurations:
        definition = factory_definition(path.parent.name)
        for member in definition["agents"]:
            if member["name"] == role:
                matches.add((path.parent / member["path"]).resolve())
    if len(matches) != 1:
        scope = f" in factory {factory}" if factory else " across factory configurations; supply a task to select its factory"
        raise ValueError(f"expected one declared agent bundle for {role}{scope}, found {len(matches)}")
    return matches.pop()


def factory_recipe(name: str) -> Path:
    factory_definition(name)
    return ROOT / "factories" / name / "README.md"
