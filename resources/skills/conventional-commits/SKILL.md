---
name: conventional-commits
description: Draft or review Conventional Commits messages from actual changes, and prepare scoped commits when the user has authorized committing.
---

# Conventional commits

Use the repository's explicit commit instructions first. This skill supplies a default message convention; it does not authorize creating commits, rewriting history, pushing, or releasing.

Inspect the actual changes, including the staged diff when preparing a commit. Describe the resulting behavior and intent, not the conversation or a list of filenames. Keep unrelated changes out of an authorized commit; never stage the entire working tree without inspecting it. Do not discard someone else's work or add credentials, runtime state, task mailboxes, or generated session logs.

Use this structure:

```text
type(optional-scope): concise description

Optional explanation of the problem, resulting behavior, and relevant tradeoff.

Refs: TEAM-123
```

Use `feat` for new behavior and `fix` for a defect. Other useful types include `docs`, `refactor`, `test`, `build`, `ci`, `perf`, and `chore`; follow local conventions rather than enforcing a fixed list. A scope names an affected component and is optional. Prefer a short, concrete subject such as `fix(cache): preserve cancellation on lookup`.

Mark an actual compatibility break with `!` before the colon or a `BREAKING CHANGE:` footer, explaining what callers must change. A large diff alone is not a breaking change. Do not invent issue references, test results, version bumps, or release actions.

When asked only for a message, return the proposed text without executing Git. When committing is authorized, inspect the staged diff, use the exact proposed message safely, and report the resulting commit and any remaining changes. Never bypass hooks or amend a commit without authorization.

Reference: [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).
