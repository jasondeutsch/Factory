# Setup and troubleshooting

Users can ask their coordinator to configure or repair the setup. The diagnostic operations below are for that agent; routine work should not turn into a user-operated script checklist.

## What must be available

| Requirement | When needed | Supplied by Factory? |
| --- | --- | --- |
| Docker Compose and a reachable comms service | Shared task records and the supplied MCP connections. | Compose deployment is supplied; SQLite is embedded in its container. |
| Python 3.10+ | Task and session helpers. | No; the helpers use only its standard library. |
| Pi with the native configuration, auth, MCP, and extension commands used here | New sessions using the provided Pi adapter. | No; an installed runner is required. Another CLI needs an explicit command adapter. |
| Selected provider, actual model ID, and credentials | Each new configured model session. | No; there is deliberately no default provider or invented model ID. |
| Go 1.26 | Build/run the optional host delivery bridge (a built binary avoids the runtime toolchain). | No, unless running the bridge from source. |
| Herdr | Automated visible pane controls and notifications. | No; manual sessions remain possible. |
| Go and project dependencies | Implementing and verifying Go code. | No. |
| gopls with MCP support | Enabled coder/reviewer gopls connections. | No; disable explicitly if the task will proceed without it. |
| golangci-lint and `gofmt` on PATH | The optional Go check script. | No; use versions compatible with the project. |
| Git and network access to the pinned source, or an available local checkout | Obtaining Ralph for a new private profile. | The package declaration is supplied; Pi obtains the code. |
| `LINEAR_API_KEY` and network access | An enabled Linear connection. | Configuration is supplied, initially disabled. |

Agent work runs on the host in ordinary sessions; no agent image or container sandbox is supplied. The [comms service](../comms/README.md) runs through Docker Compose and is required by the supplied task flow. Its SQLite database is container-local by default. Explicitly mount `/data` for durability across container replacement. No database lives under the host home directory.

## First setup through an agent

Ask your agent to copy the toolbox's contents into the application's `.factory/` directory, preserving local customization. It should read the coordinator instructions, select the factory, inspect project instructions and available tools, and configure only the roles being used. Existing global Pi settings are not imported implicitly.

Provide provider/model choices and the **name** of a credential environment variable, not its value in the conversation. Each role can have different provider, base URL, model, thinking level, tools, and MCPs. The coordinator edits team files or ignored local overlays and checks the proposed session before launching it in a visible pane. In a Git project it also excludes root `.ralph/` runtime state from version control.

For example:

> Configure the architect and Go agents for our OpenAI-compatible gateway at `<base URL>`, using `<model ID>` and `TEAM_MODEL_API_KEY`. Keep Linear disabled. Check setup without starting model work, then start a Go task from my request and bring me the spec and plan for approval.

Start the service from `.factory/comms` with `docker compose up --build -d`. Connect the existing coordinator through its runner's native MCP configuration and reload it; prepared Pi worker profiles already include comms. Set `FACTORY_COMMS_URL` for a changed port/endpoint and pass the same optional `COMMS_AUTH_TOKEN` to service and clients.

Use each role's `start.sh` for consistent startup; it loads its own configuration and skill list through the shared launcher. No task argument starts an unassigned interactive role. The agent can inspect configuration with a launch dry run and test native readiness with `--check`. Checks do not call the model, though MCP checks start processes or contact configured services and loop checks can download the declared plugin. A successful credential check confirms local resolution; the first real model request can still fail remotely.

## Common failures

| Symptom | What to establish and do |
| --- | --- |
| Comms unavailable / HTTP 401 | Check Compose health, matching `FACTORY_COMMS_URL`, and token presence in each session. Do not create a local fallback task store. |
| Task records vanished after `down` | The default database was in the removed container. Local documents cannot restore messages/decisions; use an explicit volume when durability is needed. |
| Helpers ask for the project's `.factory` directory | Confirm the toolbox was copied into the intended application root. Do not run task creation from the source toolbox or silently choose a parent project. |
| Provider or model is unset | Configure the role's native settings and, for a custom provider, its model definition. Check protocol and actual model ID instead of guessing them. |
| Credentials work in one terminal but not an agent pane | Check variable presence and the pane's inherited environment without printing values. The launcher does not source shell startup files. Restart the session with the intended environment or explicitly share its private auth file. |
| TLS, proxy, or corporate CA failure | Identify whether the failing client is Pi/Node, Git, Go, or the remote service. Apply the organization's trust/proxy setup to that client. The wrapper can map `NODE_EXTRA_CA_CERTS` to an existing variable holding a CA file path; this does not configure Git or Go trust. Keep certificate verification enabled. |
| gopls is missing or cannot load the module | Check the exact executable, Go version, module/workspace configuration, and command output. Correct the environment or explicitly disable that connection and record the limitation. Do not silently install or upgrade the toolchain. |
| Linear is unavailable | Check that the role's entry is enabled, `LINEAR_API_KEY` is present, and the key can read the intended workspace. Reconnect or start a session with the updated configuration. An unavailable connection is not evidence that the issue does not exist. |
| Ralph command is unavailable or plugin download fails | Inspect private startup diagnostics. Check source access and the pinned package declaration, or configure an existing local checkout. The application itself need not be a Git repository. |
| `--loop` refuses approval | Compare current HTML hashes and the latest artifact-specific decisions. Present any changed documents for the actual required decision. Do not manufacture approval to clear the error. |
| A startup check times out | Identify the slow check and its output. Use `check_timeout_seconds` only when the work is legitimately slow; raising it does not fix missing credentials or a hung service. Each process check defaults to 30 seconds, configurable up to 300. |
| A message was saved but notification failed | Retain the message ID. Inspect the recipient's live session and attempt notification again without duplicating the assignment. Herdr control must originate inside `HERDR_ENV=1`. |
| A pane looks idle or progress is stale | Inspect the task inbox, progress file, live terminal output, and any loop state. Determine whether the agent is waiting, failed, reached its limit, or completed. Do not mark success from terminal appearance. |
| A loop reaches its limit or repeats a failure | Preserve partial work and evidence. Reassess with the coordinator; use native resume only for the correct task and a justified assignment. Human decisions and unresolved environment failures should stop the loop. |
| Task status says cancelled but work continues | Stop the active runner through native controls. Status is a record, not a process-control signal. Inspect partial changes afterward. |
| The Go check script stops early | It checks formatting, tests, then lint, in that order. Later steps did not run after a failure. Inspect the actual output; use project-specific commands for multi-module repositories or more appropriate verification. |

Startup failures after native checking begins create `session_startup_failed` task events with a private runtime directory. Inspect its `startup-checks.json` carefully; it can contain sensitive native diagnostics. Configuration or approval errors that occur earlier appear on stderr and need not have that event. Explain the actual failure to the user without echoing credentials.

## Updating the toolbox

Copy changes selectively and preserve team recipes, role configuration, local overlays, and task history. There is no installer, migration engine, or automatic update command. Changes to authored configuration apply to new prepared sessions; existing runtime copies remain separate until native controls or a new session apply the changes. An already running role does not gain a new MCP merely because its source JSON was edited.

When changing a role's skill list, retain the desired existing entries: wrapper overlays replace the entire list. Native JSON overlays merge object keys recursively but replace arrays and scalar values. The same replacement rule applies to a native `packages` array. Details and examples live in [session configuration](../orchestrator/README.md).

## Maintainer checks

From the source toolbox's parent, run:

```bash
python3 -B -m unittest discover -s factory/tests -v
```

Inside an application project, use `.factory/tests` instead. Tests build the real comms server with Go (or use `FACTORY_TEST_COMMS` pointing to an existing binary), start isolated localhost servers with disposable databases, and exercise temporary projects and fake agent executables; optional native tests are skipped unless enabled. Set `FACTORY_TEST_PI` to an installed Pi executable for native auth and stdio MCP checks. Also set `FACTORY_TEST_RALPH_DIR` to a checkout of the pinned Ralph revision for its native transition test.

The Ralph fixture starts a localhost-only HTTP server, supplies deterministic model responses, and calls both a fixture tool and the real comms MCP across fresh sessions. It needs permission to bind localhost but no paid provider or Linear credentials. These checks do not validate live Herdr controls, corporate-network behavior, model quality, or the application's product. Role skills and handoffs still need evaluation through real tasks.

## Automatic notifications are not arriving

First establish whether the [delivery bridge](../comms/delivery.md) is actually running inside Herdr against the expected project/task and updated comms service. An empty participant `session_ref` intentionally uses manual delivery; a bare name is not a captured binding. Inspect `list_deliveries`, task report, and bridge pane for specific failures. Working/blocked/unknown sessions wait without interruption. `submitted` confirms only host input. Inspect uncertain input before the owner explicitly retries; never recreate assignments to retry a notification. A source update does not rebuild a running comms image or replace existing participant registrations.
