# Factory

A portable toolbox for working with a small team of interactive agents. Tell your agent what you want to accomplish; the coordinator uses the factory recipes, delegates work to visible sessions, and keeps the task record up to date. Small Python helpers support that work behind the scenes. Agents coordinate through a small MCP service; there is no workflow execution engine.

Start here for normal use. [Tools and tradeoffs](docs/tools.md) explains the components and why they are used. [Setup and troubleshooting](docs/troubleshooting.md) covers dependencies, configuration problems, and recovery.

The [comms service](comms/README.md) provides SQLite-backed tasks, inboxes, decisions, evidence, and HTML reports through MCP. Every role references its shared [communication skill](comms/skills/task-comms/SKILL.md) directly. Docker Compose keeps data inside the container by default; persistence is an explicit volume choice.

## Included roles

| Role | Responsibility |
| --- | --- |
| Coordinator | Your main conversation: assigns work, records decisions, tracks status, and reports evidence. Your existing agent can take this role. |
| Architect | Investigates the actual project and drafts `spec.html` and `plan.html`. |
| Architecture reviewer | Challenges assumptions, behavior, feasibility, and complexity before implementation. |
| Go coder | Implements approved work, uses tests and lint during coding, and records progress and handoffs. |
| Go code reviewer | Independently examines source and verifies the result; sends repairs back to the coder. |
| Pair writer | Collaborates interactively on technical and business writing without requiring spec/plan artifacts. |

## Install in a project

Copy this directory's **contents** into `.factory/` at your application project's root. You can ask an agent to do that:

> Copy the contents of `/path/to/factory` into this project's `.factory/` directory. Preserve any existing local customization. Then read `.factory/README.md`.

The helpers require Python 3.10 or newer. Use your existing agent as coordinator. The launcher supports native Pi bundles and explicitly configured commands for other runners. Herdr provides visible panes and notifications; start your coordinating agent inside Herdr when you want it to manage those panes. The [dependency overview](docs/troubleshooting.md#what-must-be-available) distinguishes installed requirements from optional integrations. Agents run on the host. Only the comms service runs in its supplied container.

Ask the coordinator to start `.factory/comms` with Docker Compose and configure its own native MCP connection to `http://127.0.0.1:8090/mcp` (or your selected endpoint). Prepared worker profiles include that connection. Existing sessions need an explicit reload or reconnect; reading a skill does not register tools. Use an explicit volume if task history must survive container replacement.

Before launching workers, ask the coordinator to configure each role's provider and model. For example: “Configure the architect and Go agents to use our gateway at `<base URL>` with model `<model ID>` and the API key from `TEAM_MODEL_API_KEY`. Use the appropriate API protocol for that endpoint.” Each role can use different settings. Starter bundles do not invent model IDs or silently inherit unrelated personal defaults. The Go coder and reviewer also require gopls with MCP support, or an explicit decision to disable that connection.

## Start a configured agent

Every role has an executable `start.sh`. Run it in the terminal or visible Herdr pane where you want that agent to remain interactive:

```bash
.factory/orchestrator/start.sh
.factory/agents/architect/start.sh
.factory/agents/pair_writer/start.sh
```

Without a task argument, the coordinator or writer can establish a task from your conversation; workers wait for an explicit assignment. No newest-task inference occurs. To bind a session at startup, pass its local artifact directory:

```bash
.factory/agents/coder_golang/start.sh .factory/tasks/<task-directory>
.factory/agents/coder_golang/start.sh .factory/tasks/<task-directory> --loop 5
```

The same entrypoints accept `--dry-run` to inspect the resolved launch and `--check` to check native readiness without chat. Start comms first and configure each role's actual provider/model. These scripts resolve their own location, load the role's configuration and local overlays, include every listed skill with its source directory, prepare its native MCP profile, and start in the application root. They do not rely on whichever personal Pi profile happens to be active. They work from another current directory when invoked by absolute path; relative task arguments are relative to your current directory.

All scripts delegate to one [launcher](orchestrator/session.py); they contain no separate model settings, skill lists, or lifecycle logic. Existing sessions still need an explicit reload/reconnect when settings change. See [reliable startup](orchestrator/README.md#start-a-role) for configuration precedence and checks.

## Start a Go task

In your existing agent session:

> Read `.factory/orchestrator/SKILL.md`. Use the Go development factory to start a task: reduce repeated account database reads without changing API behavior. Coordinate the agents in visible Herdr panes. Bring the spec and plan back to me for approval before implementation.

The coordinator creates the task, loads the recipe, and assigns work to the appropriate role sessions. It can use existing sessions or open new visible panes as requested. You do not need to start another coordinator or run task lifecycle commands yourself.

Names can contain uppercase letters and spaces. The agent can take a long brief directly from your conversation or read a file you provide. It should present questions and approval requests in your conversation, linking the relevant documents.

Read [the Go factory](factories/golang.md) for its working sequence. [The Herdr adapter](orchestrator/adapters/herdr/README.md) documents the mechanics for the coordinating agent.

After approval, the coder normally uses a bounded `edxeth/pi-ralph-loop` assignment: one verified plan item per fresh session, durable progress, then an independent code review. You can give feedback in its pane. Decisions and blockers stop the loop. The [Ralph reference](orchestrator/references/ralph.md) explains the integration and its one-active-loop-per-project limit.

To start from Linear, ask: “Use Linear issue TEAM-123 as the source for this task.” The coordinator configures the planner or reviewer's optional official read-only MCP connection using `LINEAR_API_KEY`, then records the relevant ticket context. See [Linear configuration](orchestrator/README.md#linear-ticket-context).

## Write interactively

> Read `.factory/agents/pair_writer/agent.md` and use the writing factory. Help me explain our proposed platform to engineering and finance. Discuss the argument with me before drafting.

Your current agent can act as the pair writer, load its writing skills, and maintain the writing task. Alternatively, ask the coordinator to open a dedicated writer pane. [The writing factory](factories/writing.md) does not require a spec or plan.

## Drive the task through conversation

- “What's happening, and what's blocked?”
- “Show me the architect's unresolved questions.”
- “Approve this spec and plan. Continue with implementation.”
- “Tell the coder to preserve the existing endpoint.”
- “Resume the account lookup task from the last review.”

The coordinator inspects the recorded state, handles messages, records your decisions, and explains the next action. It should not hand you helper commands for routine coordination. You can also enter a worker's pane and give feedback directly; that worker reports material changes back to the coordinator.

Messages are saved in the comms task inbox. Agents check at turn boundaries and can notify peers through Herdr. The mailbox alone does not wake idle sessions, and there is no background dispatcher. When notification is unavailable, the coordinator should explain the limitation and request the smallest necessary human action, such as asking a worker to check its inbox.

## What to customize

- `agents/<role>/agent.md`: responsibilities and authority.
- `agents/<role>/config.json`: runner, native configuration directory, skill paths, and environment references.
- `agents/<role>/pi/settings.json`: native model/provider selection, thinking, tools, and Pi settings.
- `agents/<role>/pi/models.json`: custom providers, endpoints, and model definitions.
- `agents/<role>/pi/mcp.json`: that role's MCP connections and tool exposure.
- `agents/<role>/skills/`: role-specific guidance.
- `agents/<role>/scripts/`: tools specific to that role, such as the Go coder's check script. Only helpers shared across roles belong in the top-level `scripts/` directory.
- `orchestrator/config.json` and `orchestrator/SKILL.md`: coordinator configuration and working agreement.
- `factories/*.md`: sequences, artifacts, approvals, and checks for different work.
- `comms/skills/task-comms/`: shared communication skill, imported by all role configurations.
- `skills/`: shared bounded iteration, conventional commits, evidence-based debugging, Linear context, and Go guidance. Roles explicitly include the skills they need.

The wrapper selects the runner, native configuration directory, skills, and environment references. Model, thinking, tools, providers, and MCPs have one authoritative location in each role's native Pi files. Native `.local.json` overlays can hold personal settings without changing team defaults; credentials remain environment references or explicitly shared private authentication. See [session configuration](orchestrator/README.md).

Skills and prompts are portable text; runner flags, tool registration, and sandboxing are not universal. The supplied adapter prepares Pi sessions. An explicit command adapter can load the same instructions into another CLI. There is no claim of automatic discovery by every agent application.

Reading a role's instructions changes its working guidance; it does not automatically switch the model or register tools in an already running session. The coordinator must use supported native controls or a configured new session when those settings need to be applied.

## Task records and runtime files

Task artifacts live in `.factory/tasks/<UTC-millisecond-timestamp>-<slug>/`: a pointer to the comms task, reviews, progress, and documents. Status, messages, acknowledgements, decisions, events, and usage live in comms. Go tasks start with `spec.html` and `plan.html`; writing tasks choose a document format with the user. Application code belongs in the project's source directories, not in the task directory. HTML files are self-contained templates, with no external scripts. [Task records and recovery](docs/task-records.md) describes each file and how agents resume work.

Private session files and assembled instructions live under `$HOME/.factory/runtime/`, separated by project, task, role, and session. `FACTORY_RUNTIME_DIR` can explicitly override this location. No automatic retention or deletion is performed.

The launcher prepares a writable native profile there, checks local credential readiness and enabled MCP connections, and starts the interactive agent from the project root. Startup failures are recorded for the coordinator. Authored bundle files and global Pi configuration are not rewritten; an explicitly linked private auth file can be updated by Pi's authentication flow.

The included `.gitignore` excludes tasks and local configuration overlays. During installation into a Git project, the coordinator also excludes the plugin's root `.ralph/` runtime directory in the application's root `.gitignore`; a nested toolbox ignore file cannot exclude that sibling. Teams can explicitly version selected task artifacts if appropriate. Keep credentials out of task records and command arguments. There is no automatic redaction of material you paste into a task.

Reviewers must not change application source. This is an instruction boundary, **not an enforced sandbox**: sessions with shell access retain the user's filesystem permissions. Begin with one source-editing agent at a time. No clean branch or Git repository is required, and no commands commit changes.

## Documentation

| Guide | Use it for |
| --- | --- |
| [Tools and tradeoffs](docs/tools.md) | External dependencies, included helpers, skills, communication, and operational limits. |
| [Comms service](comms/README.md) | Docker Compose, optional storage volumes, agent MCP connections, task/message tools, and reports. |
| [Setup and troubleshooting](docs/troubleshooting.md) | Prerequisites, corporate-network configuration, failures, updates, and maintainer verification. |
| [Task records and recovery](docs/task-records.md) | Artifacts, decisions, status, resume, retention, and project boundaries. |
| [Go factory](factories/golang.md) / [Writing factory](factories/writing.md) | The working agreements for each supplied flow. |
| [Session configuration](orchestrator/README.md) | Native model/provider, MCP, credential, skill, and runner configuration. |
| [Coordinator operations](orchestrator/references/operations.md) | Agent-facing helper arguments and bookkeeping. |
| [Herdr adapter](orchestrator/adapters/herdr/README.md) | Visible pane setup, session naming, and notifications. |
| [Ralph integration](orchestrator/references/ralph.md) | Bounded coder assignments, feedback, loop controls, and limitations. |

The operation references serve agents and maintainers; users normally work through conversation.
