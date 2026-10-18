---
name: architecture-challenge
description: Critically evaluate a proposed software specification and plan against repository evidence, failure scenarios, testability, and simpler alternatives.
---

# Architecture challenge

Use independent reasoning to find material weaknesses, not to win an argument. Start from the task brief and actual code. Apply these lenses selectively according to the change:

- **Assumptions:** Which claims require code, documentation, measurement, or a human decision? What breaks if an assumption is false?
- **Behavior:** Can a reader tell what happens for representative inputs, errors, and boundaries? Do the acceptance criteria cover the request without adding work?
- **Failure:** Consider partial completion, retries, cancellation, races, data loss, authorization, and recovery when the design involves those concerns.
- **Simplicity:** Is there an existing capability to reuse? Can a component, dependency, abstraction, or operational responsibility be removed?
- **Verification:** What evidence could falsify the proposal? Does the plan test observable behavior rather than mirror implementation?
- **Operation:** Where relevant, can the team diagnose failures, control resource use, roll back, and maintain compatibility?

For each finding, give a stable ID, severity, evidence or a clearly labeled uncertainty, triggering scenario, impact, and requested resolution. Ask a focused question when evidence is missing; do not turn a possibility into a confirmed defect.

Mark a finding blocking only when it threatens agreed behavior, correctness, safety, or implementation feasibility. Keep preferences separate. Evaluate the architect's response against the finding; close resolved findings explicitly. After the factory's review limit, surface remaining disagreements to the coordinator. A clean review is valid.
