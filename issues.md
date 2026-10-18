# Factory issues

Factory uses an orchestrator-driven approach. The orchestrator agent interprets human-readable recipes, assigns work, handles decisions, and evaluates results. Supporting code provides communication, delivery, session visibility, and evidence.

Keep SQLite, MCP, thin `start.sh` entrypoints, native runner configuration, and human-readable recipes. Do not introduce a workflow DSL, scheduler, plugin framework, or package ecosystem. Runner-specific delivery stays in small adapters; the communication contract remains agent-neutral.

## FACT-001 — Structured assignments and task participants

**Status:** Implemented · **Priority:** First

Implemented in the comms service with structured MCP operations, transactional notifications/events, version/idempotency checks, explicit instance replacement, schema-1 upgrade, reports, and updated skills. See [coordination reference](orchestrator/comms/coordination.md). Delivery and live presence remain FACT-002/FACT-003.

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

**Starting points:** [schema](orchestrator/comms/internal/comms/schema.sql), [MCP tools](orchestrator/comms/internal/comms/server.go), [message conventions](orchestrator/comms/skills/task-comms/references/messages.md).

## FACT-002 — External inbox delivery bridge with a Herdr adapter

**Status:** Implemented; live Herdr smoke test pending · **Priority:** Second · **Depends on:** FACT-001

Implemented an external Go host bridge using the official MCP client, captured Herdr bindings, durable exclusive notification claims, bounded retries/reconnects, explicit uncertain-input recovery, owner failure findings, delivery MCP queries, and report visibility. Manual mode remains available. See [delivery setup and verification](orchestrator/comms/delivery.md). Automated HTTP/Herdr-fixture tests cover both directions; real idle-pane wake-up must still be verified inside Herdr. Process/contact identity beyond the captured binding remains FACT-003.

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
