---
name: domain-modeling
description: Clarify business terminology, rules, lifecycles, ownership, and consistency boundaries using pragmatic domain-driven design when a task involves substantive domain logic.
license: MIT
---

# Domain modeling

Use when business semantics affect the design. For presentation changes, straightforward plumbing, or a simple data operation, use only the relevant reasoning; do not invent a rich domain model to justify this skill.

Establish the language used by domain experts and the existing system. Identify relevant concepts, identities, operations, lifecycle states, and rules. Distinguish confirmed business rules from behavior inferred from code. Ask the user or coordinator about material ambiguity rather than inventing policy.

Find ownership and model boundaries where meaning or responsibility changes. A bounded context is a boundary for a model and its language, not an instruction to create a service, separate database, or team. Use entities, value objects, and aggregates only where they clarify identity, validity, or atomic business rules.

Put relevant terminology, behavior, invariants, examples, and acceptance criteria in `spec.html`. Put the proposed model, ownership, transaction boundaries, integration behavior, and verification approach in `plan.html`. Reuse the existing documents; do not require a separate domain report.

Read [business rules and consistency](references/business-rules.md) when designing state changes or invariant enforcement. Read [boundaries and integration](references/boundaries-and-integration.md) when concepts span modules, contexts, or external systems. Resolve these links against this skill's source directory, not an assembled session file.

Prefer the simplest adequate model. CQRS, event sourcing, repositories, and domain events are choices requiring a task-specific reason, not a required DDD package.

See [sources and adaptation notes](SOURCES.md); the upstream MIT notice is retained in [LICENSE](LICENSE).
