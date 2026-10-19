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

Each member's factory.json `config` sets factory-specific execution choices. Its referenced bundle supplies reusable instructions, skill/runner defaults, and native profile defaults. Reusable bundles live in `resources/agents/`; factory-specific bundles live with their factory. Paths in `factory.json` are relative to that file. [Session configuration](orchestrator/README.md) explains overrides and startup behavior.

## Install in a project

Copy this directory's contents into `.factory/` at the application root, preserving existing customizations. You can ask your agent:

> Install `/path/to/factory` into this project's `.factory/` directory. Read its README and coordinator skill. Configure the Go factory for our provider and model; keep credentials in environment variables.

With Homebrew already available, `make install` installs Herdr, Git, Go, Python, golangci-lint, gopls, Pi, and the Go coder's declared extensions. Install Docker with Compose separately. `make update` updates those tools and reconciles dependencies; Ralph remains at its declared Git revision. See [setup and troubleshooting](docs/troubleshooting.md) for prerequisites, corporate-network configuration, and updates.

Set each needed member's `config.provider` and `config.model` in factory.json and provide credentials before launch. Starter profiles have no selected provider or model. The Go coder and reviewer also require gopls with MCP support, or an explicit decision to disable it.

## Start the coordinator

From the application root, select the factory whose coordinator settings should apply:

```bash
.factory/orchestrator/start.sh --factory golang
```

The entrypoint starts local Compose services and waits for MCP connectivity. Set `FACTORY_COMMS_URL` to use an existing service without managing Compose. Run inside Herdr for visible panes and a supervised delivery bridge; outside Herdr it explicitly reports that inbox polling is required. The bridge stops when the coordinator exits; Compose services remain running. No task or worker starts automatically. `make -C .factory start` is the shortcut for an unselected coordinator using its bootstrap bundle settings; it does not select a factory or apply member overrides.

You can also use an existing agent as coordinator: read `.factory/orchestrator/SKILL.md`, connect its native MCP client to `http://127.0.0.1:8090/mcp`, and reload it. Reading a skill alone does not register tools. The supplied worker profiles already declare this connection.

Use `.factory/orchestrator/start.sh --factory golang --check` to check the selected member's readiness without model inference.

## Request work

> Use the Go factory to reduce repeated account database reads without changing API behavior. Coordinate visible agents and bring the spec and plan back to me for approval.

The [Go factory](factories/golang/README.md) uses architecture drafting/review, human spec/plan approval, bounded implementation, and independent code review. Lint and tests are tools used during coding. The coordinator selects declared members as needed, creates assignments, and reports blockers and results. Decisions remain with you unless explicitly delegated.

For writing:

> Use the writing factory to explain our proposed platform to engineering and finance. Discuss the argument with me before drafting.

The [writing factory](factories/writing/README.md) uses the interactive pair writer without mandatory spec/plan documents. Both factories can start from conversation; you do not need to prepare a request file or operate task lifecycle scripts. [Linear context](orchestrator/README.md#linear-ticket-context) is optional when requested and configured.

## Observe and give feedback

Ask the coordinator for progress, unresolved questions, evidence, or the task report. Enter a worker's pane to give feedback directly; consequential changes should be reported back to the coordinator.

Each bundle has a thin `start.sh` for manual startup. An explicit task-directory argument binds the session to that task's factory. `--dry-run` previews configuration; `--check` checks native readiness without model inference. See [agent startup](orchestrator/README.md#start-a-role).

Stored messages do not wake idle agents by themselves. The optional [Herdr delivery bridge](orchestrator/comms/delivery.md) supplies background inbox hints. Manual inbox checks remain available. Notification, reading, assignment acceptance, and submitted results are separate records.

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
