---
name: go-concurrency
description: Write or review Go goroutines, shared state, channels, cancellation, deadlines, and shutdown with explicit ownership and bounded lifecycles.
license: MIT
---

# Go concurrency and context

Apply when the change starts concurrent work, shares mutable state, changes request lifetimes, or relies on cancellation or deadlines. Keep synchronous operations synchronous when concurrency provides no demonstrated benefit.

Establish who starts, stops, and waits for each goroutine. Identify which state is shared and how all accesses are synchronized. Explain channel send, receive, and close ownership; account for abandoned consumers and early failure. A channel operation alone does not transfer every alias to the sent data safely.

Propagate request context through work belonging to that request. Derived contexts need cancellation cleanup or an explicit ownership transfer. Deliberately detached work still needs a bounded lifetime, error reporting, and a shutdown owner; removing cancellation is not a durability mechanism.

Read [lifecycle and cancellation](references/lifecycle.md) for concurrent or context-sensitive changes. Reviewers trace concrete interleavings and termination paths rather than flagging concurrency constructs in isolation. Use relevant race or lifecycle checks when practical without treating a passing run as a proof of correctness.

Resolve references against this skill's source directory. See [sources](../SOURCES.md) and the retained [MIT notice](../LICENSE).
