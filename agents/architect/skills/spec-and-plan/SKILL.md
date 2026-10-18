---
name: spec-and-plan
description: Develop a Factory task's spec.html and plan.html from its brief and existing codebase, with explicit acceptance criteria and implementation evidence.
---

# Spec and plan

Inspect the actual repository and identify the smallest relevant set of files, interfaces, dependencies, and existing checks. Ask about missing information only when it affects scope, behavior, or a material technical choice. Do not require the human to supply a finished specification.

In `spec.html`, describe the problem, intended behavior, boundaries, constraints, and observable acceptance criteria. Include representative inputs and outcomes. Distinguish established facts from assumptions. Number acceptance criteria so implementation and review can refer to them.

In `plan.html`, explain the proposed design, affected components, execution order, relevant failure cases, and verification. Map planned changes and checks to acceptance criteria. Discuss alternatives only where the choice has a meaningful tradeoff. Record open decisions instead of silently resolving consequential ambiguity.

Preserve semantic HTML and accessible headings; diagrams supplement readable text. Embed styles locally and avoid external scripts or resources. Escape literal user text and code placed into HTML. Do not invent architecture or implementation details without checking the codebase. Templates are a starting structure, not a demand to write lengthy documents for small changes.

Address review findings individually: accepted and changed, resolved with evidence, or unresolved and escalated. Keep the spec about behavior and the plan about implementation. Changes to approved content need a new approval for the affected scope.
