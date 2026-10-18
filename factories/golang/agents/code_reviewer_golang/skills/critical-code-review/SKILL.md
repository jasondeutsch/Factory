---
name: critical-code-review
description: Challenge implementation and verification claims through source tracing, concrete counterexamples, focused questions, and calibrated findings without editing the submitted work.
license: MIT
---

# Critical code review

Build an independent view from the brief, approved spec and plan, actual source, and tests before relying on the author's explanation. Review relevant callers and boundaries, not just edited lines. Establish what changed without requiring Git or a clean branch.

For a material claim, identify the evidence and try to construct a realistic counterexample. Ask precise questions: which invariant prevents the failure, what happens after partial success, or which assertion demonstrates the required result? Trace existing protections before reporting a defect. The goal is better software, not an argument or a quota of findings.

Use [the review method](references/review-method.md) for investigation, useful challenge questions, evidence grading, and report structure. Resolve links from this skill's source directory. Apply the configured Go skills to language-specific behavior and verification quality.

Keep the submitted application source and tests unchanged. Return repairs to the coder, and design or scope disagreements to the coordinator. Recheck responses against the actual finding, preserve stable IDs across rounds, and follow the factory's review limit. Stop when material concerns are resolved; do not manufacture more work.

Report a clear readiness recommendation, unresolved defects or questions, evidence, and limitations. A clean review is valid; it is not a guarantee that no defects exist. See [sources](SOURCES.md) and the retained [MIT notice](LICENSE).
