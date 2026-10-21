# Factory

A lightweight framework for coordinating visible, interactive agent sessions. A factory defines its members and workflow steps; the coordinator delegates the current step, gathers results, and brings decisions back to the user. Comms records tasks, messages, approvals, evidence and usage.

This repository contains the coordinator, launchers, Pi delivery extension, and Comms MCP service. Worker agents, factory recipes, skills and model choices belong in a separate repository such as `tools`.

## Setup

Keep the framework and tools checkouts outside the application repository. Install the framework's Node dependencies once:

```sh
cd /path/to/factory
npm ci --omit=dev
export FACTORY_HOME=/path/to/factory
```

Node 22.18+ and Pi are required for the supplied Pi coordinator. Docker with Compose is needed to start the local Comms service; an existing service can be selected with `FACTORY_COMMS_URL`. Herdr is optional. The terminal monitor launcher requires Go 1.26+; a prebuilt monitor binary does not. Install any additional tools required by your chosen factory separately.

The application can contain just one Factory configuration file, `factory.json`:

```json
{
  "extends": "../factory-tools/factories/golang/factory.json"
}
```

To launch the supplied coordinator (Pi by default), run from the application root:

```sh
"$FACTORY_HOME/orchestrator/start.sh"
```

Alternatively, leave the application completely free of Factory files and select an external configuration:

```sh
"$FACTORY_HOME/orchestrator/start.sh" --factory /path/to/factory-tools/factories/golang/factory.json
```

`--project /path/to/application` explicitly selects another working directory. `--check` checks coordinator readiness; `--check-factory` checks all declared members. Both exit without starting interactive work. With no configuration at all, the built-in coordinator uses your native Pi provider/model defaults; no worker agents are configured.

## Standalone configuration with your own resources

A standalone `factory.json` can refer to your own agent bundles. Each bundle supplies its native model/provider profile, MCPs, extension packages and linked skills. Factory supplies Comms and the coordinator tools; the factory file defines the team's workflow and can override a member model/provider.

```json
{
  "name": "my-team",
  "agents": [
    {
      "name": "worker",
      "path": "../my-resources/agents/worker",
      "entrypoint": "start.sh"
    }
  ],
  "workflow": {
    "start": "work",
    "steps": [
      {
        "id": "work", "type": "agent", "agent": "worker",
        "title": "Do the work", "instructions": "Address the task brief and report evidence.",
        "inputs": ["task brief"], "outputs": ["work result"],
        "on": { "completed": "acceptance", "failed": "$blocked" }
      },
      {
        "id": "acceptance", "type": "decision", "actor": "human", "artifacts": [],
        "on": { "approve": "$done", "request_changes": "work", "reject": "$blocked" }
      }
    ]
  }
}
```

Set the provider, model, MCPs and environment-variable credential references in the worker bundle’s `pi/` files. A factory member can set `"config": {"provider": "other-provider", "model": "other-model"}`; define a custom provider in the factory’s `providers` catalog if that provider is not already in the bundle.

Your worker directory contains `agent.md`, `start.sh`, a native `pi/` profile and `skills/` links to the canonical skill definitions. The structured `workflow` sets step order, outputs, decisions and revision routes; an optional Markdown `recipe` can explain team intent. Managed sessions receive the task-comms skill from Factory. Add more workers as named entries, or point the `coordinator` member at your own bundle. See [configuration](docs/configuration.md) for member overrides and other runners.

All relative paths start at the file declaring them. These resources can remain outside the application; only `factory.json` needs to be in the project.

## Use an existing agent instead

Your current agent can serve as coordinator if it has shell/terminal access and a Comms MCP connection. Load the [coordinator skill](orchestrator/SKILL.md) and [task-comms skill](orchestrator/comms/skills/task-comms/SKILL.md), resolve the project configuration, and give it your task. This does not require a second coordinator process or a Pi coordinator.

See [both coordinator setup paths](orchestrator/README.md#two-ways-to-start-coordinating) for service startup, MCP connection, configuration loading and an example prompt. Automatic inbox delivery currently has a Pi adapter; other runners require their own adapter or explicit inbox checks. Another agent can also launch the supplied coordinator through terminal controls; Comms has no process-launch MCP tool.

Open the terminal monitor from your project directory (with Comms running):

```sh
"$FACTORY_HOME/orchestrator/monitor.sh"
```

Or run `make monitor PROJECT=/path/to/project` from the Factory checkout. The TUI shows task status, attention items, agents, message routes, and activity. Use arrows to select tasks, `1`/`2`/`3` for overview/comms/events, and `q` to quit. See [monitor controls and configuration](orchestrator/comms/README.md#terminal-monitor).

The [browser monitor](http://127.0.0.1:8090/monitor) remains available.

## Configuration and runtime

Paths in factory files resolve relative to the file that declares them, including inherited files. Native agent profiles and linked skills resolve within the agent bundle. Absolute paths and `~/` are supported. `@factory/` refers to this framework checkout.

The default runtime directory is `~/.factory/<project-name>-<path-hash>/runtime/`. It holds task documents, session profiles/transcripts, dispatch receipts and diagnostics. Set `runtime_dir` in project `factory.json` to override it. Application source changes still happen in the application root. Comms task records live in the Compose SQLite volume; filesystem runtime configuration does not relocate that database.

The entrypoint starts Compose unless `FACTORY_COMMS_URL` selects an existing service. `FACTORY_TERMINAL_HOST` selects `herdr`, `terminal`, or `iterm`; the last two support macOS. Ask the coordinator to perform work using the selected workflow. Workers stay visible for inspection and direct feedback.

Agents retain host permissions. Factory is not a sandbox or workflow scheduler. A stopped process cannot receive notifications. Recorded approvals represent declared authority, and missing usage remains unknown.

## Documentation

- [Configuration](docs/configuration.md): composition, paths, membership and overrides.
- [Agent startup](orchestrator/README.md): native profiles and readiness checks.
- [Coordinator operations](orchestrator/references/operations.md) and [dispatch recovery](orchestrator/references/dispatch.md).
- [Comms](orchestrator/comms/README.md) and [delivery contract](orchestrator/comms/delivery.md).
- [Updates](docs/toolkit-updates.md), [troubleshooting](docs/troubleshooting.md), and [tradeoffs](docs/tools.md).
- [Verification](orchestrator/tests/README.md).
