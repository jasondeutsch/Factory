# Configure a factory

Project `factory.json` is optional. Launch from the application root; Factory and tools remain in external checkouts. A project can extend a team configuration and override only its differences.

```json
{
  "extends": "../tools/factories/golang/factory.json",
  "runtime_dir": "~/factory-runs/my-project"
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
  "entrypoint": "start.sh",
  "config": { "provider": "team-gateway", "model": "review-model" }
}
```

The bundle contains `agent.md`, `start.sh`, a `pi/` native profile and `skills/` links to the team’s canonical skill definitions. Managed sessions start with the bundle’s provider, model, tools, MCPs, extension packages and linked skills. A factory member can override provider/model when necessary; an override to a new custom provider needs a `providers` catalog entry in the factory.

## Shared defaults and member overrides

For a bundle with a native `pi/` profile, those files and its linked skills own its execution configuration. A bundled member accepts only optional `provider` and `model` overrides. Framework launch defaults for `executable` and `check_timeout_seconds` still apply. Unbundled command runners retain the broader legacy configuration fields below. Edit the bundle for changes intended across all uses; never edit generated private profiles.

For example, give the architect `"config": {"provider": "team-gateway", "model": "reasoning-model"}` when this factory needs another model. Define `team-gateway` under `providers` with that model ID if the bundle does not already contain it. A member with no `config` keeps its own native profile; it does not need an empty object.

An agent’s `skills/` links and native `pi/mcp.json` and `pi/settings.json` own its selections. Bundled agents cannot replace skills, MCPs, or extensions from the factory. Pi managed sessions always require `comms`. Unbundled runner definitions retain explicit resource selection for their adapter.

| Factory field | Meaning |
| --- | --- |
| `name` | Logical identifier; independent of directory or filename. |
| `extends` | Optional local base configuration path. |
| `recipe` | Optional human-readable context loaded for the task owner. Workflow order and routing belong in `workflow`. |
| `runtime_dir` | Optional project runtime directory. Default: `~/.factory/<project-name>-<path-hash>/runtime`. `FACTORY_RUNTIME_DIR` overrides it. |
| `owner` | Member responsible for task coordination; defaults to `coordinator`. |
| `terminal_host` | Optional `herdr`, `terminal`, or `iterm`. `FACTORY_TERMINAL_HOST` overrides this; automatic selection uses Herdr when inside it, otherwise Terminal on macOS. |
| `defaults` | Shared execution settings and optional resource selections. |
| `providers` | Optional factory provider definitions for workflow-specific provider/model overrides. Agent bundles define their normal provider in `pi/models.json`. |
| `mcps` | MCP catalog for unbundled runners. Bundled agents use their own `pi/mcp.json`; Factory enables Comms for managed sessions. |
| `extensions` | Package catalog for unbundled runners. Bundles select packages in `pi/settings.json`; Factory adds its delivery extension for managed Pi sessions. |
| `artifacts` | Optional mapping of output HTML filenames to template paths relative to this factory file. Empty/omitted means no starter documents. |
| `workflow` | Optional ordered step graph for coordinated tasks, pinned to each new task. Required by the supplied team factories. |
| `agents` | Members, paths, selections, overrides, and optional handoff requirements. |

| Member field | Meaning |
| --- | --- |
| `name` | Unique role name used for launch and communication. |
| `path` | Directory containing `agent.md`, or `SKILL.md` for the coordinator. Relative to its declaring file, or absolute. |
| `entrypoint` | Optional executable path inside the agent bundle, such as `start.sh`. For Pi runners, Factory calls it with prepared instructions, native profile and session arguments. Without it, Factory calls Pi directly. |
| `config` | For a bundled agent, optional `provider` and `model` only. Unbundled runners use the execution fields below. |
| `skills`, `mcps`, `extensions` | Explicit selections for unbundled runners only; bundled agents own these resources. |
| `handoff` | Legacy assignment evidence defaults for unbundled runners. A bundled agent's evidence requirements belong on its workflow step. |

## Execution settings

These fields remain available for unbundled runners. Bundled agents configure them in their native profile, except for optional member `provider` and `model` overrides:

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

Declare an agent’s MCPs in `agents/<name>/pi/mcp.json` and Pi extension packages in its `pi/settings.json`. Factory reads those files and adds Comms and its delivery extension for managed sessions. Agent models, skills and third-party tools live with the agent bundle.

Each agent’s `skills/` entries link to definitions under the team’s shared `skills/` directory. `session.ts` loads the selected canonical `SKILL.md` files into the managed instructions, preserving their source paths for references. A skill does not install tools or register an MCP server.

Comms is shared infrastructure: `FACTORY_COMMS_URL` and `COMMS_AUTH_TOKEN` are the launch environment's connection settings for both helpers and agents. The launcher applies them to each generated comms entry. They are not per-agent model settings.

## Start and inspect

From the application root, with `FACTORY_HOME` pointing to the framework checkout:

```bash
"$FACTORY_HOME/orchestrator/start.sh"
node "$FACTORY_HOME/orchestrator/session.ts" architect --dry-run
node "$FACTORY_HOME/orchestrator/session.ts" coder_golang <task-directory>
```

Project `factory.json` is selected automatically. `--factory /path/to/factory.json` selects an external configuration instead. `--project /path/to/application` selects an explicit application root. A task records its configuration source for worker launches. There is no global coordinator profile or automatic newest-task selection. A conflicting factory/task pair fails.

`--dry-run` shows the resolved model, thinking, tools, MCP/extension names, skills, and command without writing files or starting an agent. Raw provider configuration and credential values are excluded. `--check` prepares private native files and checks credentials/MCPs without model inference; comms must already be available.

See [how the configuration reaches Pi](../orchestrator/README.md#how-configuration-reaches-pi) for the exact scripts and generated files.

## Dependencies and customization

A team may provide a Makefile with `install` and `update` for its tools. The framework Makefile installs shared runtime dependencies only. No installer schema or package ecosystem is introduced. Install affects the host, not an isolated toolchain.

Edit `factory.json` for step order, routing and execution settings; use `recipe` for explanatory team guidance. Framework and team checkouts are [updated independently](toolkit-updates.md). Existing tasks retain their pinned workflow; existing sessions retain their prepared configuration. Launch a new session to apply agent changes.

## Workflow steps and decisions

`workflow.start` names the first step. Each `workflow.steps` entry has a unique lowercase `id` and a `type` of `agent` or `decision`. The workflow lists all outcome routes explicitly; `$done` and `$blocked` are terminal targets. Missing outcome routes block progression. Unreachable steps, unknown members or artifacts, and invalid targets fail configuration validation.

```json
"workflow": {
  "start": "draft",
  "steps": [
    {
      "id": "draft", "type": "agent", "agent": "architect",
      "title": "Draft proposal", "instructions": "Develop the proposal from the task brief.",
      "inputs": ["task brief", "project source"],
      "outputs": ["spec.html", "plan.html"],
      "on": { "completed": "approval", "failed": "$blocked" }
    },
    {
      "id": "approval", "type": "decision", "actor": "human",
      "artifacts": ["spec.html", "plan.html"],
      "on": { "approve": "$done", "request_changes": "draft", "reject": "$blocked" }
    }
  ]
}
```

Agent results may be `completed`, `ready_for_review`, `changes_requested`, or `failed`. Decisions may be `approve`, `request_changes`, or `reject`. A decision actor is `human` or a named factory member. Its `artifacts` must be declared in the factory's `artifacts` map; an approval records and rechecks their exact hashes. Changing an approved artifact reopens that decision and requires a new approval before another agent step; subsequent work uses a new attempt number. Use the `changes_requested` result and route to loop back after a reviewer finds material problems. Agent steps may also declare `handoff` evidence requirements (`require_artifacts`, `require_checks`, `require_limitations`, and optional exact `artifacts`/`checks` lists) and an optional `loop` launch policy. The agent bundle still owns its skills, model, MCPs and extension packages.

The coordinator checks `node "$FACTORY_HOME/orchestrator/scripts/workflow.ts" status <task-directory>` for the current step. For a ready agent step, `workflow.ts request <task-directory>` emits the exact JSON request for `dispatch.ts --request <file>`. Dispatch rejects requests that differ from the pinned step or arrive out of order. The decision helper accepts only the due decision's actor, routed outcome, and configured artifact set. Task completion through the helper requires the `$done` route. These checks live in the Factory helpers; a client writing directly to Comms must follow the same contract. The coordinator still drives sessions and handles human conversation; this is not an automatic scheduler.

## Task document templates

```json
"artifacts": {
  "design.html": "templates/design.html",
  "plan.html": "templates/plan.html"
}
```

`task.ts attach` creates these documents once. Templates support `$title`, `$brief`, `${title}`, `${brief}`, and `$$` for a literal dollar sign; inserted values are HTML-escaped. Output names must be plain HTML filenames, and templates must resolve to existing files. Task pointers and the reviews directory are framework bookkeeping. Reattaching preserves existing document content. A factory with no artifacts gets no spec or plan implicitly.

## Optional Pi loop launch

An agent step may set this workflow-specific launch policy. The agent bundle owns the Pi extension package and bounded-iteration skill:

```json
"loop": {
  "prompt": "/my-loop --iterations={iterations} Work on the assigned task at {task_dir}.",
  "approval_artifacts": ["design.html", "plan.html"],
  "commands": ["my-loop"]
}
```

`--loop N` requires an explicit task at that agent step, a Pi runner, positive iterations, the step's `loop` settings, and current hash-matching approval of every listed artifact. Those artifacts must exist in the factory's `artifacts` map. The launcher checks the named native commands and waits for an assignment before starting; it substitutes `{iterations}` and `{task_dir}` in the prompt. The agent's extension defines actual iteration behavior. No factory or member name activates a loop automatically. This is a launch guard, not a workflow scheduler or a continuous approval monitor.

## Runtime ownership

Tasks live in `<runtime_dir>/tasks/`; sessions, dispatch receipts and diagnostics live in sibling directories. Project identity uses the canonical application path, so projects with the same name remain separate. Use a distinct runtime directory per project when overriding the default. Task pointers validate project identity before use. Comms SQLite storage remains controlled by Compose volumes, independently of this setting.

## Independent agent entrypoints

An agent entrypoint must not call back into Factory. For a Pi member with `"entrypoint": "start.sh"`, Factory owns configuration resolution, readiness checks, task registration and terminal orchestration. It calls the entrypoint in the application working directory with:

- `AGENT_EXECUTABLE`: selected Pi executable, used for both checks and launch.
- `AGENT_INSTRUCTIONS_FILE`: the complete assembled instructions, including the role and selected skills. The entrypoint must load this once instead of adding its standalone defaults again.
- `PI_CODING_AGENT_DIR`: the prepared native Pi profile.
- Native Pi arguments: session output path, additional permitted options and initial prompt.

The entrypoint forwards arguments unchanged, preserves the working directory and environment, and uses `exec` so signals and exit status belong to the runner. No Factory import, installation path or role name is required in the entrypoint. The path is explicit in member configuration; Factory does not guess from the presence of `start.sh`. Command runners continue to use `config.command` and its placeholders.

Standalone launchers use their bundle’s instructions, linked skills and copied native Pi profile. They do not resolve `factory.json`, register participants or enable Comms by default. Edit the bundle’s `pi/` files for its normal model and MCP selection. This keeps one configuration owner per launch: Pi for standalone sessions, Factory's prepared profile for managed sessions.
