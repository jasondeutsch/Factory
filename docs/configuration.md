# Configure a factory

**Edit one file: `factories/<name>/factory.json`.** It declares the members, execution defaults, provider endpoints, MCP servers, extensions, and skill selections. Agent directories contain instructions, skills, scripts, and a thin `start.sh`; they contain no independent execution configuration.

Factory configurations are supplied by your team or a separate tools repository. The framework does not ship any runnable factories. Paths below refer to the combined project `.factory/` installation.

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
| `artifacts` | Optional mapping of output HTML filenames to template paths relative to this factory file. Empty/omitted means no starter documents. |
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
| `loop` | Optional Pi loop launch settings: `prompt`, `approval_artifacts`, and `commands`; described below. |
| `command` | Command runner argv, including `{instructions_file}`. See [other runners](../orchestrator/README.md#other-runners). |

## Providers and credentials

The examples use `team-gateway`, `your-model-id`, and `https://gateway.example.com/v1`. Replace the URL, `defaults.model`, and the provider's `models[].id`. The example protocol is OpenAI-compatible Chat Completions (`openai-completions`); choose the protocol your endpoint actually supports.

`provider` matches a key in `providers`, or a built-in Pi provider. The provider's `apiKey` is `${TEAM_MODEL_API_KEY}`; export that variable in the launching environment. Do not put its value in the file. For a built-in provider, select its ID/model, remove unused custom definitions, and use its native credential mechanism.

Different members may select different providers and models. Only the selected custom provider is emitted into that agent's generated `models.json`. Other provider credentials are not required for that launch. Personal Pi settings are not imported.

## MCPs, skills, and extensions

Define MCP servers once in `mcps`, then select them by name on each member. Define Pi extension packages once in `extensions`; selection controls which profiles load them. The framework adds its local delivery extension for every Pi session. Model/provider choices, skills, third-party tools and extension sources belong to your configuration, not the framework.

Skills are explicit file selections. `session.ts` reads the full selected `SKILL.md` entrypoints into the agent's system instructions, preserving source paths for supporting references. A skill does not install tools or register an MCP server.

Comms is shared infrastructure: `FACTORY_COMMS_URL` and `COMMS_AUTH_TOKEN` are the launch environment's connection settings for both helpers and agents. The launcher applies them to each generated comms entry. They are not per-agent model settings.

## Start and inspect

From an application containing `.factory/`:

```bash
.factory/orchestrator/start.sh --factory team
node .factory/orchestrator/session.ts architect --factory team --dry-run
node .factory/orchestrator/session.ts builder <task-directory>
```

`--factory` is required unless an explicit task selects its recorded factory. There is no global coordinator profile or automatic newest-task selection. A conflicting factory/task pair fails.

`--dry-run` shows the resolved model, thinking, tools, MCP/extension names, skills, and command without writing files or starting an agent. Raw provider configuration and credential values are excluded. `--check` prepares private native files and checks credentials/MCPs without model inference; comms must already be available.

See [how the configuration reaches Pi](../orchestrator/README.md#how-configuration-reaches-pi) for the exact scripts and generated files.

## Dependencies and customization

A team may provide a Makefile with `install` and `update` for its tools. The framework Makefile installs shared runtime dependencies only. No installer schema or package ecosystem is introduced. Install affects the host, not an isolated toolchain.

Edit the recipe in `README.md` for workflow agreements and `factory.json` for execution. Team-owned factories survive [toolkit updates](toolkit-updates.md). Existing sessions retain their prepared configuration; launch a new session to apply changes. No migration guide or compatibility layer is provided for this prototype.

## Task document templates

```json
"artifacts": {
  "design.html": "templates/design.html",
  "plan.html": "templates/plan.html"
}
```

`task.ts attach` creates these documents once. Templates support `$title`, `$brief`, `${title}`, `${brief}`, and `$$` for a literal dollar sign; inserted values are HTML-escaped. Output names must be plain HTML filenames, and templates must resolve to files inside `.factory/`. Task pointers and the reviews directory are framework bookkeeping. Reattaching preserves existing document content. A factory with no artifacts gets no spec or plan implicitly.

## Optional Pi loop launch

A member may set this in `config` (or inherit it from `defaults`):

```json
"loop": {
  "prompt": "/my-loop --iterations={iterations} Work on the assigned task at {task_dir}.",
  "approval_artifacts": ["design.html", "plan.html"],
  "commands": ["my-loop"]
}
```

`--loop N` requires an explicit task, a Pi runner, positive iterations, configured loop settings, and current hash-matching approval of every listed artifact. Those artifacts must exist in the factory's `artifacts` map. The launcher checks the named native commands and waits for an assignment before starting; it substitutes `{iterations}` and `{task_dir}` in the prompt. The selected extension defines actual iteration behavior. No factory or member name activates a loop automatically. This is a launch guard, not a workflow scheduler or a continuous approval monitor.
