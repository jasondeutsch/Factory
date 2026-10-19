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

For a requested coordinated factory task inside Herdr, reuse appropriate visible role sessions or create and prepare them as needed using [the Herdr adapter](adapters/herdr/README.md). Do not wait for a separate request to open worker panes. Outside Herdr, report that host limitation before dispatching work. Start sessions only as needed and keep them interactive. Each role's `start.sh` uses the shared launcher to load its configuration and skills; omit the task only for an intentionally unassigned session. Do not execute an interactive worker launcher in your own blocking shell tool. Register the task owner with its actual instance ID when the coordinator started unassigned. Task-bound worker launchers register themselves before chat starts; discover their actual participant IDs through `list_participants`. Never pre-register guessed worker instance IDs or assign work before that registration exists. Include actual live agent names and pane IDs only when known; these are host delivery details, not the communication identity. Do not guess IDs, silently start hidden workers, or install extensions automatically.

Each member references a reusable bundle. Configure factory-specific provider, model, thinking, tools, skills, and MCP overrides in that member's factory.json config. Keep bundle files as reusable defaults; the launcher resolves overrides into private native configuration. Use --factory for a session without a task, or the task-bound launch to select its recorded factory. If a member has no effective provider/model, resolve that choice in its factory.json config before launching workers. Handle native startup checks yourself, inspect failures and actual session readiness, and explain the problem clearly. Do not install a missing runner or MCP server automatically.

Follow the selected recipe's tool, source ownership, approval, and iteration rules. Observe actual work and handoff evidence; reaching an iteration limit or encountering an error is not completion. Recipe-specific integrations require their configured skills, native tools, and actual authorization. Reading a skill never grants permission to commit or contact external systems.

Load role instructions and configured skills into existing sessions explicitly. Reading configuration is not the same as applying it: use native runner controls or a new configured session for model, tool, or MCP settings, and report unsupported configuration. Do not claim parity across runners. If pane controls or credentials are unavailable, explain the concrete limitation and ask only for the necessary human action, not a lifecycle checklist.

## Coordinate

Inside Herdr, prefer the [deterministic dispatch operation](references/dispatch.md) to start or explicitly reuse a visible member, discover its actual participant, create one assignment, and notify it. Keep its retry key and inspect partial receipts; never manually duplicate the assignment it created. For other hosts or manually managed sessions, create explicit work through comms `create_assignment`, using registered participant IDs and retaining the returned assignment ID/version. The receiver needs scope, inputs, artifact paths, expected output, relevant peers, and next decision. Inspect `get_assignment`/`list_assignments` for explicit acceptance, decline, progress, blockers, and submitted results; reading acknowledgement alone is not acceptance. Assignment operations generate their inbox messages and events atomically; do not manually duplicate them. Establish the host's actual delivery mode. Use automatic delivery only when a working integration exists; otherwise check inboxes at active turn boundaries and notify known live sessions through available controls. Notification is separate from the durable message. If an idle recipient cannot be notified, tell the human which participant needs prompting. The optional [host delivery bridge](comms/delivery.md) can notify idle Herdr sessions using captured participant bindings; the supplied coordinator entrypoint supervises it inside Herdr. Existing external coordinators can run it separately. Avoid duplicate bridges or manual prompts when enabled. For background delivery, capture the actual live session and attach its reference to the existing participant with `bind_participant`; do not replace a participant merely to add its host binding. Without a running bound bridge, use the adapter's one-shot notification after each stored handoff, including results addressed to this coordinator. Inspect delivery failures before yielding; a stored message is not a completed notification. A skill does not poll by itself. The bound bridge records expiring host contact through `get_session`; distinguish that observation from meaningful assignment progress.

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

Team recipes, member configuration, and custom agents belong under `factories/`. Use member overrides or `config.local.json` for local customization of shared bundles. When asked to update the toolkit, use the [preview-first updater](../docs/toolkit-updates.md); inspect conflicts before applying. Never overwrite team factories or delete a hash baseline to bypass a conflict. Dependency installation through `make update` remains separate from toolkit synchronization.
