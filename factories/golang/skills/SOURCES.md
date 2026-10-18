# Sources and adaptation

Reviewed 2026-10-18. These shared skills are used by the Go coder and Go code reviewer; their roles determine whether guidance is applied to implementation or inspection.

Selected guidance is adapted from [samber/cc-skills-golang](https://github.com/samber/cc-skills-golang), by Samuel Berthe. The upstream MIT notice is retained in [LICENSE](LICENSE). Consulted entry points and their declared metadata versions:

| Source | Version | Local use |
|---|---|---|
| [golang-code-style](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-code-style/SKILL.md) | 1.3.0 | Clarity and consistency in go-code-quality |
| [golang-error-handling](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-error-handling/SKILL.md) | 1.3.1 | Error contracts and reporting in go-code-quality |
| [golang-security](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-security/SKILL.md) | 1.2.2 | Relevant trust boundaries in go-code-quality |
| [golang-testing](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-testing/SKILL.md) | 1.4.0 | Behavioral tests and evidence in go-testing |
| [golang-concurrency](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-concurrency/SKILL.md) | 1.2.1 | Ownership and lifecycle in go-concurrency |
| [golang-context](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-context/SKILL.md) | 1.3.1 | Cancellation and request context in go-concurrency |

These are focused local adaptations, not a complete upstream installation. They omit upstream subagent orchestration, automatic tool installation, product-specific library requirements, blanket style thresholds, and mandatory full-repository audits. Local reference documents are written for this toolbox; the upstream reference collection is not bundled. Project instructions and supported toolchain versions govern actual use.

Additional primary references for APIs and conventions:

- [Go review comments](https://go.dev/wiki/CodeReviewComments).
- [context package documentation](https://pkg.go.dev/context).
- [Go race detector](https://go.dev/doc/articles/race_detector).

Version metadata and review dates identify the material consulted; source links track upstream main and are not pinned Git revisions. No updater or runtime dependency on these repositories is added.
