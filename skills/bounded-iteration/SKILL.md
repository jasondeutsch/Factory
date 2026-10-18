---
name: bounded-iteration
description: Execute an assigned set of approved plan items through bounded Ralph iterations, retaining durable progress and stopping for blockers, decisions, or independent review.
---

# Bounded iteration

The coordinator owns assignments and approvals. Ralph owns fresh sessions within one assignment. The starter enables the unmodified `edxeth/pi-ralph-loop` extension for the Go coder only. Use plain-prompt mode with our HTML documents; do not invoke the bundled planner or introduce another planning format.

Before starting, establish the task, assignment, approved document versions, checks, and iteration limit. Default to five iterations per assignment; a larger or renewed budget needs the coordinator's explicit assessment of progress. Iterations do not bound tokens, requests, or elapsed time. The coordinator can launch an approved coder assignment with `session.py coder_golang <task> --loop 5` in its visible pane. This is an agent operation, not a user lifecycle step. See the [runtime reference](../../orchestrator/references/ralph.md).

At every iteration:

1. Read comms task status, assignment messages, current inbox and decision events (including pagination), plus the current local `spec.html` and `plan.html`, and `progress.md`. Check for changed approvals and material feedback.
2. Select one unfinished, dependency-ready plan item by its stable ID. Implement only that item in the application root.
3. Run the relevant agreed checks. Inspect failures; stop for environment, authentication, permission, or scope blockers instead of repeating the same action.
4. Append progress: item ID, files changed, commands and actual results, remaining work, blockers, and material direct feedback. Record progress/check/failure evidence in comms and notify the coordinator through `send_message`. Record measured usage when available; never invent it.
5. End with exactly one control tag on the last nonempty line: `NEXT` for a verified item with more work available, `COMPLETE` for the finished assignment and a published review handoff, or `STOP` for a blocker or decision. Spell tags as `<promise>NEXT</promise>`, for example. Use `WAIT` only for already-running asynchronous work, not an unanswered human decision.

If an iteration cannot complete an item, preserve partial work and explain the blocker before `STOP`. After two attempts on the same item without measurable progress, stop and ask the coordinator to reassess. Never edit approved documents, mark checks successful without evidence, commit without authorization, or approve your own work. `COMPLETE` means ready for independent review, not task approval.

Persist conversation-only feedback before `NEXT`; fresh sessions cannot recover it otherwise. Do not start a second loop in the same project. Before resuming, confirm the saved loop belongs to this exact task and assignment. A limit, abort, or error leaves incomplete work for reassessment, not automatic restart.
