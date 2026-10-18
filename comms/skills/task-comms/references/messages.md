# Message conventions

These are working conventions over the existing MCP API, not server-enforced fields, assignment states, or new tools. `send_message.text` remains a string; serialize the body below as JSON with available structured tools rather than interpolating arbitrary text into shell code. Keep human-readable `summary` text. Link large documents instead of embedding them.

## Assignment

The coordinator chooses a stable `assignment_id`, unique within this task, before sending. Use the same ID for its subsequent acceptance, progress, questions, and result. A retry of the same message uses the same `request_key` and content; each genuinely new update needs a new key.

Example body for `kind: assignment`:

```json
{
  "assignment_id": "build-01",
  "summary": "Implement the approved command-line game.",
  "scope": "Only the approved plan items; no web UI.",
  "artifact_dir": "/explicit/project/.factory/tasks/selected-task",
  "inputs": ["spec.html", "plan.html"],
  "expected_output": ["application source", "tests", "reviews/build-01.md"],
  "stage": "build",
  "attempt": 1,
  "peers": [{"recipient": "code_reviewer_golang", "role": "code reviewer"}],
  "next_action": "Submit for independent code review."
}
```

The envelope still has the real `task_id`, `sender`, `recipient`, and `kind`. Do not put protocol fields at the API's top level. Include the project root separately when the artifact location does not establish it clearly. State prerequisites and approval requirements in the assignment. A peer list identifies assigned contacts, not proof that their sessions are running.

## Responses

Use `assignment_id`, `in_reply_to` (the original or most relevant message ID), `status`, and `summary` in response bodies. Add details relevant to the action:

| Meaning | Existing message kind | Body status | Useful fields |
| --- | --- | --- | --- |
| Accepted | `answer` | `accepted` | Confirm understood scope and prerequisites. |
| Declined | `answer` | `declined` | `reason`, `next_action`. |
| Meaningful progress | `progress` | `working` | Completed item IDs, evidence, remaining work. |
| Blocked | `progress` | `blocked` | Actual error, attempted action, `needed_from`, `next_action`. |
| Needs an answer | `question` | `needs_input` | Concrete question, `needed_from`, paused scope. |
| Review finding | `finding` | `changes_requested` | Finding IDs, evidence, affected scope. |
| Result | `result` | `ready_for_review` or `completed` | Artifacts, checks, limitations, next action. |

A `completed` assignment is not overall task completion or human approval. An acceptance message does not replace `acknowledge_message`. Acknowledging delivery does not establish acceptance. Semantic status in message text does not automatically change server task status or create report stage events.

Example result body:

```json
{
  "assignment_id": "build-01",
  "in_reply_to": "<actual assignment message ID>",
  "status": "ready_for_review",
  "summary": "Approved implementation is ready for independent review.",
  "artifacts": ["reviews/build-01.md", "progress.md"],
  "checks": [{"command": "go test ./...", "exit_status": 0}],
  "limitations": ["gopls was unavailable and was not exercised."],
  "next_action": "Assign the code reviewer."
}
```

Only report checks that actually ran. Also publish relevant `record_event` evidence with stage and attempt so report timelines can use it. Ask the owner for missing assignment context; do not invent approval, availability, or verification.

## Recovery and routing

Inspect earlier messages, results, and decisions before repeating work. Retain assignment correlation across context resets and fresh sessions. Duplicate notification is a prompt to inspect the existing record, not a new assignment. Direct peer messages must use known task participants and retain task/assignment context. If a peer is unknown or unreachable, route the question through the coordinator and state what is waiting.
