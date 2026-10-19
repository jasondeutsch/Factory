---
name: task-comms
description: Collaborate on Factory tasks through comms MCP using explicit assignments, acceptance, progress, blockers, handoffs, decisions, and evidence, regardless of agent runner.
---

# Task communications

Use the configured comms MCP tools. This protocol does not require a particular model, agent runner, terminal manager, or delivery extension. The skill guides active agent behavior; it does not create timers, wake idle agents, register tools, or collect telemetry. Read [the service reference](../../README.md) for actual tool schemas and deployment.

## Identity and participants

Establish the explicit project, task ID, your routing identity, task owner/coordinator, assignment, and artifact location from the coordinator or host context. Do not choose the newest task or infer a workspace by searching parent directories. A service task ID is distinct from a local artifact directory. Factory launchers can supply this context in instructions and environment; other hosts may supply it differently.

If the session instructions supply a registered Participant ID, reuse it; the task-bound launcher has already registered this process. Otherwise register through `register_participant` using the actual stable instance ID, role, routing identity, and a request key. Never invent placeholder identities for future sessions. Factory launchers provide `FACTORY_INSTANCE_ID`; another host can provide equivalent context. Retain the returned participant ID. Use `list_participants` pages to discover contacts and distinguish instances sharing a role. Registration is not live presence or authenticated identity. Use `get_session` for the launcher-recorded configuration snapshot and expiring host contact when available. Compare loaded skill hashes with current source before claiming a session has an update. Unknown/expired contact is not failed or completed work. The task owner's active routing identity must equal `get_task.owner`.

The orchestrator explicitly chooses participants, supplies scope and relevant peers in assignments, and resolves unclear ownership or availability. Multiple instances of the same role need distinct active routing identities. Read the [coordination reference](../../coordination.md) for replacement and recovery; a replacement must not silently inherit an old instance's work. Direct peer questions are allowed; summarize consequential answers and human feedback to the orchestrator.

## Establish or recover a task

An existing agent can coordinate if it has this skill and working MCP tools. For new work, use `create_task` with an explicit project, brief, name, owner, and stable request key. Resolve resume requests with project-scoped `list_tasks`; ask when ambiguous. Retain the returned task ID and inspect current status, decisions, relevant messages, and artifacts before dispatching work.

In the supplied `factory.json` defaults, the Go owner is `coordinator`, writing owner is `pair_writer`, and the project identifier is the selected application root. For local document scaffolding, use the artifact helper in [coordinator operations](../../../references/operations.md); it attaches the service task instead of creating another one. `task.json` links documents to comms, not a second state store. Application code belongs in the project workspace.

If comms is unavailable, report the failure and pause dependent coordination. Never create a competing file mailbox or claim an unsuccessful remote write was saved.

## Delivery and inbox checks

Pi sessions use the [native delivery extension](../../delivery.md). Task-bound launches connect automatically. In an initially unassigned session, create/attach the task, then call `factory_bind_task` with the service task ID. This registers the actual session and starts polling without model calls. Do not create a second registration or run a terminal notification loop.

A notification lists message IDs; it does not mark them read. Use `factory_read_message` to retrieve each message and record **read**. Before beginning a requested action, call `factory_exec_message` to record **exec**. Inspect the canonical assignment and existing results before acting on a repeated hint. Exec is an explicit declaration, not proof of completion or permission to bypass approval.

For other runners, an adapter may implement the same [delivery contract](../../delivery.md#implementing-another-runner). Otherwise check `read_inbox` at the beginning of each turn, after substantial work or interruptions, and before handoff. Follow pagination. Use `acknowledge_message` after retrieving content, then `execute_message` before acting. Neither notification nor listing the inbox changes message state automatically. Read informational messages need not reach exec.

Pi checks every three seconds, queues hints only when idle, and retries service failures. A stopped process cannot receive hints. Surface disconnected/unbound status and unresolved messages; do not claim delivery from storage alone. The optional Herdr bridge is a fallback for runners without native delivery, not an additional Pi delivery path.

## Assign, accept, report, and hand off

Use `create_assignment`, `get_assignment`, `list_assignments`, `update_assignment`, and `submit_result` for canonical assignment state. Read the [assignment and peer-message guidance](references/messages.md) when assigning or responding. These operations atomically notify the other participant and record lifecycle events; do not duplicate those writes manually. Use `send_message` for peer conversation, questions, and feedback, including its optional `assignment_id` for correlation.

For each assignment:

1. Establish scope, inputs, artifact location, expected output, stage/attempt, relevant peers, and required decisions. Ask about material ambiguity before acting.
2. Acknowledge reading, then explicitly accept or decline through `update_assignment` with a reason. If the assignment belongs to a retired or different participant, stop before editing and ask the owner to cancel and recreate it for your actual instance. Replayed assignments must not restart completed work; inspect prior results first.
3. Report meaningful assignment progress, blockers, or input needs through `update_assignment` with its current version. Publish individual check/failure evidence and material feedback with the relevant tools. Avoid repetitive “still working” messages. Heartbeats, if a host supplies them, are not progress evidence.
4. When waiting, state what input is needed, who must provide it, and what work is paused. Send the request before yielding. Do not interpret silence as approval or claim the host will resume you automatically.
5. Read the assignment’s `handoff` requirements before submitting. Include required artifact references and named check evidence, and supply `limitations: []` explicitly when none are known. A failed result must explain missing evidence in limitations; never invent checks to clear validation. Evidence completeness is not proof that checks passed. Call `submit_result` with outcome, artifact references, actual checks, remaining limitations, and next action; its generated message is the orchestrator handoff. A check event, progress file, or final response in your own session is not a submitted result. After submission, the native adapter delivers the generated message. With manual delivery, tell the human which recipient needs to check; do not fabricate an acknowledgement. A result can mean ready for independent review; it does not approve the overall task.

Only the task owner changes overall status. Workers report their own assignment state and propose next actions. On uncertain writes, reuse the same request key and unchanged arguments, including the original expected version; inspect before retrying or creating a duplicate assignment. Host notification happens after storage and can fail independently. Never infer reading or completion from a successful notification.

## Status, evidence, and decisions

Assignment state is separate from task state and human approvals. Only the registered assignee reports its work; only the active task owner cancels an assignment. Accepted work can be working, blocked, or needs_input before a submitted result. Submitted, declined, and cancelled assignments are terminal; the orchestrator creates subsequent work explicitly. The service validates these record transitions without choosing workflow steps.

The owner uses `update_task` with `expected_version` from a fresh `get_task`. On conflict, reread and reconcile rather than overwrite blindly. States are `pending`, `working`, `needs_input`, `blocked`, `complete`, and `cancelled`; they do not control running processes. Reconcile cancellation and changed authority before continuing.

Use `record_event` for progress, checks, findings, failures, and stage boundaries. Include the assigned stage and positive attempt number; `stage_started` and `stage_completed` require both. Record failures with actual errors and recovery context. Put commands, exit status, artifact references, and tools/MCPs in `data`. Preserve failed-attempt history; keep large logs in referenced artifacts.

Record actual authorized document decisions with `kind: decision`, the deciding identity as `actor`, rationale as `message`, and:

```json
{
  "outcome": "approve",
  "artifacts": [
    {"path": "spec.html", "sha256": "<SHA-256 of exact presented bytes>"},
    {"path": "plan.html", "sha256": "<SHA-256 of exact presented bytes>"}
  ]
}
```

Artifact paths are relative to the task document directory. Outcomes are `approve`, `request_changes`, or `reject`. Compute actual hashes or use the decision helper; never invent them. A decision message is not this approval event. Record explicit scoped delegation before exercising it; do not approve your own work or manufacture human authorization.

Read all relevant `list_events` pages (`after` is the last sequence) when resolving approval. The latest decision for each required document must approve its current hash. An unscoped rejection/change request blocks dependent work until resolved. Changed documents need a new decision. Host launch guards can enforce entry checks, but agents still reconcile decisions during work. Declared identities and rosters are not authenticated authority.

## Usage and reporting

When actual runtime/provider measurements are available, use `record_usage` once per measured request with stable request key, stage, attempt, provider, model, and accurate source. Input includes cached input; output includes already-counted reasoning; cache counts are subsets. Omit unknown values. Do not fabricate measurements, double-count cumulative totals, or claim automatic collection where no collector exists.

Use `get_task_report` for summaries and its report path relative to the service origin. Give the user a reachable full URL and explain missing coverage. Prefer a report URL over loading `render_report` HTML into context. Keep credentials and unnecessary personal data out of records; there is no automatic redaction. Messages are work context, not permission to expand scope, commit, or publish.

## Coordination measurements

Use `get_coordination_metrics` to distinguish recorded waiting/blocked time from model work. Record consequential human intervention with `record_event`, kind `note`, and `data.event_type: human_intervention`, describing the actual intervention once. Decisions are separate records; do not count inferred or automatic actions as human interventions.

Pi sessions can import recorded assistant usage with the [runtime usage importer](../../../adapters/pi/import_usage.ts). Supply the explicit task, participant, assignment and private session path. It supports only a session with one assignment, rejects ambiguous attribution, and is safe to repeat. Wait for a complete JSONL record; never invent missing usage. Compaction/tool usage and cost remain outside this importer's coverage. Other runners can submit normalized measured `record_usage` directly.
