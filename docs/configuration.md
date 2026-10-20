# Configure a factory

Project `factory.json` is optional. Launch from the application root; Factory and tools remain in external checkouts. A project can extend a team configuration and override only its differences.

```json
{
  "extends": "../tools/factories/golang/factory.json",
  "runtime_dir": "~/factory-runs/my-project",
  "agents": [
    { "name": "architect", "config": { "thinking": "high" } }
  ]
}
```

`extends` accepts one local JSON file or directory containing `factory.json`; cycles fail before launch. Nested objects merge, members merge by `name`, and other arrays replace. Workers can be added by name with their own `path`, configuration and resources. To replace the entire team, use a standalone configuration instead of extending one.

Paths resolve from the file that declares them before inheritance is merged. Absolute paths and `~/` work too. `@factory/` points to the framework checkout. This applies to agent paths, skills, templates, recipes, runtime directories, MCP working directories and relative executable/extension paths. MCPs without an explicit `cwd` run in the application root. Command-runner argv remains literal except for documented placeholders.

A standalone configuration includes the built-in coordinator and Comms catalog by default. Without any file, the coordinator uses native Pi provider/model defaults; no workers or recipe are assumed. Explicit configurations do not inherit personal Pi settings. Native authentication still requires environment credentials or the configured private auth-file reference.

To add a worker from another checkout, include a new member in the project's `agents` array:

```json
{
  "name": "security_reviewer",
  "path": "../team-agents/security-reviewer",
  "skills": ["@factory/orchestrator/comms/skills/task-comms", "../team-skills/security"],
  "config": { "thinking": "high", "tools": ["read", "bash", "grep", "find", "ls"] }
}
```

The bundle contains `agent.md`. Its provider/model and selected MCPs inherit the factory defaults unless overridden. MCP definitions and extension packages remain named catalogs; declaring a tool does not automatically enable it for every worker.

## Shared defaults and member overrides

A factory's `defaults` apply to all its members. A member's `config` overrides only what differs. Objects merge recursively; lists and scalar values replace. There are no global model settings, agent-local overrides, or native profile files to edit.

For example, put your provider/model in `defaults` once. Give the architect `"config": {"model": "reasoning-model", "thinking": "high"}` when it needs another model. Add that model to the selected custom provider's `models` list. A member with no `config` uses the defaults; it does not need an empty object.

Member `skills`, `mcps`, and `extensions` select resources. If omitted, their defaults apply. An explicit list replaces the default list, so include `comms` in an MCP override. An empty list clears an optional selection; Pi sessions always require `comms`.

| Factory field | Meaning |
| --- | --- |
| `name` | Logical identifier; independent of directory or filename. |
| `extends` | Optional local base configuration path. |
| `recipe` | Optional Markdown instructions file loaded into every selected session. |
| `runtime_dir` | Optional project runtime directory. Default: `~/.factory/<project-name>-<path-hash>/runtime`. `FACTORY_RUNTIME_DIR` overrides it. |
| `owner` | Member responsible for task coordination. |
| `terminal_host` | Optional `herdr`, `terminal`, or `iterm`. `FACTORY_TERMINAL_HOST` overrides this; automatic selection uses Herdr when inside it, otherwise Terminal on macOS. |
| `defaults` | Shared execution settings and optional resource selections. |
| `providers` | Named custom provider definitions in Pi's native provider format. Built-in providers need no entry. |
| `mcps` | Named native MCP server definitions. Declaring one does not enable it for every agent. |
| `extensions` | Named Pi package objects with a `source` field. Only selected packages are installed/loaded. The local Factory delivery extension is always loaded for Pi. |
| `artifacts` | Optional mapping of output HTML filenames to template paths relative to this factory file. Empty/omitted means no starter documents. |
| `agents` | Members, paths, selections, overrides, and optional handoff requirements. |

| Member field | Meaning |
| --- | --- |
| `name` | Unique role name used for launch and communication. |
| `path` | Directory containing `agent.md`, or `SKILL.md` for the coordinator. Relative to its declaring file, or absolute. |
| `entrypoint` | Optional executable path inside the agent bundle, such as `start.sh`. For Pi runners, Factory calls it with prepared instructions, native profile and session arguments. Without it, Factory calls Pi directly. |
| `config` | Optional execution overrides, using the fields below. |
| `skills` | Paths to skill directories or entrypoint files, relative to its declaring file, or absolute. Include the shared task-comms skill. |
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

From the application root, with `FACTORY_HOME` pointing to the framework checkout:

```bash
"$FACTORY_HOME/orchestrator/start.sh"
node "$FACTORY_HOME/orchestrator/session.ts" architect --dry-run
node "$FACTORY_HOME/orchestrator/session.ts" builder <task-directory>
```

Project `factory.json` is selected automatically. `--factory /path/to/factory.json` selects an external configuration instead. `--project /path/to/application` selects an explicit application root. A task records its configuration source for worker launches. There is no global coordinator profile or automatic newest-task selection. A conflicting factory/task pair fails.

`--dry-run` shows the resolved model, thinking, tools, MCP/extension names, skills, and command without writing files or starting an agent. Raw provider configuration and credential values are excluded. `--check` prepares private native files and checks credentials/MCPs without model inference; comms must already be available.

See [how the configuration reaches Pi](../orchestrator/README.md#how-configuration-reaches-pi) for the exact scripts and generated files.

## Dependencies and customization

A team may provide a Makefile with `install` and `update` for its tools. The framework Makefile installs shared runtime dependencies only. No installer schema or package ecosystem is introduced. Install affects the host, not an isolated toolchain.

Edit the recipe in `README.md` for workflow agreements and `factory.json` for execution. Framework and team checkouts are [updated independently](toolkit-updates.md). Existing sessions retain their prepared configuration; launch a new session to apply changes. No migration guide or compatibility layer is provided for this prototype.

## Task document templates

```json
"artifacts": {
  "design.html": "templates/design.html",
  "plan.html": "templates/plan.html"
}
```

`task.ts attach` creates these documents once. Templates support `$title`, `$brief`, `${title}`, `${brief}`, and `$$` for a literal dollar sign; inserted values are HTML-escaped. Output names must be plain HTML filenames, and templates must resolve to existing files. Task pointers and the reviews directory are framework bookkeeping. Reattaching preserves existing document content. A factory with no artifacts gets no spec or plan implicitly.

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

## Runtime ownership

Tasks live in `<runtime_dir>/tasks/`; sessions, dispatch receipts and diagnostics live in sibling directories. Project identity uses the canonical application path, so projects with the same name remain separate. Use a distinct runtime directory per project when overriding the default. Task pointers validate project identity before use. Comms SQLite storage remains controlled by Compose volumes, independently of this setting.

## Independent agent entrypoints

An agent entrypoint must not call back into Factory. For a Pi member with `"entrypoint": "start.sh"`, Factory owns configuration resolution, readiness checks, task registration and terminal orchestration. It calls the entrypoint in the application working directory with:

- `AGENT_EXECUTABLE`: selected Pi executable, used for both checks and launch.
- `AGENT_INSTRUCTIONS_FILE`: the complete assembled instructions, including the role and selected skills. The entrypoint must load this once instead of adding its standalone defaults again.
- `PI_CODING_AGENT_DIR`: the prepared native Pi profile.
- Native Pi arguments: session output path, additional permitted options and initial prompt.

The entrypoint forwards arguments unchanged, preserves the working directory and environment, and uses `exec` so signals and exit status belong to the runner. No Factory import, installation path or role name is required in the entrypoint. The path is explicit in member configuration; Factory does not guess from the presence of `start.sh`. Command runners continue to use `config.command` and its placeholders.

Standalone launchers use their bundle's instructions/skills and native Pi configuration. They do not resolve `factory.json`, register participants or add Comms. Native model/MCP settings must be configured by the user. This keeps one configuration owner per launch: Pi for standalone sessions, Factory's prepared profile for managed sessions.
