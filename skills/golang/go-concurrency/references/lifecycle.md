# Lifecycle and cancellation

## Goroutines and shared state

For each concurrent operation, identify the owner, termination conditions, cancellation signal, error path, and how completion is observed. Check whether it can remain blocked after its caller or consumer has stopped. A bounded queue or worker limit needs behavior for saturation as well as successful execution.

Choose a synchronization mechanism that makes ownership and invariants understandable. Channels can coordinate work; mutexes can protect related state. Neither is universally superior. Atomics do not automatically preserve a multi-field invariant, and protecting one map access does not make every access synchronized.

Account for aliases when passing pointers, slices, maps, or structures across goroutines. Closing a channel requires knowing that no sends remain; identify one responsible owner or coordinated closure mechanism. Do not apply a simplistic rule that every sender closes a shared channel.

For wait groups or error groups, inspect startup ordering, error collection, and cancellation. Ensure the parent can observe completion on failure paths too. Avoid unbounded fan-out and make the behavior of queued work clear. Use APIs supported by the project's Go version and dependency policy.

## Context and deadlines

Follow the incoming request context into HTTP calls, database operations, and blocking work that should stop with the request. A fresh background context inside that chain can hide a cancellation break. Conversely, long-lived service work may legitimately have a different lifecycle; establish the intended behavior before declaring a defect.

When deriving a timeout or cancellation context, call its cancel function or explicitly transfer that responsibility. A deadline only helps if blocking operations observe it. A select with a cancellation case may still require reasoning about side effects if work and cancellation are ready concurrently.

Do not store request-specific dependencies as hidden context values. Keep ordinary arguments explicit and reserve values for suitable request metadata with collision-resistant key types. Avoid retaining a completed request context for later independent operations.

If work must survive an HTTP response or coordinator turn, define its actual owner, timeout, shutdown, and failure reporting. An in-process goroutine is not a durable queue, and detaching a context does not preserve work across process failure.

## Review scenarios

Trace the scenarios that matter: early consumer exit, caller cancellation, partial producer failure, dependency timeout, duplicate attempts, saturation, and shutdown while work is active. Check whether resources and goroutines finish and whether the caller receives the correct result.

Use deterministic coordination where possible to test lifecycle transitions. Race detection is useful for exercised shared-memory paths; it does not prove absence of deadlock, leaks, business-level races, or unexercised failures. Report which claim the selected evidence actually supports.
