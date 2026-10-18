# Assignments and peer messages

Use canonical comms tools for assignment ownership and lifecycle. Read [the coordination reference](../../../coordination.md) for input examples, allowed transitions, replacement, pagination, and retry semantics. JSON inside message text is conversation data, not an alternative assignment record.

## Register and discover

Establish task/project, role, instance ID, and routing identity from the orchestrator or host context. Factory launchers provide `FACTORY_INSTANCE_ID`; other runners provide an equivalent stable identity. On a fresh context, look up the matching instance through `list_participants` first. Register once per task/instance with `register_participant` and a stable request key when no matching registration exists. Retain the returned participant ID across turns and context resets. The owner's routing identity must match the task's owner.

Read `list_participants` pages for assigned contacts. Distinguish role, routing identity, participant ID, and optional actual host reference. Retired registrations are historical; registration is not live availability or authentication. Ask the orchestrator about unknown/unreachable peers. Multiple instances of a role need distinct active inbox identities.

Replacement is explicit via `replaces_participant_id`, using a new instance ID with the same role and routing identity. It retires the old registration without inheriting its assignments. The orchestrator cancels obsolete assignments and creates follow-up work deliberately. Do not bypass instance checks by impersonating a previous worker.

## Canonical assignment operations

The task owner calls `create_assignment` with the assigning participant ID, assignee ID, instructions, inputs, expected output, stage/attempt, and stable request key. Save its returned assignment ID/version. The service atomically creates the inbox message and event; do not manually duplicate either.

The recipient reads its inbox, acknowledges reading using its **routing identity**, and fetches `get_assignment`. Then use `update_assignment` with its **participant ID** to accept or decline. Acknowledgement alone is not acceptance. Only accepted work can advance to working, blocked, needs_input, or a submitted result.

Report meaningful progress/blockers through `update_assignment` with a fresh expected version, explanatory summary, and new request key. The orchestrator receives the generated notification. When waiting, name the needed input and responsible participant in the summary; send a concrete question through `send_message` when needed. Do not consume turns with empty-inbox polling.

Send the final handoff using `submit_result`: outcome (`ready_for_review`, `completed`, or `failed`), summary, actual artifact/check references, limitations, and next action. It atomically notifies the orchestrator. Submitted results do not approve themselves, complete the task, or start another agent. The orchestrator decides subsequent assignments.

Use `list_assignments` with state/assignee filters when recovering outstanding work. Reuse the original request key and payload after an uncertain response. A replay returns the original response, which can be older than current state; fetch before a new action. Never restart work merely because a message was delivered again. Terminal assignments require an explicit new assignment for further work.

## Peer conversation and evidence

Use `send_message` for questions, answers, review findings, direct human feedback, and conversational context. Include `assignment_id` at the tool's top level when applicable; the server validates it belongs to the task. The message kinds remain `assignment`, `question`, `answer`, `progress`, `finding`, `result`, and `decision`, but generic messages do **not** create or transition canonical assignments.

Message text can be plain prose. State the question/finding, source evidence, relevant artifact, and requested action. Address known routing identities and summarize consequential peer exchanges to the orchestrator. Serialize text safely through structured tools; never interpolate arbitrary message content into shell code.

Assignment operations already record lifecycle events. Use `record_event` for additional checks, failures, artifact decisions, and other evidence; use `record_usage` for actual measurements. Approval decisions still require their documented event shape and exact hashes. A decision message alone is not approval evidence.
