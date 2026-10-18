# Inbox delivery

The optional host bridge polls comms through the official Go MCP client and submits inbox hints to idle Herdr agents. It does not use a model to poll, choose work, acknowledge messages, accept assignments, approve artifacts, or collect results. Comms remains usable with any agent runner and without the bridge.

## Setup through the coordinator

Ask your coordinating agent:

> Enable automatic inbox delivery for this task's existing Herdr agents. Capture their actual session bindings, register those references, and start the delivery bridge in a separate visible pane. Keep human approvals with me.

The coordinator must itself run inside Herdr (`HERDR_ENV=1`). This is a host process, not another Compose service: it needs the local Herdr CLI/socket and Go 1.26 or a locally built `cmd/bridge` binary. Comms must already be running with the updated service image. Restart/rebuild that image after updating the source; do not discard a volume containing needed history.

Before participant registration, name and inspect each actual agent using the [Herdr adapter](../orchestrator/adapters/herdr/README.md). Capture its reference from inside Herdr:

```bash
.factory/orchestrator/start-delivery.sh --bind account-architect
```

This prints one JSON object with `adapter`, `workspace_id`, `pane_id`, `terminal_id`, and `name`. Pass the **serialized object as the `session_ref` string** in `register_participant`; retain the agent's own `FACTORY_INSTANCE_ID`. Record the coordinator's binding too, so replies can wake it. Never copy example IDs or register the bridge as the worker. An empty reference intentionally selects manual delivery. A nonempty legacy plain name is insufficient and produces a visible delivery error.

Bindings are immutable participant registration data. For a participant already registered without a binding, prepare an explicit replacement with the correct instance/session identity; do not change an idempotency key to silently mutate registration. Existing assignments remain bound to the old participant: cancel/reissue them explicitly if needed. Prefer capturing bindings before creating assignments. This bridge does not automate replacement or ownership transfer.

Start in a dedicated Herdr pane, with the **exact project identifier stored in comms**, usually the application root:

```bash
.factory/orchestrator/start-delivery.sh --project "$PWD"
# Or limit the watch to one known task ID:
.factory/orchestrator/start-delivery.sh --project "$PWD" --task '<service-task-id>'
```

Use the coordinator's host controls to open this pane; do not block its conversational shell tool with a foreground daemon. The bridge stays visible; stop it with Ctrl-C. `FACTORY_COMMS_URL` and optional `COMMS_AUTH_TOKEN` select the connection; the key never belongs in command arguments. `--url`, `--interval 3s`, and `--once` support diagnostics. Do not set `HERDR_ENV=1` outside Herdr to bypass its boundary. Multiple bridges are unnecessary; service-side claims nevertheless prevent simultaneous sends to the same participant.

## Delivery and recovery

Each cycle pages project tasks, active participants, and unread inboxes. It sends at most one hint per recipient per cycle. Completed tasks remain eligible so final replies are not dropped. Empty inboxes make no Herdr or model calls. The hint contains task/message identifiers, not the stored message body; agents fetch canonical context through MCP and follow the shared comms skill.

Only `idle` and `done` sessions are eligible. Working, blocked, unknown, launching, and explicitly unready sessions wait without interruption or automatic dialog answers. Session bindings must match the recorded workspace, pane, terminal, and live name. Missing/replaced/moved sessions produce concrete errors. Herdr's named session contract and CLI response schema were checked against version 0.9.3; see the [official socket/CLI reference](https://herdr.dev/docs/socket-api/).

Delivery states are independent of inbox reading and assignment states:

| State | Meaning / next action |
| --- | --- |
| `claimed` | One bridge holds a 30-second reservation. Host calls each have an 8-second timeout. |
| `submitted` | Herdr confirmed `agent_prompted`. Reading, acceptance, and work are still unconfirmed. No automatic reminder is sent. |
| `retry` | No terminal input was attempted; retry after persisted backoff (30, 60, 120, 240 seconds). |
| `failed` | Five safe attempts failed, or the bridge explicitly recorded failure. Inspect configuration/session. |
| `uncertain` | Input may have happened, or a bridge claim expired after a crash. Inspect before any retry. |

`list_deliveries` and `get_task_report` expose the latest 200 records; report state totals cover the entire task. Events retain delivery history. The first recipient delivery failure also creates one finding in the owner's inbox. Subsequent failures do not flood that inbox. Failures delivering to the owner appear in events/report/logs without recursively generating another owner alert. Original messages remain recoverable and unacknowledged.

When a `claimed` reservation expires, the next claim check for that recipient marks it `uncertain`; no replacement notification is sent for it. A stopped bridge cannot itself publish a failure while offline. Recover the bridge/service, then inspect the records and pane. Notifications do not reassign work, including after participant replacement; old assignment offers are not delivered as fresh work to the replacement.

After inspecting the actual pane and unread message, the declared task owner can use `retry_delivery` with `task_id`, `message_id`, `participant_id`, `actor_participant_id`, and a stable `request_key`. This explicitly resets `failed`, `uncertain`, or `submitted` notification state. It never creates another assignment, acknowledges reading, or transfers ownership. For a successful input whose completion response was lost, `finish_delivery` with the same lease and payload is idempotent; stale leases are rejected. Bridge-only tools are `claim_delivery` and `finish_delivery`; agents normally need only `list_deliveries` and owner-authorized recovery.

The bridge reconnects to comms with capped backoff and preserves its ledger in the service's SQLite database. There is no host delivery database. Restart suppression survives bridge restarts, and service restarts when the selected Compose storage survives. Default container data remains ephemeral; deleting that container without a volume deletes the ledger along with task history.

## Boundaries

Herdr's state inspection and prompt submission are separate operations. The bridge rechecks readiness just before submission, but cannot atomically pin an agent process and reserve its input through the CLI. Herdr rejects blocked dialogs, but busy/readiness races remain possible. Do not manually replace, rename, move, or feed the same session while a notification is being submitted. If the same name is deliberately reused in the same terminal, the current binding cannot prove that it is the original process. Strong process identity/contact monitoring remains FACT-003.

A timeout or malformed prompt response is conservatively `uncertain`; there is no blind resend, terminal-key fallback, guaranteed interruption, or exactly-once terminal delivery. The durable lease prevents competing bridge sends, while explicit recovery handles the send/record crash window. Agents must still reconcile canonical assignment state to avoid duplicate work. Long-running agents check inboxes at their normal work boundaries; the bridge does not interrupt them.

## Verification

Automated Go tests exercise the actual MCP HTTP transport, store, assignment/reply path, restart suppression, exclusive claims, backoff, expired/stale leases, failure alerts, reports, schema upgrades, and a separate Herdr executable fixture. They do not control your live panes or prove real Herdr wake-up.

For a live smoke test **inside Herdr**, use dedicated coordinator and worker sessions connected to an isolated task:

1. Capture and register both bindings; start the bridge for that task only.
2. With both agents idle, have the coordinator create one small assignment through MCP. Confirm the worker receives a hint, reads/acknowledges it, and explicitly accepts.
3. Have the worker submit its result. Confirm a notification reaches the idle coordinator and it reads the canonical result. No human inbox prompt should be needed.
4. Restart the bridge. Confirm no duplicate input or assignment appears. Confirm submitted/read/accepted/result are separate facts in the report.
5. Make the worker busy or blocked: it must receive no input until ready. Close a dedicated recipient: confirm failure visibility and intact unread messages. Inspect before an explicit retry. Do not run destructive session tests against active work.
