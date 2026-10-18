---
name: go-code-quality
description: Write or review idiomatic Go with clear names and interfaces, useful error handling, safe data ownership, and appropriate security boundaries.
license: MIT
---

# Go code quality

Use the project's instructions, supported Go version, and existing conventions. In coding mode, implement the agreed change; in review mode, inspect and report without repairing source. These instructions do not grant permission to install tools or expand the task.

Use gofmt and existing lint configuration for mechanical checks. Judge names, package cohesion, control flow, and interfaces by whether they make the behavior easier to understand. Avoid arbitrary function-size rules, blanket wrappers, and abstractions without a real consumer.

Inspect error paths as carefully as success paths. Preserve useful causes where the contract needs them, handle failures at a useful boundary, and distinguish not-found, invalid input, and unavailable dependencies when behavior differs. Check nil and empty values, aliasing, cleanup, and accidental exposure of sensitive information.

Read [clarity, errors, and data safety](references/clarity-and-errors.md) for API or stateful code. Read [security boundaries](references/security-boundaries.md) when the change handles user input, authentication, files, commands, templates, or network requests. Resolve links from this skill's source directory.

Reviewers must show a concrete defect or maintenance consequence; personal preference is not a blocking finding. See [sources](../SOURCES.md) and the retained [MIT notice](../LICENSE).
