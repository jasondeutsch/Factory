# Factory comms

A small Go MCP server for shared task records, agent inboxes, structured evidence, and reports. It uses the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) and the pure-Go [modernc SQLite driver](https://pkg.go.dev/modernc.org/sqlite). Its Go module lives in this directory.

SQLite is embedded in the comms process. It is not a database service, so the base Compose setup starts **one container**, containing the server and its SQLite dependency. The optional admin override adds a separate browser UI container. Agents connect over Streamable HTTP; they do not open the database file.

## Start with Docker Compose

From `factory/comms`, or `.factory/comms` in an installed project:

```bash
docker compose up --build -d
docker compose ps
docker compose logs comms
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
docker compose down
```

## Container-local data by default

The database is `/data/state.db` **inside the container**. The default Compose file declares no volume, bind mount, or tmpfs. It makes no directory under the host's home directory.

Data survives a stop/start or restart of the same container. Removing/recreating the container loses its writable layer and therefore the records. `docker compose down` removes the container; a later `up` starts with a fresh database unless persistence was explicitly configured.

For optional persistence, the supplied override mounts a named volume at `/data`:

```bash
docker compose -f compose.yaml -f compose.persist.yaml up --build -d
docker compose -f compose.yaml -f compose.persist.yaml down
```

The named volume survives `down`; `down -v` deliberately deletes it. Use the same Compose project name and override when restarting. A team can instead define its own volume or bind mount at `/data`. A host bind directory must be writable by container UID/GID `65532`.

Persist or back up the **whole database directory**, including SQLite WAL/SHM files when present. Do not copy only the main database file while it is being written; stop comms before a simple filesystem backup. One comms service owns each database; this prototype does not support replicas sharing a database or a database on a network filesystem.

## SQLite admin panel

The optional [SQLite Web](https://github.com/coleifer/sqlite-web) panel uses a pinned upstream image and browses tables, schema, rows, SQL query results, and exports. Start it with the admin override:

```bash
docker compose -f compose.yaml -f compose.admin.yaml up --build -d
```

Open **http://127.0.0.1:8091/**. Set `COMMS_ADMIN_PORT` for another port. The MCP service remains on port 8090. The panel is bound to loopback, runs as the same non-root UID as comms, and opens the database read-only. Make task changes through MCP so service validation and event recording remain intact.

The override explicitly mounts the named `comms-data` volume into comms and read-only into the panel. Both see the live database and WAL/SHM files. It uses the same volume as `compose.persist.yaml`; the admin override already includes that storage configuration, so the persist override is unnecessary. The base Compose file still starts only comms with container-local ephemeral storage.

Use the same override to stop:

```bash
docker compose -f compose.yaml -f compose.admin.yaml down
```

The volume survives `down`; `down -v` deletes it. Enabling this override on an existing **unmounted** container does not migrate its database: preserve the database before recreating that container if its records matter. An existing deployment using `compose.persist.yaml` can use the admin override with the same Compose project name and volume.

The panel has its own HTTP interface; `COMMS_AUTH_TOKEN` protects MCP/reports, not the panel. Its supplied exposure is localhost-only. No panel or service is started merely by adding these files.

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
| `send_message` | `task_id`, `sender`, `recipient`, `kind`, `text` | Stores the message and an event atomically. Optional `request_key` deduplicates retries. Does not wake the recipient. |
| `read_inbox` | `task_id`, `recipient` | Unacknowledged messages, oldest first. Default 50, maximum 200. Optional `include_acknowledged`; use the last message's `id` as `after` for the next page. |
| `acknowledge_message` | `task_id`, `message_id`, `actor` | Declared recipient marks the message read. Repeating it preserves the first acknowledgement timestamp. |
| `record_event` | `task_id`, `actor`, `kind`, `message` | Appends evidence, with optional `stage`, `attempt`, structured `data`, and `request_key`. |
| `list_events` | `task_id` | Oldest first. Default 100, maximum 500. Use the last event's `sequence` as `after` for the next page. |
| `record_usage` | `task_id`, `actor`, `stage`, `attempt`, `request_key`, `usage` | Records one model-request measurement with declared provider/model and provenance. |
| `get_task_report` | `task_id` | Consistent structured totals, stage attempts, models, coverage, and a relative report URL. Avoids putting the full HTML in model context. |
| `render_report` | `task_id` | Returns self-contained HTML and the report path. The caller can save the HTML; the server writes no report file. |

Statuses are `pending`, `working`, `needs_input`, `blocked`, `complete`, and `cancelled`. They are records, not process-control commands. Cancelling a task does not interrupt an agent.

Message kinds are `assignment`, `question`, `answer`, `progress`, `finding`, `result`, and `decision`. Reading is separate from acknowledgement, and acknowledgement is separate from completion.

Event kinds are `progress`, `check`, `finding`, `failure`, `decision`, `note`, `stage_started`, and `stage_completed`. Stage start/completion events require a name and positive attempt number. The service adds its own creation, status, message, acknowledgement, and usage events. Clients cannot insert a usage record through the generic event tool.

For retryable creation, messages, and events, preserve `request_key` across retries of the same action. Reusing a key with different content fails. Keys are scoped to project for task creation, sender/task for messages, and actor/task for events and usage. The same measurement must not be resubmitted with a new key, or it will be counted again. A status update uses the optimistic task version instead; after an uncertain result, read the task and timeline before retrying.

## Agent working agreement

For example, ask a connected coordinator:

> Read `.factory/orchestrator/SKILL.md` and use comms to track a new task in this application: reduce repeated account reads. Keep the spec, plan, and review artifacts in the repository. Coordinate visible agents, record decisions and progress in comms, and show me the report URL.

The coordinator creates and retains the task ID, sends explicit assignments, and updates status. Agents read their inbox at turn/iteration boundaries, acknowledge messages, and publish evidence. Herdr or another existing session control can notify a recipient after the message is stored. The service supplies no automatic dispatcher or background agent scheduler.

Artifacts remain client-managed files. A decision event can include `data.outcome`, `data.artifacts` with paths and hashes, and a rationale in `message`. Check evidence can include commands, exit status, output references, tool versions, and MCP names. These values are submitted claims; the server does not inspect host files, run checks, validate artifact hashes, or grant approval. Secrets should never be included in event/message bodies; no automatic redaction is performed.

**Factory integration:** all six roles reference [skills/task-comms](skills/task-comms/SKILL.md) from their configuration; the launcher loads that skill and a native connection. The Python task helpers use the same MCP tools through [client.py](client.py), a narrow client for this service's stateless JSON transport. They do not open SQLite or maintain separate task state. Native agent clients remain the preferred interface.

The local `task.json` links a document directory to a service task ID and explicit project. Use `create_task` then the artifact helper's `attach` operation when working through native MCP. The launch approval guard reads comms decision events and checks current local HTML hashes. Old file-only task records are not silently imported. See [coordinator operations](../orchestrator/references/operations.md).

The launcher defaults to the supplied endpoint; `FACTORY_COMMS_URL` selects another endpoint for both helpers and prepared profiles. `COMMS_AUTH_TOKEN` is shared through an environment reference. Keep these values consistent across sessions; role-specific environment remapping cannot override task identity or service settings. Other runners and already-open sessions need their own native connection/reload. No service or agent is started merely by reading a skill.

## Usage and reports

The `usage` object requires `provider`, `model`, and `source` (`runtime`, `provider`, or `agent_report`). Optional measurements are `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`, and `cost_usd`.

Normalize input tokens to include cached input. Cache read/write counts are subsets, not values added to the total. Output includes reasoning tokens when already included by the provider; do not add them twice. Omit unknown measurements rather than supplying zero. One `request_key` identifies one model-request measurement, not a cumulative session total. If a provider only reports cumulative usage, its collector must avoid counting overlapping totals.

The service does not intercept agent sessions or provider calls. Reliable automatic usage requires a runtime hook or importer that submits these records; none is bundled yet. Declared provenance is displayed but not independently verified.

The HTML report includes current task state, recorded failures and messages, observed stage order, stage/attempt token graphs, models, cost, coverage, and structured evidence. It updates when the URL is fetched; it does not push changes into an open page. Save it for a point-in-time snapshot. With no stage or usage records, those sections explicitly show that information is missing.

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

Go flags override the corresponding environment setting. The `health` argument checks the listener's `/healthz` endpoint and is used by Compose. No application repository mount is required. Database schema version 1 is initialized on startup; a newer unsupported schema fails rather than being silently rewritten.

From this directory, with Go 1.26+ installed:

```bash
go test -race ./...
go vet ./...
go run ./cmd/comms -db ./local-state.db
```

Tests use temporary databases and the real MCP Go client over localhost HTTP. They cover concurrent writes, retry keys, project/task boundaries, acknowledgement semantics, status-version conflicts, database reopening, usage coverage, escaped HTML, and optional bearer access. Container validation additionally checks non-root writes, health, and ephemeral versus explicitly mounted data. They do not test model behavior or automatically capture usage.

For the surrounding agent setup, see the [Factory README](../README.md).
