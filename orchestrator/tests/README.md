# Coordination verification

Run infrastructure tests from the toolkit root:

```sh
python3 -B -m unittest discover -s orchestrator/tests -v
(cd orchestrator/comms && go test -race ./...)
```

For a compact recovery report:

```sh
python3 -B orchestrator/tests/eval_recovery.py --output /tmp/factory-recovery
```

The report uses a real disposable comms service and deterministic runner/Herdr fixtures. It starts no real agent, consumes no model tokens, changes no application, and does not touch the normal Compose service. `report.json`, `report.html`, and `tests.txt` distinguish measured checks from unverified live behavior. The Go suite additionally checks delivery leases, database reopen/recovery, participant replacement, binding changes, and contact expiry.

## Live acceptance eval

This is a separate run inside Herdr, with a human present. Do not turn the automated report into a claim that real sessions worked. Use a disposable application project and the Go factory, configured for the intended provider. Ask it to create a minimal Go tic-tac-toe program. Implementation and product verification belong to the assigned coder and reviewer.

1. Start comms, run the factory-wide readiness check, and start the coordinator in Herdr. Retain the actual task ID, owner participant ID, and loaded session snapshots.
2. Use deterministic dispatch for each worker. Confirm the visible pane belongs to the registered participant. Keep human spec/plan decisions explicit; pause for the human unless they delegated them for this run.
3. Start the delivery bridge in a visible pane after binding the coordinator. Verify assignment and result notifications in both directions while recipients are idle. Inspect acknowledgements, acceptance, and results separately.
4. Introduce the following faults one at a time, recording expected and observed outcomes. Only interrupt processes created for this eval.

| Scenario | Expected evidence |
| --- | --- |
| Repeat the same dispatch key | Same participant and assignment; no extra pane or duplicate notification |
| Stop and restart the bridge | Pending messages remain; previously submitted hints are not replayed |
| Restart comms with its existing database | Tasks, assignments, snapshots and delivery claims survive |
| Interrupt a worker | Contact expires or becomes unavailable; assignment does not become completed or silently transfer |
| Explicitly replace that worker | Old assignment is reconciled/cancelled by the owner; new work names the new participant |
| Change an approved document before a coder loop starts | Startup refuses stale approval; human decision is requested again |
| Leave a result unsubmitted | Coordinator reports incomplete handoff; check events alone do not count as a result |
| Give an agent direct feedback | It records consequential feedback and informs the coordinator |

For each scenario retain the task report URL, relevant record IDs, timestamps, runtime versions, actual intervention, recovery duration, and outcome. Preserve failures. Do not repair application code or supply missing agent results yourself. Usage is unknown unless measured records were submitted. End with a report separating automated checks, live observations, limitations, and new issues.
