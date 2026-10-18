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

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "comms"))
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
    if data.get("schema_version") != 2 or data.get("id") != path.name or not data.get("comms_id") or data.get("project") != str(installed_project()):
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
