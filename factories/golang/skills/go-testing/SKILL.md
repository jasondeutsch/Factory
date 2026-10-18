---
name: go-testing
description: Design, run, and assess Go tests that demonstrate behavior and meaningful failure cases, including service integration and trustworthy verification evidence.
license: MIT
---

# Go testing

Start with the task's observable behavior and existing test conventions. Select checks that could expose a meaningful defect; do not mirror implementation or add tests solely to increase a coverage percentage.

Use focused examples, table-driven cases, or integration tests according to the behavior. Keep fixtures isolated and make time, randomness, and external dependencies controllable when they affect repeatability. Use stretchr/testify when it is the agreed project convention; do not replace existing suites or add dependencies merely to restate trivial assertions.

Read [test design and evidence](references/test-design.md) when writing tests or assessing whether a claimed result demonstrates acceptance. Reviewers independently examine assertions, setup, and exercised code, then run relevant checks where practical. They do not change application source or silently repair the submitted tests.

Record the working directory, command, result, and relevant environment limits. Distinguish a listed command, cached result, mock expectation, and an observed execution. Missing tools or skipped integration checks must remain visible. Use expensive checks in proportion to the risk and the agreed verification plan.

Resolve references against this skill's source directory. See [sources](../SOURCES.md) and the retained [MIT notice](../LICENSE).
