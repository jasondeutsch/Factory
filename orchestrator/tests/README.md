# Coordination verification

From the toolkit root:

```sh
npm ci
npm run check
npm test
(cd orchestrator/comms && go test -race ./...)
```

TypeScript tests build comms with Go and start isolated localhost servers with disposable SQLite databases. They use temporary project copies and terminal fixtures, not the normal Compose service. `FACTORY_TEST_COMMS` may point to an already built comms binary. Set `FACTORY_TEST_PI` to an installed Pi executable to check real native credentials/MCP/extension loading using fixture credentials and no model request. Without it that test is explicitly skipped. Factory-specific checks, including extension behavior, belong to the tools repository.

For an automated recovery report:

```sh
node orchestrator/tests/eval_recovery.ts --output /tmp/factory-recovery
```

The report includes message-state, contact, retry, attachment, configuration, updater and usage checks. It creates JSON/HTML evidence, not an agent-quality evaluation. Go tests also cover database reopening, assignment transitions, delivery leases, replacement and contact expiry. These tests do not establish real window opening, model behavior, or corporate-network compatibility.

## Live acceptance (pending)

Use a disposable project and an installed factory, with a human present. Keep product implementation and verification with the coder and reviewer.

1. Start the coordinator, create/attach a task, and bind it. Confirm the status shows connected and the snapshot reports native contact.
2. Dispatch a worker. Verify a visible Herdr pane or Terminal/iTerm window opens with its selected configuration and skills.
3. Verify assignment and result hints reach idle sessions without manually prompting them. Inspect `sent`, `read`, and `exec` separately from assignment acceptance and submission.
4. Repeat the same dispatch key. Expect no second window or assignment.
5. Interrupt the service, then restart with the same database. Expect reconnection, preserved messages and possible repeated hints—not repeated completed work.
6. Stop a worker. Expect contact expiry and no fabricated completion. Explicitly reconcile/reassign its work before starting a replacement.
7. Change approved documents before a coder loop starts. Expect the approval guard to refuse them.
8. Give direct feedback in a worker session. Verify consequential feedback reaches the coordinator.

Retain IDs, timestamps, actual interventions, runtime versions, errors and recovery duration. Human approval stays with the user unless explicitly delegated. Do not compensate for a broken handoff by doing the agent's work yourself. Record automated checks and live observations separately.
