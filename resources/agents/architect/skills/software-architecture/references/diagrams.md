# Architecture diagrams

Use a diagram to answer a specific question. Omit it when a short explanation or table is clearer. Distinguish the current system from the proposed change, and keep labels consistent with the repository and domain vocabulary.

Use selected [C4 views](https://c4model.com/diagrams) as appropriate:

- **System context:** users, the application, and external systems; useful for scope and responsibilities.
- **Container:** running applications and data stores, with their relationships. A C4 container is not necessarily a Docker container.
- **Component:** internal responsibilities where they clarify a design choice. Do not diagram every function.

A sequence or flow diagram can explain a request, state transition, failure path, or cross-system exchange. A deployment diagram is useful when placement, network boundaries, or infrastructure affect the decision. There is no requirement to produce every view.

Label arrows with the interaction or data they represent. Show ownership or trust boundaries when relevant, and identify which component is authoritative for important state. Avoid unlabeled lines and boxes named only after technologies.

For self-contained `spec.html` and `plan.html`, use embedded SVG with a readable title and description, or a simple accessible HTML/CSS diagram. Provide nearby text explaining the important relationship. Do not add a CDN or runtime diagram library merely to render a document. If diagram source is included for editing, ensure the visible document still works without an external renderer.

Use [arc42's overview](https://arc42.org/overview/) to check whether relevant context, constraints, runtime behavior, decisions, quality needs, and risks are explained. Select useful topics; do not impose its whole document structure on every task. These are external references, not bundled templates or diagrams.
