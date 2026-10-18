# Business rules and consistency

Start with a concrete business operation and examples of valid and invalid outcomes. For each important rule, record its source and any uncertainty. Source code provides evidence of implementation, but it may encode accidental or obsolete policy; do not silently promote it into the required behavior.

## Describe the model

| Question | What to establish |
|---|---|
| Meaning | Which terms matter, and what do they mean in this context? |
| Identity | Does a concept remain the same thing as its attributes change? |
| Lifecycle | What states and transitions are valid, and who may trigger them? |
| Invariants | What must remain true after a successful operation? |
| Ownership | Which part of the system is responsible for preserving each rule? |
| Evidence | Which examples or checks demonstrate the rule? |

Use an entity when identity over time matters. Use a value object when equality and validity belong to the value itself, and that distinction improves the design. A named primitive wrapper without meaningful behavior or constraints is not automatically useful.

## Draw a consistency boundary

An aggregate is a candidate boundary for state that must be validated and committed together to preserve a business invariant. Derive that boundary from rules, operations, and lifecycle, not from table joins or a screen showing related data.

Keep the boundary as small as the actual invariant permits. Consider contention, concurrent changes, loading cost, and which operations require the whole state. Do not split required atomic behavior merely to obtain smaller types; do not combine unrelated state merely because one workflow touches it.

State how enforcement survives concurrent requests. An in-memory existence check alone does not establish global uniqueness. Depending on the rule and storage system, a database constraint, transaction, conditional write, or another concurrency mechanism may be needed. The plan must describe the chosen mechanism and how conflicting attempts behave.

Some operations legitimately involve several consistency boundaries. If temporary inconsistency is acceptable, name the intermediate states, time expectations, and reconciliation or recovery behavior. If it is not acceptable, state the actual atomicity requirement before proposing asynchronous integration.

## Make behavior reviewable

For example, a reservation request may need to reject a second confirmation, preserve capacity under concurrent requests, or distinguish expiry from cancellation. Those are different rules. Confirm which ones the task actually requires rather than importing them as a generic reservation model.

Capture relevant scenarios in the spec: initial state, actor or input, expected transition, and observable result. Include repeated, invalid, or concurrent operations only where they threaten the agreed behavior. Map the implementation approach and meaningful tests to those scenarios in the plan.

For Go, express the selected concepts idiomatically using packages, structs, functions, and small interfaces. Avoid translating an object-oriented pattern hierarchy literally. A simpler procedural approach is acceptable when it preserves the rules more clearly.
