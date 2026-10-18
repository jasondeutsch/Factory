# Boundaries and integration

Inspect the business capability and vocabulary before prescribing module boundaries. The same term can refer to different concepts in different parts of a business. Clarify differences with examples instead of forcing one universal model or duplicating a model without understanding its owner.

## Context and ownership

When useful, distinguish capabilities that differentiate the business, business-specific supporting work, and generic capabilities that might be reused. Do not infer competitive importance from a package name or invent a business strategy to classify every module.

A bounded context makes a model's meaning and ownership explicit. It may be implemented within a modular monolith, across cooperating components, or in a service. Deployment and storage choices need their own justification. Avoid one-service-per-entity and one-database-per-context rules.

For an important relationship, identify:

- The authoritative owner of the data or rule.
- What the consumer needs, including its terminology and compatibility expectations.
- Whether it uses the owner's contract directly or translates into a local model.
- How changes, errors, and unavailable dependencies affect the consumer.

Use a small translation boundary when an external model would otherwise spread assumptions through the application. Describe which differences it resolves rather than adding an adapter because a pattern catalog names one.

## Events, queries, and persistence

Use domain events when recording a meaningful business fact serves an actual need. Give them precise semantics; an event describes something that happened, not an instruction to perform an operation. Internal model events and externally supported integration contracts can have different audiences and compatibility requirements.

When publishing externally, identify what consumers may rely on, how confidential fields are excluded, and how delivery interacts with the authoritative state change. If reliable delivery, duplicate handling, or ordering matters, make the mechanism and failure behavior explicit. Do not assume that naming an event or adding a broker solves consistency.

Use persistence interfaces where they clarify ownership and testing. Do not require a repository abstraction around every table or basic query. Read-oriented projections and reports may need a different shape from invariant-enforcing operations, without requiring a full CQRS architecture.

Keep event sourcing separate from ordinary domain events: recording some facts does not imply that the event log is the authoritative persistence model. Introduce either only when the task benefits justify its complexity.

## Existing systems

Prefer a focused change preserving established contracts. Document ambiguous ownership or terminology as an uncertainty. When a new boundary is justified, describe an incremental transition, what remains shared, and any compatibility or data changes. Do not use a scoped feature request as permission to reorganize the entire domain.
