# Coordinator operations

Users request work, ask for progress, and make decisions in conversation. Agents handle bookkeeping. Read the shared [task-comms skill](../comms/skills/task-comms/SKILL.md) for communication and evidence conventions. Use native `comms` MCP tools for task operations; helpers below handle local artifacts, launch configuration, and exact document hashes.

## Connect and establish a task

The supplied coordinator entrypoint starts local Compose; each Pi session handles its own inbox delivery. For an externally hosted coordinator, run Compose separately as described in [comms setup](../comms/README.md). All participants must use the same service and explicit project identifier. In this toolbox, the project identifier is the resolved application root containing `.factory/`; do not search parent directories or infer another project. The default endpoint is `http://127.0.0.1:8090/mcp`. `FACTORY_COMMS_URL` selects another endpoint for helpers and prepared Pi profiles. If the service uses `COMMS_AUTH_TOKEN`, make the same variable available to helpers and native clients without printing it.

An existing coordinator or writer needs its runner's native MCP connection configured and reloaded; reading a skill does not register tools. No second coordinator process is needed. To start one intentionally, use `.factory/orchestrator/start.sh --factory <name>` without a task; it loads its configuration and skills before taking your request. Do not launch workers until the service and selected model/provider are usable.

Create a task through `create_task` with the user's brief, explicit project, owner, and a stable request key. Read `owner` from the selected `factory.json`; the supplied defaults are `coordinator` for Go and `pair_writer` for writing. Preserve the returned ID. Create its client-managed artifact directory:

```bash
node .factory/orchestrator/scripts/task.ts attach <comms-task-id> --factory golang
```

Create/attach validates the selected `factory.json`. `attach` checks project and configured owner, creates the local pointer and recipe-specific drafts, and returns the local path. Repeating it reuses the existing directory. Use `--factory writing` for writing without spec/plan documents. An alternate `task.ts new --factory golang --name "..." --brief "..." --request-key "..."` combines MCP creation and attachment when a shell-based coordinator needs it. It uses the same service and creates no separate mailbox. Long briefs use `--brief-file <file>` or `--brief-file -` with stdin; never interpolate user text as shell code.

To resume, use `list_tasks` with the explicit project and follow `before` pagination. Match the user's reference; ask when ambiguous. Find its local pointer in `.factory/tasks/*/task.json`, or attach it if documents have not yet been scaffolded. Inspect remote status, all relevant inbox/event pages, and local reviews and progress. There is no global current-task pointer. Local documents do not restore a deleted database volume.

## Prepare visible sessions

Read-only configuration inspection:

```bash
.factory/resources/agents/architect/start.sh <artifact-directory> --dry-run
```

Read `cwd`, instructions, and `runner_configuration`. Configure per-member provider/model and native overrides in factory.json before launch; use --factory to select it for an unassigned session. A normal dry run has no network calls; `--loop` additionally **reads comms approval events**. Neither writes files. Task metadata displayed in an offline dry run comes from the local pointer; actual launch refreshes it from comms.

`--check` prepares a private native profile, reads task state, checks credentials and enabled MCP connections, and verifies extension command registration. It does not call a model. It can start MCP processes, contact services, or install the declared Ralph package. Live launch performs these checks automatically. Failures remain visible through stderr and private diagnostics, with a comms failure event when possible. If the service itself is down, no remote event can be recorded; do not hide the original error.

Open or reuse a visible session through [dispatch](dispatch.md) using the configured terminal host, then run that role's `start.sh <artifact-directory>` **in that pane**. All role scripts use the shared session launcher. Do not run an interactive launcher in the coordinator's blocking shell or replace an occupied agent. Record actual live names and pane IDs with `record_event` so peers can notify one another. Do not guess IDs.

The launcher supplies project, role, artifact directory, and comms task ID in its instructions/environment. Every configured role imports the skill directly from `orchestrator/comms/skills/task-comms`; Pi gets the matching MCP endpoint and optional token reference. Native profiles and transcripts live under `$HOME/.factory/runtime/`. Authored profiles and global Pi settings are not rewritten. Non-Pi command adapters must configure their runner's MCP connection separately. See [session configuration](../README.md).

For approved Go implementation, use the already registered coder session, create its assignment with stage and positive attempt number, and start its native Ralph loop in the same pane. Alternatively launch with `--loop 5`, discover its new registration, and create its assignment during the launcher's bounded wait. The entry guard reads comms decision events and hashes the actual HTML files. Follow the [Ralph reference](ralph.md) for bounded progress, independent review, and recovery.

## Dispatch and report through MCP

In Pi, call `factory_bind_task` after task attachment; it registers the actual instance and starts delivery. Other runners register an unassigned owner with its actual stable instance ID. Task-bound launchers register their workers after native checks; discover those actual IDs with `list_participants` before assigning work. Never register guessed worker identities. The owner's routing identity must equal the task's `owner`. Use `list_participants` to discover IDs; registration is not live availability. Find an existing matching instance registration after a context reset instead of inventing another instance.

Use `create_assignment` with registered participant IDs, instructions, inputs, expected output, stage, attempt, and a stable request key. Retain its canonical ID. The service atomically creates its inbox notification and event. Workers acknowledge reading separately, accept/decline through `update_assignment`, report progress/blockers with versions, and hand off through `submit_result`. These writes notify the other participant atomically; do not manually duplicate them as ordinary messages or lifecycle events.

Use `get_assignment` and paginated `list_assignments` filters to inspect actual outstanding work. Stale versions require reconciliation, not blind overwrites. The orchestrator explicitly chooses follow-up review or repair assignments; submission never approves itself or completes the task. See [the coordination reference](../comms/coordination.md) for argument examples, allowed transitions, retries, and replacement.

Use `send_message` with optional `assignment_id` for questions, findings, peer exchanges, and direct human feedback. Read/acknowledge inbox messages using routing identity. Pi's native extension delivers hints automatically. Use `factory_read_message` to retrieve content and record `read`, then `factory_exec_message` immediately before acting. Other runners use the same MCP tools through their adapter. Do not add a competing terminal notification loop.

Use `get_task` then `update_task` with `expected_version` for owner status changes. Record stage starts, completions, checks, failures, session mapping, and scoped delegation using `record_event`. Read paginated history on recovery. Present `get_task_report` metrics and the full report URL, including missing usage coverage. A stopped process or idle pane is not evidence of completion.

The shell helper's `send --kind assignment` remains a generic message and does not create structured ownership; use `create_assignment` for work. Its `status`, `send`, `inbox`, `ack`, `note`, and `decide` operations call the same MCP service. They exist for deterministic operations and shell-only clients, not a second required lifecycle. Delivery is handled separately by the running recipient adapter. Stable `--request-key` is available for message/task retries after an uncertain response. Preserve the original key and payload; do not blindly repeat a write with a new identity.

## Record an actual conversational decision

Present the exact documents and receive the required approval or scoped delegation. To compute exact hashes and publish the standard comms decision event:

```bash
node .factory/orchestrator/scripts/task.ts decide <artifact-directory> --outcome approve --by human \
  --artifact spec.html --artifact plan.html --text "Approved for implementation."
```

Use the actual decision, deciding identity, and rationale. This is an agent operation, not a command to hand to the user. Alternatively hash the exact files and call `record_event` using the [shared skill's decision shape](../comms/skills/task-comms/SKILL.md). A decision message alone does not satisfy the launcher guard. Changed files, a later artifact-specific rejection, or an unscoped change request block a new loop until resolved. Declared actor names are not authenticated authority.

## Maintainer verification

From the toolkit root, run `npm ci`, `npm run check`, and `npm test`; run `go test -race ./...` in `orchestrator/comms`. See [verification](../tests/README.md) for isolation, optional native checks, and live acceptance limits.


For coordinated handoffs, use [deterministic dispatch](dispatch.md) instead of separately opening a pane, guessing a participant, creating an assignment, and prompting it. The orchestrator owns the requested work; the helper owns recovery of that one operation.

Task attachment stages the complete scaffold before publishing its directory and serializes concurrent attaches. Repeating attachment preserves authored documents and retries the same comms evidence write. An incomplete older attachment fails explicitly rather than overwriting its contents. Hidden `.attaching-*` directories left by a killed process are unpublished scaffolds, not tasks.
