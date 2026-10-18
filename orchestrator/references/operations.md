# Coordinator operations

Users request work, ask for progress, and make decisions in conversation. Agents handle bookkeeping. Read the shared [task-comms skill](../../comms/skills/task-comms/SKILL.md) for communication and evidence conventions. Use native `comms` MCP tools for task operations; helpers below handle local artifacts, launch configuration, and exact document hashes.

## Connect and establish a task

Run the supplied Compose service as described in [comms setup](../../comms/README.md). All participants must use the same service and explicit project identifier. In this toolbox, the project identifier is the resolved application root containing `.factory/`; do not search parent directories or infer another project. The default endpoint is `http://127.0.0.1:8090/mcp`. `FACTORY_COMMS_URL` selects another endpoint for helpers and prepared Pi profiles. If the service uses `COMMS_AUTH_TOKEN`, make the same variable available to helpers and native clients without printing it.

An existing coordinator or writer needs its runner's native MCP connection configured and reloaded; reading a skill does not register tools. No second coordinator process is needed. To start one intentionally, use `.factory/orchestrator/start.sh` without a task; it loads its configuration and skills before taking your request. Do not launch workers until the service and selected model/provider are usable.

Create a task through `create_task` with the user's brief, explicit project, owner, and a stable request key. Use `coordinator` as owner for Go and `pair_writer` for writing. Preserve the returned ID. Create its client-managed artifact directory:

```bash
python3 .factory/scripts/task.py attach <comms-task-id> --factory golang
```

`attach` checks project and owner, creates the local pointer and HTML drafts, and returns the local path. Repeating it reuses the existing directory. Use `--factory writing` for writing without spec/plan documents. An alternate `task.py new --factory golang --name "..." --brief "..." --request-key "..."` combines MCP creation and attachment when a shell-based coordinator needs it. It uses the same service and creates no separate mailbox. Long briefs use `--brief-file <file>` or `--brief-file -` with stdin; never interpolate user text as shell code.

To resume, use `list_tasks` with the explicit project and follow `before` pagination. Match the user's reference; ask when ambiguous. Find its local pointer in `.factory/tasks/*/task.json`, or attach it if documents have not yet been scaffolded. Inspect remote status, all relevant inbox/event pages, and local reviews and progress. There is no global current-task pointer. Local documents do not restore a removed ephemeral database.

## Prepare visible sessions

Read-only configuration inspection:

```bash
.factory/agents/architect/start.sh <artifact-directory> --dry-run
```

Read `cwd`, instructions, and `runner_configuration`. Configure the actual provider/model and enabled dependencies before launch. A normal dry run has no network calls; `--loop` additionally **reads comms approval events**. Neither writes files. Task metadata displayed in an offline dry run comes from the local pointer; actual launch refreshes it from comms.

`--check` prepares a private native profile, reads task state, checks credentials and enabled MCP connections, and records a session check in comms. It does not call a model. It can start MCP processes, contact services, or install the declared Ralph package. Live launch performs these checks automatically. Failures remain visible through stderr and private diagnostics, with a comms failure event when possible. If the service itself is down, no remote event can be recorded; do not hide the original error.

Open or reuse a visible pane through the [Herdr adapter](../adapters/herdr/README.md), then run that role's `start.sh <artifact-directory>` **in that pane**. All role scripts use the shared session launcher. Do not run an interactive launcher in the coordinator's blocking shell or replace an occupied agent. Record actual live names and pane IDs with `record_event` so peers can notify one another. Do not guess IDs.

The launcher supplies project, role, artifact directory, and comms task ID in its instructions/environment. Every configured role imports the skill directly from `comms/skills/task-comms`; Pi gets the matching MCP endpoint and optional token reference. Native profiles and transcripts live under `$HOME/.factory/runtime/`. Authored profiles and global Pi settings are not rewritten. Non-Pi command adapters must configure their runner's MCP connection separately. See [session configuration](../README.md).

For approved Go implementation, record an assignment with stage and positive attempt number, then launch the coder with `--loop 5`. The entry guard reads comms decision events and hashes the actual HTML files. Follow the [Ralph reference](ralph.md) for bounded progress, independent review, and recovery.

## Dispatch and report through MCP

Use `send_message`, `read_inbox`, and `acknowledge_message` for handoffs. Include the task ID, role identities, assignment scope, artifact directory, expected output, stage, and attempt. Send peer exchanges and direct human feedback back to the coordinator. Saved messages do not wake agents. After saving, notify the actual live Herdr session using available controls. A notification failure must not duplicate the saved assignment.

Use `get_task` then `update_task` with `expected_version` for owner status changes. Record stage starts, completions, checks, failures, session mapping, and scoped delegation using `record_event`. Read paginated history on recovery. Present `get_task_report` metrics and the full report URL, including missing usage coverage. A stopped process or idle pane is not evidence of completion.

The shell helper's `status`, `send`, `inbox`, `ack`, `note`, and `decide` operations call the same MCP service. They exist for deterministic operations and shell-only clients, not a second required lifecycle. `send --notify <live-name>` saves first and then attempts Herdr notification. Stable `--request-key` is available for message/task retries after an uncertain response. Preserve the original key and payload; do not blindly repeat a write with a new identity.

## Record an actual conversational decision

Present the exact documents and receive the required approval or scoped delegation. To compute exact hashes and publish the standard comms decision event:

```bash
python3 .factory/scripts/task.py decide <artifact-directory> --outcome approve --by human \
  --artifact spec.html --artifact plan.html --text "Approved for implementation."
```

Use the actual decision, deciding identity, and rationale. This is an agent operation, not a command to hand to the user. Alternatively hash the exact files and call `record_event` using the [shared skill's decision shape](../../comms/skills/task-comms/SKILL.md). A decision message alone does not satisfy the launcher guard. Changed files, a later artifact-specific rejection, or an unscoped change request block a new loop until resolved. Declared actor names are not authenticated authority.

## Maintainer verification

Run `python3 -B -m unittest discover -s .factory/tests -v` inside an installed project, or use `factory/tests` from the toolbox's parent. Tests build the real comms server (Go required), start isolated localhost instances with disposable SQLite files, and use fake agent executables. `FACTORY_TEST_COMMS` can select an already built server binary. Optional native Pi/Ralph tests are described in [troubleshooting](../../docs/troubleshooting.md). No model quality or live Herdr behavior is implied by these checks.
