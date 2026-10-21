import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import {
  position,
  validateWorkflow,
  type HistoryEvent,
} from "../lib/workflow.ts";

const workflow = validateWorkflow(
  {
    start: "draft",
    steps: [
      {
        id: "draft",
        type: "agent",
        agent: "writer",
        title: "Draft",
        instructions: "Write",
        inputs: ["brief"],
        outputs: ["spec.html"],
        on: { completed: "review", failed: "$blocked" },
      },
      {
        id: "review",
        type: "agent",
        agent: "reviewer",
        title: "Review",
        instructions: "Review",
        inputs: ["spec.html"],
        outputs: ["findings"],
        on: {
          completed: "approve",
          changes_requested: "draft",
          failed: "$blocked",
        },
      },
      {
        id: "approve",
        type: "decision",
        actor: "human",
        artifacts: ["spec.html"],
        on: { approve: "$done", request_changes: "draft", reject: "$blocked" },
      },
    ],
  },
  ["writer", "reviewer"],
  ["spec.html"],
);

test("workflow rejects invalid references and unreachable steps", () => {
  assert.throws(
    () =>
      validateWorkflow(
        { ...workflow, start: "missing" },
        ["writer", "reviewer"],
        ["spec.html"],
      ),
    /start/,
  );
  assert.throws(
    () =>
      validateWorkflow(
        {
          ...workflow,
          steps: [...workflow.steps, { ...workflow.steps[0], id: "orphan" }],
        },
        ["writer", "reviewer"],
        ["spec.html"],
      ),
    /unreachable/,
  );
  assert.throws(
    () =>
      validateWorkflow(
        {
          ...workflow,
          steps: [
            { ...workflow.steps[0], on: { completed: "typo" } },
            ...workflow.steps.slice(1),
          ],
        },
        ["writer", "reviewer"],
        ["spec.html"],
      ),
    /Unknown workflow target/,
  );
  assert.throws(
    () =>
      validateWorkflow(
        {
          ...workflow,
          steps: [
            {
              ...workflow.steps[0],
              loop: {
                prompt: "Run",
                approval_artifacts: ["spec.html"],
                commands: ["ralph-loop"],
              },
            },
            ...workflow.steps.slice(1),
          ],
        },
        ["writer", "reviewer"],
        ["spec.html"],
      ),
    /needs \{iterations\}/,
  );
  assert.throws(
    () =>
      validateWorkflow(
        {
          ...workflow,
          steps: [
            { ...workflow.steps[0], handoff: null },
            ...workflow.steps.slice(1),
          ],
        },
        ["writer", "reviewer"],
        ["spec.html"],
      ),
    /Invalid handoff/,
  );
});

test("workflow follows review loops and verifies exact approved documents", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "factory-workflow-"));
  try {
    const document = path.join(directory, "spec.html");
    fs.writeFileSync(document, "approved draft");
    const hash = createHash("sha256")
      .update(fs.readFileSync(document))
      .digest("hex");
    const events: HistoryEvent[] = [];
    const created = (stage: string, assignment_id: string) =>
      events.push({
        sequence: events.length + 1,
        kind: "assignment_created",
        stage,
        data: { assignment_id },
      });
    const submitted = (assignment_id: string, outcome: string) =>
      events.push({
        sequence: events.length + 1,
        kind: "assignment_submitted",
        data: { assignment_id, result: { outcome } },
      });
    assert.equal(position(workflow, events, directory).step?.id, "draft");
    created("draft", "draft-1");
    assert.equal(position(workflow, events, directory).state, "active");
    events.push({
      sequence: events.length + 1,
      kind: "assignment_updated",
      data: { assignment_id: "draft-1", state: "blocked" },
    });
    assert.equal(position(workflow, events, directory).state, "blocked");
    events.push({
      sequence: events.length + 1,
      kind: "assignment_updated",
      data: { assignment_id: "draft-1", state: "working" },
    });
    assert.equal(position(workflow, events, directory).state, "active");
    submitted("draft-1", "completed");
    created("review", "review-1");
    submitted("review-1", "changes_requested");
    assert.deepEqual(
      {
        state: position(workflow, events, directory).state,
        step: position(workflow, events, directory).step?.id,
        attempt: position(workflow, events, directory).attempt,
      },
      { state: "ready", step: "draft", attempt: 2 },
    );
    created("draft", "draft-2");
    submitted("draft-2", "completed");
    created("review", "review-2");
    submitted("review-2", "completed");
    assert.equal(position(workflow, events, directory).state, "decision");
    events.push({
      sequence: events.length + 1,
      kind: "decision",
      actor: "human",
      stage: "approve",
      data: {
        outcome: "approve",
        artifacts: [{ path: "spec.html", sha256: hash }],
      },
    });
    assert.equal(position(workflow, events, directory).state, "complete");
    fs.writeFileSync(document, "changed after approval");
    assert.equal(position(workflow, events, directory).state, "decision");
    assert.match(position(workflow, events, directory).reason || "", /changed/);
    events.push({
      sequence: events.length + 1,
      kind: "decision",
      actor: "human",
      stage: "approve",
      data: {
        outcome: "approve",
        artifacts: [
          {
            path: "spec.html",
            sha256: createHash("sha256")
              .update(fs.readFileSync(document))
              .digest("hex"),
          },
        ],
      },
    });
    assert.equal(position(workflow, events, directory).state, "complete");
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

test("replaced cancelled or declined assignments follow their replacement", () => {
  for (const terminalState of ["cancelled", "declined"]) {
    const events: HistoryEvent[] = [
      {
        sequence: 1,
        kind: "assignment_created",
        stage: "draft",
        attempt: 1,
        data: { assignment_id: "original" },
      },
      {
        sequence: 2,
        kind: "assignment_updated",
        data: { assignment_id: "original", state: terminalState },
      },
    ];
    assert.equal(position(workflow, events).state, "blocked");
    events.push({
      sequence: 3,
      kind: "assignment_created",
      stage: "draft",
      attempt: 2,
      data: { assignment_id: "replacement" },
    });
    assert.deepEqual(
      {
        state: position(workflow, events).state,
        assignment: position(workflow, events).assignment_id,
        attempt: position(workflow, events).attempt,
      },
      { state: "active", assignment: "replacement", attempt: 2 },
    );
    events.push({
      sequence: 4,
      kind: "assignment_submitted",
      data: { assignment_id: "replacement", result: { outcome: "completed" } },
    });
    assert.deepEqual(
      {
        state: position(workflow, events).state,
        step: position(workflow, events).step?.id,
      },
      { state: "ready", step: "review" },
    );
  }
});

test("recovery creation recorded before cancellation still follows the replacement", () => {
  const events: HistoryEvent[] = [
    {
      sequence: 1,
      kind: "assignment_created",
      stage: "draft",
      attempt: 1,
      data: { assignment_id: "original" },
    },
    {
      sequence: 2,
      kind: "assignment_created",
      stage: "draft",
      attempt: 2,
      data: { assignment_id: "replacement" },
    },
    {
      sequence: 3,
      kind: "assignment_updated",
      data: { assignment_id: "original", state: "cancelled" },
    },
  ];
  assert.equal(position(workflow, events).assignment_id, "replacement");
  assert.equal(position(workflow, events).state, "active");
});

test("reapproval after implementation requires a new build attempt", () => {
  const directory = fs.mkdtempSync(
    path.join(os.tmpdir(), "factory-reapproval-"),
  );
  try {
    const file = path.join(directory, "spec.html");
    fs.writeFileSync(file, "v1");
    const approvedHash = () =>
      createHash("sha256").update(fs.readFileSync(file)).digest("hex");
    const graph = validateWorkflow(
      {
        start: "draft",
        steps: [
          {
            id: "draft",
            type: "agent",
            agent: "writer",
            title: "Draft",
            instructions: "Write",
            inputs: ["brief"],
            outputs: ["spec.html"],
            on: { completed: "approval" },
          },
          {
            id: "approval",
            type: "decision",
            actor: "human",
            artifacts: ["spec.html"],
            on: { approve: "build" },
          },
          {
            id: "build",
            type: "agent",
            agent: "writer",
            title: "Build",
            instructions: "Implement",
            inputs: ["spec.html"],
            outputs: ["result"],
            on: { completed: "$done" },
          },
        ],
      },
      ["writer"],
      ["spec.html"],
    );
    const events: HistoryEvent[] = [
      {
        sequence: 1,
        kind: "assignment_created",
        stage: "draft",
        attempt: 1,
        data: { assignment_id: "draft-1" },
      },
      {
        sequence: 2,
        kind: "assignment_submitted",
        data: { assignment_id: "draft-1", result: { outcome: "completed" } },
      },
      {
        sequence: 3,
        kind: "decision",
        actor: "human",
        stage: "approval",
        data: {
          outcome: "approve",
          artifacts: [{ path: "spec.html", sha256: approvedHash() }],
        },
      },
      {
        sequence: 4,
        kind: "assignment_created",
        stage: "build",
        attempt: 1,
        data: { assignment_id: "build-1" },
      },
      {
        sequence: 5,
        kind: "assignment_submitted",
        data: { assignment_id: "build-1", result: { outcome: "completed" } },
      },
    ];
    assert.equal(position(graph, events, directory).state, "complete");
    fs.writeFileSync(file, "v2");
    assert.equal(position(graph, events, directory).step?.id, "approval");
    events.push({
      sequence: 6,
      kind: "decision",
      actor: "human",
      stage: "approval",
      data: {
        outcome: "approve",
        artifacts: [{ path: "spec.html", sha256: approvedHash() }],
      },
    });
    assert.deepEqual(
      {
        state: position(graph, events, directory).state,
        attempt: position(graph, events, directory).attempt,
      },
      { state: "ready", attempt: 2 },
    );
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
