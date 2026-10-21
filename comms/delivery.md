# Inbox delivery

Comms stores messages. The recipient's running adapter brings them into its session. Terminal hosts open visible sessions; they do not define the messaging protocol or choose workflow steps.

## Message states

| State | Evidence | Operation |
| --- | --- | --- |
| `sent` | Message committed to SQLite | `send_message`, or an assignment operation |
| `read` | Recipient retrieved and acknowledged the message | `acknowledge_message` |
| `exec` | Recipient explicitly declared it began acting on the request | `execute_message` |

Listing an inbox or displaying a notification does not advance these states. `exec` requires `read` and is idempotent. An informational message may remain read. These records do not prove understanding, execution correctness, approval, or completion. Assignment acceptance, progress, and `submit_result` remain separate.

## Pi adapter

The launcher always includes [delivery.ts](../orchestrator/extensions/delivery.ts) in the private Pi profile. `--check` verifies its `factory-inbox` command is registered. Run `npm ci --omit=dev` in `factory/orchestrator` before launching, and rebuild the comms service after updating its source.

A task-bound launch registers its real process and session snapshot. The extension binds that task automatically. For a coordinator started without a task, create/attach the task first, then call `factory_bind_task`. The task must belong to the configured project/factory. Each process binds one task; use a separate process for another task.

The extension polls every three seconds, outside model turns, and records native contact with a 30-second expiry. While Pi is busy it keeps contact current but does not queue another inbox hint. When idle, it queues a visible follow-up containing message IDs, using Pi's native extension API. This can start a model turn. The model retrieves content with `factory_read_message`, and calls `factory_exec_message` before acting. `/factory-inbox` requests an immediate check.

Service failures keep messages in SQLite and show a disconnected/retrying status. Binding is retried too. Hints are deduplicated within a running adapter; reconnecting can repeat a hint. After a crash, inspect read messages and canonical assignments as well as unread messages—an acknowledgement can have succeeded before its tool response arrived. There is no exactly-once execution guarantee. Use message/assignment IDs and idempotent request keys, and never repeat completed work just because a hint reappears.

The extension cannot wake a stopped process. A heartbeat is contact evidence, not work progress. The launcher does not automatically replace a retired instance or transfer its assignments. Restart/replacement remains an explicit host/coordinator action. Pi sessions do not need a Herdr binding or the Go bridge.

## Terminal hosts

Set `FACTORY_TERMINAL_HOST=herdr`, `terminal`, or `iterm`; alternatively set `terminal_host` in `factory.json`. The environment wins. Automatic selection uses Herdr inside a genuine Herdr pane, otherwise Terminal on macOS.

Herdr opens a split; Terminal/iTerm open a macOS `.command` document in a visible window. Those two adapters currently support macOS only. Command files contain argv and project paths, not copied credentials. Ensure provider credentials, custom comms URL/token, proxy and CA settings are available in the new terminal's shell environment. Exporting them only in the coordinator process does not configure an already running terminal application. Manual startup remains available on other hosts.

A closed window is not an assignment result. See [dispatch](../orchestrator/references/dispatch.md) for recovery without duplicate launches.

## Implementing another runner

Use the official MCP SDK for your runtime and the same comms endpoint/token. No Pi or Herdr API is required:

1. Receive an explicit task, project, role and stable **process instance ID**. Register that actual process using `register_participant`; preserve the returned participant and routing identity. Do not register a future process in advance.
2. Publish `record_session` once with `delivery: "native"`, runner name, public configuration fingerprint, skills/MCP names, prepared timestamp and check status. Keep credentials out. Retry the same snapshot using the same request key.
3. While running, call `record_runner_contact` with `task_id`, `participant_id`, `instance_id`, and `idle`, `working`, `blocked`, or `unavailable`. Contact expires after 30 seconds. The instance must match an active participant. A five-second heartbeat is adequate.
4. Poll all `read_inbox` pages for the exact routing identity. While idle, enqueue an inbox hint using the runner's native API. Preserve task/message IDs. Never silently interrupt active work or mark the hint read merely because it was queued.
5. Provide an explicit read operation that retrieves addressed content and calls `acknowledge_message`. Provide an explicit begin-action operation using `execute_message`. Reconcile assignments before work and use normal assignment/result tools for its lifecycle.
6. Reconnect on outages; tolerate repeated notifications and uncertain writes. Stop polling on shutdown, mark contact unavailable where possible, and close connections. Never transfer an old instance's work without coordinator reconciliation.

Factory's `command` runner supplies instructions and identity but defaults to manual delivery. A native adapter must own its registration/snapshot/contact integration; set `delivery: "native"` in the command runner's configuration only after implementing those operations. This is an integration contract, not a plugin registry.

The server validates declared identities and transitions. The optional shared bearer token protects the service connection, **not individual agent identities**. An agent with the service token is not cryptographically restricted to its own actor name. Native-contact checks prevent accidental stale-instance use; they are not per-agent authentication.

## Optional terminal fallback

The [Herdr bridge](herdr-delivery.md) remains available for manually integrated runners without a native adapter. Do not enable both paths for one recipient. Its submitted/uncertain delivery ledger describes terminal input attempts and is separate from message `sent/read/exec`. Live Herdr and Terminal/iTerm acceptance is still a separate operator check; automated fixtures do not establish it.
