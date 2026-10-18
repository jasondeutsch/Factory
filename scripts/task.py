#!/usr/bin/env python3
"""Create task artifacts and exchange durable messages; no agent or workflow engine."""

from __future__ import annotations

import argparse
import hashlib
import html
import json
import os
import re
import subprocess
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path
from string import Template

sys.dont_write_bytecode = True
from factorylib import Client, remote_task, ROOT, actor, event, identifier, installed_project, print_json, read_task, write_json

STATES = ("pending", "working", "needs_input", "blocked", "complete", "cancelled")
KINDS = ("assignment", "question", "answer", "progress", "finding", "result", "decision")


def input_text(args: argparse.Namespace, field: str) -> str:
    value = getattr(args, field, None)
    source = getattr(args, field + "_file", None)
    if source:
        value = sys.stdin.read() if source == "-" else Path(source).read_text(encoding="utf-8")
    if not value or not value.strip():
        raise ValueError(f"{field} must not be empty")
    return value.strip()


def new_task(args: argparse.Namespace) -> None:
    installed_project()
    if not re.fullmatch(r"[a-z][a-z0-9_-]*", args.factory):
        raise ValueError("factory must name a recipe in factories/")
    recipe = ROOT / "factories" / (args.factory + ".md")
    if not recipe.is_file():
        raise ValueError(f"factory recipe not found: {args.factory}")
    brief = input_text(args, "brief")
    name = args.name.strip()
    if not name:
        raise ValueError("task name must not be empty")
    owner = "pair_writer" if args.factory == "writing" else "coordinator"
    client = Client()
    record = client.call("create_task", project=str(installed_project()), name=name, brief=brief,
                         owner=owner, request_key=args.request_key or identifier())
    attach_record(record, args.factory)


def attach_record(record: dict, recipe: str) -> None:
    """Bind service identity to client-managed documents; never duplicate status."""
    project = installed_project()
    if record["project"] != str(project):
        raise ValueError("task project must match the selected application root")
    if not re.fullmatch(r"[a-z][a-z0-9_-]*", recipe) or not (ROOT / "factories" / (recipe + ".md")).is_file():
        raise ValueError("factory must name a recipe in factories/")
    expected_owner = "pair_writer" if recipe == "writing" else "coordinator"
    if record["owner"] != expected_owner:
        raise ValueError("task owner does not match the selected factory")
    for pointer in (ROOT / "tasks").glob("*/task.json"):
        metadata = json.loads(pointer.read_text(encoding="utf-8"))
        if metadata.get("comms_id") == record["id"]:
            if metadata.get("factory") != recipe:
                raise ValueError("task already attached to another factory")
            print(pointer.parent)
            return
    name, brief = record["name"], record["brief"]
    slug = re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")[:64].rstrip("-") or "task"
    now = datetime.now(timezone.utc)
    task_id = now.strftime("%Y%m%dT%H%M%S") + f"{now.microsecond // 1000:03d}Z-{slug}"
    path = ROOT / "tasks" / task_id
    path.parent.mkdir(parents=True, exist_ok=True)
    try:
        path.mkdir()
    except FileExistsError:
        path = path.with_name(task_id + "-" + uuid.uuid4().hex[:8])
        path.mkdir()
    write_json(path / "task.json", {
        "schema_version": 2, "id": path.name, "comms_id": record["id"], "project": record["project"],
        "name": name, "factory": recipe, "brief": brief, "owner": record["owner"], "created_at": record["created_at"],
    })
    (path / "reviews").mkdir()
    if recipe == "golang":
        for document in ("spec", "plan"):
            template = Template((ROOT / "templates" / f"{document}.html").read_text(encoding="utf-8"))
            (path / f"{document}.html").write_text(
                template.substitute(title=html.escape(name), brief=html.escape(brief)), encoding="utf-8"
            )
    event(path, "artifacts_attached", record["owner"], artifact_dir=str(path), factory=recipe)
    print(path.relative_to(Path.cwd()) if path.is_relative_to(Path.cwd()) else path)


def attach(args: argparse.Namespace) -> None:
    attach_record(Client().call("get_task", task_id=args.task_id), args.factory)


def status(args: argparse.Namespace) -> None:
    task, metadata = read_task(args.task)
    client = Client()
    record = remote_task(metadata, client)
    if args.state:
        if not args.summary:
            raise ValueError("changing status requires --summary")
        record = client.call("update_task", task_id=record["id"], actor=metadata["owner"],
                             expected_version=record["version"], status=args.state, summary=args.summary)
    elif args.summary:
        raise ValueError("--summary requires --state")
    print_json({"task": record, "artifact_dir": str(task)})


def send(args: argparse.Namespace) -> None:
    task, metadata = read_task(args.task)
    sender, recipient = actor(args.sender), actor(args.recipient)
    client = Client()
    remote_task(metadata, client)
    record = client.call("send_message", task_id=metadata["comms_id"], sender=sender, recipient=recipient,
                         kind=args.kind, text=input_text(args, "text"), request_key=args.request_key or identifier())
    receipt = {"message_id": record["id"], "stored": True, "notification": "not_requested"}
    if args.notify:
        receipt["notification"] = "failed"
        try:
            if os.environ.get("HERDR_ENV") != "1":
                raise ValueError("notifications must be sent from inside a Herdr pane (HERDR_ENV=1)")
            notice = (
                f"Factory message {record['id']} from {sender} to {recipient}. "
                f"Task: {task}. Check your durable inbox and acknowledge after reading."
            )
            result = subprocess.run(
                ["herdr", "agent", "prompt", args.notify, notice],
                capture_output=True, text=True, timeout=20, check=False,
            )
            if result.returncode:
                raise ValueError(result.stderr.strip() or result.stdout.strip() or "Herdr notification failed")
            receipt["notification"] = "submitted"
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            event(task, "notification", sender, message_id=record["id"], target=args.notify, outcome="failed")
            print_json(receipt)
            raise ValueError(f"message is saved as {record['id']}; notification failed: {error}") from error
        event(task, "notification", sender, message_id=record["id"], target=args.notify, outcome="submitted")
    print_json(receipt)


def inbox(args: argparse.Namespace) -> None:
    _, metadata = read_task(args.task)
    client = Client()
    remote_task(metadata, client)
    print_json(list(client.inbox(metadata["comms_id"], actor(args.recipient), args.all)))


def acknowledge(args: argparse.Namespace) -> None:
    _, metadata = read_task(args.task)
    client = Client()
    remote_task(metadata, client)
    print_json(client.call("acknowledge_message", task_id=metadata["comms_id"],
                           message_id=args.message_id, actor=actor(args.by)))


def decide(args: argparse.Namespace) -> None:
    task, _ = read_task(args.task)
    artifacts = []
    for value in args.artifact:
        path = (task / value).resolve()
        if not path.is_relative_to(task) or not path.is_file():
            raise ValueError(f"decision artifact must be a file within the task: {value}")
        artifacts.append({"path": str(path.relative_to(task)), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
    print_json(event(task, "decision", args.by, outcome=args.outcome, text=input_text(args, "text"), artifacts=artifacts))


def note(args: argparse.Namespace) -> None:
    task, _ = read_task(args.task)
    print_json(event(task, "note", args.by, text=input_text(args, "text")))


def text_options(parser: argparse.ArgumentParser, field: str) -> None:
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--" + field)
    group.add_argument("--" + field + "-file", help="UTF-8 file, or - for stdin")


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    new = commands.add_parser("new", help="create a task in this project's .factory/tasks")
    new.add_argument("--factory", required=True)
    new.add_argument("--name", required=True)
    text_options(new, "brief")
    new.add_argument("--request-key", help="stable key to recover the same task after an uncertain response")
    new.set_defaults(run=new_task)
    binding = commands.add_parser("attach", help="create local documents for an existing comms task")
    binding.add_argument("task_id")
    binding.add_argument("--factory", required=True)
    binding.set_defaults(run=attach)
    state = commands.add_parser("status", help="inspect status, or update it as the designated task owner")
    state.add_argument("task")
    state.add_argument("--state", choices=STATES)
    state.add_argument("--summary")
    state.set_defaults(run=status)
    message = commands.add_parser("send", help="save a message, optionally notifying a named Herdr agent")
    message.add_argument("task")
    message.add_argument("--from", dest="sender", required=True)
    message.add_argument("--to", dest="recipient", required=True)
    message.add_argument("--kind", choices=KINDS, required=True)
    message.add_argument("--notify")
    message.add_argument("--request-key", help="reuse the original key when retrying uncertain delivery")
    text_options(message, "text")
    message.set_defaults(run=send)
    receive = commands.add_parser("inbox", help="list unread messages for a role")
    receive.add_argument("task")
    receive.add_argument("--for", dest="recipient", required=True)
    receive.add_argument("--all", action="store_true", help="include acknowledged messages")
    receive.set_defaults(run=inbox)
    ack = commands.add_parser("ack", help="record that the recipient read a message")
    ack.add_argument("task")
    ack.add_argument("message_id")
    ack.add_argument("--by", required=True)
    ack.set_defaults(run=acknowledge)
    decision = commands.add_parser("decide", help="record a decision and optional artifact version hashes")
    decision.add_argument("task")
    decision.add_argument("--by", required=True)
    decision.add_argument("--outcome", choices=("approve", "request_changes", "reject"), required=True)
    decision.add_argument("--artifact", action="append", default=[])
    text_options(decision, "text")
    decision.set_defaults(run=decide)
    annotation = commands.add_parser("note", help="record context, delegation, or live session names")
    annotation.add_argument("task")
    annotation.add_argument("--by", required=True)
    text_options(annotation, "text")
    annotation.set_defaults(run=note)
    return root


def main() -> int:
    try:
        args = parser().parse_args()
        args.run(args)
        return 0
    except (ValueError, OSError) as error:
        print(f"factory: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
