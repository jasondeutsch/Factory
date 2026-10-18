# Toolkit resources

Reusable agents and skills shared by the factory toolkit. These are independent of any particular factory recipe.

- `agents/architect/`: project investigation, architecture, and spec/plan drafting.
- `agents/architecture_reviewer/`: critical review of architecture and proposed work.
- `skills/`: conventional commits, evidence-based debugging, bounded iteration, and Linear context.

Factories reference these resources rather than copying them. Factory-specific agents, skills, and templates stay under `factories/<name>/`. The communication skill belongs to its MCP server in `orchestrator/comms/skills/task-comms/`.

Factories include these bundles through explicit names and paths in `factory.json`. Bundle configuration supplies reusable defaults; each factory member's `config` overrides them for that factory. Adding a resource does not automatically enable it. See the [factory configuration reference](../factories/README.md) for membership and launch resolution.

See [factory recipes](../factories/README.md) and [session configuration](../orchestrator/README.md).
