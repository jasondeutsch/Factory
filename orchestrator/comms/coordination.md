# Participants and assignments

The coordinator follows the task's pinned workflow when one is configured, chooses the actual participant for its next agent step, and assesses evidence and user decisions. Comms records and validates the collaboration. It does not execute workflows, choose assignees, wake agents, or grant approvals.

## Identities

A participant is one declared agent instance **within a task**. Register it with `register_participant`:

```json
{
  "task_id": "<actual task ID>",
  "instance_id": "<stable session/process identity>",
  "role": "coder_golang",
  "routing_identity": "coder-1",
  "session_ref": "<optional actual host reference>",
  "request_key": "register-coder-1"
}
```

Keep the returned participant `id`. A role describes responsibility; a routing identity addresses an inbox; the participant ID identifies the exact registered instance assigned to work. Two instances can share a role but must have different active routing identities. Actor IDs are submitted declarations, not authenticated credentials.

The task owner also registers a participant. Its `routing_identity` must equal `get_task.owner` (normally `coordinator`, or `pair_writer` for writing). Only that active declared owner can create or cancel assignments. Workers do not create downstream assignments or approve work merely by submitting a result.

Factory launchers supply `FACTORY_INSTANCE_ID` and an `Instance ID` instruction. The ID remains available through runner-managed context resets; do not generate a new registration every turn. Other hosts supply an equivalent stable instance reference. An unassigned session registers after a task is selected. `list_participants` returns registration facts, including retired instances, **not live availability**. Use `get_session` for the recorded configuration and expiring native/host contact. Contact is separate from assignment progress; expired contact is unknown, not proof of a stopped process.

### Host binding for an existing instance

After a running agent has a captured host reference, call `bind_participant` with `task_id`, `participant_id`, `actor_participant_id`, `expected_session_ref`, `session_ref`, and `request_key`. The actor must be that participant or the active task owner. Use the current reference from `list_participants` (an empty string when absent) as the expected value. Stale updates fail, and retries with the original key replay the original response.

This attaches or clears delivery information without retiring the instance or changing its assignments. It does not verify the host reference or establish liveness. Use replacement for a different process; use binding to enable delivery to the existing one. See [Herdr binding setup](herdr-delivery.md#setup-through-the-coordinator).

### Replacement

A genuinely new process/session uses a new instance ID. To reuse an old routing identity, explicitly include `replaces_participant_id` in registration. The referenced participant must belong to the task and have the same role and routing identity. The transaction retires the old instance and registers the replacement; it never transfers assignments. Old instance records and evidence remain.

A retired instance cannot report new assignment progress or results. The current task owner can cancel its outstanding assignments and create new ones referencing relevant prior evidence. A replacement owner can similarly cancel assignments created by its predecessor. Cancel any still-running work through host controls separately. No replacement, disconnect, or timeout automatically retries work.

## Create work

After resolving prerequisites and actual approvals, the orchestrator calls `create_assignment`:

```json
{
  "task_id": "<actual task ID>",
  "assigning_participant_id": "<owner participant ID>",
  "assignee_id": "<worker participant ID>",
  "title": "Implement the approved game",
  "instructions": "Implement approved plan items only; report blockers and hand off for independent review.",
  "inputs": ["spec.html", "plan.html"],
  "expected_output": ["application source and tests", "reviews/build-01.md"],
  "stage": "build",
  "attempt": 1,
  "request_key": "assign-build-01"
}
```

The service returns the canonical assignment ID, `state: offered`, and `version: 1`. Inputs and outputs are references/descriptions, not paths resolved inside the container. Stage and positive attempt number provide reporting context, not an executable workflow graph. Assignments cannot be created on a complete/cancelled task until its owner explicitly reopens it.

The same transaction creates the assignee's `assignment` inbox message and an `assignment_created` timeline event. Notifications expose `message.assignment_id`; their text is a service-generated assignment snapshot. Use `get_assignment` for current state, not an old message snapshot. These records are stored notifications, not terminal delivery.

## Accept, work, and submit

Acknowledge reading with `acknowledge_message` using the **routing identity** as its `actor`. This leaves assignment state unchanged. Then use `update_assignment` with the **participant ID** as `actor_participant_id`:

```json
{
  "task_id": "<actual task ID>",
  "assignment_id": "<canonical assignment ID>",
  "actor_participant_id": "<assignee participant ID>",
  "expected_version": 1,
  "state": "accepted",
  "summary": "Scope and prerequisites understood.",
  "request_key": "accept-build-01"
}
```

Use a fresh version for each new update. A summary is required, including a concrete reason when declining, blocked, or needing input. Supported transitions are:

| Current state | Allowed next states |
| --- | --- |
| `offered` | `accepted`, `declined`, `cancelled` |
| `accepted` | `working`, `blocked`, `needs_input`, `submitted`, `cancelled` |
| `working`, `blocked`, `needs_input` | `working`, `blocked`, `needs_input`, `submitted`, `cancelled` |
| `submitted`, `declined`, `cancelled` | Terminal; create another assignment for further work |

Only the active assignee can report acceptance, decline, progress, blockers, or results. Only the active task owner can cancel. Repeated `working` updates are allowed for **meaningful progress**, not heartbeat chatter. Task completion/cancellation blocks new worker updates/results; cancellation of an outstanding assignment remains available to the owner.

`submitted` is reached only through `submit_result`, after acceptance:

```json
{
  "task_id": "<actual task ID>",
  "assignment_id": "<canonical assignment ID>",
  "actor_participant_id": "<assignee participant ID>",
  "expected_version": 3,
  "request_key": "result-build-01",
  "result": {
    "outcome": "ready_for_review",
    "summary": "Implementation is ready for independent review.",
    "artifacts": ["reviews/build-01.md", "progress.md"],
    "checks": [{"command": "go test ./...", "exit_status": 0}],
    "limitations": ["gopls was unavailable."],
    "next_action": "Assign the independent reviewer."
  }
}
```

Allowed outcomes are `ready_for_review`, `completed`, `changes_requested`, or `failed`. All produce assignment state `submitted`; the outcome distinguishes the reported result. A reviewer uses `changes_requested` for actionable revision findings, which a configured workflow can route back to an earlier step. A failed result is still a useful handoff, not a success. Check entries require a command and explicit exit status, with an optional `output_ref`. Record only checks actually executed. The service validates structure and ownership, not the truth of submitted evidence or approval authority.

Each successful assignment update/submission atomically stores its new version, a timeline event, and a notification to the assigning participant's routing identity. Cancellation notifies the assignee. The orchestrator reads the result and decides whether to review, assign repairs, seek input, or update overall task status. **Submission is not review, human approval, or task completion.**

## Query, retry, and recover

| Tool | Inputs and behavior |
| --- | --- |
| `register_participant` | Task, instance ID, role, routing identity, request key; optional session reference and replacement ID. |
| `list_participants` | Task ID; optional `after` and `limit`. Includes retired instances. |
| `create_assignment` | Task, assigning participant, assignee, title, instructions, expected output, stage, positive attempt, request key; optional inputs. |
| `get_assignment` | Explicit task ID and assignment ID. |
| `list_assignments` | Task ID; optional `assignee_id`, `state`, `after`, `limit`. Query each desired state to collect outstanding work. |
| `update_assignment` | Task/assignment ID, actor participant ID, expected version, state, summary, request key. |
| `submit_result` | Task/assignment ID, actor participant ID, expected version, structured result, request key. |

Lists are oldest first, default 50, maximum 200; pass the last returned ID as `after`. All records/references are task-scoped. Unknown references, cross-task assignees, retired actors, invalid transitions, and stale versions fail explicitly. Ordinary `send_message` can include `assignment_id` for peer questions and feedback; it validates the task reference but does not mutate assignment state.

Every coordination mutation requires a stable request key. Registration keys are scoped to task/instance; assignment mutation keys are scoped to task/actor participant and shared across operation names. Reuse the **original key and identical arguments, including expected version**, after an uncertain response. Successful retries return the original saved response without duplicating state, messages, or events. The replayed response can be older than current state; use `get_assignment` to reconcile before a new action. Reusing a key with different content/operation is an error.

On a stale update, fetch current state and reconcile before issuing a new mutation with a new key. A retry of the original acceptance must not restart implementation. A cancelled/declined/submitted assignment cannot be resurrected by reporting progress. The orchestrator explicitly creates any follow-up assignment.

## Reports and compatibility

`get_task_report` includes participant/assignment totals, assignment state counts, and up to 200 participant/assignment records in creation order. Use list pagination for additional rows. HTML shows ownership, instance registration/retirement, instructions, results and evidence. Assignment events contribute their recorded states to stage/attempt presentation; `submitted` is displayed as submitted, not completed. Stage/attempt labels should identify the intended activity; separate unrelated assignments into distinct labels or attempts.

Current task status remains independently owned by the orchestrator. Existing messages, acknowledgement semantics, approval events, and usage measurements remain available. Do not manually duplicate canonical assignment operations as generic messages/events. Peer conversation, individual checks, approval decisions, and measured usage still use their existing tools.

The service initializes or upgrades its supported SQLite schema transactionally, preserving recorded evidence. See [service configuration](README.md#configuration-and-development) for the current schema. Stored conversation is not automatically converted into assignments.

Rebuild/restart Comms after source updates and reconnect clients. The supplied Compose volume preserves the database across container replacement; `down -v` deletes it. Existing sessions retain their loaded skill snapshot. Native delivery and expiring contact are supplied by the [runner adapter](delivery.md), with an optional Herdr fallback for other runners.

## Recover stopped work

1. Call `get_recovery_plan` with the exact task and assignment. Inspect its saved inputs, summary, participant/contact, and related messages/checks. Contact expiry does not prove that its process stopped.
2. Stop the old process using its native host controls. Register and bind the actual replacement instance; do not invent a placeholder or reuse the stopped instance's identity.
3. The owner calls `recover_assignment` with the current assignment version, reason, stable request key, and `action: reassign`. Supply the replacement participant ID and `previous_session_stopped: true` only after that confirmation. For cancellation alone use `action: cancel`.
4. Inspect the receipt. Cancellation and the linked replacement assignment occur in one transaction. A retry returns that same receipt. The previous summary/evidence remains intact; the replacement gets the original instructions/requirements and a reference to prior work, not a fabricated completed result.
5. Use the replacement’s configured delivery adapter; Pi receives the saved notification through its native extension. Resolve related attention items after reconciliation. Recovery does not start/stop panes, approve work, transfer participant identity, or silently close pending decisions.

Terminal assignments cannot be recovered this way; create an explicit new assignment when revisiting submitted or already-cancelled work. The declared owner can cancel a retired participant's outstanding assignment. All identity and stopped-process assertions are declared by the caller, not authenticated host proofs.
