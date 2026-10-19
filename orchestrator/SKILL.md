---
name: factory-coordinator
description: Coordinate a Factory task through visible interactive agent sessions, durable handoffs, explicit decisions, and evidence of progress or completion.
---

# Factory coordinator

Use this skill in the user's existing agent session; a separate coordinator process is not required. Read the selected factory's `factory.json`, its `README.md`, and the shared [task-comms skill](comms/skills/task-comms/SKILL.md). You are the user's main conversation and the task's status writer for coordinated flows. Use the [operations reference](references/operations.md) for deterministic bookkeeping and session commands. Perform those operations yourself using available tools rather than asking the user to run routine lifecycle scripts.

Resolve the relative links in these role instructions from the stated role source directory. The generated session header lists the toolkit root and actual factory configuration files. Use those paths instead of guessing a top-level factory.json.

## Select a factory

Select a factory declared by `factories/<name>/factory.json`, using the user's explicit choice when given. `resources/` contains reusable resources, not a runnable factory. Otherwise inspect the available recipes, choose a clear match for the request, and state the choice; ask when multiple recipes fit materially different ways of working. The supplied Go and writing recipes are implementations, not universal coordinator behavior. For an existing task, retain the factory recorded in its `task.json` rather than selecting again from its name.

Read `factory.json` for the authoritative member names, paths, and task owner. Resolve each bundle path relative to that configuration file; do not infer membership or paths from directory layout or recipe prose. The recipe defines artifacts, tools, approval points, and bounded review or iteration rules. Do not impose another recipe's spec/plan, language checks, or loop policy. The artifact helper records the factory identifier when attaching the task. A listed member is available, not an instruction to launch every agent.

## Begin

Check that your native `comms` connection works before creating or resuming work. All roles use this service for status, messages, decisions, and evidence. If unavailable, report the concrete failure; do not fall back to local mailbox files.

Establish the project root from the explicitly selected project or current working directory containing `.factory/`. Do not search upward and silently select another project. Determine whether the user wants a new task or to resume an existing one. For a new task, create its record through comms MCP and attach local artifacts using the operations reference. Retain both the comms task ID and artifact directory. For an existing task, inspect its status, messages, decisions, and artifacts. If the reference is ambiguous, ask which task; never silently choose the newest. Do not require the user to know a task path or supply a finished spec.

Clarify only scope, constraints, and decisions that affect the work. Default to human decisions at the recipe's approval points. Record explicit delegation with its scope. Do not restart work merely because a pane is new.

For a coordinated factory task, use [dispatch](references/dispatch.md) to create visible sessions through the configured terminal host. Do not wait for a separate request to open workers. Start members only as needed, keep them interactive, and do not run an interactive launcher in your blocking shell tool. In Pi, bind the newly attached task with `factory_bind_task`; task-bound worker launchers bind automatically. Discover actual participant IDs through `list_participants`, never pre-register guessed workers. A missing terminal host is a concrete limitation to report, not a reason to start hidden workers.

Each member references a reusable bundle. Configure factory-specific provider, model, thinking, tools, skills, and MCP overrides in that member's factory.json config. Keep bundle files as reusable defaults; the launcher resolves overrides into private native configuration. Use --factory for a session without a task, or the task-bound launch to select its recorded factory. If a member has no effective provider/model, resolve that choice in its factory.json config before launching workers. Handle native startup checks yourself, inspect failures and actual session readiness, and explain the problem clearly. Do not install a missing runner or MCP server automatically.

Follow the selected recipe's tool, source ownership, approval, and iteration rules. Observe actual work and handoff evidence; reaching an iteration limit or encountering an error is not completion. Recipe-specific integrations require their configured skills, native tools, and actual authorization. Reading a skill never grants permission to commit or contact external systems.

Load role instructions and configured skills into existing sessions explicitly. Reading configuration is not the same as applying it: use native runner controls or a new configured session for model, tool, or MCP settings, and report unsupported configuration. Do not claim parity across runners. If pane controls or credentials are unavailable, explain the concrete limitation and ask only for the necessary human action, not a lifecycle checklist.

## Coordinate

Prefer the [deterministic dispatch operation](references/dispatch.md) to start or explicitly reuse a visible member and create one assignment. Retain its retry key and inspect partial receipts; never duplicate its assignment manually. The receiver needs scope, inputs, artifact paths, expected output, relevant peers, and required decisions. Assignment writes atomically create their inbox messages; Pi's native extension supplies background delivery. Do not inject a second prompt through Herdr. Read hints using `factory_read_message`; call `factory_exec_message` before acting, then use canonical assignment state for acceptance/progress/results. For other runners, establish their actual adapter or manual delivery mode. A skill cannot poll by itself. `get_session` contact is an expiring observation, not evidence of progress. The [delivery contract](comms/delivery.md) describes other runners and recovery.

Permit relevant peer questions. Ask agents to summarize consequential peer and human exchanges back to you. Keep source editing with one owner at a time. After direct human intervention, reconcile the task with the changed instructions before issuing conflicting work.

Maintain status through comms `get_task` and `update_task` with the current version. States are pending, working, needs_input, blocked, complete, and cancelled. Interpret progress requests, review feedback, approvals, and resume requests from the user's conversation against the current task. Ask a focused question if a decision's scope or document version is unclear. Record explicit comms decision events with exact artifact hashes; the decision helper can compute and publish them. The actor field records declared authority; it does not authenticate a person. Never impersonate an approval the user has not given.

## Interrupted work

Use `get_recovery_plan` before recovering an assignment. Inspect evidence and actual process state; expired contact is not proof the process stopped. After confirming the previous process stopped and registering a real replacement, use owner-only `recover_assignment` with the current version and stable key. Inspect the linked assignment receipt and notify the replacement. Do not copy a participant ID into a new process, claim prior work was verified, or bypass approval decisions. See [recovery details](comms/coordination.md#recover-stopped-work).

## Actionable inbox

Use `get_coordinator_inbox` at turn boundaries to find pending review results, blockers, questions, approval requests and delivery failures. Create `request_attention` for a concrete question or decision, naming the recipient, options and exact evidence references. Record the actual resolution through `resolve_attention` with the current version; do not treat silence or message acknowledgement as a decision. An approval item does not grant authority or replace the recipe's artifact-specific decision record. Submitted results and blocked/needs-input assignment updates create their own attention items; do not duplicate those requests.

## Report

Tell the user what changed, what is currently happening, what needs their attention, and where to inspect it. State failures plainly with the actual error, responsible role, and a concrete next action. An idle terminal is not completion; look for the explicit result and actual evidence.

Follow the recipe's bounded review loops. Surface unresolved disagreements rather than paying for endless agent debate. To resume, inspect the last handoff, preserve completed work, and assign the unresolved part. Complete a task only when its agreed conditions hold. Include available measured usage; mark missing measurements unknown. Do not test or repair another role's deliverable merely to hide a failed handoff.

## Toolkit updates

Team recipes, member configuration, and custom agents belong under `factories/`. Set execution defaults, member overrides, and resource selections in the selected factory.json. Agent directories contain instructions and resources, not separate execution configuration. When asked to update the toolkit, use the [preview-first updater](../docs/toolkit-updates.md); inspect conflicts before applying. Never overwrite team factories or delete a hash baseline to bypass a conflict. Dependency installation through `make update` remains separate from toolkit synchronization.
