# Go development factory

Use for changes to Go applications and services, including existing codebases. Member names and bundle paths are defined in [factory.json](factory.json). Load members only when needed.

The user asks their existing agent to use this factory. The coordinator handles task creation, sessions, messages, and decision records using the [operations reference](../../orchestrator/references/operations.md). Users give requests, feedback, and approvals in conversation; they do not need to operate the helper scripts.

## Working sequence

1. The coordinator clarifies the task brief, material constraints, and the user's approval policy. Record the policy in a task event. Human approval is the default; delegated decisions must name their scope.
2. The architect inspects the real project and writes `spec.html` (behavior, scope, acceptance criteria) and `plan.html` (technical approach, affected components, verification). Questions belong in those documents and the inbox. Initial templates are drafts, not approvals.
3. The architecture reviewer examines both documents and relevant code, writes `reviews/architecture-01.md`, and messages findings to the architect and coordinator. The architect addresses findings explicitly and updates the documents. Use numbered files for subsequent reviews.
4. The coordinator presents unresolved decisions and links the proposed documents in the user's conversation, unless authority to decide has been explicitly delegated. When the user answers, the coordinator records the decision and exact document hashes using the decision helper. The coordinator may transcribe a human decision but must not invent one. Revisions after approval require a new decision for the changed scope.
5. The coordinator normally launches a bounded coder session with five Ralph iterations, discovers its actual launcher registration, then creates the approved canonical assignment for that instance while the launcher waits, using the [runtime reference](../../orchestrator/references/ralph.md). The coder implements one stable plan item per iteration in the project root, runs the relevant checks, appends `progress.md`, and records handoff evidence in `reviews/build-01.md`. Lint and tests are tools used during implementation; they are not separate model stages. Record commands, results, skipped checks, and relevant environment limitations. Stop for blockers or decisions. Completing the coder's loop means ready for independent review.
6. The code reviewer independently examines the changes and verifies relevant behavior, writes `reviews/code-01.md`, and returns actionable findings. The coder repairs defects and the reviewer rechecks them. Do not treat a built binary or the coder's summary as sufficient review evidence.
7. The coordinator reports the outcome, limitations, and evidence links to the user and marks the task complete only when the agreed acceptance conditions hold.

After two review rounds with unresolved material findings, surface the disagreement to the human rather than silently continuing. The human can authorize more rounds. A blocked command or unavailable tool is a reported limitation, not a successful check.

## Tools and project context

First inspect the repository's own instructions, build configuration, Go version, module layout, and existing tests. Do not replace working conventions just to match this example. Record the starting state and files changed so independent review can distinguish task work from pre-existing changes. Git diffs may help when available; Git is not required.

Typical tools are `gofmt`, `go test`, `go vet`, and `golangci-lint`. Prefer the project's existing Make targets and lint configuration. The Go coder's optional `.factory/factories/golang/agents/coder_golang/scripts/check-go.sh` checks formatting, runs tests, and invokes golangci-lint using the project's configuration; agree on the checks first. Never install or upgrade the Go toolchain or other dependencies without authorization.

The Go coder and reviewer bundles configure gopls as a native Pi MCP server. It must already be installed with MCP support; startup checks connect without installing it. If the team intentionally runs without it, disable it in the native MCP configuration and record the limitation. Other MCPs use the same per-role files. Record which tools were actually available and used, not merely listed in a skill.

The architect and both reviewers include an optional official Linear MCP connection and shared context skill. Enable it when the task asks for ticket context. Its default endpoint exposes read tools only; retrieving a ticket does not authorize commenting, assigning it, or changing its status. Record the source and flag conflicts with the task's approved documents.

Only the coder edits application source during this flow. Reviewers can write their task review documents and run agreed checks; neither instruction nor the supplied Pi tool list enforces filesystem isolation.

## Dependencies

From the application root:

```bash
make -C .factory/factories/golang install
make -C .factory/factories/golang update
```

Installs the shared coordinator tools plus golangci-lint, gopls, and the coder's pinned Ralph extension. Run `update` only when intending to upgrade host tools. In the toolkit checkout, omit `.factory/`. Docker with Compose, credentials, and model configuration remain separate setup. See [dependency setup](../../docs/troubleshooting.md#dependency-commands).
