#!/usr/bin/env python3
"""Import measured Pi assistant usage for one unambiguous assignment; no model calls."""
import argparse
import hashlib
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "scripts"))
from factorylib import Client, read_task, remote_task


def measurements(path):
    """Pi 1.0.x session v3: cache counts are additional input; reasoning is already output."""
    with path.open() as source:
        header = json.loads(next(source))
        if header.get("type") != "session" or header.get("version") != 3 or not header.get("id"):
            raise ValueError("expected a Pi version-3 session header")
        seen = set()
        for number, line in enumerate(source, 2):
            if not line.endswith("\n"):
                raise ValueError("session ends in a partial record; retry after the writer flushes")
            row = json.loads(line)
            message = row.get("message", {})
            if row.get("type") != "message" or message.get("role") != "assistant":
                continue
            entry = row.get("id")
            if not entry or entry in seen:
                raise ValueError(f"missing or duplicate entry identity on line {number}")
            seen.add(entry)
            raw = message.get("usage")
            if not raw:
                yield entry, None
                continue
            if message.get("stopReason") == "pending":
                raise ValueError("assistant measurement is still pending")
            counts = [raw.get(key) for key in ("input", "output", "cacheRead", "cacheWrite")]
            if any(type(value) is not int or value < 0 for value in counts):
                raise ValueError(f"incomplete or invalid token measurement on line {number}")
            if not message.get("provider") or not message.get("model"):
                raise ValueError(f"missing model attribution on line {number}")
            if raw.get("totalTokens") is not None and raw["totalTokens"] != sum(counts):
                raise ValueError(f"token totals disagree on line {number}; inspect the runtime format")
            usage = {"provider":message["provider"], "model":message.get("responseModel") or message["model"], "source":"runtime",
                     "input_tokens":counts[0]+counts[2]+counts[3], "output_tokens":counts[1],
                     "cache_read_tokens":counts[2], "cache_write_tokens":counts[3]}
            # Pi cost is locally estimated from configured prices, not billing.
            yield header["id"] + ":" + entry, usage


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("task")
    parser.add_argument("--session", type=Path, required=True)
    parser.add_argument("--participant", required=True)
    parser.add_argument("--assignment", required=True)
    args = parser.parse_args()
    _, metadata = read_task(args.task)
    client = Client()
    remote_task(metadata, client)
    assignment = client.call("get_assignment", task_id=metadata["comms_id"], assignment_id=args.assignment)
    if assignment["assignee_id"] != args.participant:
        raise ValueError("assignment does not belong to the supplied participant")
    assignments = client.call("list_assignments", task_id=metadata["comms_id"], assignee_id=args.participant, limit=2)["assignments"]
    if len(assignments) != 1:
        raise ValueError("session attribution is ambiguous across assignments; use explicitly attributed runtime records instead")
    participants = []
    after = ""
    while True:
        page = client.call("list_participants", task_id=metadata["comms_id"], after=after, limit=200)["participants"]
        participants.extend(page)
        if len(page)<200: break
        after=page[-1]["id"]
    participant = next((p for p in participants if p["id"]==args.participant),None)
    if participant is None or args.session.resolve().parent.name != participant["instance_id"]:
        raise ValueError("session file must be from this participant's launcher runtime directory")
    with args.session.open() as source:
        if json.loads(next(source)).get("cwd") != metadata["project"]:
            raise ValueError("session project differs from the task project")
    rows = list(measurements(args.session))  # Validate the complete input before writing any usage.
    imported=0
    for identity,usage in rows:
        if usage is None: continue
        key=hashlib.sha256((args.participant+":"+identity).encode()).hexdigest()
        client.call("record_usage", task_id=metadata["comms_id"], actor=participant["routing_identity"], stage=assignment["stage"],
                    attempt=assignment["attempt"], request_key="pi-"+key, usage=usage)
        imported+=1
    print(json.dumps({"measurements_submitted_or_replayed":imported,"assistant_records_without_usage":len(rows)-imported,
                      "coverage":"assistant messages only; tool and compaction usage not imported; cost unknown"}))

if __name__ == "__main__":
    try: main()
    except (ValueError,OSError,StopIteration) as error:
        print(f"usage import: {error}",file=sys.stderr);sys.exit(1)
