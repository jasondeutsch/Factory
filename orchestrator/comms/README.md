# Orchestrator MCP server

The orchestrator's Go MCP server (`comms`) provides shared task records, participants, assignments, agent inboxes, structured evidence, and reports. It uses the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) and the pure-Go [modernc SQLite driver](https://pkg.go.dev/modernc.org/sqlite). Its Go module lives in this directory.

SQLite is embedded in the comms process. One Compose file starts **two containers**: the MCP server and a read-only SQLite Web admin panel. Both use the same named data volume. No separate database service is needed. Agents connect over Streamable HTTP; they do not open the database file.

## Start with Docker Compose

From `factory/orchestrator/comms`, or `.factory/orchestrator/comms` in an installed project:

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
| `http://127.0.0.1:8090/tasks/<task-id>/report.html` | Current task report, generated on each request. |

Set `COMMS_PORT` before starting Compose to use another host port; set `FACTORY_COMMS_URL` in the coordinating/worker environment to the matching full `/mcp` URL. The default port mapping binds to loopback. The image runs as UID/GID `65532`, contains a static Go executable, and needs no separate SQLite installation, Python, Node, or shell at runtime. Building requires access to the Go builder image and module downloads. `GO_IMAGE` is an overridable Docker build argument if the team supplies a compatible builder image with its corporate trust configuration.

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

Every supplied role already includes this connection in its native Pi `mcp.json`. An existing coordinator or another runner can configure the equivalent connection:

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

Every operation uses an explicit task ID; project identifiers are supplied strings, not paths that the container tries to discover. Task names can contain spaces and uppercase letters. Task IDs include a UTC timestamp and random suffix.

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

The service also exposes seven structured coordination tools:

| Tool | Purpose |
| --- | --- |
| `register_participant` | Register a declared task instance and inbox routing identity, or explicitly replace an old instance. |
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

> Read `.factory/orchestrator/SKILL.md` and use comms to track a new task in this application: reduce repeated account reads. Keep the spec, plan, and review artifacts in the repository. Coordinate visible agents, record decisions and progress in comms, and show me the report URL.

The orchestrator creates and retains the task ID, registers itself and workers, creates canonical assignments, and updates overall status. Workers accept/decline, report progress/blockers, and submit structured results using the assignment tools. Agents read their inbox at turn/iteration boundaries, acknowledge messages, and publish evidence. Herdr or another existing session control can notify a recipient after the message is stored. The service supplies no agent scheduler. The optional [host bridge](delivery.md) provides background inbox notification; it must actually be running.

Artifacts remain client-managed files. A decision event can include `data.outcome`, `data.artifacts` with paths and hashes, and a rationale in `message`. Check evidence can include commands, exit status, output references, tool versions, and MCP names. These values are submitted claims; the server does not inspect host files, run checks, validate artifact hashes, or grant approval. Secrets should never be included in event/message bodies; no automatic redaction is performed.

**Factory integration:** all six roles reference [skills/task-comms](skills/task-comms/SKILL.md) from their configuration; the launcher loads that skill and a native connection. The Python task helpers use the same MCP tools through [client.py](client.py), a narrow client for this service's stateless JSON transport. They do not open SQLite or maintain separate task state. Native agent clients remain the preferred interface.

The local `task.json` links a document directory to a service task ID and explicit project. Use `create_task` then the artifact helper's `attach` operation when working through native MCP. The launch approval guard reads comms decision events and checks current local HTML hashes. Old file-only task records are not silently imported. See [coordinator operations](../references/operations.md).

The launcher defaults to the supplied endpoint; `FACTORY_COMMS_URL` selects another endpoint for both helpers and prepared profiles. `COMMS_AUTH_TOKEN` is shared through an environment reference. Keep these values consistent across sessions; role-specific environment remapping cannot override task identity or service settings. Other runners and already-open sessions need their own native connection/reload. No service or agent is started merely by reading a skill.

## Usage and reports

The `usage` object requires `provider`, `model`, and `source` (`runtime`, `provider`, or `agent_report`). Optional measurements are `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`, and `cost_usd`.

Normalize input tokens to include cached input. Cache read/write counts are subsets, not values added to the total. Output includes reasoning tokens when already included by the provider; do not add them twice. Omit unknown measurements rather than supplying zero. One `request_key` identifies one model-request measurement, not a cumulative session total. If a provider only reports cumulative usage, its collector must avoid counting overlapping totals.

The service does not intercept agent sessions or provider calls. Reliable automatic usage requires a runtime hook or importer that submits these records; none is bundled yet. Declared provenance is displayed but not independently verified.

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

Go flags override the corresponding environment setting. The `health` argument checks the listener's `/healthz` endpoint and is used by Compose. No application repository mount is required. Schema 2 is initialized on startup. Existing schema-1 tasks/messages/events are upgraded transactionally without discarding evidence; legacy prose messages do not automatically become canonical assignments. A newer unsupported schema fails rather than being silently rewritten.

From this directory, with Go 1.26+ installed:

```bash
go test -race ./...
go vet ./...
go run ./cmd/comms -db ./local-state.db
```

Tests use temporary databases and the real MCP Go client over localhost HTTP. They cover concurrent writes, retry keys, project/task boundaries, acknowledgement semantics, status-version conflicts, database reopening, usage coverage, escaped HTML, and optional bearer access. Container validation additionally checks non-root writes, health, and shared-volume persistence, explicit reset, and live read-only admin access. They do not test model behavior or automatically capture usage.

For the surrounding agent setup, see the [Factory README](../../README.md).

## Optional host delivery

The [delivery bridge](delivery.md) wakes eligible idle Herdr recipients through recorded session bindings. It uses the official Go MCP client, keeps notification attempts in this service's SQLite database, and leaves assignment decisions to agents. The report includes delivery state and failures. Manual delivery remains supported. Rebuild the comms image when updating to these tools/schema; source changes alone do not update an existing container.

Bridge operations: `claim_delivery`, `finish_delivery`, `list_deliveries`, and task-owner `retry_delivery`. Notification submission is separate from reading, acceptance, and results. See the delivery reference for reservation, backoff, uncertain outcomes, restart recovery, and live validation.
