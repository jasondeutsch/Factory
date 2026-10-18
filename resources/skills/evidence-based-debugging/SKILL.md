---
name: evidence-based-debugging
description: Investigate failed checks, runtime errors, and contradictory behavior using reproducible evidence, distinguishing product defects from environment or tooling failures.
---

# Evidence-based debugging

Capture the failing command or action, working directory, relevant versions, exit status, and useful error output. Preserve the original evidence before changing configuration. Keep credentials out of task records; refer to private diagnostics when output is sensitive.

State what the evidence establishes and what remains a hypothesis. Trace the failure to its owning layer: application, agent, tool, provider, permissions, network, or host configuration. A failed check is not automatically a product defect, and a successful retry does not establish a root cause.

Choose the smallest observation or reproduction that distinguishes plausible causes. Change one relevant factor at a time and explain why it should affect the failure. Use the project's existing diagnostic tools; do not install packages, change global configuration, disable certificate validation, or widen permissions merely to make the failure disappear.

Verify the relevant behavior after a fix. Record the original failure, supported cause, change, verification, and remaining uncertainty. Reviewers return repair requests to the implementation owner unless explicitly assigned the repair.

Stop when the same failure repeats without new evidence or a different justified approach. Report the blocker and the specific information or decision needed instead of consuming more retries. Preserve partial work so another session can continue.
