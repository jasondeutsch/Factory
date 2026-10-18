---
name: software-architecture
description: Evaluate consequential software design choices about component boundaries, interfaces, dependencies, state, and runtime behavior against existing code and concrete quality needs.
license: MIT
---

# Software architecture

Use for changes with meaningful structural or operational decisions. Keep local fixes local; do not turn ordinary implementation work into a system redesign.

Inspect the current system and constraints. Identify the actual decision, the affected users or operators, and the evidence available. Translate vague goals such as scalability into a workload, relevant condition, and desired outcome; label missing targets as unknown rather than inventing them.

Compare practical approaches, including extending the current design. Explain the responsibilities, data ownership, interfaces, dependencies, failure behavior, and operational work each adds. Recommend an approach with explicit costs and tradeoffs, and state what new evidence would change it. Separate observed behavior, assumptions, and proposals.

Put the behavior and quality expectations in `spec.html`; put the approach, material alternatives, risks, and verification in `plan.html`. A separate architecture report or decision-record file is not required. Leave review and approval to the factory's designated participants.

Read [design decisions](references/design-decisions.md) when comparing structures, coordinating state, or planning an incremental change. Read [diagrams](references/diagrams.md) when a visual would clarify boundaries or runtime behavior. Resolve these links against this skill's source directory, not an assembled session file.

Use the domain-modeling skill when the decision depends on business semantics. Avoid prescribing microservices, queues, caches, or architecture patterns without a demonstrated need.

See [sources and adaptation notes](SOURCES.md); the upstream MIT notice is retained in [LICENSE](LICENSE).
