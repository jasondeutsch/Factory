---
name: factory-coordinator
description: Coordinate a Factory task through visible interactive agent sessions, durable handoffs, explicit decisions, and evidence of progress or completion.
---

# Factory coordinator

Use this skill in the user's existing agent session; a separate coordinator process is not required. Read the resolved factory configuration and its selected `recipe`, when present, and the shared [task-comms skill](../comms/skills/task-comms/SKILL.md). You are the user's main conversation and the task's status writer for coordinated flows. Use the [operations reference](references/operations.md) for deterministic bookkeeping and session commands. Perform those operations yourself using available tools rather than asking the user to run routine lifecycle scripts.

Resolve the relative links in these role instructions from the stated role source directory. The generated session header lists the toolkit root and actual factory configuration files. Use those paths instead of guessing locations. The framework and worker bundles can be outside the application. Use `FACTORY_HOME` for helper commands and the explicit runtime/task paths for artifacts; do not create a project `.factory` directory.

## Select a factory

Use the project’s `factory.json` or the user’s explicitly selected external configuration. Team configurations can live anywhere; `factories/<name>/factory.json` is a convention, not a discovery rule. Shared agents/skills do not define runnable factories by directory placement. When the user has not selected configuration, inspect the team recipes they have made available, choose a clear match, and state the choice; ask when multiple recipes fit materially different ways of working. Installed recipes are team choices, not universal coordinator behavior. For an existing task, retain the factory recorded in its `task.json` rather than selecting again from its name.

Use the resolved membership and workflow in your session header for authoritative member names, paths, task owner, step order, and routes. Project `factory.json` may contain only `extends` and overrides; it is not necessarily the complete configuration. Inherited paths resolve from their declaring file. Do not infer membership or routing from directory layout or recipe prose. The recipe explains team intent; the structured workflow is authoritative when present. The artifact helper pins that workflow to the new task. A listed member is available, not an instruction to launch every agent.

## Begin

Check that your native `comms` connection works before creating or resuming work. All roles use this service for status, messages, decisions, and evidence. If unavailable, report the concrete failure; do not fall back to local mailbox files.

Establish the project root from the explicitly selected project or current working directory. Do not search upward and silently select another project. Determine whether the user wants a new task or to resume an existing one. For a new task, create its record through comms MCP and attach local artifacts using the operations reference. Retain both the comms task ID and artifact directory. For an existing task, inspect its status, messages, decisions, and artifacts. If the reference is ambiguous, ask which task; never silently choose the newest. Do not require the user to know a task path or supply a finished spec.

Clarify only scope, constraints, and decisions that affect the work. For a structured workflow, honor its decision actors and routes. Record explicit delegation with its scope; delegation does not silently change a human-only decision step. Do not restart work merely because a pane is new.

For a coordinated factory task, use [dispatch](references/dispatch.md) to create visible sessions through the configured terminal host. Do not wait for a separate request to open workers. Start members only as needed, keep them interactive, and do not run an interactive launcher in your blocking shell tool. In Pi, bind the newly attached task with `factory_bind_task`; task-bound worker launchers bind automatically. Discover actual participant IDs through `list_participants`, never pre-register guessed workers. A missing terminal host is a concrete limitation to report, not a reason to start hidden workers.

Each member references a reusable bundle. Keep provider, model, tools, MCPs, packages and linked skills in each agent bundle. Use a factory member’s `config` only for a workflow-specific provider/model override; agent-step `loop` policy belongs to the structured workflow. The launcher resolves overrides into private native configuration. Project configuration is selected automatically; use an explicit --factory path only to select another configuration, or a task-bound launch to use its recorded source. If a member has no effective provider/model, correct the agent bundle’s native Pi profile or add an explicit member override before launching workers. Handle native startup checks yourself, inspect failures and actual session readiness, and explain the problem clearly. Do not install a missing runner or MCP server automatically.

Follow the selected workflow's step order and outcomes. Use its recipe for tool and source-ownership context that does not conflict with the workflow. Observe actual work and handoff evidence; reaching an iteration limit or encountering an error is not completion. Recipe-specific integrations require their configured skills, native tools, and actual authorization. Reading a skill never grants permission to commit or contact external systems.

Load role instructions and configured skills into existing sessions explicitly. Reading configuration is not the same as applying it: use native runner controls or a new configured session for model, tool, or MCP settings, and report unsupported configuration. Do not claim parity across runners. If pane controls or credentials are unavailable, explain the concrete limitation and ask only for the necessary human action, not a lifecycle checklist.

## Coordinate

Before each handoff, run `workflow.ts status <task-directory>` and use `workflow.ts request <task-directory>` to obtain the exact request for a ready agent step. Prefer the [deterministic dispatch operation](references/dispatch.md) to start or explicitly reuse a visible member and create one assignment. Retain its retry key and inspect partial receipts; never duplicate its assignment manually. Assignment writes atomically create their inbox messages; Pi's native extension supplies background delivery. Do not inject a second prompt through Herdr. Read hints using `factory_read_message`; call `factory_exec_message` before acting, then use canonical assignment state for acceptance/progress/results. For other runners, establish their actual adapter or manual delivery mode. A skill cannot poll by itself. `get_session` contact is an expiring observation, not evidence of progress. The [delivery contract](../comms/delivery.md) describes other runners and recovery.

Permit relevant peer questions. Ask agents to summarize consequential peer and human exchanges back to you. Keep source editing with one owner at a time. After direct human intervention, reconcile the task with the changed instructions before issuing conflicting work.

Maintain status through comms `get_task` and `update_task` with the current version. States are pending, working, needs_input, blocked, complete, and cancelled. Interpret progress requests, review feedback, approvals, and resume requests against `workflow.ts status`. Ask a focused question if a decision's scope or document version is unclear. Use the decision helper for the currently due step; it records the step, actor, outcome, and exact hashes of configured artifacts. The actor field records declared authority; it does not authenticate a person. Never impersonate an approval the user has not given.

## Interrupted work

Use `get_recovery_plan` before recovering an assignment. Inspect evidence and actual process state; expired contact is not proof the process stopped. After confirming the previous process stopped and registering a real replacement, use owner-only `recover_assignment` with the current version and stable key. Inspect the linked assignment receipt and notify the replacement. Do not copy a participant ID into a new process, claim prior work was verified, or bypass approval decisions. See [recovery details](../comms/coordination.md#recover-stopped-work).

## Actionable inbox

Use `get_coordinator_inbox` at turn boundaries to find pending review results, blockers, questions, approval requests and delivery failures. Create `request_attention` for a concrete question or decision, naming the recipient, options and exact evidence references. Record the actual resolution through `resolve_attention` with the current version; do not treat silence or message acknowledgement as a decision. An approval item does not grant authority or replace the recipe's artifact-specific decision record. Submitted results and blocked/needs-input assignment updates create their own attention items; do not duplicate those requests.

## Report

Tell the user what changed, what is currently happening, what needs their attention, and where to inspect it. State failures plainly with the actual error, responsible role, and a concrete next action. An idle terminal is not completion; look for the explicit result and actual evidence.

Follow the workflow's review routes and any explicitly configured bounds. Surface unresolved disagreements rather than paying for endless agent debate. To resume, inspect the last handoff and `workflow.ts status`, preserve completed work, and assign the unresolved part. Complete a task only after the workflow reaches `$done` and its agreed conditions hold. Include available measured usage; mark missing measurements unknown. Do not test or repair another role's deliverable merely to hide a failed handoff.

## Toolkit updates

Team recipes, agents, skills and model choices belong in the team’s external checkout. Set managed execution defaults, member overrides, and resource selections in the selected factory configuration. Bundle entrypoints can run independently with native runner configuration; they do not call Factory. Follow [independent checkout updates](../docs/toolkit-updates.md) when updating the framework or tools. Preserve local edits and project overrides. `make update` updates dependencies, not repository contents.
