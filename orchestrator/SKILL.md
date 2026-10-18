---
name: factory-coordinator
description: Coordinate a Factory task through visible interactive agent sessions, durable handoffs, explicit decisions, and evidence of progress or completion.
---

# Factory coordinator

Use this skill in the user's existing agent session; a separate coordinator process is not required. Read the selected `factories/<factory>.md` and shared [task-comms skill](../comms/skills/task-comms/SKILL.md). You are the user's main conversation and the task's status writer for coordinated flows. Use the [operations reference](references/operations.md) for deterministic bookkeeping and session commands. Perform those operations yourself using available tools rather than asking the user to run routine lifecycle scripts.

## Begin

Check that your native `comms` connection works before creating or resuming work. All roles use this service for status, messages, decisions, and evidence. If unavailable, report the concrete failure; do not fall back to local mailbox files.

Establish the project root from the explicitly selected project or current working directory containing `.factory/`. Do not search upward and silently select another project. Determine whether the user wants a new task or to resume an existing one. For a new task, create its record through comms MCP and attach local artifacts using the operations reference. Retain both the comms task ID and artifact directory. For an existing task, inspect its status, messages, decisions, and artifacts. If the reference is ambiguous, ask which task; never silently choose the newest. Do not require the user to know a task path or supply a finished spec.

Clarify only scope, constraints, and decisions that affect the work. Default to human decisions at the recipe's approval points. Record explicit delegation with its scope. Do not restart work merely because a pane is new.

Reuse appropriate existing role sessions when available. If the user requests visible agent panes, create and prepare them using [the Herdr adapter](adapters/herdr/README.md) from inside Herdr; that request authorizes the requested pane setup. Start sessions only as needed and keep them interactive. Each role's `start.sh` uses the shared launcher to load its configuration and skills; omit the task only for an intentionally unassigned session. Do not execute an interactive worker launcher in your own blocking shell tool. Register the task owner and workers with `register_participant` and discover their task-scoped identities through `list_participants`, following the shared skill. Include actual live agent names and pane IDs only when known; these are host delivery details, not the communication identity. Do not guess IDs, silently start hidden workers, or install extensions automatically.

Each role owns a runnable bundle: instructions and skills plus native model/provider, tool, and MCP configuration. Configure these through the selected runner's files; do not duplicate model fields in the wrapper. If the user has not selected a provider and model, resolve that choice before launching workers. Handle native startup checks yourself, inspect failures and actual session readiness, and explain the problem clearly. Do not install a missing runner or MCP server automatically.

For approved Go implementation, use the coder's bounded Ralph loop, normally five iterations, through the [Ralph reference](references/ralph.md). Record the assignment before launch and let only one loop own source edits in a project. Observe loop termination as well as task messages; reaching a limit or encountering an error is not completion. Human approval, review rounds, and task status remain your responsibility. Keep reviewers and the pair writer interactive. During installation into a Git project, exclude the plugin's root `.ralph/` runtime directory from version control.

When asked to pull a Linear issue, use a planner or reviewer with the configured Linear context skill. Enable that role's optional read-only connection only when requested and credentials are available; perform the setup rather than handing the user a checklist. Apply changed MCP configuration through native controls or a new session. Do not expose keys or silently change Linear issue state. The communication skill lives in `comms/skills/`; other shared skills live in `skills/`. All are explicitly included by role configuration; the conventional-commits skill guides messages but never grants permission to commit.

Load role instructions and configured skills into existing sessions explicitly. Reading configuration is not the same as applying it: use native runner controls or a new configured session for model, tool, or MCP settings, and report unsupported configuration. Do not claim parity across runners. If pane controls or credentials are unavailable, explain the concrete limitation and ask only for the necessary human action, not a lifecycle checklist.

## Coordinate

Create explicit work through comms `create_assignment`, using registered participant IDs and retaining the returned assignment ID/version. The receiver needs scope, inputs, artifact paths, expected output, relevant peers, and next decision. Inspect `get_assignment`/`list_assignments` for explicit acceptance, decline, progress, blockers, and submitted results; reading acknowledgement alone is not acceptance. Assignment operations generate their inbox messages and events atomically; do not manually duplicate them. Establish the host's actual delivery mode. Use automatic delivery only when a working integration exists; otherwise check inboxes at active turn boundaries and notify known live sessions through available controls. Notification is separate from the durable message. If an idle recipient cannot be notified, tell the human which participant needs prompting. The optional [host delivery bridge](../comms/delivery.md) can notify idle Herdr sessions using captured participant bindings; run it in a separate visible pane, and avoid duplicate manual prompts when enabled. Capture references before registering participants and assigning work. A skill does not poll by itself, and live presence monitoring remains separate work.

Permit relevant peer questions. Ask agents to summarize consequential peer and human exchanges back to you. Keep source editing with one owner at a time. After direct human intervention, reconcile the task with the changed instructions before issuing conflicting work.

Maintain status through comms `get_task` and `update_task` with the current version. States are pending, working, needs_input, blocked, complete, and cancelled. Interpret progress requests, review feedback, approvals, and resume requests from the user's conversation against the current task. Ask a focused question if a decision's scope or document version is unclear. Record explicit comms decision events with exact artifact hashes; the decision helper can compute and publish them. The actor field records declared authority; it does not authenticate a person. Never impersonate an approval the user has not given.

## Report

Tell the user what changed, what is currently happening, what needs their attention, and where to inspect it. State failures plainly with the actual error, responsible role, and a concrete next action. An idle terminal is not completion; look for the explicit result and actual evidence.

Follow the recipe's bounded review loops. Surface unresolved disagreements rather than paying for endless agent debate. To resume, inspect the last handoff, preserve completed work, and assign the unresolved part. Complete a task only when its agreed conditions hold. Include available measured usage; mark missing measurements unknown. Do not test or repair another role's deliverable merely to hide a failed handoff.
