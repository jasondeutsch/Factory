# Agent startup and configuration

The configuration you edit is **`factories/<name>/factory.json`**. It contains shared defaults and member overrides, plus named providers, MCPs, extensions, and skill selections. The [factory reference](../factories/README.md) documents every field. Agent directories contain behavior and reusable resources, not competing configuration files.

## How configuration reaches Pi

Pi does not read `factory.json`. Factory's Python script **[session.py](session.py)** translates it into Pi's native files before starting each agent:

```text
factory.json: defaults + selected member
                 |
                 v
session.py resolves configuration, instructions, and skills
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

The Pi translation and native startup checks live in [adapters/pi/profile.py](adapters/pi/profile.py). Each agent gets its own directory, model configuration, tools, and session transcript. Files are private launch output under `~/.factory/runtime`, or an absolute `FACTORY_RUNTIME_DIR`. **Do not edit generated files to configure future launches.** Change the factory file instead.

There are two configuration levels: factory defaults, then member overrides. Objects merge recursively; lists replace. Member resource selections replace their defaults. No agent-local `config.json`, native profile input, local overlay, or personal Pi configuration participates.

The launcher appends the role instruction, selected factory recipe, and every selected skill entrypoint to the system instructions. Supporting documents retain their source directory for relative links. Raw provider/MCP configuration is not copied into model instructions.

## Which script runs

| Entry point | What it does |
| --- | --- |
| `orchestrator/start.sh` | Calls `startup.py`, which manages service readiness and delivery, then calls `session.py coordinator`. |
| An agent's `start.sh` | Calls `session.py <role>` directly. |
| `session.py` | Selects the factory member, prepares its private profile, checks native readiness, registers a task-bound instance, and replaces itself with the agent process. |
| `adapters/pi/profile.py` | Builds native Pi JSON and invokes Pi's own credential/MCP checks. It is an internal module, not another process to operate. |

## Start a role

From the application root:

```bash
.factory/orchestrator/start.sh --factory golang
.factory/resources/agents/architect/start.sh --factory golang
.factory/factories/golang/agents/coder_golang/start.sh <task-directory>
```

Without a task, `--factory` is required. With a task directory, its recorded factory is used; a conflicting `--factory` fails. No session selects the newest task automatically. Launch workers in their visible panes through [dispatch](references/dispatch.md), not in the coordinator's blocking shell.

The coordinator entrypoint validates configuration before starting services. It starts local Compose and waits for MCP connectivity, supporting both `docker compose` and `docker-compose`. Set `FACTORY_COMMS_URL` to connect to an existing endpoint without managing Compose. `COMMS_PORT` selects the local port otherwise. Docker/Compose remains an explicit prerequisite.

Inside Herdr, `startup.py` builds and supervises a project-scoped delivery bridge. The bridge stops when the coordinator exits; Compose stays running for other sessions. Unexpected bridge exit stops the coordinator with a diagnostic path. The bridge reconnects after service interruptions. Outside Herdr, startup reports that manual inbox polling is required. No task, worker, or approval is created automatically.

A running bridge is not a participant binding. The coordinator still registers its actual task identity and captures its actual pane using the [Herdr binding procedure](adapters/herdr/README.md). Live Herdr acceptance verification remains pending.

`make -C .factory start FACTORY=golang` is an equivalent coordinator shortcut.

## Preview and check

```bash
.factory/resources/agents/architect/start.sh --factory golang --dry-run
.factory/resources/agents/architect/start.sh --factory golang --check
.factory/orchestrator/start.sh --check-factory --factory golang
```

`--dry-run` writes nothing and does not connect or start an agent; a loop preview reads its task approvals. It reports the selected model, thinking, tools, MCP/extension names, instructions, and paths, excluding raw credentials and native provider settings.

`--check` writes a private profile, asks Pi to resolve credentials (`auth check --no-refresh`), then connect to MCPs (`mcp list --json`). It does not call a model or advertised MCP tools. Checks may start stdio servers. Each has a configurable timeout; timeout terminates its process group. Failures prevent chat startup and point to private `startup-checks.json` diagnostics. Credential resolution does not prove remote model access.

`--check-factory` checks each configured member without registering participants or starting chat. Command runners without a native check report unsupported. Check-only commands do not start Compose or delivery; the service must already be available.

Normal Pi startup runs the same checks, then starts interactive Pi in the application root. `--no-approve` prevents repository `.pi` settings from replacing the prepared profile; project context instructions still load normally. The launcher does not source shell startup files. Export credentials first.

## Credentials and MCPs

Use environment references in factory provider definitions and MCP headers. The supplied examples are placeholders, not usable endpoints. Only selected MCPs are materialized; unselected services do not require credentials.

Stdio MCPs run from the application root. A relative MCP `cwd` resolves from that root. For native Pi settings that refer to filesystem resources, use absolute paths; there is no authored native profile directory to copy. Git/registry extension sources remain native Pi package definitions.

To reuse native authentication, set `auth_file_env` or `mcp_auth_file_env` to an environment variable holding an absolute private file path outside the toolkit. The launcher links that file into the generated profile, explicitly sharing refreshes. Otherwise newly written auth stays in the private session. The host environment is inherited; `env` supplements it rather than sandboxing it. `FACTORY_*`, `COMMS_AUTH_TOKEN`, and `PI_CODING_AGENT_DIR` cannot be remapped per member.

Enable Linear by selecting the factory's `linear` MCP for that member and exporting `LINEAR_API_KEY`. Its supplied endpoint is read-only. Reading the Linear skill alone does not establish a connection; restart with the updated configuration. gopls is selected for Go coder/reviewer and must be installed on PATH.

## Task artifacts and recovery

Comms owns task state, assignments, messages, decisions, and evidence. Local `.factory/tasks/<UTC-millisecond-timestamp>-<slug>/` directories contain documents and a `task.json` service link. Go tasks scaffold `spec.html` and `plan.html`; other factories receive the pointer and reviews directory without those documents. Progress and reviews remain with the task.

Task-bound launches register the actual process instance and publish an immutable public session snapshot: runner, model, thinking, selected MCPs, instruction/skill hashes, and startup-check status. It does not fingerprint every provider option or credential. The bridge records expiring host contact; `working` is host observation, not proof of progress.

The Compose named volume retains SQLite history across restarts; `down -v` deletes it. Private session files are not automatically removed. Moving the project changes its path-based identity. Resume requires explicit reconciliation of the task, inbox, assignments, and native session state; `start.sh` creates a fresh process rather than silently taking over old work.

See [operations](references/operations.md), [dispatch and recovery](references/dispatch.md), and [database storage](comms/README.md#shared-database-volume).

## Ralph implementation loops

The Go factory defines the pinned Ralph package under `extensions` and selects it only for the coder. `--loop 5` requires a task, current spec/plan approvals, and an actual active assignment. The launcher checks native command registration, registers itself, waits for assignment, and rechecks approval before starting the loop. This can download the package; configure an existing local source for offline use.

Ralph runs interactively in the coder's pane. It is not a Factory scheduler or a separate agent. The native plugin's `.ralph/` state permits one active loop per workspace. See [loop behavior and recovery](references/ralph.md).

## Other runners

Set the member's `config.runner` to `command` and provide `command` as an argv list containing `{instructions_file}`. Other placeholders are `{project_root}`, `{task_dir}`, and `{session_dir}`. The launcher executes without a shell and supplies the same role/skill instructions and task identity.

Pi model/tool settings inherited from factory defaults do not apply to command runners. Configure that runner's models and native MCP transport through its command and environment; Factory does not translate Pi configuration into another CLI's schema. A command runner has no automatic native readiness check or sandbox. Reading instructions does not register tools.

## Add a member

Create a directory with `agent.md`, skills/scripts as needed, and a thin start script calling `session.py <role>`. Add its name, relative path, resource selections, and any execution overrides to the factory file. Keep the recipe human-readable. Adding a member does not introduce a workflow DSL or generalize the Go-specific document scaffolding and loop shortcut.
