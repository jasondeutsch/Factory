#!/usr/bin/env python3
"""Deterministic host boundary for dispatch tests; never touches real Herdr."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

root = Path(os.environ["HOST_FIXTURE"])
args = sys.argv[1:]
with (root / "calls.jsonl").open("a") as log:
    log.write(json.dumps(args) + "\n")
state_file = root / "host.json"
state = json.loads(state_file.read_text()) if state_file.exists() else {
    "workspace_id": "fixture-workspace", "pane_id": "fixture-pane", "terminal_id": "fixture-terminal",
    "name": "unnamed", "agent": "pi", "agent_status": "idle", "interactive_ready": True,
}
if args[:2] == ["pane", "split"]:
    if os.environ.get("UNCERTAIN_SPLIT"):
        sys.exit(1)
    result = {"pane": {"pane_id": state["pane_id"]}}
elif args[:2] == ["pane", "get"]:
    result = {"pane": {"pane_id": state["pane_id"]}}
elif args[:2] == ["pane", "run"]:
    with (root / "launch.log").open("w") as log:
        launched = subprocess.run(shlex.split(args[3]), stdout=log, stderr=log, timeout=10)
    if launched.returncode or os.environ.get("UNCERTAIN_LAUNCH"):
        sys.exit(1)
    result = {"type": "pane_run"}
elif args[:2] == ["agent", "rename"]:
    state["name"] = args[3]
    state_file.write_text(json.dumps(state))
    result = {"type": "agent_info", "agent": state}
elif args[:2] == ["agent", "get"]:
    result = {"type": "agent_info", "agent": state}
elif args[:2] == ["agent", "prompt"]:
    if os.environ.get("UNCERTAIN_PROMPT"):
        sys.exit(1)
    result = {"type": "agent_prompted", "agent": state}
else:
    sys.exit(2)
print(json.dumps({"result": result}))
