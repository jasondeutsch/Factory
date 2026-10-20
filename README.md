# Factory

A lightweight framework for coordinating visible, interactive agent sessions. The coordinator delegates work, gathers results, and brings decisions back to the user. Comms records tasks, messages, approvals, evidence and usage; agents follow a human-readable recipe.

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
  "extends": "../tools/factories/golang/factory.json",
  "agents": [
    { "name": "architect", "config": { "thinking": "high" } }
  ]
}
```

To launch the supplied coordinator (Pi by default), run from the application root:

```sh
"$FACTORY_HOME/orchestrator/start.sh"
```

Alternatively, leave the application completely free of Factory files and select an external configuration:

```sh
"$FACTORY_HOME/orchestrator/start.sh" --factory /path/to/tools/factories/golang/factory.json
```

`--project /path/to/application` explicitly selects another working directory. `--check` checks coordinator readiness; `--check-factory` checks all declared members. Both exit without starting interactive work. With no configuration at all, the built-in coordinator uses your native Pi provider/model defaults; no worker agents are configured.

## Standalone configuration with your own resources

This `factory.json` uses your own worker, skill, recipe and MCP server. It does not extend a bundled factory. Factory supplies the coordinator and Comms infrastructure automatically.

```json
{
  "name": "my-team",
  "recipe": "../my-resources/workflow.md",
  "defaults": {
    "runner": "pi",
    "provider": "team-gateway",
    "model": "your-model-id",
    "mcps": ["comms"]
  },
  "providers": {
    "team-gateway": {
      "baseUrl": "https://gateway.example.com/v1",
      "api": "openai-completions",
      "apiKey": "${TEAM_API_KEY}",
      "models": [{ "id": "your-model-id" }]
    }
  },
  "mcps": {
    "team-tools": {
      "url": "http://127.0.0.1:9000/mcp"
    }
  },
  "agents": [
    {
      "name": "worker",
      "path": "../my-resources/agents/worker",
      "skills": [
        "@factory/orchestrator/comms/skills/task-comms",
        "../my-resources/skills/my-skill"
      ],
      "mcps": ["comms", "team-tools"]
    }
  ]
}
```

Replace the example endpoints, model ID and paths, and export `TEAM_API_KEY` before launching. The provider example assumes an OpenAI-compatible Chat Completions API. Remove `team-tools` from both the catalog and worker selection if no custom MCP is needed.

Your worker directory contains `agent.md`; your skill directory contains `SKILL.md`. `workflow.md` describes how the coordinator should use your workers, including approval points and expected outputs. The task-comms skill supplies the shared communication protocol. Add more workers as named entries, or add a `coordinator` entry to select your own coordinator skills. See [configuration](docs/configuration.md) for member overrides and other runners.

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

Paths resolve relative to the JSON file that declares them, including inherited files. Absolute paths and `~/` are supported. `@factory/` refers to this framework checkout, so team configurations can select coordinator resources without assuming where it is installed.

The default runtime directory is `~/.factory/<project-name>-<path-hash>/runtime/`. It holds task documents, session profiles/transcripts, dispatch receipts and diagnostics. Set `runtime_dir` in project `factory.json` to override it. Application source changes still happen in the application root. Comms task records live in the Compose SQLite volume; filesystem runtime configuration does not relocate that database.

The entrypoint starts Compose unless `FACTORY_COMMS_URL` selects an existing service. `FACTORY_TERMINAL_HOST` selects `herdr`, `terminal`, or `iterm`; the last two support macOS. Ask the coordinator to perform work using the selected recipe. Workers stay visible for inspection and direct feedback.

Agents retain host permissions. Factory is not a sandbox or workflow scheduler. A stopped process cannot receive notifications. Recorded approvals represent declared authority, and missing usage remains unknown.

## Documentation

- [Configuration](docs/configuration.md): composition, paths, membership and overrides.
- [Agent startup](orchestrator/README.md): native profiles and readiness checks.
- [Coordinator operations](orchestrator/references/operations.md) and [dispatch recovery](orchestrator/references/dispatch.md).
- [Comms](orchestrator/comms/README.md) and [delivery contract](orchestrator/comms/delivery.md).
- [Updates](docs/toolkit-updates.md), [troubleshooting](docs/troubleshooting.md), and [tradeoffs](docs/tools.md).
- [Verification](orchestrator/tests/README.md).
