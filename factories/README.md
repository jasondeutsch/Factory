# Configure a factory

**Edit one file: `factories/<name>/factory.json`.** It declares the members, execution defaults, provider endpoints, MCP servers, extensions, and skill selections. Agent directories contain instructions, skills, scripts, and a thin `start.sh`; they contain no independent execution configuration.

See the complete [Go factory](golang/factory.json) and [writing factory](writing/factory.json). Both contain illustrative gateway settings that you must replace before launch.

## Shared defaults and member overrides

A factory's `defaults` apply to all its members. A member's `config` overrides only what differs. Objects merge recursively; lists and scalar values replace. There are no global model settings, agent-local overrides, or native profile files to edit.

For example, put your provider/model in `defaults` once. Give the architect `"config": {"model": "reasoning-model", "thinking": "high"}` when it needs another model. Add that model to the selected custom provider's `models` list. A member with no `config` uses the defaults; it does not need an empty object.

Member `skills`, `mcps`, and `extensions` select resources. If omitted, their defaults apply. An explicit list replaces the default list, so include `comms` in an MCP override. An empty list clears an optional selection; Pi sessions always require `comms`.

| Factory field | Meaning |
| --- | --- |
| `name` | Identifier matching its directory under `factories/`. |
| `owner` | Member responsible for task coordination. |
| `terminal_host` | Optional `herdr`, `terminal`, or `iterm`. `FACTORY_TERMINAL_HOST` overrides this; automatic selection uses Herdr when inside it, otherwise Terminal on macOS. |
| `defaults` | Shared execution settings and optional resource selections. |
| `providers` | Named custom provider definitions in Pi's native provider format. Built-in providers need no entry. |
| `mcps` | Named native MCP server definitions. Declaring one does not enable it for every agent. |
| `extensions` | Named Pi package sources or package objects. Only selected packages are installed/loaded. The local Factory delivery extension is always loaded for Pi. |
| `agents` | Members, paths, selections, overrides, and optional handoff requirements. |

| Member field | Meaning |
| --- | --- |
| `name` | Unique role name used for launch and communication. |
| `path` | Directory containing `agent.md`, or `SKILL.md` for the coordinator. Relative to this factory file, inside the toolkit. |
| `config` | Optional execution overrides, using the fields below. |
| `skills` | Paths to skill directories or entrypoint files, relative to this factory file, inside the toolkit. Include the shared task-comms skill. |
| `mcps` | Names from the factory's `mcps` map. |
| `extensions` | Names from the factory's `extensions` map. |
| `handoff` | Assignment evidence defaults; see [dispatch](../orchestrator/references/dispatch.md). |

## Execution settings

These belong in `defaults` or a member's `config`:

| Field | Meaning |
| --- | --- |
| `runner` | `pi`, or `command` for an explicit native runner command. |
| `delivery` | Command runners only: `manual` (default), or `native` when their adapter implements the delivery contract. Pi always uses native delivery. |
| `provider`, `model` | Selected provider and model IDs. |
| `thinking` | Model-supported thinking level, such as `medium`, `high`, or `off`. |
| `tools` | Pi built-in tool selection. MCP exposure is configured separately. |
| `executable` | Pi executable name/path; defaults to `pi`. |
| `check_timeout_seconds` | Native startup check timeout, default 30, maximum 300. |
| `env` | Child environment names mapped to parent environment variable names. Values are references, not credentials. |
| `auth_file_env`, `mcp_auth_file_env` | Environment variables containing absolute private native authentication-file paths. |
| `args` | Additional permitted Pi flags. Cannot override model, tools, credentials, session ownership, or interactivity. |
| `pi.settings` | Additional native settings, such as retry policy. Use the explicit fields above for model/tools and `extensions` for packages. |
| `command` | Command runner argv, including `{instructions_file}`. See [other runners](../orchestrator/README.md#other-runners). |

## Providers and credentials

The examples use `team-gateway`, `your-model-id`, and `https://gateway.example.com/v1`. Replace the URL, `defaults.model`, and the provider's `models[].id`. The example protocol is OpenAI-compatible Chat Completions (`openai-completions`); choose the protocol your endpoint actually supports.

`provider` matches a key in `providers`, or a built-in Pi provider. The provider's `apiKey` is `${TEAM_MODEL_API_KEY}`; export that variable in the launching environment. Do not put its value in the file. For a built-in provider, select its ID/model, remove unused custom definitions, and use its native credential mechanism.

Different members may select different providers and models. Only the selected custom provider is emitted into that agent's generated `models.json`. Other provider credentials are not required for that launch. Personal Pi settings are not imported.

## MCPs, skills, and extensions

The Go factory defines `gopls` once and selects it for the coder and reviewer. `linear` is defined but unselected. To enable Linear for the architect, set its `mcps` to `["comms", "linear"]`, provide `LINEAR_API_KEY`, and start a new session. No native MCP file needs editing.

The Go factory defines the pinned Ralph package once in `extensions.ralph`; only the coder selects `"extensions": ["ralph"]`. An absolute path to an existing package checkout can replace its source for offline use. Extension installation requires network access unless the package is already available locally. Factory dependency targets check/install selected packages in private generated profiles; each fresh agent profile may still need to obtain its own package copy.

Skills are explicit file selections. `session.ts` reads the full selected `SKILL.md` entrypoints into the agent's system instructions, preserving source paths for supporting references. A skill does not install tools or register an MCP server.

Comms is shared infrastructure: `FACTORY_COMMS_URL` and `COMMS_AUTH_TOKEN` are the launch environment's connection settings for both helpers and agents. The launcher applies them to each generated comms entry. They are not per-agent model settings.

## Start and inspect

From an application containing `.factory/`:

```bash
.factory/orchestrator/start.sh --factory golang
.factory/resources/agents/architect/start.sh --factory golang --dry-run
.factory/factories/golang/agents/coder_golang/start.sh <task-directory>
```

`--factory` is required unless an explicit task selects its recorded factory. There is no global coordinator profile or automatic newest-task selection. A conflicting factory/task pair fails.

`--dry-run` shows the resolved model, thinking, tools, MCP/extension names, skills, and command without writing files or starting an agent. Raw provider configuration and credential values are excluded. `--check` prepares private native files and checks credentials/MCPs without model inference; comms must already be available.

See [how the configuration reaches Pi](../orchestrator/README.md#how-configuration-reaches-pi) for the exact scripts and generated files.

## Dependencies and customization

Each factory owns a simple Makefile with `install` and `update`. These call shared prerequisites, then add factory-specific tools. No installer schema or package ecosystem is introduced. Install affects the host, not an isolated toolchain.

Edit the recipe in `README.md` for workflow agreements and `factory.json` for execution. Team-owned factories survive [toolkit updates](../docs/toolkit-updates.md). Existing sessions retain their prepared configuration; launch a new session to apply changes. No migration guide or compatibility layer is provided for this prototype.
