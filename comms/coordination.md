# Participants and assignments

The orchestrator agent decides who works, what to do next, and whether results satisfy the task. Comms records and validates the collaboration. It does not execute recipes, choose assignees, wake agents, or grant approvals.

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

Factory launchers supply `FACTORY_INSTANCE_ID` and an `Instance ID` instruction. The ID remains available through runner-managed context resets; do not generate a new registration every turn. Other hosts supply an equivalent stable instance reference. An unassigned session registers after a task is selected. `list_participants` returns registration facts, including retired instances, **not live availability**. There is no heartbeat or presence monitor yet.

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

Allowed outcomes are `ready_for_review`, `completed`, or `failed`. All produce assignment state `submitted`; the outcome distinguishes the reported result. A failed result is still a useful handoff, not a success. Check entries require a command and explicit exit status, with an optional `output_ref`. Record only checks actually executed. The service validates structure and ownership, not the truth of submitted evidence or approval authority.

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

Startup upgrades schema 1 to schema 2 transactionally, preserving tasks, messages, events and usage. Schema 2 adds coordination tables and optional `messages.assignment_id`. No legacy prose assignment is automatically promoted into canonical ownership. Use a newly created structured assignment when continuing such work. An older binary that only supports schema 1 cannot reopen the upgraded database.

Rebuild/restart the comms deployment to expose the new tools, and reconnect existing clients. Preserve container-local data before recreating an unmounted container; use the existing opt-in volume when persistence is wanted. Source file edits do not update an already-running container or a session's loaded skill snapshot. No automatic delivery bridge or live contact collection is included in this feature.
