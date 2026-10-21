# Pi inbox delivery extension

`delivery.ts` connects a running Pi session to Factory Comms. It lets a session receive inbox notifications and explicitly record when it reads a message and begins acting on it. The extension is a Pi adapter; Comms owns task and message state, while the coordinator owns assignments, workflow decisions, and approvals.

## What it does

- Binds the Pi process to one explicit Factory task and its registered participant.
- Records whether the session is idle, working, or unavailable. Contact expires, so a stale process is not shown as live indefinitely.
- Polls the bound participant's inbox every three seconds while the process is running. It queues a visible Pi follow-up when new messages arrive and Pi is idle.
- Provides `factory_read_message` and `factory_exec_message` tools so message state advances only when the agent performs those explicit actions.
- Reconnects after Comms errors and leaves messages in Comms for recovery.
- For a workflow member with a bounded loop, checks the task and the process's original assignment before allowing action tools. A submitted, blocked, or cancelled assignment cannot silently turn into a new repair batch.

The extension sends message IDs in a notification, not message contents. A notification does not count as reading, accepting an assignment, executing the requested work, or completing it.

## How it is loaded

Factory's session launcher creates a private Pi profile and includes this extension automatically. The launcher also starts Pi with task, participant, factory, project, session-manifest, Comms URL, and optional bearer-token environment values. Do not add a second copy of the extension to a team's factory configuration or edit the generated Pi profile; change the framework launcher/profile code instead.

The toolkit needs its production dependencies installed, and the Comms service must be reachable. The default endpoint is `http://127.0.0.1:8090/mcp`; set `FACTORY_COMMS_URL` to use another Streamable HTTP MCP endpoint. If the service requires bearer authentication, set `COMMS_AUTH_TOKEN` in the launch environment. The token is not placed in agent instructions.

## Binding a task

A task-bound launch binds automatically. For a coordinator session launched before a task exists:

1. Create and attach the task to the selected project and factory through the normal coordination tools.
2. Call `factory_bind_task` with the Comms task ID.

The extension verifies the task's project and selected factory, finds or registers this process instance, records its session manifest, then binds it. A Pi process can bind to only one task; launch another process for another task. If binding fails, the extension shows a retrying status and retries on its next poll.

## Message handling

When idle, the extension places a follow-up in Pi containing the pending message IDs. The agent should:

1. Call `factory_read_message` with an ID to retrieve the addressed message and record `read`.
2. Check its assignment, role, and approval boundaries.
3. Immediately before beginning the requested action, call `factory_exec_message` to record `exec`.
4. Use the normal assignment and result operations to report progress and submit work.

`exec` requires `read`. Neither operation grants approval or changes assignment ownership/status. Reading a message does not mean the requested action is authorized; follow the workflow's human-approval rules.

Use `/factory-inbox` to request an immediate poll. The extension also checks after agent turns and on its three-second timer.

## Recovery and limits

The extension cannot wake a stopped Pi process, interrupt an active turn, or guarantee exactly-once message execution. Notifications are deduplicated only within the current process, so a restart may show an already-seen hint again. Comms message and assignment IDs are the durable record; reconcile them before repeating work. If the Comms service is unavailable, messages remain stored and the Pi status shows that it is retrying.

The assignment guard blocks Pi tool calls that could perform work after the bound assignment ends; it still allows built-in read-only inspection and the task-binding/read-message tools. It does not sandbox other processes or prevent a text-only model response. Other runners need their own equivalent guard.

Contact is a short-lived observation, not evidence that the agent is making progress. `read` and `exec` are agent declarations, not proof of comprehension or correct work. Acknowledgement can succeed even if its response is lost, so after an uncertain tool call inspect the inbox state before trying again.

This code depends on Pi's extension API and is not part of the agent-neutral coordination contract. Another runner should implement the same registration, session snapshot, contact, inbox notification, read, and exec behavior using its native APIs. See [the runner delivery contract](../comms/delivery.md#implementing-another-runner).

## Verification

The framework tests exercise inbox state transitions, recipient isolation, retry/reconnect behavior, retired-instance rejection, overlapping polls, and extension tool registration. Run the framework's TypeScript check and tests from the Factory repository; a real idle-Pi notification check remains a separate runtime acceptance test.
