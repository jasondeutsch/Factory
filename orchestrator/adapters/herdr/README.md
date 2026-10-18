# Visible sessions with Herdr

This reference is for the coordinating agent. The user asks for visible role sessions; handle the pane controls and helper calls on their behalf. The default is ordinary interactive sessions, each in a pane. No extension or MCP server is required. Only control Herdr from a Herdr-managed pane (`HERDR_ENV=1`). Outside Herdr, explain that the user must open their coordinating session there first; do not operate another focused session implicitly.

## Manual panes

The user can open role panes themselves if preferred. Each agent can load its role instructions and the relevant skills, recipe, and task context in an existing session. Use the [operations reference](../../references/operations.md) for new configured sessions. There is no automatic message watcher or completion push in manual mode. The optional [host delivery bridge](../../comms/delivery.md) provides inbox notifications without a runner extension.

## Named sessions and notifications

For discoverable names, name the agents already running in your panes. The installed CLI is the authority for syntax: run `herdr agent` and `herdr pane` to inspect it. With the inspected 0.9.3 CLI:

1. Reuse an available role session, or open a pane and start `session.py <role> <task-path>` there. The launcher applies that role's configuration and starts the runner in the project root.
2. From the coordinator's Herdr pane, run `herdr agent list`. Read the actual pane IDs and identify the role sessions. Do not invent IDs from examples.
3. Name a session with `herdr agent rename <actual-pane-id> account-architect`. A name must be unique among live agents.

When the user requests new visible panes, use `herdr pane split --current --direction right --cwd "$PWD" --no-focus` from the project root. Use `down` when that gives more usable space and read `.result.pane.pane_id` from the response. Start the session with `herdr pane run <returned-id> <shell-quoted-launch-command>`, where the command runs `python3 .factory/orchestrator/session.py <role> <actual-task-path>` from that project root. Quote each launcher argument and then the command as required by your shell tool; do not splice arbitrary user text into it. Do not start a second instance in an occupied pane or run the interactive launcher in the coordinator's own blocking shell tool.

Inspect the pane to establish whether startup succeeded before assigning work. The launcher first prepares the native profile and checks credential readiness and enabled MCP connections. Required environment references must exist in that pane; do not assume it matches every coordinator subprocess. If startup fails, inspect the task event and private diagnostics, explain the actual error without credentials, and request only the needed correction. Never claim an agent is ready merely because its command was submitted or preflight passed.

For a coder loop, first record the approved assignment durably, then add `--loop 5` to its pane launch; the initial native command starts the bounded assignment instead of waiting for another turn. Confirm startup and actual progress afterward. Keep this ordering distinct from an ordinary waiting worker, and follow the [Ralph reference](../../references/ralph.md). The loop remains in the same visible pane while Pi replaces its sessions.

Name live sessions distinctly, such as `account-architect` and `account-coordinator`. Record the mapping in the task with `task.py note`. Herdr names have their own syntax and length rules; they are not the task's display name.

Create canonical work with comms `create_assignment` using the registered participant IDs. It saves the assignment notification and event atomically; do not create another assignment message with a shell helper. After storage, notify the actual live session:

```bash
herdr agent prompt account-architect "Check your comms inbox for the assigned task."
```

Use only a known live name and safe argument quoting. Terminal input submission is not acknowledgement, acceptance, or completion. A notification failure leaves the assignment and message pending; inspect the agent and re-notify without duplicating work. Workers explicitly accept/decline and submit results through the [assignment tools](../../comms/coordination.md).

Use `herdr agent get <name>` and `herdr agent read <name> --source recent-unwrapped --lines 120` to inspect sessions. For explicit notification without creating another message, use `herdr agent prompt <name> "Check your task inbox."`. Herdr may reject blocked dialogs; do not answer an approval prompt on the user's behalf without authority. Terminal output is a view, not a durable task result.

Workers explicitly send results to the coordinator's mailbox. Humans can enter any pane and give feedback; workers report material changes to the coordinator. Native session history stays available through the selected agent runner.

## Optional integrations

[pi-herdr](https://github.com/AndrewJacop/pi-herdr) adds Pi-native spawning, peer messaging, result collection, and interactive session support. It is an optional alternative to these manual operations, not a bundled dependency. Check its current behavior and runner-specific limits before adopting it. Configure interactive sessions so panes remain available for feedback, and continue recording consequential handoffs in the task.

The core toolbox neither installs this extension nor assumes all runners support the same callbacks. Comms MCP stores task messages and evidence. Herdr supplies live notification and terminal controls; the service itself does not wake sessions.

## Automatic inbox delivery

Use the [delivery bridge setup](../../comms/delivery.md) to capture each live named session as structured `session_ref` before participant registration. Start its host process in a separate Herdr pane and register the coordinator too. It submits inbox hints only when the bound session is idle/done, records attempts in comms, and leaves reading/acceptance/results to agents. When enabled, do not also manually prompt for each stored notification. Busy/blocked sessions are not interrupted. Report or inspect failures rather than silently duplicating assignments.
