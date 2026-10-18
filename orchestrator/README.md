# Orchestration foundation

This directory owns the reusable coordination mechanics. Its [comms MCP server](comms/README.md) stores task and assignment state, messages, evidence, usage, and reports. The optional [delivery bridge](comms/delivery.md) submits inbox notifications. `session.py`, `adapters/`, and `scripts/` prepare native sessions and support local task artifacts.

Specific factory definitions live under [factories](../factories/README.md); reusable agents and skills live under [resources](../resources/README.md). Those files define the work and its review policy. The coordination protocol does not prescribe Go development or writing. The current launcher still includes a Pi/Ralph-specific `--loop` helper for the supplied Go recipe; runner support is not universal.

This is an implementation reference for coordinating agents and maintainers. Users normally configure and run agents through conversation. The current agent can take the coordinator or pair-writer role without relaunching; native runner settings require actual application through that runner's controls or a new configured session.

## Factory membership

Each factory's `factory.json` declares member names, bundle paths, per-member execution overrides, and the task owner. Paths are relative to that file. See the [configuration reference](../factories/README.md#factory-configuration). The recipe describes the process; it does not configure membership.

A task-bound launch resolves its role from that factory's declaration. Without a task, the coordinator uses its bootstrap bundle; other role names must resolve to one distinct declared bundle across factories. A directory alone does not register an agent. New launches read current definitions; existing sessions retain their prepared configuration.

## Start a role

Use `--factory golang` to apply member configuration before selecting a task. A task argument selects its recorded factory instead. Neither option creates work automatically.

Each agent directory and `orchestrator/` contains an executable `start.sh`. It fixes only the role name and passes arguments unchanged to `orchestrator/session.py`. This keeps configuration behavior in one implementation.

```bash
.factory/resources/agents/architect/start.sh                     # interactive, no selected task
.factory/orchestrator/start.sh --factory golang       # use the Go factory coordinator config
.factory/resources/agents/architect/start.sh <task-directory>    # bind explicit task context
.factory/resources/agents/architect/start.sh --dry-run           # inspect settings and loaded skills
.factory/resources/agents/architect/start.sh --check             # native credential/MCP readiness
```

Replace placeholders with actual paths. Launch in the intended visible pane. Scripts fix the role name; the shared launcher resolves its configured bundle independently of the caller's working directory. Task paths remain explicit. The toolbox must be installed as the application's `.factory/` directory. `start.sh` does not silently create a task, select the newest, start Docker, or install a runner.

Reliable startup follows one path:

1. Resolve the member from `factory.json`, then load its bundle's `config.json` and optional `config.local.json`. The unassigned coordinator uses its bootstrap bundle.
2. Apply the member's `factory.json` config, then resolve and load every configured skill entrypoint, retaining its source directory for references. Missing files stop startup. The same comms skill is referenced by all roles.
3. Merge native `pi/settings.json`, `models.json`, and `mcp.json` with their `.local.json` overlays. Resolve shared service settings and declared environment references.
4. Write a private per-session profile and assembled instructions under `$HOME/.factory/runtime/`. Keep authored profiles and personal Pi defaults separate.
5. Check model credential readiness and enabled MCP connections, then execute the interactive runner from the application root. Ralph additionally checks current approvals and required plugin commands.

An unassigned session receives role and project identity, not inherited task variables from another session. After establishing an assignment, the agent retains the selected task ID, documents, and recipe through comms and conversation. An explicitly bound session receives those in instructions and environment. `--loop` without a task is rejected.

The dry-run result lists loaded skill files, profile source, enabled MCPs, and selected model/thinking without credential values. A regular dry run is offline; a loop dry run reads approval events. `--check` uses real native startup checks and may contact services or install the declared extension, but does not call a model. It is evidence of current configuration readiness, not a guarantee that future inference succeeds. Custom command adapters still need runner-specific MCP setup.

## Bundle layout

Each role has its own native Pi profile, including the coordinator here:

```text
factories/golang/agents/coder_golang/
  agent.md
  config.json
  pi/
    settings.json
    models.json
    mcp.json
  skills/
  scripts/
```

`config.json` describes launch and instruction loading:

```json
{
  "runner": "pi",
  "config_dir": "pi",
  "skills": ["../../../../orchestrator/comms/skills/task-comms", "skills/go-implementation"],
  "args": [],
  "env": {}
}
```

Skill paths and `config_dir` are relative to the role directory and must stay inside the toolbox. Every supplied role explicitly imports the shared `orchestrator/comms/skills/task-comms` skill; retain that entry in skill-list overrides. Other shared skills use explicit paths such as `../../../../resources/skills/conventional-commits`. `executable` can select a specific Pi binary instead of `pi` on PATH. Adjacent `config.local.json` overrides top-level wrapper keys; lists and objects at that level replace previous values.

The launcher appends the full text of each configured skill entrypoint to the role's instructions; it does not depend on native skill auto-discovery. Supporting reference files remain available at the included source paths and should be read only as needed. Adding a directory under `resources/skills/` alone does not include it in a role.

Wrapper options are:

| Field | Meaning |
| --- | --- |
| `runner` | `pi` by default, or `command` for an explicit argv adapter. |
| `config_dir` | Native Pi profile directory, default `pi`; remove or null it for a command adapter. |
| `skills` | Toolbox-contained skill paths relative to this role directory. A local overlay replaces this list. |
| `executable` | Pi executable name or path, default `pi`. |
| `args` | Additional allowed native Pi arguments; command adapters instead use `command`. Relative argument paths use the project working directory. |
| `env` | Child environment variable names mapped to existing parent variable names. This supplements the inherited environment; it is not an allowlist. |
| `auth_file_env` | Environment variable containing an absolute private Pi auth-file path. |
| `mcp_auth_file_env` | Equivalent explicit sharing for native MCP OAuth data. |
| `check_timeout_seconds` | Each native process check's timeout, default 30, greater than zero and at most 300. |
| `command` | Explicit argv for `runner: "command"`, including the instructions-file placeholder. |

The bundle wrapper keeps reusable launch defaults; factory-specific provider, model, thinking, and tools belong in the member's `factory.json` config and are translated into the private native profile. `args` accepts other native flags, such as display preferences and explicit extension paths. Model/tool flags, API keys, batch mode, project-trust overrides, and session selection are rejected there to avoid conflicting configuration or accidentally noninteractive workers.

## Models and providers

Set each member's `config.provider` and `config.model` in `factory.json` for factory-specific choices. `config.thinking` and `config.tools` override native Pi defaults as well. Reusable defaults can live in the bundle's `pi/settings.json` or ignored local overlay. Starter profiles and factory overrides select no provider/model. Personal Pi defaults are not imported implicitly.

Example native settings, with illustrative provider and model values to replace:

```json
{
  "defaultProvider": "team-gateway",
  "defaultModel": "your-model-id",
  "defaultThinkingLevel": "medium",
  "defaultTools": ["read", "bash", "edit", "write", "grep", "find", "ls"],
  "cacheWarming": "off"
}
```

Use a built-in provider when appropriate. A factory-specific custom endpoint belongs under `config.pi.models`; its native object format is the same as a bundle's `pi/models.json`:

```json
{
  "providers": {
    "team-gateway": {
      "baseUrl": "https://gateway.example.com/v1",
      "api": "openai-completions",
      "apiKey": "${TEAM_MODEL_API_KEY}",
      "models": [{"id": "your-model-id"}]
    }
  }
}
```

Choose the actual protocol and capabilities supported by the endpoint. Pi's native model metadata, sampling parameters, and compatibility options remain available; the toolbox does not translate them into another schema. Credentials stay in environment references or private authentication data. Built-in provider environment variables inherit normally. The launcher does not source shell startup files or print resolved keys.

Native `settings.local.json`, `models.local.json`, and `mcp.local.json` recursively override object keys in their corresponding base files; arrays and scalars replace existing values. They are ignored by the supplied `.gitignore`. The launcher copies the profile and supporting resources to the private session directory, preserving relative native resource paths, then writes the effective JSON files there. Keep credentials, logs, and trust state out of authored profiles.

To reuse a private Pi `auth.json`, set wrapper `auth_file_env` to an environment variable containing its absolute path. `mcp_auth_file_env` similarly references a private `mcp-auth.json` for MCP OAuth credentials. The launcher links these files into the private runtime profile; it does not copy them into the project. This explicitly shares native authentication state, including refreshes made by Pi. Otherwise credentials written during a session remain in its runtime profile. Personal Pi configuration, packages, MCPs, and saved authentication are not imported implicitly. Complete any required native OAuth login before retrying a failed check, and explicitly share its private auth file for future sessions.

Wrapper `env` maps child environment names to existing variable names. For example, `{"env":{"NODE_EXTRA_CA_CERTS":"TEAM_CA_FILE"}}` passes the path stored in `TEAM_CA_FILE` to Node. `PI_CODING_AGENT_DIR` is managed by the launcher and cannot be overridden through this mapping.

## MCP connections

Each role owns `pi/mcp.json`. Stdio servers run from the application root by default. Relative MCP `cwd` resolves from that root and becomes absolute in the runtime copy. Use profile-relative paths for native Pi resources and project-relative paths for MCP working directories. No MCP protocol implementation is added to the toolbox.

The launcher does not pass a fixed `--tools` list, allowing native MCP support to activate codemode or tool search. Keep `builtin:mcp` enabled and the intended access mechanism available in native settings. Preflight connectivity does not prove a tool's authorization, safety, or successful use by a model; inspect the running session's actual tools when needed.

The Go coder and reviewer include an enabled stdio gopls connection:

```json
{
  "mcpServers": {
    "gopls": {
      "command": "gopls",
      "args": ["mcp"],
      "description": "Explore and diagnose the Go project using gopls.",
      "exposure": "codemode"
    }
  }
}
```

gopls with MCP support must already be installed on PATH; the toolbox does not install it. Set `enabled: false` when intentionally running without it, for example with this `pi/mcp.local.json` overlay:

```json
{"mcpServers": {"gopls": {"enabled": false}}}
```

Add stdio commands or HTTP URLs, credential environment references, and tool exposure directly using Pi's native format. Missing environment references on enabled servers fail before connection.

### Shared communications

All roles have the `comms` native MCP entry. The launcher uses `FACTORY_COMMS_URL` (default `http://127.0.0.1:8090/mcp`) for both helpers and prepared Pi profiles, and adds a `Bearer ${COMMS_AUTH_TOKEN}` header reference when the shared token is set. These settings and session identity cannot be remapped per role. Start the service through its [Compose setup](comms/README.md) before creating tasks or starting sessions.

The shared skill lives in [comms/skills/task-comms](comms/skills/task-comms/SKILL.md); each role configuration references it directly rather than copying it. It describes tasks, inboxes, handoffs, decisions, evidence, usage, and reports. The launcher supplies the project, role, task ID, and local document directory in instructions and environment. Comms holds authoritative state; local task files hold documents and a service link.

An existing coordinator or pair writer needs its own runner connection configured and reloaded. A custom command adapter must configure that runner's MCP transport itself; instructions alone cannot register tools.

## Linear ticket context

The architect, architecture reviewer, and Go code reviewer include a disabled optional Linear server. To activate it for a requested ticket, the coordinator ensures `LINEAR_API_KEY` is available in the agent environment and adds this native `pi/mcp.local.json` overlay for the relevant role:

```json
{
  "mcpServers": {
    "linear": {"enabled": true}
  }
}
```

The base entry uses `https://mcp.linear.app/mcp/readonly`, `Authorization: Bearer ${LINEAR_API_KEY}`, and codemode exposure. This official endpoint exposes only read tools. A Linear API key with only Read permission further limits credentials. No intermediary server, npm bridge, or Linear client implementation is required. Keep the key in the environment; the launcher preserves the reference in private configuration rather than writing its value. When the connection is disabled, missing Linear credentials do not block other tasks.

The [Linear context skill](../resources/skills/linear-context/SKILL.md) covers source attribution, conflicting requirements, and ticket changes after approval. Reading tickets does not change their workflow status. See [Linear's official MCP documentation](https://linear.app/docs/mcp) for authentication and workspace access.

## Startup and runtime state

The coordinator can inspect a launch without creating files or connecting:

```bash
python3 .factory/orchestrator/session.py architect <task-path> --dry-run
```

The output includes instructions and configuration summaries, not raw provider/MCP configuration or resolved environment values. An unset provider or model is reported as `configured: false`.

For native startup checks without starting chat:

```bash
python3 .factory/orchestrator/session.py architect <task-path> --check
```

The live launcher performs the same checks automatically before starting the interactive runner in its visible pane:

1. Prepare writable files under `$HOME/.factory/runtime/<project>/<task>/<role>/<session>/` and set `PI_CODING_AGENT_DIR` to the private profile.
2. Ask Pi to resolve the selected model and credentials using `auth check --no-refresh`. This makes no model inference request and does not prove the remote endpoint accepts the credentials.
3. Ask Pi to connect to configured MCPs using `mcp list --json`; record connection state, exposure, and advertised tools. These checks start server processes or contact their URLs, but do not call the advertised tools.
4. Start fresh interactive Pi in the application root. `--no-approve` prevents project `.pi` settings and MCP files from replacing the role's profile; repository context instructions still load normally.

Each check has a 30-second process timeout. Optional wrapper `check_timeout_seconds` can change it up to 300 seconds. On POSIX, a timed-out check and its process group are terminated. Failure prevents chat startup, records a task event, and names the private `startup-checks.json` diagnostic file. Earlier configuration and approval validation failures are reported on stderr before native checking and need not produce that event. Native output stays private because it can contain sensitive configuration. The coordinator should inspect it carefully and explain the actual problem without exposing credentials.

Checks close their MCP connections; interactive Pi reconnects normally. Availability can change after preflight. Runtime copies are not deleted automatically. Resume through Pi's retained-session mechanism using its associated runtime profile, then reconcile task state.

## Task artifacts and recovery

Comms holds authoritative task state; local files hold documents and a service link. Each task gets `.factory/tasks/<UTC-millisecond-timestamp>-<slug>/`, containing `task.json`, a `reviews/` directory, and factory-specific documents. `task.json` links the local directory to its comms task ID. Go tasks scaffold draft `spec.html` and `plan.html`; implementation progress belongs in `progress.md`. Writing tasks choose their own document format. Display names may contain spaces and uppercase letters; only directory slugs are normalized.

The Compose `comms-data` volume stores SQLite history and survives container replacement and `docker-compose down`. `down -v` deletes it; local documents cannot reconstruct that history. See [database storage and backup](comms/README.md#shared-database-volume). Private session files live under `$HOME/.factory/runtime/<project>/<task>/<role>/<session>/`; `FACTORY_RUNTIME_DIR` selects another absolute root. Documents and private sessions are not automatically deleted. Moving a checkout changes its path-based project identity; cross-machine remapping is manual.

Ask the coordinator to resume a named task. It finds the project-scoped service record and matching local pointer, then reconciles inboxes, events, reviews, and progress before assigning more work. See [coordinator operations](references/operations.md#connect-and-establish-a-task). Reuse an existing session when practical: `start.sh` creates a fresh session, while native runner resume uses the retained transcript and its private profile. For an interrupted loop, inspect its task and assignment before following [Ralph recovery](references/ralph.md#state-and-resume).

## Ralph implementation loops

The coder's native settings declare the pinned `edxeth/pi-ralph-loop` package. Pi can install that declared dependency in the private profile; normal launcher credential/MCP checks do not load extensions. For a bounded coder launch, `session.py coder_golang <task-path> --loop 5` additionally loads the real runtime and checks Ralph command registration without sending a model prompt, then starts the loop in the interactive pane. This extra check can download the declared package and needs Git/network access unless a local source is configured. Its process timeout uses `check_timeout_seconds` too.

The loop checks current HTML document approvals at launch and uses the shared bounded-iteration skill. The plugin's root `.ralph/` state permits one active loop per workspace. Progress stays with the task, and replacement session transcripts stay private. See the [Ralph reference](references/ralph.md) for assignment, visibility, stop/resume behavior, and limits. This adds no Factory loop engine or automatic commit behavior.

## Other agent CLIs

Set `runner: "command"`, remove or null `config_dir`, and provide an explicit native argv list. Supported placeholders are `{instructions_file}`, `{project_root}`, `{task_dir}`, and `{session_dir}`. The command must load `{instructions_file}`, containing role instructions and skill bodies. Configure models and MCPs through that runner's own files and arguments. The launcher does not invent equivalent flags or apply Pi files to another runner.

The command adapter executes without a shell in the project root. It does not enforce interactivity, sandboxing, or native configuration checks. `--check` is only provided by the Pi adapter. All runners use the shared task mailbox contract.

Native format references: [Pi models](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models.md), [Pi MCP](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md), and [gopls MCP](https://go.dev/gopls/features/mcp).

## Extend a role or factory

To add shared guidance, create a focused `resources/skills/<name>/SKILL.md` and explicitly list it in the consuming roles' `config.json`. Keep agent-specific scripts with their role. To add a reusable role, supply `resources/agents/<role>/agent.md`; for a factory-specific role, use `factories/<factory>/agents/<role>/agent.md`. Include an adjacent `config.json` and the native profile files for its chosen adapter; add its name and relative bundle path to the relevant `factory.json`, then describe its assignments and handoffs in the recipe.

Recipes in `factories/<factory>/README.md` are instructions interpreted by the coordinator. The task helper requires a valid `factory.json`, takes ownership from its `owner`, and scaffolds HTML drafts only for `golang`. Other factories receive a task pointer and reviews directory without HTML drafts. The `--loop` shortcut is specialized to `coder_golang` in `golang`. Adding Markdown does not generate another executable workflow or generalize these shortcuts automatically.

The [tool guide](../docs/tools.md) explains these tradeoffs.
