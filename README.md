# Factory

A lightweight framework for coordinating visible, interactive agent sessions. A coordinating agent delegates work, gathers results, and brings decisions back to the user. Comms records tasks, messages, approvals, evidence and usage; the framework does not execute a workflow graph.

This repository owns the shared runtime. **Individual factories, agents, skills, templates, models and tool choices live in a separate repository**, such as `tools`, or in your team's own configuration.

## What belongs here

```text
orchestrator/  # Coordinator instructions, launchers, Pi delivery adapter, Go MCP service, tests
docs/         # Framework setup, configuration and operating reference
```

The coordinator and task-comms skill are framework infrastructure. A Go coder, architecture reviewer, writing agent, or third-party plugin selection is team content and belongs outside this repo.

## Install into a project

Use one combined `.factory/` directory at the application root:

```text
project/.factory/
  orchestrator/ docs/  Makefile  package.json  package-lock.json  ...  # Framework
  factories/    # Your named factory.json configurations and recipes
  agents/       # Your shared agent bundles
  skills/       # Your shared skills
```

From this framework checkout, install its dependencies and use the preview-first copier:

```sh
npm ci --omit=dev
node orchestrator/scripts/sync_toolkit.ts /path/to/project/.factory
node orchestrator/scripts/sync_toolkit.ts /path/to/project/.factory --apply
```

Then copy `factories/`, `agents/`, and `skills/` from your tools repository into that `.factory/`. Copy directories selectively when a project already has customizations. Do not copy the tools repository's `.git` or root README over the framework. Run `npm ci --omit=dev` in the installed `.factory/`.

Configure provider/model choices and environment references in the selected `factory.json`. Install shared dependencies with `make -C .factory install` if needed; use the tools repository's factory-specific instructions for additional dependencies. Docker with Compose is a separate prerequisite.

## Run and observe

From the application root, using an installed factory name:

```sh
.factory/orchestrator/start.sh --factory <name>
```

The entrypoint starts Compose unless `FACTORY_COMMS_URL` points to an existing service. Pi sessions load the native inbox adapter. `FACTORY_TERMINAL_HOST` selects `herdr`, `terminal`, or `iterm`; the latter two currently support macOS. No task, worker, or approval is created automatically.

Ask the coordinator to perform work using the selected recipe. Workers stay visible so you can inspect them and provide direct feedback. Pi's adapter delivers message hints independently of the terminal host. Message states are `sent → read → exec`; assignment acceptance and completion are separate.

Comms and its read-only SQLite admin panel run together through Compose. Task records persist in its named volume. Project documents live in `.factory/tasks/`; private session profiles/transcripts live under `$HOME/.factory/runtime/`. Application changes belong in the application workspace.

## Boundaries

Agents retain host permissions. This framework is not a sandbox, scheduler, model provider, or per-agent authentication system. A stopped process cannot receive notifications. Recorded approvals represent declared authority; missing usage remains unknown. Committing or contacting external systems requires authorization.

Framework updates manage only framework files; they do not update or delete your `factories/`, `agents/`, or `skills/`. Existing sessions keep their prepared configuration until restarted.

## Documentation

- [Configuration contract](docs/configuration.md): membership, resources, task templates and optional loop settings.
- [Agent startup](orchestrator/README.md): native profiles, checks and runner integration.
- [Delivery contract](orchestrator/comms/delivery.md): Pi and other runners; sent/read/exec semantics.
- [Coordinator operations](orchestrator/references/operations.md) and [dispatch recovery](orchestrator/references/dispatch.md).
- [Comms](orchestrator/comms/README.md): service, storage and reports.
- [Toolkit updates](docs/toolkit-updates.md), [troubleshooting](docs/troubleshooting.md), and [tradeoffs](docs/tools.md).
- [Verification](orchestrator/tests/README.md): automated checks and pending live acceptance.
