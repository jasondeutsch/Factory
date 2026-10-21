# Orchestrator MCP server

The orchestrator's Go MCP server (`comms`) provides shared task records, participants, assignments, agent inboxes, structured evidence, and reports. It uses the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) and the pure-Go [modernc SQLite driver](https://pkg.go.dev/modernc.org/sqlite). Its Go module lives in this directory.

SQLite is embedded in the comms process. One Compose file starts **two containers**: the MCP server and a read-only SQLite Web admin panel. Both use the same named data volume. No separate database service is needed. Agents connect over Streamable HTTP; they do not open the database file.

## Start with Docker Compose

From `factory/orchestrator/comms`, or `$FACTORY_HOME/orchestrator/comms` in an installed project:

```bash
docker-compose up --build -d
docker-compose ps
docker-compose logs comms
```

The default endpoints are:

| Endpoint | Purpose |
| --- | --- |
| `http://127.0.0.1:8090/mcp` | Agent MCP connection. |
| `http://127.0.0.1:8090/healthz` | Process/database readiness. |
| `http://127.0.0.1:8090/monitor` | Project task monitor; select a project and open a task's detailed status view. |
| `http://127.0.0.1:8090/tasks/<task-id>/report.html` | Current task report, generated on each request. |

Set `COMMS_PORT` before starting Compose to use another host port; set `FACTORY_COMMS_URL` in the coordinating/worker environment to the matching full `/mcp` URL. The default port mapping binds to loopback. The image runs as UID/GID `65532`, contains a static Go executable, and needs no separate SQLite installation, Python, Node, or shell at runtime. Go dependencies are vendored, so the Docker build does not contact a Go module proxy. Docker still needs the configured Go builder image; `GO_IMAGE` is an overridable build argument if the team supplies a compatible image with its corporate trust configuration.

The monitor's HTML shell contains no task data and can load without a token. If `COMMS_AUTH_TOKEN` is set, enter the same token used by Comms in the monitor page; it stays in the page's memory and is sent in request headers to the protected read-only monitor endpoints. The page refreshes the selected project's newest tasks every ten seconds and can load older pages; refresh pauses while browsing older tasks. Task stage is shown only when an active assignment records it; otherwise the task is marked as having no active assignment. Select a task to see its participants, messages, assignments, attention items, and full activity details.

When changing Comms dependencies, update `go.mod`/`go.sum` and regenerate `vendor/` from the module directory with `go mod vendor`. The Docker build and offline test command use `-mod=vendor` to ensure the vendored tree is complete.

Stop and remove the default service with:

```bash
docker-compose down
```

## Shared database volume

Both services mount the named `comms-data` volume at `/data`; the admin mount is read-only. The database is `/data/state.db`, with its SQLite WAL/SHM files. There is no host-home database.

Data written to this volume survives service restart, container replacement, and `docker-compose down`. An older container's unmounted database is not copied into the volume automatically. Keep the same Compose project name to reuse the same volume. For an explicit reset that deletes its task history:

```bash
docker-compose down -v
```

Back up the whole data directory. Stop services before a simple filesystem backup; copying only the live main database file can miss committed WAL records. One comms service owns a database; replicas and network-filesystem databases are not supported.

## SQLite admin panel

The [SQLite Web](https://github.com/coleifer/sqlite-web) panel starts with the MCP service from the same Compose file. Open **http://127.0.0.1:8091/** to browse tables, schema, rows, SQL query results, and exports. Set `COMMS_ADMIN_PORT` for another port; `COMMS_PORT` selects the MCP port.

The panel uses a pinned image, runs as UID/GID `65532`, binds to loopback, and reads the shared database/WAL without writing it. Make task changes through MCP so service validation and event recording remain intact. `COMMS_AUTH_TOKEN` protects MCP/reports, not the panel's separate localhost interface.

## Connect an agent

Factory prepares this connection in managed Pi profiles. Standalone agents and existing coordinator sessions must configure the equivalent connection through their runner:

```json
{
  "mcpServers": {
    "comms": {
      "url": "http://127.0.0.1:8090/mcp",
      "description": "Shared Factory task records, inbox messages, evidence, usage, and reports.",
      "exposure": "codemode"
    }
  }
}
```

Reconnect/reload through the runner or start a configured session. Reading this README does not register a connection in an existing agent. Other MCP clients can use the same HTTP endpoint through their native configuration. A client in another container must use the reachable service address, such as `http://comms:8080/mcp` on the same Compose network, rather than its own localhost.

Optional `COMMS_AUTH_TOKEN` enables a shared bearer token for MCP and reports. Supply the same variable to Compose and the agent environment, and, for an existing/manual client, add `"headers": {"Authorization": "Bearer ${COMMS_AUTH_TOKEN}"}` to the MCP entry. The Factory Pi launcher adds this environment reference automatically when the variable is set. Do not paste the token into configuration or task records. Health checks remain unauthenticated. Reports behind token authentication require a client that sends the header, or can be retrieved through the authenticated `render_report` tool and saved as HTML.

This shared token controls access to the service; it does not authenticate individual role names or establish human approval. With no token, the default loopback service is available to local clients. Broader network exposure needs appropriate authentication/TLS in the deployment; the provided Compose configuration is for local use.

## MCP tools

Task-scoped operations use an explicit task ID. Creation and project listings use a project identifier. Identifiers are supplied strings; the container does not discover host paths. Task names can contain spaces and uppercase letters. Task IDs include a UTC timestamp and random suffix.

| Tool | Required inputs | Result and behavior |
| --- | --- | --- |
| `create_task` | `project`, `name`, `brief`, `owner` | Creates pending metadata and a creation event. Optional `request_key` makes retries safe. Does not scaffold host files. |
| `list_tasks` | `project` | Newest first. Default 20, maximum 100. Use the last task's `sequence` as `before` for the next page. |
| `get_task` | `task_id` | Current metadata, state, and version. |
| `update_task` | `task_id`, `actor`, `expected_version`, `status`, `summary` | Updates status and appends an event atomically. Requires the declared owner and current version; stale updates fail. |
| `send_message` | `task_id`, `sender`, `recipient`, `kind`, `text` | Stores conversation and an event atomically. Optional `request_key` deduplicates retries; optional `assignment_id` links an existing assignment in the task. Does not change assignment state or wake the recipient. |
| `read_inbox` | `task_id`, `recipient` | Unacknowledged messages, oldest first. Default 50, maximum 200. Optional `include_acknowledged`; use the last message's `id` as `after` for the next page. |
| `acknowledge_message` | `task_id`, `message_id`, `actor` | Declared recipient marks the message read. Repeating it preserves the first acknowledgement timestamp. |
| `record_event` | `task_id`, `actor`, `kind`, `message` | Appends evidence, with optional `stage`, `attempt`, structured `data`, and `request_key`. |
| `list_events` | `task_id` | Oldest first. Default 100, maximum 500. Use the last event's `sequence` as `after` for the next page. |
| `record_usage` | `task_id`, `actor`, `stage`, `attempt`, `request_key`, `usage` | Records one model-request measurement with declared provider/model and provenance. |
| `get_task_report` | `task_id` | Consistent structured totals, stage attempts, models, coverage, and a relative report URL. Avoids putting the full HTML in model context. |
| `render_report` | `task_id` | Returns self-contained HTML and the report path. The caller can save the HTML; the server writes no report file. |

The service also exposes these structured coordination tools:

| Tool | Purpose |
| --- | --- |
| `register_participant` | Register a declared task instance and inbox routing identity, or explicitly replace an old instance. |
| `bind_participant` | Attach or clear a captured host binding for the same instance, with an expected current reference; preserves assignments. |
| `list_participants` | Discover task participants, including retired registrations; not live presence. |
| `create_assignment` | The registered task owner assigns work and atomically creates its notification/event. |
| `get_assignment` | Read current state, version, ownership, inputs, and result. |
| `list_assignments` | Query task work by assignee/state, with pagination. |
| `update_assignment` | Assignee acceptance, decline, progress, blockers/input needs; owner cancellation. |
| `submit_result` | Publish the assignee's structured outcome/evidence and notify the orchestrator atomically. |

Read [participants and assignments](coordination.md) for actual inputs, examples, lifecycle, idempotency, replacement, and reporting. All coordination mutations require a request key. Submitted results remain separate from review, human approval, and overall task completion. These operations record work; they do not schedule it.

Statuses are `pending`, `working`, `needs_input`, `blocked`, `complete`, and `cancelled`. They are records, not process-control commands. Cancelling a task does not interrupt an agent.

Message kinds are `assignment`, `question`, `answer`, `progress`, `finding`, `result`, and `decision`. Reading is separate from acknowledgement, and acknowledgement is separate from completion.

Event kinds are `progress`, `check`, `finding`, `failure`, `decision`, `note`, `stage_started`, and `stage_completed`. Stage start/completion events require a name and positive attempt number. The service adds its own creation, status, message, acknowledgement, and usage events. Clients cannot insert a usage record through the generic event tool.

For retryable creation, messages, and events, preserve `request_key` across retries of the same action. Reusing a key with different content fails. Keys are scoped to project for task creation, sender/task for messages, and actor/task for events and usage. The same measurement must not be resubmitted with a new key, or it will be counted again. A status update uses the optimistic task version instead; after an uncertain result, read the task and timeline before retrying.

## Agent working agreement

For example, ask a connected coordinator:

> Read `$FACTORY_HOME/orchestrator/SKILL.md` and use comms to track a new task in this application: reduce repeated account reads. Keep task documents in the configured runtime directory and application code in the project. Coordinate visible agents, record decisions and progress in comms, and show me the report URL.

The orchestrator creates and retains the task ID, reuses the actual launcher registrations (or registers its own existing instance), creates canonical assignments, and updates overall status. Workers accept/decline, report progress/blockers, and submit structured results using the assignment tools. Agents retrieve messages, acknowledge `read`, explicitly declare `exec` before acting, and publish assignment evidence. Herdr or another existing session control can notify a recipient after the message is stored. The service supplies no agent scheduler. The [native Pi adapter](delivery.md) handles background inbox hints; other runners can implement its MCP contract.

Artifacts remain client-managed files. A decision event can include `data.outcome`, `data.artifacts` with paths and hashes, and a rationale in `message`. Check evidence can include commands, exit status, output references, tool versions, and MCP names. These values are submitted claims; the server does not inspect host files, run checks, validate artifact hashes, or grant approval. Secrets should never be included in event/message bodies; no automatic redaction is performed.

**Factory integration:** team factory members should reference [skills/task-comms](skills/task-comms/SKILL.md) from their configuration; the launcher loads the selected skill and native connection. The TypeScript helpers use the same MCP tools through the [official SDK wrapper](../lib/client.ts). They do not open SQLite or maintain separate task state. Native agent clients remain the preferred interface.

The local `task.json` links a document directory to a service task ID and explicit project. Use `create_task` then the artifact helper's `attach` operation when working through native MCP. The launch approval guard reads comms decision events and checks current local HTML hashes. Old file-only task records are not silently imported. See [coordinator operations](../references/operations.md).

The launcher defaults to the supplied endpoint; `FACTORY_COMMS_URL` selects another endpoint for both helpers and prepared profiles. `COMMS_AUTH_TOKEN` is shared through an environment reference. Keep these values consistent across sessions; role-specific environment remapping cannot override task identity or service settings. Other runners and already-open sessions need their own native connection/reload. No service or agent is started merely by reading a skill.

## Usage and reports

The `usage` object requires `provider`, `model`, and `source` (`runtime`, `provider`, or `agent_report`). Optional measurements are `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`, and `cost_usd`.

Normalize input tokens to include cached input. Cache read/write counts are subsets, not values added to the total. Output includes reasoning tokens when already included by the provider; do not add them twice. Omit unknown measurements rather than supplying zero. One `request_key` identifies one model-request measurement, not a cumulative session total. If a provider only reports cumulative usage, its collector must avoid counting overlapping totals.

The service does not intercept agent sessions or provider calls. Reliable automatic usage requires a runtime hook; none is bundled. The explicit Pi importer below can submit measured assistant usage from a supported session file. Declared provenance is displayed but not independently verified.

The HTML report includes task participant registrations, assignment ownership/state/result evidence, failed-result counts, current task state, recorded failures and messages, observed stage order, stage/attempt token graphs, models, cost, coverage, and structured evidence. It updates when the URL is fetched; it does not push changes into an open page. Save it for a point-in-time snapshot. With no stage or usage records, those sections explicitly show that information is missing.

Input/output/cost cards sum available values and show how many usage records supplied them. A combined token total is available only when every recorded usage request has both input and output counts. Coverage describes submitted records, not completeness of the entire real session. Stage durations use server receipt timestamps of start/completion events, not a profiler. Recorded errors remain visible even if a later attempt succeeds.

Totals cover the entire task. The browser timeline shows at most the latest 500 events and 200 messages, with displayed/total counts; pagination tools provide older records. Large raw terminal logs belong in separate artifacts, with references in events. The server's own operational JSON logs go to stderr and are available through Compose logs.

## Configuration and development

| Setting | Default | Meaning |
| --- | --- | --- |
| `COMMS_ADDR` / `-addr` | `127.0.0.1:8080` in the binary, `:8080` in Compose | HTTP listener. |
| `COMMS_DB_PATH` / `-db` | `/data/state.db` | Container-local database path. |
| `COMMS_AUTH_TOKEN` | Empty | Optional shared bearer token. |
| `COMMS_PORT` | `8090` | Compose's loopback host port. |
| `COMMS_ADMIN_PORT` | `8091` | Optional SQLite Web panel loopback port. |
| `FACTORY_COMMS_URL` | `http://127.0.0.1:8090/mcp` | Client/launcher endpoint; does not configure the server listener. |

Go flags override the corresponding environment setting. The `health` argument checks the listener's `/healthz` endpoint and is used by Compose. No application repository mount is required. Schema 8 is initialized on startup. Existing databases are upgraded transactionally without discarding evidence. A newer unsupported schema fails rather than being silently rewritten.

From this directory, with Go 1.26+ installed:

```bash
go test -race ./...
go vet ./...
go run ./cmd/comms -db ./local-state.db
```

Tests use temporary databases and the real MCP Go client over localhost HTTP. They cover concurrent writes, retry keys, project/task boundaries, acknowledgement semantics, status-version conflicts, database reopening, usage coverage, escaped HTML, and optional bearer access. Container validation additionally checks non-root writes, health, and shared-volume persistence, explicit reset, and live read-only admin access. They do not test model behavior or automatically capture usage.

For the surrounding agent setup, see the [Factory README](../../README.md).

## Optional host delivery

The optional [Herdr fallback bridge](herdr-delivery.md) wakes eligible idle Herdr recipients through recorded session bindings. It uses the official Go MCP client, keeps notification attempts in this service's SQLite database, and leaves assignment decisions to agents. The report includes delivery state and failures. Manual delivery remains supported. Rebuild the comms image when updating to these tools/schema; source changes alone do not update an existing container.

Bridge operations: `claim_delivery`, `finish_delivery`, `list_deliveries`, and task-owner `retry_delivery`. Notification submission is separate from reading, acceptance, and results. See the delivery reference for reservation, backoff, uncertain outcomes, restart recovery, and live validation.


## Session snapshots and contact

`record_session` publishes one immutable public manifest for an active participant; retry with its original request key. `get_session` reads that snapshot and current contact. Neither operation changes assignments. `record_contact` is a host-adapter observation against the current captured binding, with a server-set 30-second expiry. Contact observations overwrite the latest contact row instead of flooding the event timeline. The report shows snapshots and contact for its displayed participants.

The launcher records instruction/skill paths and hashes, model/provider/thinking, runner, MCP names, preparation time, and native-check status. It excludes credential values and raw profiles. The configuration fingerprint represents public instruction/model/tool selections, not every native option. A retired participant or expired observation reads as `unknown`; keep assignment progress and results separate. These are declared observations, not per-agent authentication or proof of useful work.

Rebuild/restart comms to expose the updated tools. Native adapters use `record_runner_contact` with the exact active instance ID and a manifest declaring `delivery: native`; no terminal binding is required. Its contact also expires after 30 seconds. Existing sessions without a recorded manifest remain unobserved until explicitly relaunched; do not invent a historical snapshot from current source files.

## Live task status

Open `/tasks/<task-id>/status` for a read-only page refreshed every ten seconds. `get_task_status` and `/tasks/<task-id>/status.json` expose the same summary. Existing bearer-token protection applies to all three; use a client/proxy that supplies the authorization header when enabled. The page contains no mutation controls.

Local task attachment publishes only the selected factory's name, owner, and member names/paths through `set_task_factory`. That task snapshot is immutable and contains no native provider configuration. Configured members without instances appear as not started; actual registrations, assignments, handoffs, messages and contact come from comms. The diagram shows actual assignment edges, not an inferred recipe sequence. Older tasks without a snapshot explicitly show membership as unknown. Large views flag bounded assignment/participant data; use MCP pagination for the rest.

## Coordinator inbox

`get_coordinator_inbox` returns open attention items and unresolved delivery failures. `request_attention` supports questions, approvals, reviews, failures, and blockers, with a named recipient, options, and evidence references. Submitted assignment results and blocked/needs-input updates create items transactionally. Dispatch failures link their private recovery receipt.

Only the declared task owner records `resolve_attention`, using the current version and actual explanation. Retried resolutions are idempotent. This does not authenticate a human, approve an artifact, change assignment state, or authorize work. Resolve historical blockers after reconciliation; an assignment progressing again does not silently answer its earlier question. The status page displays this inbox read-only. Existing message acknowledgement remains separate.

## Coordination measurements

`get_coordination_metrics` and the report show offered-to-accepted time, closed blocked/needs-input intervals, first-notification latency, pending assignments, submitted handoffs, failures, decisions and explicitly recorded human interventions. Timing uses server-recorded events across the complete task, not the bounded browser timeline. No samples means unknown. Open blocked intervals are excluded until closed; submitted handoffs are not verified review cycles. Record human feedback once with `record_event` and `data.event_type: human_intervention`.

The Pi adapter provides a repeatable, explicit importer:

```sh
node $FACTORY_HOME/orchestrator/adapters/pi/import_usage.ts <task-directory> \
  --session <private-runtime-instance/session.jsonl> \
  --participant <participant-id> --assignment <assignment-id>
```

It reads Pi v3 session records, verifies the runtime instance and single-assignment attribution, and deduplicates by participant/session/entry ID. It attributes the session's assistant requests, including startup overhead, to that sole assignment. Cache input is included once; reasoning is already in output. It rejects incomplete records and inconsistent counts. It does not import tool/compaction usage, estimate unreported tokens, or treat Pi's locally estimated price as billed cost. Coverage is therefore partial, explicitly reported, and cost remains unknown. Other runtimes use `record_usage` directly with stable request keys.

## Message execution state

`execute_message` takes `task_id`, `message_id`, and `actor` (the declared recipient). It requires a prior read acknowledgement and is idempotent. Returned messages expose `status: sent | read | exec` and an optional `executed_at`. This describes the recipient's declaration, not completed work. See the [delivery contract](delivery.md).

## Terminal monitor

With Comms running, launch from the application repository:

```sh
"$FACTORY_HOME/orchestrator/monitor.sh"
```

The launcher uses Go and the vendored dependencies; it does not install packages or start agents. Alternatively, run `make monitor PROJECT=/path/to/project` from the Factory checkout. For a standalone executable, build from `orchestrator/comms`:

```sh
go build -mod=vendor -o /tmp/factory-monitor ./cmd/monitor
/tmp/factory-monitor -project /path/to/project
```

The endpoint defaults to `http://127.0.0.1:8090/mcp`. Override it with `FACTORY_COMMS_URL` or `-url`. Set `COMMS_AUTH_TOKEN` if the service requires authentication. `-project` selects the exact project identifier recorded in Comms; existing local paths are resolved to absolute paths. The default is the calling directory.

The display refreshes every three seconds. Overview highlights task status, open attention, active assignments, and agent contact. Comms shows observed sender-to-recipient routes and message contents; events shows recorded activity. Route counts represent each message's current **sent**, **read**, or **exec** state, not three cumulative totals. Contact is distinct from work progress; expired contact is unknown. This monitor shows recorded activity; use the workflow resolver for the pinned graph and next step.

| Key | Action |
| --- | --- |
| Arrows / `j`, `k` | Select a task, or scroll the focused detail pane |
| Tab | Switch focus between tasks and details |
| `1`, `2`, `3` | Overview, communications, events |
| Page Up / Page Down / Home | Scroll details / return to top |
| `p` | Cycle recorded projects |
| `n`, `b` | Older task page / newest task page |
| `r`, Space | Refresh now / pause automatic refresh |
| `q`, Ctrl-C | Quit |

Requires at least 65 columns × 18 rows. Task totals apply to the current page of up to 50 tasks. The API bounds detailed records; the monitor identifies partial responses and shows message/event coverage. A failed refresh retains the last successful snapshot and marks it stale. The monitor is read-only and uses the existing HTTP status endpoints; it does not acknowledge messages or change task state. The browser monitor remains available separately.
