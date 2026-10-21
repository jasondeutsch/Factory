# Visible sessions with Herdr

Pi uses native inbox delivery. Use [dispatch](../../references/dispatch.md) for visible panes. The binding and notification operations below are optional fallbacks for runners without a native adapter; do not add them to Pi sessions.

This reference is for the coordinating agent. For coordinated factory work, open or reuse visible role sessions as needed; handle pane controls and helper calls on the user's behalf. The default is ordinary interactive sessions, each in a pane. Herdr pane controls themselves need no extension or MCP server; Factory task coordination still requires Comms. Only control Herdr from a Herdr-managed pane (`HERDR_ENV=1`). Outside Herdr, explain that the user must open their coordinating session there first; do not operate another focused session implicitly.

For normal coordinator dispatch, use the [single dispatch operation](../../references/dispatch.md). The manual controls below remain useful for inspection and explicit recovery.

## Manual panes

The user can open role panes themselves if preferred. Each agent can load its role instructions and the relevant skills, recipe, and task context in an existing session. Use the [operations reference](../../references/operations.md) for new configured sessions. There is no automatic message watcher or completion push in manual mode. The optional [host delivery bridge](../../../comms/delivery.md) provides inbox notifications without a runner extension.

## Named sessions and notifications

For discoverable names, name the agents already running in your panes. The installed CLI is the authority for syntax: run `herdr agent` and `herdr pane` to inspect it. With the inspected 0.9.3 CLI:

1. Reuse an available role session, or open a pane and start `session.ts <role> <task-path>` there. The launcher applies that role's configuration and starts the runner in the project root.
2. From the coordinator's Herdr pane, run `herdr agent list`. Read the actual pane IDs and identify the role sessions. Do not invent IDs from examples.
3. Name a session with `herdr agent rename <actual-pane-id> account-architect`. A name must be unique among live agents.

When a required role needs a new visible pane, use `herdr pane split --current --direction right --cwd "$PWD" --no-focus` from the project root. Use `down` when that gives more usable space and read `.result.pane.pane_id` from the response. Start the session with `herdr pane run <returned-id> <shell-quoted-launch-command>`, where the command runs `node $FACTORY_HOME/orchestrator/session.ts <role> <actual-task-path>` from that project root. Quote each launcher argument and then the command as required by your shell tool; do not splice arbitrary user text into it. Do not start a second instance in an occupied pane or run the interactive launcher in the coordinator's own blocking shell tool.

Inspect the pane to establish whether startup succeeded before assigning work. The launcher first prepares the native profile and checks credential readiness and enabled MCP connections. Required environment references must exist in that pane; do not assume it matches every coordinator subprocess. If startup fails, inspect the task event and private diagnostics, explain the actual error without credentials, and request only the needed correction. Never claim an agent is ready merely because its command was submitted or preflight passed.

For a coder loop, add `--loop 5` to its pane launch. After checks, the launcher registers its real instance and waits up to `check_timeout_seconds` for an active assignment. Discover that participant and create the approved assignment during this wait; then the native command starts the loop. Do not pre-register a placeholder to assign work before launch. Confirm startup and actual progress afterward. Keep this ordering distinct from an ordinary waiting worker, and follow the [loop configuration](../../../docs/configuration.md). The loop remains in the same visible pane while Pi replaces its sessions.

Name live sessions distinctly, such as `account-architect` and `account-coordinator`. Record the mapping in the task with `task.ts note`. Herdr names have their own syntax and length rules; they are not the task's display name.

Create canonical work with comms `create_assignment` using the registered participant IDs. It saves the assignment notification and event atomically; do not create another assignment message with a shell helper. For a runner using the terminal fallback, notify the actual live session after storage. Pi’s native adapter handles this automatically:

Use the [one-shot notification operation](#one-shot-inbox-notification) below so delivery claims prevent duplicate input.

Use only a known live name and safe argument quoting. Terminal input submission is not acknowledgement, acceptance, or completion. A notification failure leaves the assignment and message pending; inspect the agent and re-notify without duplicating work. Workers explicitly accept/decline and submit results through the [assignment tools](../../../comms/coordination.md).

Use `herdr agent get <name>` and `herdr agent read <name> --source recent-unwrapped --lines 120` to inspect sessions. For explicit notification without creating another message, use the one-shot bridge below. Herdr may reject blocked dialogs; do not answer an approval prompt on the user's behalf without authority. Terminal output is a view, not a durable task result.

Workers explicitly send results to the coordinator's mailbox. Humans can enter any pane and give feedback; workers report material changes to the coordinator. Native session history stays available through the selected agent runner.

## Optional integrations

[pi-herdr](https://github.com/AndrewJacop/pi-herdr) adds Pi-native spawning, peer messaging, result collection, and interactive session support. It is an optional alternative to these manual operations, not a bundled dependency. Check its current behavior and runner-specific limits before adopting it. Configure interactive sessions so panes remain available for feedback, and continue recording consequential handoffs in the task.

The core toolbox neither installs this extension nor assumes all runners support the same callbacks. Comms MCP stores task messages and evidence. Herdr supplies live notification and terminal controls; the service itself does not wake sessions.

## Automatic inbox delivery

Use the [fallback bridge setup](../../../comms/herdr-delivery.md) to capture each live named session as structured `session_ref` and attach it to its existing participant with `bind_participant`. Start this fallback separately only for runners without native delivery. The supplied Pi coordinator does not need or start it. Register and bind the coordinator too; process startup alone does not create a task binding. It submits inbox hints only when the bound session is idle/done, records attempts in comms, and leaves reading/acceptance/results to agents. When enabled, do not also manually prompt for each stored notification. Busy/blocked sessions are not interrupted. Report or inspect failures rather than silently duplicating assignments.

## One-shot inbox notification

When no bound background bridge is running, notify after saving an assignment, peer message, or result. Run this from a Herdr-managed pane, using the actual service task ID, routing identity, and live agent name:

```bash
$FACTORY_HOME/orchestrator/start-delivery.sh --project "$PWD" --task <comms-task-id> \
  --notify <recipient-routing-identity> --agent <actual-live-agent-name>
```

The operation captures and verifies the live binding, resolves the active participant, and uses the same durable delivery claims as the polling bridge. It does not replace participants, paste stored message bodies, acknowledge messages, or submit results. Busy or blocked recipients cause a nonzero exit; uncertain input is recorded without blind retries. Inspect `list_deliveries` for the exact outcome. A repeated call does not resubmit already claimed/submitted notifications. No pending messages means no input is sent.

Workers use this for results addressed to the coordinator too. The coordinator must supply its actual live name and peers' names in assignment context. For continuous delivery, use the bound background bridge instead; do not run competing manual prompts.
