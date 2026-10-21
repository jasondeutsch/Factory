# Agent startup and configuration

The project’s **`factory.json`** selects a team factory and optional overrides. Factory owns the default coordinator bundle in `orchestrator/agent/`; worker bundles can live in a separate checkout. Each bundle owns its instructions, native Pi provider/model/MCP/extension profile and linked skills. Factory reads those files for managed sessions, then applies explicit member overrides. The [configuration reference](../docs/configuration.md) documents both layers.

## Two ways to start coordinating

The coordinator does not have to be Pi. Choose one session to own coordination for a task: a new configured coordinator, or the agent you are already talking to. Both use the same Comms service, task records, skills and dispatch helpers.

### Launch the supplied coordinator

Install the framework dependencies and set `FACTORY_HOME` to its external checkout. Export the credentials needed by your factory. From the application root containing the optional `factory.json`:

```sh
export FACTORY_HOME=/path/to/factory
"$FACTORY_HOME/orchestrator/start.sh"
```

The launcher resolves project configuration, starts local Comms when needed, prepares the coordinator's configuration and skills, and starts an interactive session. The supplied configuration defaults to Pi. Give that session your task request; it launches workers as needed.

To select configuration outside the application, or launch from another directory:

```sh
"$FACTORY_HOME/orchestrator/start.sh" \
  --project /path/to/application \
  --factory /path/to/tools/factories/golang/factory.json
```

Another agent with shell and terminal controls can run this same launcher in a visible pane/window. It must leave the interactive process running there, rather than block its own shell tool waiting for it to exit. Starting a process alone does not submit a task or connect the calling agent's conversation to it. Give the new coordinator its request in that session; subsequent task communication can use Comms. There is no MCP tool for launching a coordinator process.

### Use your existing agent as coordinator

An existing agent needs access to the application and external checkouts, shell/terminal tools for the host helpers, and a native Streamable HTTP MCP connection. Pi is not required for this coordinator; selected workers may still require Pi.

1. Start Comms, or use an existing service:

   ```sh
   export FACTORY_HOME=/path/to/factory
   docker compose -f "$FACTORY_HOME/comms/compose.yaml" up --build -d
   ```

   Use `docker-compose` if that is your installed Compose command. Set `FACTORY_COMMS_URL` in the helper environment when using a different endpoint.

2. Add **Comms** to the existing agent's native MCP configuration with URL `http://127.0.0.1:8090/mcp`, then reconnect/reload its MCP tools. Configuration syntax depends on the runner. If `COMMS_AUTH_TOKEN` is enabled, configure the matching bearer header through that client's credential mechanism. See [MCP connection details](../comms/README.md#connect-an-agent). Reading a skill does not establish this connection.

3. Have the agent read the [coordinator skill](SKILL.md) and [task-comms skill](../comms/skills/task-comms/SKILL.md). From the application root, have it resolve project configuration with:

   ```sh
   node "$FACTORY_HOME/orchestrator/session.ts" coordinator --dry-run
   ```

   Add `--factory /path/to/factory.json` when using external configuration without a project file. The preview supplies resolved membership, instructions, selected skills, source paths and runtime locations. This handles inherited configurations without asking the agent to reimplement their merge rules. It neither starts another agent nor changes the existing agent's model, tools or MCP connections. Apply any required runner settings through its native controls.

4. Give the existing agent your task. It follows [coordinator operations](references/operations.md) to create or resume the task, register its actual participant, attach documents and use [dispatch](references/dispatch.md) to launch workers. Keep the returned task and participant IDs. Do not start a second coordinator for the same work merely to obtain a launcher-generated identity.

For example, tell your existing agent:

> Act as the Factory coordinator for /path/to/application. The framework is at /path/to/factory, and Comms is connected. Load the coordinator and task-comms skills, resolve the project's factory.json using the coordinator dry run, and create a task for a minimal Hello World program in Go. Launch the configured workers as needed. Pause for my required approvals and report progress and blockers here.

The prepared Pi coordinator includes automatic inbox delivery. An existing session receives that behavior only if it has the delivery adapter configured. Other runners need an adapter implementing the [delivery contract](../comms/delivery.md), or the agent must explicitly check its inbox and coordinator attention items at turn boundaries and while supervising work. A skill cannot wake an idle or stopped agent. Do not claim automatic notifications where no adapter exists.

### Observe either mode

Run `"$FACTORY_HOME/orchestrator/monitor.sh"` from the application root to see tasks, agents, attention items and message routes in the terminal. The source launcher requires Go 1.26+ and an already running Comms service; a prebuilt binary avoids the Go requirement. See [controls and connection settings](../comms/README.md#terminal-monitor). The [browser monitor](http://127.0.0.1:8090/monitor) shows recorded status too. Port 8091 is the SQLite administration panel. You can also ask the coordinating agent for status. Both monitors read the same Comms records.

## How configuration reaches Pi

Pi does not read `factory.json`. Factory's TypeScript entrypoint **[session.ts](session.ts)** translates it into Pi's native files before starting each agent:

```text
agent bundle: pi/*.json + skills/ links
factory.json: membership + optional overrides
                 |
                 v
session.ts resolves configuration, instructions, and skills
                 |
                 v
private session directory
  instructions.md
  pi/settings.json  — model, thinking, built-in tools, extensions
  pi/models.json    — selected custom provider definition
  pi/mcp.json       — selected MCP servers
                 |
                 v
Pi starts with PI_CODING_AGENT_DIR=<session>/pi
and --append-system-prompt <session>/instructions.md
```

The Pi translation and native startup checks live in [lib/pi.ts](lib/pi.ts). Each agent gets its own directory, model configuration, tools, and session transcript. Files are private launch output under `~/.factory/<project-name>-<path-hash>/runtime`, or an absolute `FACTORY_RUNTIME_DIR`. **Do not edit generated files to configure future launches.** Change the agent bundle’s authored `pi/` files or the factory’s explicit overrides.

Managed sessions read the agent bundle first, then apply factory fallback values and explicit member overrides. Agent skill links and native Pi files supply the normal provider, model, tools, MCPs and packages. A worker member normally needs only `name`, `path` and `entrypoint`; the built-in coordinator needs no member declaration. With no team configuration, the coordinator uses its bundled Pi profile.

The launcher includes the pinned task workflow (or the selected factory workflow for an unassigned session), role instructions, and every selected skill entrypoint. It includes the recipe for the task owner as explanatory context. Supporting documents retain their source directory for relative links. Raw provider/MCP configuration is not copied into model instructions.

## Which script runs

| Entry point | What it does |
| --- | --- |
| `orchestrator/start.sh` | Calls `startup.ts`, which manages service readiness, then calls `session.ts coordinator`. |
| An agent's `start.sh` | Independent runner entrypoint owned by the agent bundle. A configured member `entrypoint` lets Factory call it with the prepared profile and instructions. |
| `session.ts` | Selects the factory member, prepares its private profile, checks native readiness, registers a task-bound instance, and runs the interactive agent process. |
| `lib/pi.ts` | Builds native Pi JSON and invokes Pi's own credential/MCP checks. It is an internal module, not another process to operate. |

## Start a role

From the application root:

```bash
$FACTORY_HOME/orchestrator/start.sh --factory /path/to/tools/factories/team/factory.json
node $FACTORY_HOME/orchestrator/session.ts architect --factory /path/to/tools/factories/team/factory.json
node $FACTORY_HOME/orchestrator/session.ts coder_golang <task-directory>
```

Without a task, project `factory.json` is used, or an explicit `--factory` path. With neither, only the built-in coordinator is available. With a task directory, its recorded factory is used; a conflicting `--factory` fails. No session selects the newest task automatically. Launch workers in their visible panes through [dispatch](references/dispatch.md), not in the coordinator's blocking shell.

The coordinator entrypoint validates configuration before starting services. It starts local Compose and waits for MCP connectivity, supporting both `docker compose` and `docker-compose`. Set `FACTORY_COMMS_URL` to connect to an existing endpoint without managing Compose. `COMMS_PORT` selects the local port otherwise. Docker/Compose remains an explicit prerequisite.

Every prepared Pi profile loads the toolkit's native delivery extension. It polls comms while the process is running and queues inbox hints when Pi is idle. No terminal prompt injection or Go delivery bridge is needed for Pi. After creating and attaching a task in an initially unassigned session, call `factory_bind_task` with its service ID. Task-bound launches bind automatically. See the [Pi extension guide](extensions/README.md) for binding, message handling, recovery, and limits.

Set `FACTORY_TERMINAL_HOST=herdr`, `terminal`, or `iterm`, or set `terminal_host` at the top of `factory.json`. The environment overrides configuration. The default is Herdr when `HERDR_ENV=1`, otherwise Terminal on macOS. Terminal and iTerm support is currently macOS-only. Other hosts can launch sessions manually. Terminal windows must already have credentials and service settings in their shell environment; the launcher does not copy secrets into command files. Live window/pane acceptance remains pending.

`make -C "$FACTORY_HOME" start PROJECT="$PWD"` is an equivalent coordinator shortcut.

## Preview and check

```bash
node $FACTORY_HOME/orchestrator/session.ts architect --factory /path/to/tools/factories/team/factory.json --dry-run
node $FACTORY_HOME/orchestrator/session.ts architect --factory /path/to/tools/factories/team/factory.json --check
$FACTORY_HOME/orchestrator/start.sh --check-factory --factory /path/to/tools/factories/team/factory.json
```

`--dry-run` writes nothing and does not connect or start an agent; a loop preview reads its task approvals. It reports the selected model, thinking, tools, MCP/extension names, instructions, and paths, excluding raw credentials and native provider settings.

`--check` writes a private profile, asks Pi to resolve credentials (`auth check --no-refresh`), then connect to MCPs (`mcp list --json`). It also starts an RPC session to verify the `factory-inbox` command is loaded. It does not call a model or advertised MCP tools. Add `--probe-model` to `--check` to make one minimal real request with the selected model and thinking level; this uses provider tokens and can detect remote capability errors. Checks may start stdio servers. Each has a configurable timeout; timeout terminates its process group. Failures prevent chat startup and point to private `startup-checks.json` diagnostics. Credential resolution alone does not prove remote model access.

`--check-factory` checks each configured member without registering participants or starting chat. Command runners without a native check report unsupported. Check-only commands do not start Compose or delivery; the service must already be available.

Normal Pi startup runs the same checks, then starts interactive Pi in the application root. `--no-approve` prevents repository `.pi` settings from replacing the prepared profile; project context instructions still load normally. The launcher does not source shell startup files. Export credentials first.

## Credentials and MCPs

Use environment references in factory provider definitions and MCP headers. Any illustrative endpoints/models in a tools repository must be replaced with usable values. Only selected MCPs are materialized; unselected services do not require credentials.

Stdio MCPs run from the application root. An explicit MCP `cwd` resolves from the file declaring it. For native Pi settings that refer to filesystem resources, use absolute paths; there is no authored native profile directory to copy. Git/registry extension sources remain native Pi package definitions.

To reuse native authentication, set `auth_file_env` or `mcp_auth_file_env` to an environment variable holding an absolute private file path outside the toolkit. The launcher links that file into the generated profile, explicitly sharing refreshes. Otherwise newly written auth stays in the private session. The host environment is inherited; `env` supplements it rather than sandboxing it. `FACTORY_*`, `COMMS_AUTH_TOKEN`, and `PI_CODING_AGENT_DIR` cannot be remapped per member.

Enable a tool by defining/selecting its MCP in your factory and providing its dependencies and credentials. Reading a skill does not establish a connection; restart with updated configuration. The framework does not bundle application-specific MCP definitions.

## Task artifacts and recovery

Comms owns task state, assignments, messages, decisions, and evidence. Project `<runtime_dir>/tasks/<UTC-millisecond-timestamp>-<slug>/` directories contain documents and a `task.json` service link. Only documents declared in the selected factory's `artifacts` map are scaffolded; every task receives its pointer and reviews directory. Progress and reviews remain with the task.

Task-bound launches register the actual process instance and publish an immutable public session snapshot: runner, model, thinking, selected MCPs, instruction/skill hashes, and startup-check status. It does not fingerprint every provider option or credential. The extension records expiring native runner contact; `working` is host observation, not proof of progress.

The Compose named volume retains SQLite history across restarts; `down -v` deletes it. Private session files are not automatically removed. Moving the project changes its path-based identity. Resume requires explicit reconciliation of the task, inbox, assignments, and native session state; `start.sh` creates a fresh process rather than silently taking over old work.

See [operations](references/operations.md), [dispatch and recovery](references/dispatch.md), and [database storage](../comms/README.md#shared-database-volume).

## Configured loop launches

Factories may configure a member's `loop` settings and required Pi extension commands. The launcher verifies its selected artifact approvals, registers the instance, waits for an assignment, and rechecks approvals before starting. The extension owns iteration behavior; Factory neither chooses a loop plugin nor prescribes its prompts. See [configuration](../docs/configuration.md#optional-pi-loop-launch).

## Other runners

Set the member's `config.runner` to `command` and provide `command` as an argv list containing `{instructions_file}`. Other placeholders are `{project_root}`, `{task_dir}`, and `{session_dir}`. The launcher executes without a shell and supplies the same role/skill instructions and task identity.

Pi model/tool settings inherited from factory defaults do not apply to command runners. Configure that runner's models and native MCP transport through its command and environment; Factory does not translate Pi configuration into another CLI's schema. A command runner has no automatic native readiness check or sandbox. For automatic inbox delivery, implement the [runner delivery contract](../comms/delivery.md#implementing-another-runner). Otherwise use explicit manual inbox checks. Reading instructions does not register tools.

## Add a member

Create a worker bundle with `agent.md`, a `pi/` native profile, `skills/` links, scripts as needed and a standalone `start.sh`. Point the factory member at the bundle and entrypoint. Factory reads its profile and skills for managed Pi sessions. Set only workflow-specific model/provider overrides and handoff or loop rules in the factory. The bundle entrypoint must not call `session.ts`. You can override the built-in coordinator by declaring a `coordinator` member pointing at another bundle.
