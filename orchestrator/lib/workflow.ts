import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";

export type AgentStep = {
  id: string;
  type: "agent";
  agent: string;
  title: string;
  instructions: string;
  inputs: string[];
  outputs: string[];
  handoff?: {
    require_artifacts?: boolean;
    require_checks?: boolean;
    require_limitations?: boolean;
    artifacts?: string[];
    checks?: string[];
  };
  loop?: { prompt: string; approval_artifacts: string[]; commands: string[] };
  on: Record<string, string>;
};
export type DecisionStep = {
  id: string;
  type: "decision";
  actor: string;
  artifacts: string[];
  on: Record<string, string>;
};
export type Step = AgentStep | DecisionStep;
export type Workflow = { start: string; steps: Step[] };
export type HistoryEvent = {
  sequence: number;
  kind: string;
  actor?: string;
  attempt?: number;
  stage?: string;
  data?: Record<string, any>;
};
export type Position = {
  state: "ready" | "active" | "decision" | "complete" | "blocked";
  step?: Step;
  attempt?: number;
  assignment_id?: string;
  reason?: string;
  trace: { step: string; outcome: string; target: string }[];
};
const agentOutcomes = new Set([
  "completed",
  "ready_for_review",
  "changes_requested",
  "failed",
]);
const decisionOutcomes = new Set(["approve", "reject", "request_changes"]);
const identifier = /^[a-z][a-z0-9_-]*$/;
const stringList = (value: unknown, label: string) => {
  if (
    !Array.isArray(value) ||
    value.some((item) => typeof item !== "string" || !item.trim())
  )
    throw new Error(`${label} must be a list of nonempty strings`);
};
const fields = (
  value: Record<string, unknown>,
  allowed: string[],
  label: string,
) => {
  for (const field of Object.keys(value))
    if (!allowed.includes(field))
      throw new Error(`Unknown ${label} field: ${field}`);
};

export function validateWorkflow(
  value: unknown,
  members: string[],
  artifacts: string[],
): Workflow {
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new Error("workflow must be an object");
  const workflow = value as Workflow;
  fields(
    workflow as unknown as Record<string, unknown>,
    ["start", "steps"],
    "workflow",
  );
  if (!Array.isArray(workflow.steps) || !workflow.steps.length)
    throw new Error("workflow.steps must be nonempty");
  const ids = new Set<string>();
  for (const step of workflow.steps) {
    if (
      !step ||
      typeof step !== "object" ||
      Array.isArray(step) ||
      !identifier.test(step.id) ||
      ids.has(step.id)
    )
      throw new Error("workflow step IDs must be unique lowercase identifiers");
    ids.add(step.id);
    if (step.type === "agent") {
      fields(
        step as unknown as Record<string, unknown>,
        [
          "id",
          "type",
          "agent",
          "title",
          "instructions",
          "inputs",
          "outputs",
          "handoff",
          "loop",
          "on",
        ],
        `step ${step.id}`,
      );
      if (
        !members.includes(step.agent) ||
        typeof step.title !== "string" ||
        !step.title.trim() ||
        typeof step.instructions !== "string" ||
        !step.instructions.trim()
      )
        throw new Error(
          `Agent step ${step.id} needs a member, title, and instructions`,
        );
      stringList(step.inputs, `${step.id}.inputs`);
      stringList(step.outputs, `${step.id}.outputs`);
      if (!step.outputs.length)
        throw new Error(`Agent step ${step.id} needs outputs`);
      if ("handoff" in step) {
        if (
          !step.handoff ||
          typeof step.handoff !== "object" ||
          Array.isArray(step.handoff)
        )
          throw new Error(`Invalid handoff for ${step.id}`);
        fields(
          step.handoff,
          [
            "require_artifacts",
            "require_checks",
            "require_limitations",
            "artifacts",
            "checks",
          ],
          `${step.id}.handoff`,
        );
        for (const flag of [
          "require_artifacts",
          "require_checks",
          "require_limitations",
        ] as const)
          if (
            step.handoff[flag] !== undefined &&
            typeof step.handoff[flag] !== "boolean"
          )
            throw new Error(`Invalid ${step.id}.handoff.${flag}`);
        for (const list of ["artifacts", "checks"] as const)
          if (step.handoff[list] !== undefined)
            stringList(step.handoff[list], `${step.id}.handoff.${list}`);
      }
      if ("loop" in step) {
        if (
          !step.loop ||
          typeof step.loop !== "object" ||
          Array.isArray(step.loop)
        )
          throw new Error(`Invalid loop for ${step.id}`);
        fields(
          step.loop,
          ["prompt", "approval_artifacts", "commands"],
          `${step.id}.loop`,
        );
        if (
          typeof step.loop.prompt !== "string" ||
          !step.loop.prompt.includes("{iterations}")
        )
          throw new Error(
            `Loop for ${step.id} needs {iterations} in its prompt`,
          );
        stringList(
          step.loop.approval_artifacts,
          `${step.id}.loop.approval_artifacts`,
        );
        stringList(step.loop.commands, `${step.id}.loop.commands`);
        if (!step.loop.commands.length)
          throw new Error(`Loop for ${step.id} needs commands`);
        if (
          !step.loop.approval_artifacts.length ||
          step.loop.approval_artifacts.some((name) => !artifacts.includes(name))
        )
          throw new Error(
            `Loop for ${step.id} needs configured approval artifacts`,
          );
      }
    } else if (step.type === "decision") {
      fields(
        step as unknown as Record<string, unknown>,
        ["id", "type", "actor", "artifacts", "on"],
        `step ${step.id}`,
      );
      if (step.actor !== "human" && !members.includes(step.actor))
        throw new Error(
          `Decision ${step.id} must name human or a factory member`,
        );
      stringList(step.artifacts, `${step.id}.artifacts`);
      if (step.artifacts.some((name) => !artifacts.includes(name)))
        throw new Error(`Decision ${step.id} names an unconfigured artifact`);
    } else
      throw new Error(
        `Invalid workflow step type: ${(step as { id: string }).id}`,
      );
    if (
      !step.on ||
      typeof step.on !== "object" ||
      Array.isArray(step.on) ||
      !Object.keys(step.on).length
    )
      throw new Error(`Step ${step.id} needs outcome routes`);
    for (const [outcome, target] of Object.entries(step.on)) {
      const allowed = step.type === "agent" ? agentOutcomes : decisionOutcomes;
      if (!allowed.has(outcome) || typeof target !== "string" || !target)
        throw new Error(`Invalid route ${step.id}.${outcome}`);
    }
  }
  if (!ids.has(workflow.start))
    throw new Error("workflow.start must name a step");
  for (const step of workflow.steps)
    for (const target of Object.values(step.on))
      if (!ids.has(target) && target !== "$done" && target !== "$blocked")
        throw new Error(`Unknown workflow target: ${target}`);
  const visited = new Set<string>();
  const visit = (id: string) => {
    if (visited.has(id)) return;
    visited.add(id);
    const step = workflow.steps.find((entry) => entry.id === id)!;
    for (const target of Object.values(step.on))
      if (ids.has(target)) visit(target);
  };
  visit(workflow.start);
  if (visited.size !== ids.size)
    throw new Error("workflow contains unreachable steps");
  if (!workflow.steps.some((step) => Object.values(step.on).includes("$done")))
    throw new Error("workflow needs a route to $done");
  return workflow;
}

export function position(
  workflow: Workflow,
  events: HistoryEvent[],
  taskDirectory?: string,
): Position {
  const ordered = [...events].sort((a, b) => a.sequence - b.sequence);
  let cursor = 0;
  let current = workflow.start;
  const trace: Position["trace"] = [];
  const createdFor = (stage: string) =>
    ordered.filter(
      (event) => event.kind === "assignment_created" && event.stage === stage,
    );
  const nextAttempt = (stage: string) => {
    const created = createdFor(stage);
    return (
      Math.max(created.length, ...created.map((event) => event.attempt || 0)) +
      1
    );
  };
  // Each transition consumes at least one history event, so loops cannot spin on malformed history.
  for (let hops = 0; hops <= ordered.length + 1; hops++) {
    const step = workflow.steps.find((entry) => entry.id === current)!;
    if (step.type === "agent") {
      const created = ordered.find(
        (event) =>
          event.sequence > cursor &&
          event.kind === "assignment_created" &&
          event.stage === step.id,
      );
      if (!created)
        return {
          state: "ready",
          step,
          attempt: nextAttempt(step.id),
          trace,
        };
      const assignmentID = created.data?.assignment_id;
      if (typeof assignmentID !== "string" || !assignmentID)
        return {
          state: "blocked",
          step,
          reason: `Assignment creation for ${step.id} lacks an ID`,
          trace,
        };
      const attempt =
        created.attempt ||
        createdFor(step.id).filter(
          (event) => event.sequence <= created.sequence,
        ).length;
      const submitted = ordered.find(
        (event) =>
          event.sequence > created.sequence &&
          event.kind === "assignment_submitted" &&
          event.data?.assignment_id === assignmentID,
      );
      if (!submitted) {
        const updates = ordered.filter(
          (event) =>
            event.sequence > created.sequence &&
            event.kind === "assignment_updated" &&
            event.data?.assignment_id === assignmentID,
        );
        const state = updates.at(-1)?.data?.state;
        if (["cancelled", "declined"].includes(state)) {
          const replacement = ordered.find(
            (event) =>
              event.sequence > created.sequence &&
              event.kind === "assignment_created" &&
              event.stage === step.id,
          );
          if (replacement) {
            cursor = created.sequence;
            continue;
          }
        }
        if (["blocked", "needs_input", "declined", "cancelled"].includes(state))
          return {
            state: "blocked",
            step,
            attempt,
            assignment_id: assignmentID,
            reason: `Assignment ${assignmentID} is ${state}`,
            trace,
          };
        return {
          state: "active",
          step,
          attempt,
          assignment_id: assignmentID,
          trace,
        };
      }
      const outcome = submitted.data?.result?.outcome;
      if (!agentOutcomes.has(outcome))
        return {
          state: "blocked",
          step,
          assignment_id: assignmentID,
          reason: `Invalid result for ${step.id}`,
          trace,
        };
      cursor = submitted.sequence;
      const target = step.on[outcome];
      if (!target)
        return {
          state: "blocked",
          step,
          assignment_id: assignmentID,
          reason: `No ${outcome} route from ${step.id}`,
          trace,
        };
      trace.push({ step: step.id, outcome, target });
      if (target === "$done") return { state: "complete", trace };
      if (target === "$blocked")
        return {
          state: "blocked",
          step,
          reason: `${step.id} returned ${outcome}`,
          trace,
        };
      current = target;
    } else {
      const candidates = ordered.filter(
        (event) =>
          event.sequence > cursor &&
          event.kind === "decision" &&
          event.stage === step.id,
      );
      let staleApproval = false;
      const decision = candidates.find((event) => {
        if (event.data?.outcome !== "approve" || !step.artifacts.length)
          return true;
        if (!taskDirectory) return true;
        const hashes = event.data?.artifacts;
        const matches =
          Array.isArray(hashes) &&
          hashes.length === step.artifacts.length &&
          step.artifacts.every((name) => {
            const file = path.join(taskDirectory, name);
            const recorded = hashes.find(
              (item: any) => item.path === name,
            )?.sha256;
            return (
              fs.existsSync(file) &&
              recorded ===
                createHash("sha256").update(fs.readFileSync(file)).digest("hex")
            );
          });
        if (!matches) staleApproval = true;
        return matches;
      });
      if (!decision)
        return {
          state: "decision",
          step,
          ...(staleApproval
            ? {
                reason: `Approved artifacts changed after ${step.id}; a new decision is required`,
              }
            : {}),
          trace,
        };
      const outcome = decision.data?.outcome;
      if (!decisionOutcomes.has(outcome))
        return {
          state: "blocked",
          step,
          reason: `Invalid decision for ${step.id}`,
          trace,
        };
      if (decision.actor !== undefined && decision.actor !== step.actor)
        return {
          state: "blocked",
          step,
          reason: `Unexpected decision actor for ${step.id}`,
          trace,
        };
      if (outcome === "approve" && step.artifacts.length && !taskDirectory)
        return {
          state: "blocked",
          step,
          reason: "Task directory needed to verify approval",
          trace,
        };
      cursor = decision.sequence;
      const target = step.on[outcome];
      if (!target)
        return {
          state: "blocked",
          step,
          reason: `No ${outcome} route from ${step.id}`,
          trace,
        };
      trace.push({ step: step.id, outcome, target });
      if (target === "$done") return { state: "complete", trace };
      if (target === "$blocked")
        return {
          state: "blocked",
          step,
          reason: `${step.id} returned ${outcome}`,
          trace,
        };
      current = target;
    }
  }
  return {
    state: "blocked",
    reason: "Workflow history exceeds its transition limit",
    trace,
  };
}

export function assignmentRequest(step: AgentStep, attempt: number) {
  return {
    role: step.agent,
    title: step.title,
    instructions: step.instructions,
    inputs: step.inputs,
    expected_output: step.outputs,
    ...(step.handoff ? { handoff: step.handoff } : {}),
    stage: step.id,
    attempt,
  };
}
