# Review method

## Establish scope and evidence

Read the request and acceptance criteria, then inspect the implementation paths and tests that should satisfy them. Use the coder's changed-file list and recorded baseline, plus available diffs, snapshots, or direct source inspection. Do not attribute unrelated existing defects to the change. If the baseline is uncertain, state that limitation and avoid inventing an exact change history.

Assess the implementation first; then use the coder's explanation to resolve specific uncertainties. Separate a defect in implementation, a flaw in the approved design, and a newly proposed feature. Important behavior outside the written criteria can still expose a regression or security defect, but speculation about additional features does not silently expand the approved task.

## Challenge claims with concrete scenarios

| Claim | Useful challenge |
|---|---|
| "It handles errors." | Which errors can this dependency return, and what does the caller observe for each relevant case? |
| "The operation is safe to retry." | What if the first attempt committed its effect but the response was lost? |
| "Cancellation works." | Which blocking call observes the context, and who waits for cleanup? |
| "The map is protected." | Are reads, writes, initialization, and returned aliases covered by the same ownership rule? |
| "The input is validated." | Where does it originate, which validation runs on this route, and what still reaches the sensitive operation? |
| "The tests cover it." | Which assertion exercises the real behavior, and what plausible broken implementation would fail it? |
| "It is faster." | What baseline, workload, and measurement support that claim? |

Select questions relevant to the change rather than interrogating every topic. Read surrounding code before asking for evidence already available. A strong question names a concrete scenario and what evidence would resolve it. Do not reject a justified design merely because a different approach is possible.

## Grade what you know

- **Demonstrated defect:** a reproduced failure or a concrete execution path establishes the wrong outcome. Explain the trigger and impact.
- **Unresolved question:** a material contract, precondition, or environment fact is missing. State what evidence would resolve it; do not present the hypothesis as a confirmed bug.
- **Suggestion:** a readability, maintenance, or design improvement whose current behavior still meets the agreement. Explain value without making it a blocker by default.
- **Verification limitation:** a relevant check was unavailable, skipped, or unable to exercise the claimed behavior. Distinguish that from a failing product assertion.

Severity reflects actual impact and reachability. A spelling preference is not critical; a possible authorization bypass requires tracing the actor, resource, route, and existing controls. Block readiness for material unresolved acceptance or correctness concerns, and explain why. Missing optional evidence alone does not prove the code is defective.

## Write the review

Start with a recommendation: ready, changes_requested, or blocked, with a short rationale. This is a technical recommendation, not permission to merge or publish. Include:

1. Scope reviewed and acceptance coverage, including relevant paths not examined.
2. Findings with stable IDs such as `CR-001`, classification, severity, blocking status, source location, triggering scenario, impact, and requested resolution or question.
3. Actual commands, results, and verification limitations. Link task evidence rather than dumping excessive terminal output.
4. Remaining decisions for the coordinator and any useful nonblocking suggestions.

Do not require a prose compliment or a minimum number of issues. Summarize sound aspects only when that helps explain the assessment. Avoid a blanket approval when important behavior remains unexamined.

## Resolve findings

Send actionable findings to the coder and the verdict to the coordinator. For each response, inspect the repair or counterevidence and mark the finding resolved, still open, or withdrawn. Withdraw an incorrect finding plainly. Keep its ID so the next round can follow the reasoning.

Request a focused reproduction or regression test when it would settle a material dispute. Do not repair source, remove an assertion, or install a different toolchain yourself. If an experiment needs changes, use an isolated scratch copy and describe it honestly. Do not run destructive or external probes without authorization.

After the factory's bounded review rounds, return unresolved disagreements to the coordinator. More rounds require a reason and the agreed authority. Do not invent a fresh criterion after all original concerns have been answered.
