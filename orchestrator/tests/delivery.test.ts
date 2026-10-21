import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import { fixture } from "./fixture.ts";
import deliveryExtension, {
  InboxDelivery,
  assignmentPermission,
  type Binding,
} from "../extensions/delivery.ts";
import { Comms, type RecordValue } from "../lib/client.ts";
let f: Awaited<ReturnType<typeof fixture>>;
before(async () => {
  f = await fixture();
});
after(async () => {
  await f?.close();
});
test("native inbox preserves sent/read/exec, recipient isolation, reconnect and retired-instance rejection", async () => {
  const task = await f.client.call("create_task", {
    project: f.project,
    name: "Delivery",
    brief: "Fixture",
    owner: "coordinator",
    request_key: "delivery",
  });
  const participant = await f.client.call("register_participant", {
    task_id: task.id,
    instance_id: "fixture-coder",
    role: "coder",
    routing_identity: "coder",
    request_key: "coder",
  });
  const manifest = {
    runner: "pi",
    delivery: "native",
    provider: "fixture",
    model: "fixture",
    thinking: "",
    skills: [],
    mcps: [],
    prepared_at: new Date().toISOString(),
    startup_checked: true,
    configuration_fingerprint: "a".repeat(64),
  };
  await f.client.call("record_session", {
    task_id: task.id,
    participant_id: participant.id,
    manifest,
    request_key: "manifest",
  });
  const binding: Binding = {
    task_id: task.id,
    participant_id: participant.id,
    instance_id: "fixture-coder",
    recipient: "coder",
  };
  const inbox = new InboxDelivery(f.client, binding);
  const message = await f.client.call("send_message", {
    task_id: task.id,
    sender: "coordinator",
    recipient: "coder",
    kind: "assignment",
    text: "Implement one item",
    request_key: "message",
  });
  const other = await f.client.call("send_message", {
    task_id: task.id,
    sender: "coordinator",
    recipient: "reviewer",
    kind: "assignment",
    text: "Review",
    request_key: "other",
  });
  const notifications: string[][] = [];
  await inbox.poll(false, (ids) => notifications.push(ids));
  assert.equal(notifications.length, 0);
  await inbox.poll(true, (ids) => notifications.push(ids));
  await inbox.poll(true, (ids) => notifications.push(ids));
  assert.deepEqual(notifications, [[message.id]]);
  assert.equal(
    (
      await f.client.pages("read_inbox", "messages", {
        task_id: task.id,
        recipient: "coder",
      })
    )[0].status,
    "sent",
  );
  await assert.rejects(inbox.exec(message.id), /read/);
  await assert.rejects(inbox.read(other.id), /recipient inbox/);
  const reconnect = new Comms(f.env);
  try {
    const recovered = new InboxDelivery(reconnect, binding);
    await recovered.poll(true, (ids) => notifications.push(ids));
    assert.equal(notifications.length, 2);
    assert.equal((await recovered.read(message.id)).status, "read");
    assert.equal((await recovered.exec(message.id)).status, "exec");
    assert.equal((await recovered.exec(message.id)).status, "exec");
  } finally {
    await reconnect.close();
  }
  await inbox.poll(true, (ids) => notifications.push(ids));
  assert.equal(notifications.length, 2);
  await f.client.call("register_participant", {
    task_id: task.id,
    instance_id: "replacement",
    role: "coder",
    routing_identity: "coder",
    replaces_participant_id: participant.id,
    request_key: "replacement",
  });
  await assert.rejects(inbox.read(message.id));
  await assert.rejects(inbox.exec(message.id));
});
test("overlapping polls are serialized and failed notification can be retried", async () => {
  let calls = 0,
    release: () => void = () => {};
  const held = new Promise<void>((resolve) => (release = resolve));
  const port = {
    async call() {
      calls++;
      await held;
      return {};
    },
    async pages() {
      return [{ id: "m1" }];
    },
  };
  const inbox = new InboxDelivery(port, {
    task_id: "t",
    participant_id: "p",
    instance_id: "i",
    recipient: "r",
  });
  const first = inbox.poll(true, () => {
    throw new Error("notification failed");
  });
  await inbox.poll(true, () => assert.fail());
  assert.equal(calls, 1);
  release();
  await assert.rejects(first, /notification failed/);
  let notified = 0;
  await inbox.poll(true, () => {
    notified++;
  });
  assert.equal(notified, 1);
});
test("extension exposes explicit read and exec tools without a terminal dependency", () => {
  const tools: RecordValue[] = [];
  const commands: string[] = [];
  deliveryExtension({
    registerTool: (tool: RecordValue) => tools.push(tool),
    registerCommand: (name: string) => commands.push(name),
    on: () => {},
  } as any);
  assert.deepEqual(
    tools.map((t) => t.name),
    ["factory_bind_task", "factory_read_message", "factory_exec_message"],
  );
  assert.deepEqual(commands, ["factory-inbox"]);
});
test("assignment guard stays bound to one assignment across review feedback", async () => {
  const binding: Binding = {
    task_id: "task",
    participant_id: "coder",
    instance_id: "instance",
    recipient: "coder",
  };
  let taskStatus = "working";
  const assignments: RecordValue[] = [];
  const port = {
    async call() {
      return { status: taskStatus };
    },
    async pages() {
      return assignments;
    },
  };
  assert.equal((await assignmentPermission(port, binding)).allowed, false);
  assignments.push({ id: "first", state: "working" });
  const started = await assignmentPermission(port, binding);
  assert.deepEqual(started, {
    allowed: true,
    assignment_id: "first",
    reason: "",
  });
  assignments[0].state = "completed";
  assignments.push({ id: "review-repair", state: "offered" });
  assert.equal(
    (await assignmentPermission(port, binding, started.assignment_id)).allowed,
    false,
  );
  taskStatus = "cancelled";
  assert.equal(
    (await assignmentPermission(port, binding, started.assignment_id)).reason,
    "Task is terminal",
  );
});
