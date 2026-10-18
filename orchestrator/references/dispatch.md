# Dispatch one assignment

The coordinator uses `orchestrator/dispatch.py` inside Herdr to open a visible member session, discover its actual participant, bind it, create an assignment, and check notification delivery. The coordinator still chooses what happens next. This operation does not implement a workflow, grant approvals, or start a Ralph loop.

Write the intended assignment as JSON in the task's documents, for example:

```json
{
  "role": "architect",
  "title": "Draft the proposed change",
  "instructions": "Prepare spec.html and plan.html for review. Do not implement.",
  "inputs": ["task brief"],
  "expected_output": ["spec.html", "plan.html"],
  "stage": "design",
  "attempt": 1
}
```

From the coordinator's pane, substitute actual paths and the active owner participant ID:

```sh
python3 .factory/orchestrator/dispatch.py <task-directory> \
  --request <assignment.json> --key design-1 --owner <owner-participant-id> \
  --direction right
```

Use `down` for a narrow pane. The caller keeps focus. Required provider environment variables must already be available in the new pane's environment. The helper never sources shell startup files or puts credentials in the launch command.

The JSON receipt contains the pane, actual participant, assignment ID, phase, and any error. `notification_checked` confirms a delivery check, not reading, acceptance, or completion. Inspect canonical assignment state through comms. Supply the coordinator's known live name in assignment context or bind the coordinator and run the background bridge for result notifications.

## Recovery

Repeat the same command with the same `--key` and unchanged assignment. Receipts live under `FACTORY_RUNTIME_DIR/dispatch` (default `~/.factory/runtime/dispatch`), with a process lock per operation. Preserve these receipts until the task is complete. Do not delete one to force a retry or use a new key to bypass an uncertain result.

- **Lost pane-creation response:** inspect Herdr. If the new empty pane exists, retry with `--recover-pane <actual-id>`. This is explicit operator confirmation; the helper does not guess or split again.
- **Lost launch response or startup failure:** the helper looks for the exact instance it launched. It never repeats the launch command. Inspect the visible pane and startup diagnostics. If that instance failed, reconcile it and authorize a fresh dispatch rather than assigning a stopped process.
- **Lost assignment response:** comms replays the same idempotent creation. A retry does not create another assignment.
- **Busy agent:** the assignment remains saved. Let the bound bridge deliver when idle, or retry the same dispatch after the agent settles.
- **Uncertain notification:** inspect `list_deliveries` and the actual pane before the owner uses `retry_delivery`. Repeating dispatch alone cannot clear uncertainty.
- **Changed/retired session:** stop and reconcile ownership. Dispatch never silently transfers work or overwrites a different binding.

For an already running instance, pass both `--reuse-participant <id>` and `--agent <actual-name>`. Its recorded binding must already match the live session. Bind it explicitly with `bind_participant` first if needed. A live name alone is not proof of identity.

For manual or unsupported hosts, use the [session launcher](../README.md) and comms assignment tools directly. The service itself cannot operate host panes. Deterministic fixtures cover retries and failure boundaries; real Herdr acceptance remains a separate [live eval](../tests/README.md).
