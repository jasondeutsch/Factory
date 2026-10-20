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

Node 22.18+, Pi, and Docker with Compose are required for the supplied Pi coordinator. Herdr is optional. Install any additional tools required by your chosen factory separately.

The application can contain just one Factory configuration file, `factory.json`:

```json
{
  "extends": "../tools/factories/golang/factory.json",
  "agents": [
    { "name": "architect", "config": { "thinking": "high" } }
  ]
}
```

Run from the application root:

```sh
"$FACTORY_HOME/orchestrator/start.sh"
```

Alternatively, leave the application completely free of Factory files and select an external configuration:

```sh
"$FACTORY_HOME/orchestrator/start.sh" --factory /path/to/tools/factories/golang/factory.json
```

`--project /path/to/application` explicitly selects another working directory. `--check` checks coordinator readiness; `--check-factory` checks all declared members. Both exit without starting interactive work. With no configuration at all, the built-in coordinator uses your native Pi provider/model defaults; no worker agents are configured.

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
