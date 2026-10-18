# Clarity, errors, and data safety

## APIs and structure

Use names that express the domain or operation and fit their scope. Prefer cohesive packages to catch-all `utils` or `common` namespaces. Keep interfaces near consumers when they describe the behavior those consumers need. Do not introduce an interface solely because every implementation is supposed to have one.

Keep control flow understandable. Handle exceptional conditions before burying the normal operation in nested branches. Split responsibilities when it improves reasoning; neither a fixed line count nor four arguments automatically proves a design problem. Comments should explain non-obvious contracts, reasons, or constraints rather than narrate each statement.

Check compatibility at actual boundaries: public APIs, serialized values, stored data, configuration, and callers. Nil and empty slices can be equivalent internally but differ in JSON output. Preserve the intended contract instead of enforcing one universal initialization rule.

## Error behavior

Handle a returned error or explain a deliberate best-effort choice. Add useful operation context without repeatedly wrapping the same information. Use `errors.Is` or `errors.As` when the contract involves wrapped sentinels or types. Use `%w` when preserving the cause is appropriate; inspect whether exposing that cause becomes part of the caller's contract.

Keep expected failures as ordinary results or errors rather than panics. A recover boundary needs a clear owner and an observable failure response; it is not a substitute for handling errors. Avoid redundant logging and returning of the same failure at every layer, and preserve useful diagnostics at the boundary responsible for reporting it.

Inspect cleanup on early returns. The right response to a close or rollback error depends on the resource and operation: silently dropping a failed final write may lose data, while masking the original error with an irrelevant cleanup error can obscure the failure.

## Ownership and values

Examine typed nil interfaces, map initialization before writes, slice backing-array sharing, pointer aliases, and structures containing synchronization primitives. A shallow copy may still share mutable storage; a value copy containing a mutex is not a safe way to duplicate live state.

Verify boundary arithmetic and conversions where untrusted or large values affect sizes, offsets, or allocation. Do not infer a race from shared pointers alone; follow actual ownership and synchronization. Conversely, sending a pointer over a channel does not automatically make later mutation safe.

Use standard library and dependency APIs available in the project's actual Go version. Do not propose a newer language feature, toolchain, or library without establishing compatibility and authorization.
