# Factories

Each factory has two files:

- `factory.json` defines its name, task owner, and member agents with explicit bundle paths.
- `README.md` describes how those agents work together: artifacts, approvals, checks, and handoffs.

The supplied factories are [Go development](golang/README.md) and [interactive writing](writing/README.md). Reusable agents and skills live in [resources/](../resources/README.md); coordination infrastructure lives in [orchestrator/](../orchestrator/README.md).

## Factory configuration

The [Go factory configuration](golang/factory.json) contains:

```json
{
  "name": "golang",
  "owner": "coordinator",
  "agents": [
    {
      "name": "coordinator",
      "path": "../../orchestrator",
      "config": {}
    },
    {
      "name": "architect",
      "path": "../../resources/agents/architect",
      "config": {}
    },
    {
      "name": "architecture_reviewer",
      "path": "../../resources/agents/architecture_reviewer",
      "config": {}
    },
    {
      "name": "coder_golang",
      "path": "agents/coder_golang",
      "config": {}
    },
    {
      "name": "code_reviewer_golang",
      "path": "agents/code_reviewer_golang",
      "config": {}
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `name` | Factory identifier, matching its directory under `factories/`. |
| `owner` | Member agent name responsible for overall task status. |
| `agents[].name` | Unique role name within this factory, used for launch and coordination. |
| `agents[].config` | Per-factory execution overrides for this member; an empty object uses bundle defaults. |
| `agents[].path` | Bundle directory relative to this `factory.json`. It must remain inside the toolkit and contain `config.json` plus `agent.md` (or `SKILL.md` for `coordinator`). |

Membership comes from this file, not directory placement or a list in prose. A member can reference a factory-specific bundle or reusable toolkit resource. Listing an agent does not automatically launch it; the recipe and coordinator determine when it is needed. The [writing configuration](writing/factory.json) includes its pair writer plus optional coordinator and architecture-reviewer members.

Task creation validates the definition and takes ownership from `owner`; changing this field does not rewrite existing comms task owners. Task attachment checks that the existing comms task has that owner. Bound launches resolve only members of the task's selected factory and include its configuration and recipe in the session instructions. Unknown members, duplicate names, missing bundles, and paths outside the toolkit fail explicitly.

An unassigned coordinator starts from `orchestrator/` before factory selection. Other unassigned agents resolve their name across declared factory memberships; shared references to the same bundle are valid. If factories map the same name to different bundles, supply a task so its factory selects the path.

## Configure a member

Set factory-specific execution settings in the member's `config`, rather than editing the reusable bundle. For example:

```json
{
  "name": "coder_golang",
  "path": "agents/coder_golang",
  "config": {
    "provider": "openrouter",
    "model": "deepseek/deepseek-v4-flash",
    "thinking": "high"
  }
}
```

Each member can use a different configuration. Credentials stay in environment variables or private authentication files, not factory definitions. Starter `config` objects are empty; select actual providers and models for your team.

| Config fields | Behavior |
| --- | --- |
| `provider`, `model`, `thinking`, `tools` | Pi adapter maps these to native `defaultProvider`, `defaultModel`, `defaultThinkingLevel`, and `defaultTools`. |
| `skills` | Replaces the bundle's skill list. Paths are relative to factory.json and must remain inside the toolkit. Retain the comms skill. |
| `env` | Maps child variable names to existing parent variable names; values are references, not secrets. |
| `pi.settings`, `pi.models`, `pi.mcp` | Native Pi object overrides for other settings, custom providers, extensions, and MCP connections. |
| Other launcher fields | `runner`, `config_dir`, `args`, `executable`, `command`, auth-file references, and startup timeout use the [bundle configuration rules](../orchestrator/README.md). `config_dir` stays relative to the bundle. |

For example, `config.pi.mcp.mcpServers.linear.enabled: false` disables that member's Linear connection without changing its bundle. Native override objects merge recursively; lists replace. Factory overrides win over bundle defaults and local overlays. The four shorthand Pi settings above win over the equivalent fields in `pi.settings`. The launcher writes the effective native configuration only to the private runtime profile.

A command runner uses `command` and `env` for its own native configuration. Pi-specific overrides with `runner: "command"` are rejected rather than silently ignored.

To select a factory before a task exists, use:

```bash
.factory/orchestrator/start.sh --factory golang --check
.factory/orchestrator/start.sh --factory golang
```

This applies the coordinator member's config and loads the selected definition/recipe without creating a task. A task-bound session selects its recorded factory automatically; a conflicting `--factory` is rejected. Without either, no factory overrides apply: the coordinator uses its bootstrap bundle and other roles require an unambiguous declared bundle.

## Customize

Edit `factory.json` to change membership, paths, or per-factory execution settings. Edit the referenced bundle only when changing reusable defaults. Edit the recipe for working agreements. Keep lifecycle state and handoffs in comms. See [session configuration](../orchestrator/README.md) for bundle details.

These files configure an agent-driven process, not a workflow execution engine. The task helper currently scaffolds spec/plan HTML only for `golang`; other factories create task pointers and review directories without those documents. The native `--loop` shortcut remains specific to the supplied Pi Go coder. Adding a factory does not create new artifact-scaffolding or loop behavior automatically.

Install the complete toolkit as the project's `.factory/` directory. Factory definitions and reusable resources are separate from generated `.factory/tasks/` artifacts, private runtime profiles, and application source. Resume retains the factory identifier in `task.json`; configuration is read anew for new launches, so edits do not change already-running sessions.

## Factory dependencies

Each factory owns a simple `Makefile` with `install` and `update` targets. Call the root shared target, then list only that factory's additional tools and extensions. Keep these commands aligned with the configured agents; changing `factory.json` does not rewrite installation commands. See [Go](golang/Makefile) and [writing](writing/Makefile) for examples. Installation affects the host, without per-factory version isolation.
