#!/usr/bin/env python3
"""Dispatch one explicit assignment to a visible Herdr session; no scheduling."""
from __future__ import annotations

import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import time
import uuid

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent / "scripts"))
from factorylib import ROOT, Client, factory_definition, read_task, remote_task, write_json, print_json


def host(*args):
    result = subprocess.run(["herdr", *args], capture_output=True, text=True, timeout=10)
    if result.returncode:
        raise ValueError(f"Herdr {args[0]} {args[1]} failed; inspect the visible pane before retrying")
    return json.loads(result.stdout)["result"]


def participants(client, task_id):
    after = ""
    while True:
        page = client.call("list_participants", task_id=task_id, after=after, limit=200)["participants"]
        yield from page
        if len(page) < 200:
            return
        after = page[-1]["id"]


def bridge(*args):
    result = subprocess.run([str(ROOT / "orchestrator/start-delivery.sh"), *args], capture_output=True, text=True, timeout=60)
    if result.returncode:
        raise ValueError("Delivery operation failed; inspect list_deliveries and the live pane. Do not recreate the assignment")
    return result.stdout


def dispatch(args):
    if os.environ.get("HERDR_ENV") != "1":
        raise ValueError("dispatch must run inside a Herdr pane; do not override HERDR_ENV")
    task, metadata = read_task(args.task)
    request = json.loads(Path(args.request).read_text())
    fields = {"role", "title", "instructions", "inputs", "expected_output", "stage", "attempt"}
    if not isinstance(request, dict) or set(request) - fields or not fields - {"inputs"} <= request.keys():
        raise ValueError("request requires role, title, instructions, expected_output, stage, attempt; inputs is optional")
    if any(not isinstance(request[key], str) or not request[key].strip() for key in ("role", "title", "instructions", "stage")):
        raise ValueError("role, title, instructions and stage must be nonempty strings")
    if type(request["attempt"]) is not int or request["attempt"] < 1:
        raise ValueError("attempt must be a positive integer")
    for field in ("inputs", "expected_output"):
        value = request.get(field, [])
        if not isinstance(value, list) or any(not isinstance(item,str) or not item.strip() for item in value) or (field == "expected_output" and not value):
            raise ValueError(f"{field} must be a list of nonempty strings")
    members = factory_definition(metadata["factory"])["agents"]
    if request["role"] not in [member["name"] for member in members]:
        raise ValueError("role is not a member of this task's factory")
    client = Client()
    record = remote_task(metadata, client)
    if record["status"] in ("cancelled", "complete"):
        raise ValueError("cannot dispatch a terminal task")
    known = list(participants(client, record["id"]))
    owner = next((p for p in known if p["id"] == args.owner and p["routing_identity"] == record["owner"] and not p.get("retired_at")), None)
    if owner is None:
        raise ValueError("--owner must be this task's active owner participant ID")
    if bool(args.reuse_participant) != bool(args.agent):
        raise ValueError("reuse requires both --reuse-participant and --agent")
    key = hashlib.sha256((record["id"] + "\0" + args.key).encode()).hexdigest()
    runtime = Path(os.environ.get("FACTORY_RUNTIME_DIR", str(Path.home() / ".factory/runtime"))).expanduser()
    if not runtime.is_absolute():
        raise ValueError("FACTORY_RUNTIME_DIR must be absolute")
    directory = runtime / "dispatch"
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    path = directory / (key + ".json")
    signature = {"task": record["id"], "owner": args.owner, "request": request,
                 "reuse_participant": args.reuse_participant, "agent": args.agent, "direction": args.direction}
    fingerprint = hashlib.sha256(json.dumps(signature, sort_keys=True).encode()).hexdigest()
    with (directory / (key + ".lock")).open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        receipt = json.loads(path.read_text()) if path.exists() else {
            "fingerprint": fingerprint, "task_id": record["id"], "instance_id": uuid.uuid4().hex,
            "name": args.agent or "factory-" + key[:16], "phase": "prepared", "receipt": str(path),
        }
        if receipt["fingerprint"] != fingerprint:
            raise ValueError("dispatch key already used with different arguments")
        def save(phase):
            receipt["phase"] = phase
            write_json(path, receipt)
        try:
            save(receipt["phase"])
            if args.reuse_participant:
                participant = next((p for p in known if p["id"] == args.reuse_participant and not p.get("retired_at") and p["role"] == request["role"]), None)
                if participant is None:
                    raise ValueError("reuse participant must be active and have the requested role")
                captured = json.loads(bridge("--bind", args.agent))
                # Reuse is only safe once the existing instance has been bound;
                # a name alone cannot prove which process owns its assignments.
                if not participant.get("session_ref") or json.loads(participant["session_ref"]) != captured:
                    raise ValueError("reuse requires the participant's existing binding to match the live agent")
            else:
                if receipt["phase"] == "prepared":
                    save("split_pending")
                    pane = host("pane", "split", "--current", "--direction", args.direction, "--cwd", record["project"], "--no-focus")["pane"]
                    receipt["pane_id"] = pane["pane_id"]
                    save("pane_created")
                if receipt["phase"] == "split_pending":
                    if not args.recover_pane:
                        raise ValueError("pane creation outcome is uncertain; inspect Herdr then supply --recover-pane with the actual new empty pane")
                    pane = host("pane", "get", args.recover_pane)["pane"]
                    receipt["pane_id"] = pane["pane_id"]
                    save("pane_created")
                if receipt["phase"] == "pane_created":
                    argv = [sys.executable, str(ROOT / "orchestrator/session.py"), request["role"], str(task),
                            "--instance-id", receipt["instance_id"], "--routing-identity", receipt["name"]]
                    save("launch_pending")
                    host("pane", "run", receipt["pane_id"], shlex.join(argv))
                    save("launch_submitted")
                deadline = time.monotonic() + args.timeout
                participant = None
                while time.monotonic() < deadline:
                    participant = next((p for p in participants(client, record["id"]) if p["instance_id"] == receipt["instance_id"] and not p.get("retired_at")), None)
                    if participant:
                        break
                    if client.call("get_task", task_id=record["id"])["status"] in ("cancelled", "complete"):
                        raise ValueError("task ended while waiting for startup")
                    time.sleep(0.25)
                if participant is None:
                    raise ValueError("no actual participant registered before timeout; inspect startup in the recorded pane; retry never resends the launch command")
                receipt["participant_id"] = participant["id"]
                # Herdr may detect the runner after registration; retrying this
                # rename after inspection is safe and never starts another agent.
                host("agent", "rename", receipt["pane_id"], receipt["name"])
                captured = json.loads(bridge("--bind", receipt["name"]))
                previous_binding = receipt.get("binding")
                if previous_binding and previous_binding != captured:
                    raise ValueError("live binding changed since dispatch; inspect the instance, do not rebind automatically")
            receipt["participant_id"] = participant["id"]
            binding = json.dumps(captured, sort_keys=True)
            if participant.get("session_ref", "") and json.loads(participant["session_ref"]) != captured:
                raise ValueError("participant already has a different host binding")
            if not participant.get("session_ref"):
                client.call("bind_participant", task_id=record["id"], participant_id=participant["id"],
                            actor_participant_id=args.owner, expected_session_ref="", session_ref=binding, request_key="dispatch-bind-" + key)
            receipt["binding"] = captured
            save("bound")
            assignment = client.call("create_assignment", task_id=record["id"], assigning_participant_id=args.owner,
                                     assignee_id=participant["id"], request_key="dispatch-" + key,
                                     **{k:v for k,v in request.items() if k != "role"})
            receipt["assignment_id"] = assignment["id"]
            save("assigned")
            bridge("--project", record["project"], "--task", record["id"], "--notify", participant["routing_identity"], "--agent", receipt["name"])
            receipt.pop("error", None)
            save("notification_checked")
            print_json(receipt)
            return 0
        except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
            receipt["error"] = str(error)
            write_json(path, receipt)
            print_json(receipt)
            return 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("task")
    parser.add_argument("--request", required=True, help="JSON assignment file")
    parser.add_argument("--key", required=True, help="stable dispatch retry key")
    parser.add_argument("--owner", required=True, help="actual owner participant ID")
    parser.add_argument("--reuse-participant")
    parser.add_argument("--agent", help="actual bound agent name when reusing")
    parser.add_argument("--direction", choices=("right", "down"), default="right")
    parser.add_argument("--recover-pane", help="inspected new empty pane after an uncertain split; never resends a launch")
    parser.add_argument("--timeout", type=float, default=60)
    args = parser.parse_args()
    if not args.key.strip():
        parser.error("key must not be empty")
    if not 0 < args.timeout <= 300:
        parser.error("timeout must be greater than zero and at most 300")
    os.umask(0o077)
    try:
        return dispatch(args)
    except (ValueError, OSError, KeyError, subprocess.SubprocessError) as error:
        print(f"factory dispatch: {error}", file=sys.stderr)
        return 1

if __name__ == "__main__":
    sys.exit(main())
