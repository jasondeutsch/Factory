---
name: go-review
description: Independently review Go changes for acceptance coverage, defects, maintainability, and credible test evidence without editing the implementation.
---

# Go review

Read the task, approved design, actual changed source, and relevant callers. Establish the baseline before assigning pre-existing defects to this task. Review the smallest complete behavior path, not just individual edited lines.

Check error propagation, nil and zero-value behavior, data validation, resource cleanup, context cancellation, concurrency ownership, and compatibility where relevant. Review tests for assertions about behavior, useful failure cases, and false positives. Compare acceptance criteria to implementation and evidence.

Run relevant project checks independently where practical. If the environment prevents a check, record what remains unverified. A passing binary launch is not evidence of all required behavior. A linter finding caused by an incompatible toolchain is not automatically an application defect; preserve the output and identify the mismatch.

Use the configured critical-code-review skill to challenge claims, document findings, and assess readiness. Use the shared Go skills to examine language-specific risks and the credibility of verification. Recheck fixes against the original failure scenario. Do not patch source yourself, suppress checks to obtain a green result, or manufacture findings to justify the role.

Useful references: [Go review comments](https://go.dev/wiki/CodeReviewComments) and [Go diagnostics](https://go.dev/doc/diagnostics). The project’s own standards take precedence over generic stylistic preferences.
