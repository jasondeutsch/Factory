# Setup and troubleshooting

Users can ask their coordinator to configure or repair the setup. The diagnostic operations below are for that agent; routine work should not turn into a user-operated script checklist.

## What must be available

| Requirement | When needed | Supplied by Factory? |
| --- | --- | --- |
| Docker Compose and a reachable comms service | Shared task records and the supplied MCP connections. | Compose deployment is supplied; SQLite is embedded in its container. |
| Node.js 22.18+ | Task and session helpers. | Install Node and run `npm ci --omit=dev` in the toolkit for its SDK/runtime dependencies. |
| Pi with the native configuration, auth, MCP, and extension commands used here | New sessions using the provided Pi adapter. | No; an installed runner is required. Another CLI needs an explicit command adapter. |
| Selected provider, actual model ID, and credentials | Each new configured model session. | No; replace the clearly labeled gateway/model examples in the factory defaults/provider definitions and any member overrides. |
| Go 1.26 | Build/run the optional host delivery bridge (a built binary avoids the runtime toolchain). | No, unless running the bridge from source. |
| Herdr or macOS Terminal/iTerm | Automated visible agent sessions. | Select with `FACTORY_TERMINAL_HOST`; manual startup remains possible elsewhere. |
| Dependencies selected by your factory | Language tools, MCP servers and third-party extensions. | Supplied/configured by your tools repository, not the framework. |

Agent work runs on the host in ordinary sessions; no agent image or container sandbox is supplied. The [comms service](../orchestrator/comms/README.md) runs through Docker Compose and is required by the supplied task flow. It starts comms and the read-only admin panel together, with a shared named SQLite volume that survives container replacement. No database lives under the host home directory.

## Dependency commands

Run `make -C "$FACTORY_HOME" install` for shared framework dependencies. Install Docker/Compose separately. Follow the tools repository's factory-specific dependency instructions in that checkout. No copying into the application is required.

## First setup through an agent

Keep framework and tools in separate checkouts. Ask your agent to create project `factory.json` extending the selected team configuration. It should read the coordinator instructions, select the factory, inspect project instructions and available tools, and configure only the roles being used. Existing global Pi settings are not imported implicitly.

Provide provider/model choices and the **name** of a credential environment variable, not its value in the conversation. Each member can have different execution settings in its factory.json config, including native Pi overrides for custom providers and MCPs. The coordinator configures those member entries and checks the selected factory before launching sessions. In a Git project it also excludes root `.ralph/` runtime state from version control.

For example:

> Configure the architect and Go agents for our OpenAI-compatible gateway at `<base URL>`, using `<model ID>` and `TEAM_MODEL_API_KEY`. Keep Linear disabled. Check setup without starting model work, then start a Go task from my request and bring me the spec and plan for approval.

Start the service from `$FACTORY_HOME/orchestrator/comms` with `docker compose up --build -d`. Connect the existing coordinator through its runner's native MCP configuration and reload it; prepared Pi worker profiles already include comms. Set `FACTORY_COMMS_URL` for a changed port/endpoint and pass the same optional `COMMS_AUTH_TOKEN` to service and clients.

Use each role's `start.sh` for consistent startup; the shared launcher resolves the declared member path and loads its configuration and skills. No task argument starts an unassigned interactive role. The agent can inspect configuration with a launch dry run and test native readiness with `--check`. Checks do not call the model, though MCP checks start processes or contact configured services and loop checks can download the declared plugin. A successful credential check confirms local resolution; the first real model request can still fail remotely.

## Common failures

| Symptom | What to establish and do |
| --- | --- |
| Comms unavailable / HTTP 401 | Check Compose health, matching `FACTORY_COMMS_URL`, and token presence in each session. Do not create a local fallback task store. |
| Task records are missing | Check the Compose project name and volume. `down -v` deletes task history; ordinary `down` preserves it. Local documents cannot reconstruct a deleted database. |
| Helpers ask for a project `.factory` directory | You are running an older framework checkout. Use the current external checkout's launcher from the application root, or pass `--project`. |
| Factory member cannot launch | Check its name and bundle path in `factories/<name>/factory.json`. Paths are relative to that file. A task-bound launch only allows its factory's declared members. |
| Provider or model is unset | Set provider/model in factory defaults or member overrides; define custom endpoints in the factory's providers map. Check protocol and actual model ID instead of guessing them. |
| Credentials work in one terminal but not an agent pane | Check variable presence and the pane's inherited environment without printing values. The launcher does not source shell startup files. Restart the session with the intended environment or explicitly share its private auth file. |
| TLS, proxy, or corporate CA failure | Identify whether the failing client is Pi/Node, Git, Go, or the remote service. Apply the organization's trust/proxy setup to that client. The wrapper can map `NODE_EXTRA_CA_CERTS` to an existing variable holding a CA file path; this does not configure Git or Go trust. Keep certificate verification enabled. |
| gopls is missing or cannot load the module | Check the exact executable, Go version, module/workspace configuration, and command output. Correct the environment or explicitly disable that connection and record the limitation. Do not silently install or upgrade the toolchain. |
| Linear is unavailable | Check that the role's entry is enabled, `LINEAR_API_KEY` is present, and the key can read the intended workspace. Reconnect or start a session with the updated configuration. An unavailable connection is not evidence that the issue does not exist. |
| Selected extension command is unavailable or plugin download fails | Inspect private startup diagnostics. Check source access and the factory package declaration, or configure an existing local checkout. The application itself need not be a Git repository. |
| `--loop` refuses approval | Compare current HTML hashes and the latest artifact-specific decisions. Present any changed documents for the actual required decision. Do not manufacture approval to clear the error. |
| A startup check times out | Identify the slow check and its output. Use `check_timeout_seconds` only when the work is legitimately slow; raising it does not fix missing credentials or a hung service. Each process check defaults to 30 seconds, configurable up to 300. |
| A message was saved but notification failed | Retain the message ID. Inspect the recipient extension's binding and connection, then retry without duplicating the assignment. |
| A pane looks idle or progress is stale | Inspect the task inbox, progress file, live terminal output, and any loop state. Determine whether the agent is waiting, failed, reached its limit, or completed. Do not mark success from terminal appearance. |
| A loop reaches its limit or repeats a failure | Preserve partial work and evidence. Reassess with the coordinator; use native resume only for the correct task and a justified assignment. Human decisions and unresolved environment failures should stop the loop. |
| Task status says cancelled but work continues | Stop the active runner through native controls. Status is a record, not a process-control signal. Inspect partial changes afterward. |
| The Go check script stops early | It checks formatting, tests, then lint, in that order. Later steps did not run after a failure. Inspect the actual output; use project-specific commands for multi-module repositories or more appropriate verification. |

Startup failures after native checking begins create `session_startup_failed` task events with a private runtime directory. Inspect its `startup-checks.json` carefully; it can contain sensitive native diagnostics. Configuration or approval errors that occur earlier appear on stderr and need not have that event. Explain the actual failure to the user without echoing credentials.

## Updating the toolbox

Use the [preview-first toolkit updater](toolkit-updates.md) to preserve factory definitions, local overrides and task history. Existing copies should establish their baseline before updating the source checkout. `make update` updates dependencies, not your toolbox checkout or customizations. Changes to authored configuration apply to new prepared sessions; existing runtime copies remain separate until native controls or a new session apply the changes. An already running role does not gain a new MCP merely because its source JSON was edited.

When changing a role's skill list, retain the desired existing entries: wrapper overlays replace the entire list. Native JSON overlays merge object keys recursively but replace arrays and scalar values. The same replacement rule applies to a native `packages` array. Details and examples live in [session configuration](../orchestrator/README.md).

## Maintainer checks

Run `npm ci`, `npm run check`, and `npm test` at the toolkit root, then `go test -race ./...` in `orchestrator/comms`. See [verification](../orchestrator/tests/README.md) for native checks and their limits.

## Automatic notifications are not arriving

Inspect the Pi status line and `/factory-inbox`. Confirm the session was bound with `factory_bind_task`, or started with its explicit task directory. `get_session` must show native delivery with recent contact. An expired heartbeat can mean a stopped process, outage, or failed adapter; it is not proof of completion.

The service image must include `execute_message` and `record_runner_contact`; rebuild Compose after source updates. Verify the selected profile loads the extension with `--check`. `npm ci --omit=dev` installs its local dependencies. Do not use terminal injection to conceal a disconnected extension. Retain message IDs; reconnect/retry does not require another assignment.

Native Terminal/iTerm windows need credentials and service variables in their shell environment. Variables exported only in the coordinator shell are not automatically transferred to another terminal application. Keep secrets out of `.command` files and factory configuration.
