# Test design and evidence

## Choose behavior worth testing

Connect acceptance criteria to representative outcomes. For a defect fix, identify the triggering case and a regression test that distinguishes the old failure from the correction when practical. For a new behavior, include meaningful failures and boundary conditions instead of enumerating every imaginable input.

Prefer public behavior or a stable internal contract over incidental call ordering and private implementation details. Use integration tests where the important risk is the relationship between components, such as a database constraint or actual HTTP handler wiring. A mock returning the expected value does not prove an adapter, query, or route works.

Use named table cases when they make differences readable; a single clear test need not become a table. Follow the project's existing organization and integration-test selection. Do not impose build tags, test-file ordering, or a new framework without a task-specific reason.

## Make failures real and reproducible

Ensure assertions actually check the important result. Watch for discarded errors, conditional early returns that skip assertions, tautologies, incorrect mock configuration, or tests asserting only that execution did not panic.

With testify, use `require` when continuing would be invalid and `assert` when additional observations remain meaningful. Fatal assertions must stay on the test goroutine; return worker errors or values to it rather than calling `require` inside a worker. Preserve cleanup even when a prerequisite fails.

Use temporary directories, controlled clocks or randomness where useful, and isolated fixtures. Avoid arbitrary sleeps as synchronization. Run tests in parallel only when their fixtures, environment changes, and resources support it. Do not rely on execution order or reuse shared mutable state accidentally.

## Assess verification honestly

Start with the relevant package or behavior, then run agreed broader checks when justified. `go test -count=1` can request a fresh test execution when cached success would not answer the review question. Test selection, build tags, working directory, and environment may determine whether the intended code was exercised.

For concurrency changes, consider race detection and cancellation or shutdown scenarios. A passing race run only covers paths executed during that run. Fuzzing is useful for parsers or complex input spaces when its runtime is bounded and the environment permits it. Neither race detection nor fuzzing is a mandatory ritual for every change.

For performance claims, ask for a relevant workload, baseline, and measurement rather than accepting intuition. Do not add a benchmark suite when the task does not make a performance claim.

Record failing output before diagnosing it. A missing service, network restriction, incompatible linter, or unsupported toolchain is different from a product assertion failure. Do not skip the failure, install a new toolchain, or weaken a check merely to report success.

Reviewers can use an isolated scratch copy for focused experiments when needed; leave submitted source and tests unchanged. Record modifications made to that copy and do not present the experiment as an unmodified project test run. Avoid tests with destructive or external effects unless explicitly authorized.
