# Factory issues

Factory uses an orchestrator-driven approach. The orchestrator agent interprets human-readable recipes, assigns work, handles decisions, and evaluates results. Supporting code provides communication, delivery, session visibility, and evidence.

Keep SQLite, MCP, thin `start.sh` entrypoints, native runner configuration, and human-readable recipes. Do not introduce a workflow DSL, scheduler, plugin framework, or package ecosystem. Runner-specific delivery stays in small adapters; the communication contract remains agent-neutral.

## FACT-001 — Structured assignments and task participants

**Status:** Implemented · **Priority:** First

Implemented in the comms service with structured MCP operations, transactional notifications/events, version/idempotency checks, explicit instance replacement, schema-1 upgrade, reports, and updated skills. See [coordination reference](comms/coordination.md). Delivery and live presence remain FACT-002/FACT-003.

### Problem

Comms stores tasks, messages, and events, but assignment identity, acceptance, ownership, results, and participant information currently depend on prose or JSON embedded in message text. The orchestrator cannot query outstanding assignments or validate a handoff without interpreting those conventions. Role names also do not distinguish separate running instances.

### Proposed scope

- Add task-scoped participant records with a unique participant/instance identity, role, routing identity, and optional host session reference. Registration declares identity; it does not authenticate it or prove ongoing availability.
- Add assignments with a stable ID, task, assigning participant, assignee, instructions, inputs, expected output, stage/attempt, current state, version, and timestamps.
- Track acceptance/decline, meaningful progress, blockers, requests for input, result submission, and cancellation. A submitted result is not independent review, human approval, or overall task completion.
- Give results structured artifact references, verification evidence, limitations, and the requested next action. Store claims as claims; the service does not execute checks or inspect host artifacts.
- Expose participant registration/listing and assignment create/get/list/update/result operations through MCP. The orchestrator explicitly selects assignees and decides subsequent work.
- Publish corresponding inbox messages and timeline events in the same transaction as assignment mutations. Avoid requiring agents to manually duplicate each operation across three tools.
- Use idempotency keys and optimistic versions. Reject cross-task references, invalid transitions, stale updates, and result submissions from a different declared assignee. Do not silently reassign work after a disconnect.
- Keep ordinary peer messages and arbitrary evidence available. Reference the canonical assignment ID instead of maintaining competing assignment status inside message text.
- Update the shared comms skill and report to use actual supported operations. Preserve existing task/message/event records when extending the schema.

### Acceptance

- The orchestrator can query which work is pending, accepted, blocked, awaiting input, or submitted, and which participant owns it.
- Two sessions with the same role remain distinguishable. Repeated registration/delivery does not create duplicate ownership or work.
- Acknowledgement of reading remains distinct from assignment acceptance.
- Retried writes do not duplicate assignments, results, notifications, or timeline events.
- Ownership/version conflicts and cross-task references have explicit errors and tests.
- No workflow ordering, automatic assignment selection, heartbeat monitor, or delivery daemon is introduced in this issue.

**Starting points:** [schema](comms/internal/comms/schema.sql), [MCP tools](comms/internal/comms/server.go), [message conventions](comms/skills/task-comms/references/messages.md).

## FACT-002 — External inbox delivery bridge with a Herdr adapter

**Status:** Implemented; live Herdr smoke test pending · **Priority:** Second · **Depends on:** FACT-001

Implemented an external Go host bridge using the official MCP client, captured Herdr bindings, durable exclusive notification claims, bounded retries/reconnects, explicit uncertain-input recovery, owner failure findings, delivery MCP queries, and report visibility. Manual mode remains available. See [delivery setup and verification](comms/delivery.md). Automated HTTP/Herdr-fixture tests cover both directions; real idle-pane wake-up must still be verified inside Herdr. Process/contact identity beyond the captured binding remains FACT-003.

### Problem

Stored messages do not wake idle agents. Current delivery depends on active-turn inbox checks and manual terminal notifications. Implementing the solution solely inside a Pi extension would make reliable orchestration depend on one runner.

### Proposed scope

- Add a small external process that polls comms without model requests and delivers actionable notifications through the selected host adapter.
- Start with Herdr and explicit, recorded session references. Do not guess pane IDs, scrape terminal text as authoritative task state, or replace occupied sessions.
- Track notification attempts separately from reading acknowledgements, assignment acceptance, and results. Suppress duplicate notifications without losing pending messages.
- Handle reconnects, backoff, unavailable sessions, and restart recovery. Expose concrete delivery failures to the orchestrator.
- Treat busy sessions according to the adapter's verified capabilities. Document unsupported delivery behavior rather than promising reliable interruption or queuing.
- Preserve a manual mode for other environments. The bridge never chooses workflow steps, grants approvals, reassigns work, or interprets successful terminal input as completed work.

### Acceptance

- An idle recipient can receive an assignment and the orchestrator can receive its reply without human inbox prompting in the supported Herdr setup.
- Empty inboxes do not invoke a model.
- Duplicate polling and bridge restarts do not create duplicate assignments or notification floods.
- Unavailable recipients are reported and their messages remain recoverable.

## FACT-003 — Inspectable session configuration and contact state

**Status:** Implemented; live contact verification pending · **Priority:** Third · **Depends on:** FACT-001; integrate with FACT-002

### Problem

Implementation: immutable session snapshots, `get_session`, expiring `record_contact`, bridge observations, and task-report visibility. Fingerprints cover public instruction/model/tool selections; they exclude private native options. See [session configuration](orchestrator/README.md).

A role can be configured correctly without its live session using the latest configuration. Startup checks, a running process, heartbeat contact, meaningful progress, and successful work are currently easy to confuse.

### Proposed scope

- Record a session manifest containing project/task identity, participant/instance ID, runner, model/provider, loaded skill paths and content hashes, MCP names, preparation time, and a configuration fingerprint.
- Exclude credential values and raw environment contents. A manifest must remain useful without exposing private native profiles.
- Distinguish configuration prepared, startup checked, session responding, last contact, and latest assignment progress/result.
- Add bounded contact reporting through supported host integration and explicit expiry handling. Missing contact means unavailable/unknown, not that an assignment failed or completed.
- Expose session facts and configuration snapshots through comms queries and the report. Make changed-source versus loaded-snapshot differences inspectable.
- Record startup failures after registration as failed contact, without silently transferring ownership. The launcher now records such failures, but registration alone can still outlive its process.
- Preserve instance identity across runner-managed context resets where appropriate; a genuinely new process must not silently inherit another instance's ownership.

### Acceptance

- The orchestrator can identify the configuration and skills a participant actually loaded, rather than infer them from current source files.
- A skill edit does not falsely mark an existing session as updated.
- A responsive but blocked agent is distinguishable from an unreachable agent and an agent making progress.
- Contact reporting does not consume model tokens or imply verification of the work product.

## FACT-004 — Coordination reliability eval with interruption and recovery

**Status:** Automated recovery eval implemented; live Herdr eval pending · **Priority:** Fourth · **Depends on:** FACT-001 through FACT-003

This is the next integration checkpoint: run the actual coordinator and worker sessions in Herdr, then repeat the setup on another machine. Automated fixtures cannot establish that pane startup, native message delivery, recovery, and the coordinator's decisions work together in a real task. Record setup failures as well as task outcomes; a successful product alone is insufficient evidence of coordination reliability.

### Problem

Implementation: [recovery eval and live checklist](orchestrator/tests/README.md), with an automated JSON/HTML report. No live agent behavior or product verification is claimed by the fixture run.

Startup and transport tests do not establish that an orchestrator-driven collaboration handles interruptions, unanswered assignments, or incomplete handoffs correctly.

### Proposed scope

- Repeat a minimal Go tic-tac-toe task through the supplied factory, using visible role sessions and the real comms tools.
- Keep human approval points explicit. Delegate them only when the human authorizes it for this run.
- Exercise an idle recipient, duplicate delivery, an interrupted worker, bridge/service restart, changed approval documents, and a missing result/handoff.
- Check whether the orchestrator detects and explains each condition, preserves completed work, and resumes without duplicating assignments or granting itself more authority.
- Record agent versions/configuration fingerprints, actual transitions, operator interventions, observed recovery time, measured usage and its coverage, failures, and unresolved limitations.
- Leave implementation and product verification to the assigned coder and independent reviewer. The evaluating coordinator must not silently repair their work to make the run pass.
- Pair the live eval with deterministic protocol/recovery tests where possible; do not treat a single successful model run as proof of reliability.

### Acceptance

- Produce a report with expected versus observed outcomes for each fault scenario and evidence links.
- Show that delivery, reading, acceptance, submission, review, and overall completion remain distinct.
- Demonstrate a worker interruption and explicit orchestrator-directed recovery without duplicate work.
- Turn observed shortcomings into specific follow-up issues; do not hide them through undocumented manual interventions.


## FACT-005 — Deterministic dispatch to visible agent sessions

**Status:** Implemented; live Herdr acceptance pending · **Priority:** Next · **Depends on:** FACT-001 and FACT-002

Implementation: [dispatch and recovery](orchestrator/references/dispatch.md).

The coordinator still has to correctly sequence session startup, actual participant registration, assignment creation, and notification. Instructions alone have not made this reliable: the eval required human prompting to open panes, and work was assigned to placeholder identities.

Provide one host operation that starts or explicitly reuses a configured visible session, waits for its actual registration, creates the requested assignment, and notifies that participant. Return a structured receipt for each completed action and any failure. Retried dispatch must reconcile earlier actions without duplicate panes or assignments. An uncertain notification must remain visible and require inspection before retry.

The orchestrator still chooses the factory, assignee, scope, and next action. This operation handles a single explicit dispatch; it introduces no scheduler or workflow language. Preserve direct human access to each agent and a manual path for unsupported hosts.

**Acceptance:** A fixture test and a real Herdr eval demonstrate dispatch, startup failure, interrupted dispatch, and retry without placeholder participants or duplicate work.

## FACT-006 — Recoverable task artifact attachment

**Status:** Implemented · **Priority:** Next

`attach_record` publishes `task.json` before rendering documents. A template or filesystem failure can leave an incomplete directory that a retry treats as already attached. Concurrent requests can also create two artifact directories for the same service task.

Prepare the complete scaffold before publishing the task pointer and serialize attachment for a service task. Validate existing attachments without overwriting authored documents. Distinguish a completed local attachment from a failed comms event write so recovery preserves existing work.

**Acceptance:** Failure injection during document rendering leaves no apparently complete attachment; concurrent calls return one directory; retry preserves edited documents and does not duplicate the service task.

## FACT-007 — Check readiness for every configured factory member

**Status:** Implemented · **Priority:** After dispatch recovery

A successful coordinator check does not establish that the architect, coder, or reviewer has usable credentials, required tools, or MCP connections. Discovering those failures during a handoff interrupts the task unnecessarily.

Offer a factory-wide check using the existing member startup checks. Report readiness separately for each member, with actionable failures and private diagnostic paths. Do not start model work, register participants, create tasks, print credentials, or claim that resolving credentials proves remote model access. Keep this a small extension of existing checks, not a new configuration layer.

## FACT-008 — Dynamic task status view

**Status:** Implemented

A read-only auto-refreshing task page and `get_task_status` share the same records. Immutable public factory membership distinguishes configured members from actual instances. Assignment edges show recorded dispatches; contact, work state, messages and results remain distinct. No Go-specific members or planned graph are hardcoded. A terminal monitor now provides project/task navigation, attention, contact, observed message routes and sent/read/exec states using the same read-only endpoints; it refreshes every three seconds and flags stale data.

## FACT-009 — Actionable coordinator inbox

**Status:** Implemented

Versioned, explicitly resolved attention items collect decisions, review handoffs and blockers. Dispatch failures and delivery failures are visible without interpreting terminal prose. Human authority and artifact approvals remain explicit; closing an inbox item does not change work state.

## FACT-010 — Structured handoff evidence requirements

**Status:** Implemented

Assignments retain explicit artifact/check/limitation requirements. Factory members provide defaults for dispatch; each assignment can override them. Incomplete successful submissions are rejected before publishing, while failures can explain missing evidence. Completeness validation does not execute checks or make review judgments.

## FACT-011 — Coordination overhead and measured usage

**Status:** Implemented

Task-wide recorded timing separates assignment acceptance, closed blocked intervals and first notification. Reports count explicit decisions/interventions and submitted handoffs without inferring productivity or review quality. A Pi session importer deduplicates measured assistant usage, rejects ambiguous attribution, and reports incomplete coverage; unknown usage/cost remains unknown.

## FACT-012 — Inspectable assignment recovery

**Status:** Implemented

An explicit recovery plan precedes owner-only cancellation or atomic reassignment to a real replacement participant. Version checks and stable request keys prevent duplicate work. Prior evidence and summary survive; reassignment requires operator confirmation that the old process stopped, never inferred from contact expiry.

## FACT-013 — Toolkit updates without overwriting team factories

**Status:** Implemented

A preview-first copy updater records managed file hashes and refuses conflicting upstream/local edits. The copier remains optional for separately maintained copies. Normal setup now uses independent framework and tools checkouts, with project `factory.json` overrides; no copy into the application is needed. Local overrides, task/runtime data and credentials are excluded. No upstream directory, registry, dependency resolver or package ecosystem is introduced.

## FACT-014 — Coordinated startup

**Status:** Implemented; live Herdr acceptance pending

The coordinator entrypoint validates configuration, starts local Compose or connects to an explicit endpoint, and waits for MCP connectivity. Pi sessions now use native inbox delivery; terminal hosts only manage visible sessions. Check-only modes remain free of service startup. Shared Compose services remain running after exit. An unassigned Pi coordinator binds its actual task through factory_bind_task. Live terminal acceptance remains pending.

## FACT-015 — One authored factory configuration

**Status:** Implemented

`factory.json` owns team membership, workflow rules, and optional model/provider overrides. Each bundled agent owns its native Pi profile, skills, MCPs, and extensions; Factory owns the built-in coordinator bundle. `session.ts` resolves the selected configuration and generates private Pi files before launch. Factory selection is explicit, with no mutable home-directory coordinator profile. Tests cover isolation, inheritance, generated settings, unused resources, credential boundaries, and native Pi startup without model inference.

## FACT-016 — Follow recovered assignment lineages

**Status:** Implemented

The workflow resolver follows a later assignment for the same stage when an earlier assignment was cancelled or declined. Without a replacement, the terminal assignment still blocks the step. Regression coverage includes the recovery service's event order, which publishes the replacement before the cancellation update.

## FACT-017 — Confirm worker contact before assignment

**Status:** Implemented

Dispatch waits for a fresh native inbox contact after a worker registers and before it creates an assignment. A worker that exits after registration no longer receives an assignment it cannot process. Retrying the same dispatch can continue when that worker establishes contact; the retry neither launches a duplicate pane nor creates duplicate work.

## FACT-018 — Acknowledge Herdr command execution

**Status:** Implemented

A Herdr pane launch writes a private marker only after its shell begins executing the submitted command. Dispatch waits for that marker before accepting participant registration as launch success. If Herdr accepts a command but the pane never runs it, the receipt identifies the launch failure and retry does not submit another command automatically. This protects against duplicate workers while making the failure distinct from missing registration or inbox contact. A live Herdr check remains necessary because the automated test uses a pane fixture.

## FACT-019 — Surface worker failures to the coordinator

**Status:** Implemented

Session startup and nonzero runner exits record a task failure event with the instance ID and a redacted, actionable summary. Dispatch reads matching failures while waiting for registration or native inbox contact and includes the summary in its receipt. This makes a failed credential or MCP check visible in the monitor and to the coordinator without copying private subprocess output or credentials into Comms. The worker pane and private `startup-checks.json` retain detailed diagnostics.

## FACT-020 — Keep bounded loops within their assignment

**Status:** Implemented

For members attached to a workflow step with a loop, the Pi delivery adapter checks the current task and canonical assignment before allowing tools that can perform work. It binds to one assignment for the life of the process. When that assignment is submitted, cancelled, or blocked, it refuses subsequent work even if a new assignment appears for the same participant; the coordinator must explicitly start the next batch. Read-only inspection remains available. The Go coder's loop instructions also require stopping immediately after the review handoff. This is an execution guard for Pi tools, not a general filesystem sandbox or an enforcement layer for other runners.

## FACT-021 — Probe configured model capabilities on demand

**Status:** Implemented

`session.ts --check --probe-model` performs a minimal real request against the selected provider, model, and thinking level after local credential, MCP, and extension checks. It reports a provider rejection before assigning work and retains detailed output only in private startup diagnostics. The probe is explicit because it consumes provider tokens and network access; ordinary `--check` still makes no model request. The real provider remains an external dependency, so a successful probe does not guarantee later availability.

## FACT-022 — Verify gopls is reachable after installation

**Status:** Implemented in Factory Tools

The Go factory's install and update targets now fail if `gopls` was installed but is not on the launching shell's `PATH`, with the Go binary directory in the error. The example documentation calls out this common `go install` behavior. This catches the mismatch before an agent session tries to start its gopls MCP connection; the Factory session check still verifies the actual MCP connection.

## FACT-023 — Make developer verification independent of runtime installation

**Status:** Implemented

`make verify` installs the full Node development dependency set, then runs TypeScript checking, the TypeScript test suite, and Go tests. Runtime installation can continue using production dependencies only. The coordinator preview test changes its copied native Pi profile and confirms the launcher uses that selected provider and model, avoiding an OpenRouter-specific assertion that fails when teams customize the profile.

## Future issues

These are candidates for work after the live FACT-004 eval. Keep the orchestrator-driven design; each issue should be refined using observed failures before implementation.

### FACT-024 — Isolate project work by task

**Status:** Proposed

Agents currently inherit host filesystem permissions, and multiple tasks can target one project checkout. Define task workspace ownership and an explicit policy for concurrent writes. Start with a reliable single-writer guard if isolated workspaces are not yet available. Make the coordinator and monitor show which task owns a workspace, and ensure recovery does not leave an invisible lock or start a second writer. Define how reviewed changes reach the project's intended checkout. This does not require a container runtime as the first implementation.

**Acceptance:** Two tasks cannot silently modify the same checkout concurrently; a stopped or failed task has a documented recovery path; the reviewer can inspect the exact changes attributed to one task.

### FACT-025 — Connect task cancellation to process control

**Status:** Proposed

Comms cancellation changes a record but does not stop a running agent. Define a host-side cancellation operation for sessions Factory launched, with an explicit receipt that distinguishes requested, stopped, still running, and unknown. Never infer process termination from stale contact or a cancelled assignment. Retain a manual path for externally managed sessions and ensure retries cannot start replacement work while the old writer may still be active.

**Acceptance:** A user can cancel a launched task and determine whether its agent process actually stopped; uncertain termination blocks unsafe reassignment and is visible in task status.

### FACT-026 — Make inbox delivery available to other runners

**Status:** Proposed

Pi has a native inbox adapter; other runners rely on manual inbox checks or their own adapter. Specify and implement one additional runner adapter against the existing Comms delivery contract. Keep runner-specific behavior in the adapter, including how an idle session is notified, how messages become read and exec, and what happens when the process is stopped. Report unsupported capabilities rather than implying parity with Pi.

**Acceptance:** A non-Pi coordinator or worker can receive a task message without a human prompt, and sent, read, and exec remain distinct and inspectable through Comms.

### FACT-027 — Make new-machine setup diagnosable

**Status:** Proposed

Framework and tools are separate checkouts, while project configuration, local paths, credentials, service access, runner plugins, and native dependencies must agree. Provide one read-only setup check that resolves the selected factory and reports missing or incompatible prerequisites by member before task creation. Give precise remediation for path, dependency, provider, MCP, and Comms failures; do not expose secrets or make paid model requests by default.

**Acceptance:** A new user can run the check on another machine and identify the failing prerequisite without starting an agent or task. The check does not claim that a local credential check proves remote model availability.

### FACT-028 — Separate task documents from starter templates

**Status:** Proposed

The factory's `artifacts` map currently supplies starter files for task scaffolding while decision validation also uses it to decide which documents can be approved. Model the document contract separately: expected outputs, optional starter templates, and approval targets. A produced document must not require a starter file merely to be eligible for approval. Preserve existing task files and pinned workflow definitions when changing configuration.

**Acceptance:** A factory can require and approve a document created entirely by an agent; a starter template can be used without implying approval; existing tasks retain their prior artifact and decision semantics.

### FACT-029 — Show provenance and coverage of task evidence

**Status:** Proposed

Reports already display measured Pi usage and mark unknown usage, while many checks, artifact references, tool claims, and non-Pi usage still arrive as agent-submitted evidence. Distinguish host-observed events, runner-measured usage, and agent claims in the task report and status API. Define coverage for every reported metric, especially tokens, cost, checks, and artifact hashes. Add automatic collection only at reliable integration points; do not turn absent observations into zero or verified success.

**Acceptance:** A reviewer can tell which metrics and checks were observed, which were claimed by an agent, and which are unknown. The report remains truthful when an agent omits usage or evidence.

### FACT-030 — Select agent models and credentials at the factory level

**Status:** Proposed

#### Problem

Teams should be able to reuse an agent library while choosing different models, providers, endpoints, and credential references for a project. Today each bundled agent owns a Pi profile; managed launches permit per-member `provider` and `model` overrides, but factory defaults do not replace bundled model defaults, the same provider/model pair must be repeated for several members, and a factory provider cannot replace a same-named provider from a bundle. The configuration also describes Pi's native provider shape rather than a runner-independent choice. Editing library bundles to change a project's model defeats reuse.

#### Contract and ownership

- Factory defines a small, runner-independent **model selection**: provider reference, model ID, and optional thinking level. The factory owns named model profiles and provider/credential references for managed sessions. A default profile is optional; each member, including the built-in coordinator, may select another profile. The agent bundle supplies its normal standalone model as the final fallback.
- Resolution order for a managed launch is: explicit member profile, factory default profile, then the bundle's default. A member may still specify a direct provider/model override for a one-off choice, but it must have a clear precedence relative to a named profile; reject contradictory selections rather than guessing.
- Agent bundles continue to own role instructions, skills, tools, MCPs, extensions, and their standalone launch configuration. A managed factory selection changes only the requested model runtime settings. It must not silently replace an agent's other capabilities.
- The coordinator chooses which member and profile to launch from the configured factory; it does not translate settings into Pi, Claude, or another runner's files. A runner adapter performs that translation once for all agents using that runner. Keep adapters small and explicit, without a plugin framework or a translator in every agent bundle.

An illustrative project configuration (field names are to be finalized during implementation):

```json
{
  "extends": "../factory-tools/factories/golang/factory.json",
  "defaults": { "model_profile": "standard" },
  "model_profiles": {
    "standard": { "provider": "team-gateway", "model": "coding-model" },
    "design": { "provider": "team-gateway", "model": "reasoning-model", "thinking": "high" }
  },
  "providers": {
    "team-gateway": {
      "baseUrl": "https://ai.example.com/v1",
      "api": "openai-completions",
      "apiKey": "$TEAM_AI_KEY",
      "models": [{ "id": "coding-model" }, { "id": "reasoning-model" }]
    }
  },
  "agents": [
    { "name": "coordinator", "model_profile": "standard" },
    { "name": "architect", "model_profile": "design" },
    { "name": "coder_golang", "model_profile": "standard" }
  ]
}
```

#### Runner and credential behavior

- First implement translation for Pi using its existing profile-generation path. The adapter validates that the selected provider/model and requested thinking level can be represented, then writes the native settings and selected provider definition into the private session directory. A provider selected from the factory must be able to supersede the bundle's same-named provider for that managed launch; an unselected provider must not be materialized or require credentials.
- Define the adapter input and capability/error contract independently of Pi. A command runner without a translator must fail clearly when a factory model profile is selected; do not claim that passing a provider/model name as text configures an arbitrary runner. A later runner integration supplies its own translation and verification while using the same resolved selection.
- Store credential **references**, never key values, in `factory.json`. Resolve an environment variable or private auth-file reference only for the selected provider and pass it through the runner's supported native mechanism. Do not copy keys into task records, model instructions, public session manifests, reports, or diagnostic errors. A missing key reference must fail before starting model work, with the variable name and member identified.
- Record the resolved profile name, provider, model, runner, and non-secret configuration fingerprint for each actual session. A new session may use a changed profile; the report must show what each attempt used instead of retroactively rewriting earlier sessions. The configured profile is not proof that the provider accepted a request or that the advertised model was served.

#### Acceptance

- A project can select one shared profile for its coordinator and workers and override the architect or reviewer with a second profile without editing any agent bundle.
- Two project factories can use the same agent library with different providers, endpoints, model IDs, and key references; standalone launches retain the bundle defaults.
- Pi's generated private profile contains the selected provider/model and no unrelated provider; startup checks identify missing credentials and unsupported selections without printing key values.
- An unsupported runner or model option produces an actionable error before the session starts. Tests cover precedence, same-name provider replacement, coordinator selection, secret handling, and accurate session reporting.
