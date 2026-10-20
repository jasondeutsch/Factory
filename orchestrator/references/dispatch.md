# Dispatch one assignment

The coordinator uses `orchestrator/dispatch.ts` to open a visible member session through the configured terminal host, discover its actual participant, create an assignment, and check native delivery contact. The coordinator still chooses what happens next. This operation does not implement a workflow, grant approvals, or start a configured loop.

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

From the coordinator's session, substitute actual paths and the active owner participant ID:

```sh
node $FACTORY_HOME/orchestrator/dispatch.ts <task-directory> \
  --request <assignment.json> --key design-1 --owner <owner-participant-id> \
  --direction right
```

Use `down` for a narrow pane. The caller keeps focus. Required provider environment variables must already be available in the new pane's environment. The helper never sources shell startup files or puts credentials in the launch command.

The JSON receipt contains the host, pane (when applicable), actual participant, assignment ID, phase, and any error. `native_delivery_ready` means the adapter reported fresh idle/working contact; it does not mean the message was read or accepted. `manual_delivery` explicitly means this runner still needs an adapter or manual inbox checks. No terminal input is used to deliver Pi messages.

## Recovery

Repeat the same command with the same `--key` and unchanged assignment. Receipts live under `FACTORY_RUNTIME_DIR/dispatch` (default `~/.factory/<project-name>-<path-hash>/runtime/dispatch`), with a process lock per operation. Preserve these receipts until the task is complete. Do not delete one to force a retry or use a new key to bypass an uncertain result.

- **Lost pane-creation response:** inspect Herdr. If the new empty pane exists, retry with `--recover-pane <actual-id>`. This is explicit operator confirmation; the helper does not guess or split again.
- **Lost launch response or startup failure:** the helper looks for the exact instance it launched. It never repeats the launch command. Inspect the visible pane and startup diagnostics. If that instance failed, reconcile it and authorize a fresh dispatch rather than assigning a stopped process.
- **Lost assignment response:** comms replays the same idempotent creation. A retry does not create another assignment.
- **Busy agent:** the assignment remains saved. The Pi extension queues its hint when idle; contact remains separate from reading.
- **Disconnected adapter:** inspect Pi's comms status and service reachability. Retry the same dispatch after recovery; do not duplicate the assignment or inject a competing prompt.
- **Changed/retired session:** stop and reconcile ownership. Dispatch never silently transfers work or overwrites a different binding.

For an already running instance, pass `--reuse-participant <id>`. It must have the requested role and fresh idle native contact. A live terminal name alone is not proof of identity. Reuse does not reset assignment history.

For Terminal/iTerm, opening a window has no returned pane identifier. An uncertain launch is preserved for inspection and never automatically repeated. Other hosts can use the [session launcher](../README.md) and comms tools manually. Automated fixtures exercise retry boundaries; [live terminal acceptance](../tests/README.md) remains separate.

## Handoff evidence

Each factory member may declare `handoff` defaults next to its name/path/config. Dispatch copies these into the assignment unless its request explicitly supplies its own `handoff`. Changes after assignment creation do not weaken existing requirements. Supported fields are `require_artifacts`, `require_checks`, `require_limitations`, `artifacts` (exact required references), and `checks` (required `check.name` values). Direct MCP callers supply the same object to `create_assignment`.

Submission checks the evidence shape before writing a result or notifying the owner. An empty limitations array explicitly means none known. Failed results can omit otherwise-required evidence but must explain the limitation; they are not successful gate completion. Check exit statuses are recorded honestly, including failures. Review judgment and human approvals remain separate.
