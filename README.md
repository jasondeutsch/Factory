# Factory

A portable toolkit for coordinating interactive agents. An agent follows your team's factory recipe, delegates work to visible sessions, and records assignments, decisions, and evidence through an MCP service. You drive the task through conversation.

## Layout and configuration

```text
orchestrator/       # Coordinator, MCP server, launchers, delivery, and infrastructure tests
resources/          # Reusable architect/reviewer agents and shared skills
factories/
  golang/           # Go factory definition, recipe, agents, skills, templates, and tests
  writing/          # Writing factory definition, recipe, and pair-writer agent
docs/               # Setup, troubleshooting, and tool tradeoffs
```

**Each factory's `factory.json` declares its member names, bundle paths, and task owner.** Its `README.md` describes how those members work together. Directory placement does not define membership. See the [configuration reference](factories/README.md), [Go configuration](factories/golang/factory.json), and [writing configuration](factories/writing/factory.json).

`factory.json` is the single authored configuration source: set shared execution choices in `defaults`, define providers/MCPs/extensions once, and override only what differs for a member. Agent directories hold instructions, skills, and scripts. [How configuration reaches Pi](orchestrator/README.md#how-configuration-reaches-pi) explains how `session.ts` generates each agent's private native files.

## Install in a project

Copy this directory's contents into `.factory/` at the application root, preserving existing customizations. You can ask your agent:

> Install `/path/to/factory` into this project's `.factory/` directory. Read its README and coordinator skill. Configure the Go factory for our provider and model; keep credentials in environment variables.

With Homebrew already available, install only the factory you need: `make -C factories/golang install` or `make -C factories/writing install`. Each includes the shared coordinator dependencies. The root `make install` installs only Herdr, Git, Go, Node.js 22.18+ (for the TypeScript helpers), Pi, and the npm/comms dependencies. The Go factory additionally installs golangci-lint, gopls, and the coder's pinned Ralph extension; writing adds no extra dependencies. Use `update` in the same directory to update that set. Install Docker with Compose separately. See [setup and troubleshooting](docs/troubleshooting.md) for prerequisites and corporate-network configuration.

The launchers require Node.js 22.18+ and the locked npm dependencies in each toolkit copy (`npm ci --omit=dev`). The Makefile installs those dependencies.

The examples select `team-gateway` and `your-model-id` in `defaults`, with a placeholder URL and `${TEAM_MODEL_API_KEY}` in `providers`. Replace these shared values once, then add member overrides only when needed. See the [factory reference](factories/README.md). The Go coder and reviewer also need gopls with MCP support.

## Start the coordinator

From the application root, select the factory whose coordinator settings should apply:

```bash
.factory/orchestrator/start.sh --factory golang
```

The entrypoint starts local Compose services and waits for MCP connectivity. Set `FACTORY_COMMS_URL` to use an existing service without managing Compose. Pi sessions load a native inbox extension that polls independently of model turns. Choose Herdr panes or macOS Terminal/iTerm windows with `FACTORY_TERMINAL_HOST`. Compose stays running when the coordinator exits. No task or worker starts automatically. `make -C .factory start FACTORY=golang` is the equivalent shortcut. Factory selection is explicit; there is no global coordinator configuration.

You can also use an existing agent as coordinator: read `.factory/orchestrator/SKILL.md`, connect its native MCP client to `http://127.0.0.1:8090/mcp`, and reload it. Reading a skill alone does not register tools. The supplied factories select this connection for their members.

Use `.factory/orchestrator/start.sh --factory golang --check` to check the selected member's readiness without model inference.

## Request work

> Use the Go factory to reduce repeated account database reads without changing API behavior. Coordinate visible agents and bring the spec and plan back to me for approval.

The [Go factory](factories/golang/README.md) uses architecture drafting/review, human spec/plan approval, bounded implementation, and independent code review. Lint and tests are tools used during coding. The coordinator selects declared members as needed, creates assignments, and reports blockers and results. Decisions remain with you unless explicitly delegated.

For writing:

> Use the writing factory to explain our proposed platform to engineering and finance. Discuss the argument with me before drafting.

The [writing factory](factories/writing/README.md) uses the interactive pair writer without mandatory spec/plan documents. Both factories can start from conversation; you do not need to prepare a request file or operate task lifecycle scripts. [Linear context](orchestrator/README.md#credentials-and-mcps) is optional when requested and configured.

## Observe and give feedback

Ask the coordinator for progress, unresolved questions, evidence, or the task report. Enter a worker's pane to give feedback directly; consequential changes should be reported back to the coordinator.

Each bundle has a thin `start.sh` for manual startup. An explicit task-directory argument binds the session to that task's factory. `--dry-run` previews configuration; `--check` checks native readiness without model inference. See [agent startup](orchestrator/README.md#start-a-role).

Pi's [native delivery extension](orchestrator/comms/delivery.md) brings inbox notifications into running sessions. It works independently of the terminal host. Message states are **sent → read → exec**; execution means the recipient declared it began acting, not that the work finished. A stopped process still needs to be restarted by its host. Other runners can implement the same MCP contract or check inboxes manually.

## Records and limits

The [orchestrator MCP server](orchestrator/comms/README.md), called `comms` in agent connections, stores task status, assignments, messages, decisions, events, and measured usage. The same Compose file starts its read-only SQLite admin panel at `http://127.0.0.1:8091/`. Both share a named volume that retains task history across container replacement; `docker-compose down -v` explicitly deletes it.

Local documents live in `.factory/tasks/`, created when needed. Private profiles and transcripts live under `$HOME/.factory/runtime/`. Application changes belong in the application workspace. See [task artifacts and recovery](orchestrator/README.md#task-artifacts-and-recovery) for identities, storage, resume, and retention.

This is an agent-driven process, not a workflow execution engine or enforced sandbox. Agents retain host permissions; prompts do not isolate reviewers or authenticate approvals. Missing usage remains unknown. Git is optional for application work, and committing requires authorization. [Tools and tradeoffs](docs/tools.md) describes these boundaries.

## References


- [Factory configuration](factories/README.md): membership, paths, ownership, and customization.
- [Session configuration](orchestrator/README.md): profiles, skills, providers, and MCPs.
- [Coordinator operations](orchestrator/references/operations.md): agent-facing helpers and bookkeeping.
- [Participants and assignments](orchestrator/comms/coordination.md): lifecycle, ownership, retries, and recovery.
- [Setup and troubleshooting](docs/troubleshooting.md): dependencies, failures, and developer tests.
- [Ralph integration](orchestrator/references/ralph.md): bounded iteration and recovery limits.

For reliable coordinator handoffs, see [dispatch and recovery](orchestrator/references/dispatch.md). For infrastructure evidence and the separate live acceptance run, see [coordination verification](orchestrator/tests/README.md).

Install or update a project copy with the [toolkit updater](docs/toolkit-updates.md). Team-owned `factories/` stays separate from maintained orchestration/resources; updates preview changes and stop on conflicting edits.
