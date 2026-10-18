---
name: task-comms
description: Collaborate on Factory tasks through comms MCP using explicit assignments, acceptance, progress, blockers, handoffs, decisions, and evidence, regardless of agent runner.
---

# Task communications

Use the configured comms MCP tools. This protocol does not require a particular model, agent runner, terminal manager, or delivery extension. The skill guides active agent behavior; it does not create timers, wake idle agents, register tools, or collect telemetry. Read [the service reference](../../README.md) for actual tool schemas and deployment.

## Identity and participants

Establish the explicit project, task ID, your routing identity, task owner/coordinator, assignment, and artifact location from the coordinator or host context. Do not choose the newest task or infer a workspace by searching parent directories. A service task ID is distinct from a local artifact directory. Factory launchers can supply this context in instructions and environment; other hosts may supply it differently.

Register each task participant through `register_participant` using its stable instance ID, role, routing identity, and a request key. Factory launchers provide `FACTORY_INSTANCE_ID`; another host can provide equivalent context. Retain the returned participant ID. Use `list_participants` pages to discover contacts and distinguish instances sharing a role. Registration is not live presence or authenticated identity. The task owner's active routing identity must equal `get_task.owner`.

The orchestrator explicitly chooses participants, supplies scope and relevant peers in assignments, and resolves unclear ownership or availability. Multiple instances of the same role need distinct active routing identities. Read the [coordination reference](../../coordination.md) for replacement and recovery; a replacement must not silently inherit an old instance's work. Direct peer questions are allowed; summarize consequential answers and human feedback to the orchestrator.

## Establish or recover a task

An existing agent can coordinate if it has this skill and working MCP tools. For new work, use `create_task` with an explicit project, brief, name, owner, and stable request key. Resolve resume requests with project-scoped `list_tasks`; ask when ambiguous. Retain the returned task ID and inspect current status, decisions, relevant messages, and artifacts before dispatching work.

In the supplied Factory setup, the Go owner is `coordinator`, writing owner is `pair_writer`, and the project identifier is the selected application root. For local document scaffolding, use the artifact helper in [coordinator operations](../../../orchestrator/references/operations.md); it attaches the service task instead of creating another one. `task.json` links documents to comms, not a second state store. Application code belongs in the project workspace.

If comms is unavailable, report the failure and pause dependent coordination. Never create a competing file mailbox or claim an unsuccessful remote write was saved.

## Delivery and inbox checks

Establish which delivery mode the host actually provides:

- **Automatic delivery:** Process incoming comms messages supplied by the host integration. Use their task and message IDs to read authoritative context, avoid duplicate processing, and acknowledge reading. Do not start a competing notification loop or claim such an integration exists without evidence.
- **Manual delivery:** Check the inbox at the beginning of each active turn, after substantial work, after an interruption, and before a handoff. If an idle recipient needs prompting, notify its known live session through available host controls or tell the human exactly which recipient needs to check. Empty-inbox polling or sleeping inside a model turn does not create reliable background delivery.

The current comms service stores messages and participant registrations but provides no automatic wake-up, polling bridge, or live presence monitor. A skill alone cannot provide periodic polling. Terminal controls such as Herdr are optional host adapters, not part of the communication protocol.

At each check, `get_task` to reconcile status/version, then `read_inbox` with your recipient identity. Follow `after` pagination; use `include_acknowledged` when recovering prior assignments. After reading, call `acknowledge_message` with the task/message ID and your identity. Reading, acceptance, progress, result submission, and approval are different facts.

## Assign, accept, report, and hand off

Use `create_assignment`, `get_assignment`, `list_assignments`, `update_assignment`, and `submit_result` for canonical assignment state. Read the [assignment and peer-message guidance](references/messages.md) when assigning or responding. These operations atomically notify the other participant and record lifecycle events; do not duplicate those writes manually. Use `send_message` for peer conversation, questions, and feedback, including its optional `assignment_id` for correlation.

For each assignment:

1. Establish scope, inputs, artifact location, expected output, stage/attempt, relevant peers, and required decisions. Ask about material ambiguity before acting.
2. Acknowledge reading, then explicitly accept or decline through `update_assignment` with a reason. Replayed assignments must not restart completed work; inspect prior results first.
3. Report meaningful assignment progress, blockers, or input needs through `update_assignment` with its current version. Publish individual check/failure evidence and material feedback with the relevant tools. Avoid repetitive “still working” messages. Heartbeats, if a host supplies them, are not progress evidence.
4. When waiting, state what input is needed, who must provide it, and what work is paused. Send the request before yielding. Do not interpret silence as approval or claim the host will resume you automatically.
5. Call `submit_result` with outcome, artifact references, actual checks, remaining limitations, and next action; its generated message is the orchestrator handoff. A final response in your own session is not a handoff. A result can mean ready for independent review; it does not approve the overall task.

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
