# Design decisions

Use the parts relevant to the decision. The goal is a defensible implementation approach, not completion of a universal checklist.

## Establish what is actually built

Inspect the relevant entry points, packages, interfaces, schemas, configuration, and checks. Follow a representative operation through the system. Record evidence for current behavior and distinguish it from intended behavior in documentation. Avoid reading the entire repository when a focused investigation is sufficient.

Name constraints that affect the task: compatibility, data ownership, existing integrations, available infrastructure, team maintenance capacity, or deployment restrictions. Ask about unknowns only when they change a consequential decision.

## Make quality needs concrete

A useful quality scenario identifies a condition, expected response, and evidence. Examples of questions to resolve:

| Concern | Concrete question |
|---|---|
| Performance | Which operation, workload, and latency or resource target matter? |
| Reliability | What happens if a dependency fails or an operation completes only partly? |
| Correctness | Which updates must be atomic, and which temporary states are acceptable? |
| Security | Who can perform the operation, and where are permissions checked? |
| Compatibility | Which clients, stored data, or integrations must keep working? |
| Maintainability | Who owns this capability, and which changes should stay localized? |
| Cost | Which resources or operational obligations does the approach introduce? |

Use established targets when available. A guessed throughput or reliability requirement is an assumption to discuss, not an acceptance criterion to assert.

## Compare approaches

Include the smallest viable extension of the existing design. Compare additional approaches only when they resolve a real tradeoff. For each, explain:

- What owns the behavior and state, and which dependencies cross its boundary.
- Which agreed needs it meets and which compromises remain.
- What must be built, migrated, operated, and maintained.
- What evidence supports the decision and what uncertainty remains.

Do not treat a named pattern as justification. Separate modular code from separate deployment: clear internal boundaries can exist within one process. A new service needs a reason for the additional deployment, network, failure, and ownership obligations.

## Examine state and failure paths

For stateful work, trace the authoritative data and any copies, caches, or projections. Explain how derived state stays valid, and whether it is necessary to store it at all.

When operations cross a process or service boundary, identify timeout, retry, duplicate, cancellation, ordering, and partial-success behavior where relevant. A retry is safe only if the operation's semantics and deduplication or concurrency controls support it. Do not assume a local transaction covers remote effects, or that a transport guarantee proves a business effect happens once.

Choose consistency based on the business rule. Eventual consistency requires defined temporary states and recovery; it is not a default solution for every integration. Compensating work may fail and does not necessarily erase an external effect. Document the actual mechanisms rather than promising reliability through terminology.

## Plan the change and its evidence

For an existing system, describe the smallest useful increment and the ordering constraints. Include data migration, coexistence with old clients, or recovery from a failed rollout when the task requires them. Preserve unrelated behavior and human changes.

Record significant decisions in `plan.html` with context, alternatives, rationale, consequences, and open questions. Identify a test, observation, or focused experiment that would challenge a critical assumption. The architect proposes verification; the assigned implementation and review roles carry it out according to the factory agreement.
