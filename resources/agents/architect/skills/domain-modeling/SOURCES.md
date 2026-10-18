# Sources and adaptation

Reviewed 2026-10-18.

Selected DDD guidance is adapted from [luckys/agent-skills: ddd-best-practices](https://github.com/luckys/agent-skills/blob/main/skills/ddd-best-practices/SKILL.md), whose metadata identifies version 1.1.0. The upstream author is Luis Ramírez Calle; the [MIT license](https://github.com/luckys/agent-skills/blob/main/LICENSE) is retained locally in `LICENSE`.

The adaptation keeps business-first discovery, shared terminology, explicit context boundaries, invariant-based aggregate decisions, and the distinction between internal events and integration contracts. It makes pattern selection conditional, integrates with Factory's existing spec and plan, and omits upstream sibling-skill dependencies and language-specific pattern catalogs. The local references are focused guidance written for this toolbox, not a copy of the upstream reference collection.

[Eric Evans's DDD Reference](https://www.domainlanguage.com/ddd/reference/) is an additional primary reference for domain-driven design terminology. Its PDF and glossary text are not bundled or reproduced.

These are local adaptations, not an installed upstream package or an automatically updating dependency. The review date and source version identify the material consulted; they are not a pinned Git revision.
