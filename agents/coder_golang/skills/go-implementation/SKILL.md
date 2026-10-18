---
name: go-implementation
description: Implement scoped changes in Go applications using existing project conventions, clear errors and interfaces, and meaningful verification.
---

# Go implementation

Read `go.mod`, project instructions, relevant packages, and existing tests before editing. Use the project's Go version and dependency strategy. Avoid broad upgrades, global toolchain changes, generated binaries in source, or unrelated restructuring.

Prefer descriptive names, cohesive packages, explicit dependencies, and small interfaces defined near their consumer. Avoid catch-all helpers and speculative abstractions. Handle errors at a useful boundary, preserve useful causes, and include context that lets a maintainer diagnose the failure. Do not log credentials or duplicate the same error at every layer.

For services, follow existing context cancellation, timeout, resource cleanup, and shutdown patterns. Examine concurrency and ownership where shared state is involved; do not add goroutines without a lifecycle. Validate data at trust boundaries and preserve existing authorization behavior.

Use gofmt and the project's configured linter. Test meaningful behavior, failures, and regressions appropriate to the change. Follow the existing test library; for new team tests, stretchr/testify is a reasonable convention when already agreed and available. Do not add a dependency solely to restate a trivial assertion.

Prefer existing project check commands. The optional [Go check script](../../scripts/check-go.sh) checks formatting without rewriting files, runs `go test ./...`, and invokes `golangci-lint run` with any supplied lint arguments. Run it through `sh` from the application module root when those checks match the agreed plan. It uses installed tools and does not install them or supply a lint configuration.

Inspect command output. Distinguish product defects from environment or tool-version failures and report evidence before proposing a fix. Record changed files and acceptance coverage for the independent reviewer. Build, lint, and tests are tools available during coding, not reasons to expand task scope.

Use the configured shared Go skills for detailed code quality, testing, concurrency, and context guidance. For additional questions, consult [Go review comments](https://go.dev/wiki/CodeReviewComments) and [Effective Go](https://go.dev/doc/effective_go). This role does not install an upstream skill collection or its tools automatically.
