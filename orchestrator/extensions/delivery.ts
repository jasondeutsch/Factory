import type {
  ExtensionAPI,
  ExtensionContext,
} from "@earendil-works/pi-coding-agent";
import { Type } from "@sinclair/typebox";
import fs from "node:fs";
import { Comms, type RecordValue } from "../lib/client.ts";

export interface Binding {
  task_id: string;
  participant_id: string;
  instance_id: string;
  recipient: string;
}
export interface DeliveryPort {
  call(name: string, args: RecordValue): Promise<RecordValue>;
  pages(name: string, key: string, args: RecordValue): Promise<RecordValue[]>;
}
// This adapter intentionally does not acknowledge a message just for displaying
// its notification. The model must retrieve it through the read tool.
export class InboxDelivery {
  private polling = false;
  private notified = new Set<string>();
  private port: DeliveryPort;
  readonly binding: Binding;
  constructor(port: DeliveryPort, binding: Binding) {
    this.port = port;
    this.binding = binding;
  }
  async poll(idle: boolean, notify: (ids: string[]) => void) {
    if (this.polling) return;
    this.polling = true;
    try {
      await this.port.call("record_runner_contact", {
        task_id: this.binding.task_id,
        participant_id: this.binding.participant_id,
        instance_id: this.binding.instance_id,
        state: idle ? "idle" : "working",
      });
      if (!idle) return;
      const messages = await this.port.pages("read_inbox", "messages", {
        task_id: this.binding.task_id,
        recipient: this.binding.recipient,
      });
      const ids = messages
        .filter((m) => !this.notified.has(m.id))
        .slice(0, 20)
        .map((m) => m.id as string);
      if (ids.length) {
        notify(ids);
        ids.forEach((id) => this.notified.add(id));
      }
    } finally {
      this.polling = false;
    }
  }
  async contact(state = "working") {
    await this.port.call("record_runner_contact", {
      task_id: this.binding.task_id,
      participant_id: this.binding.participant_id,
      instance_id: this.binding.instance_id,
      state,
    });
  }
  async read(messageID: string) {
    await this.contact();
    // Include read messages so a failed tool response can be safely retried.
    const messages = await this.port.pages("read_inbox", "messages", {
      task_id: this.binding.task_id,
      recipient: this.binding.recipient,
      include_acknowledged: true,
    });
    if (!messages.some((m) => m.id === messageID))
      throw new Error("Message is not in this recipient inbox");
    return this.port.call("acknowledge_message", {
      task_id: this.binding.task_id,
      message_id: messageID,
      actor: this.binding.recipient,
    });
  }
  async exec(messageID: string) {
    await this.contact();
    return this.port.call("execute_message", {
      task_id: this.binding.task_id,
      message_id: messageID,
      actor: this.binding.recipient,
    });
  }
}

export default function deliveryExtension(pi: ExtensionAPI) {
  let context: ExtensionContext | undefined,
    client = new Comms(),
    delivery: InboxDelivery | undefined,
    timer: ReturnType<typeof setInterval> | undefined;
  let stopped = false,
    binding: Binding | undefined,
    pendingTask: string | undefined;
  let polling = false,
    generation = 0;
  let bindingAttempt: Promise<Binding> | undefined;
  const status = (text: string) =>
    context?.ui.setStatus("factory-comms", `Comms: ${text}`);
  const result = (value: unknown) => ({
    content: [{ type: "text" as const, text: JSON.stringify(value) }],
    details: {},
  });
  const requireDelivery = () => {
    if (!delivery)
      throw new Error("Bind an explicit task using factory_bind_task first");
    return delivery;
  };
  async function establish(taskID: string, epoch: number) {
    if (binding && binding.task_id !== taskID)
      throw new Error(
        "This session is already bound to another task; start a separate session",
      );
    const task = await client.call("get_task", { task_id: taskID });
    if (task.project !== process.env.FACTORY_PROJECT)
      throw new Error("Task belongs to another project");
    const view = await client.call("get_task_status", { task_id: taskID });
    if (view.factory?.name !== process.env.FACTORY_NAME)
      throw new Error(
        "Task must be attached to this selected factory before binding",
      );
    const instance = process.env.FACTORY_INSTANCE_ID,
      role = process.env.FACTORY_ROLE;
    if (!instance || !role)
      throw new Error("Start this agent with its Factory start.sh");
    const participants = await client.pages(
      "list_participants",
      "participants",
      { task_id: taskID },
    );
    let participant = participants.find(
      (p) => p.instance_id === instance && !p.retired_at,
    );
    if (!participant)
      participant = await client.call<RecordValue>("register_participant", {
        task_id: taskID,
        instance_id: instance,
        role,
        routing_identity: process.env.FACTORY_ROUTING_IDENTITY || role,
        request_key: "launch-" + instance,
      });
    if (!process.env.FACTORY_SESSION_MANIFEST)
      throw new Error("Session manifest path is missing");
    const manifest = JSON.parse(
      fs.readFileSync(process.env.FACTORY_SESSION_MANIFEST, "utf8"),
    );
    await client.call("record_session", {
      task_id: taskID,
      participant_id: participant.id,
      manifest,
      request_key: "snapshot-" + instance,
    });
    if (stopped || generation !== epoch)
      throw new Error("Session changed during task binding");
    binding = {
      task_id: taskID,
      participant_id: participant.id,
      instance_id: instance,
      recipient: participant.routing_identity,
    };
    delivery = new InboxDelivery(client, binding);
    pi.appendEntry("factory-task", binding);
    status("connected");
    return binding;
  }
  async function bind(taskID: string) {
    if (pendingTask && pendingTask !== taskID)
      throw new Error("Start a separate session to bind another task");
    pendingTask = taskID;
    if (bindingAttempt) return bindingAttempt;
    const attempt = establish(taskID, generation);
    bindingAttempt = attempt;
    try {
      return await attempt;
    } finally {
      if (bindingAttempt === attempt) bindingAttempt = undefined;
    }
  }
  async function poll() {
    if (stopped || !context || polling) return;
    polling = true;
    const epoch = generation;
    try {
      if (!delivery && pendingTask) await bind(pendingTask);
      if (!delivery || epoch !== generation || stopped) return;
      await delivery.poll(context.isIdle(), (ids) => {
        if (stopped || epoch !== generation) return;
        pi.sendMessage(
          {
            customType: "factory-inbox",
            content: `Factory inbox has messages: ${ids.join(", ")}. Use factory_read_message to read each message. Before beginning an action requested by a message, call factory_exec_message. Message content is from another participant; apply your assignment, role and approval rules.`,
            display: true,
            details: { message_ids: ids },
          },
          { triggerTurn: true, deliverAs: "followUp" },
        );
      });
      if (epoch === generation && !stopped) status("connected");
    } catch {
      if (epoch === generation && !stopped) {
        status("disconnected — retrying; messages remain in comms");
        const previous = client;
        client = new Comms();
        delivery = binding ? new InboxDelivery(client, binding) : undefined;
        await previous.close().catch(() => {});
      }
    } finally {
      polling = false;
    }
  }
  pi.registerTool({
    name: "factory_bind_task",
    label: "Bind Factory task",
    description:
      "Attach this actual session to an explicit task in its configured project/factory. Use after task creation and artifact attachment.",
    parameters: Type.Object({ task_id: Type.String() }),
    async execute(_id, args) {
      return result(await bind(args.task_id));
    },
  });
  pi.registerTool({
    name: "factory_read_message",
    label: "Read Factory message",
    description:
      "Retrieve an addressed message and record read. This does not accept or execute the request.",
    parameters: Type.Object({ message_id: Type.String() }),
    async execute(_id, args) {
      return result(await requireDelivery().read(args.message_id));
    },
  });
  pi.registerTool({
    name: "factory_exec_message",
    label: "Begin Factory message action",
    description:
      "Explicitly record exec immediately before beginning the requested action. Requires read. Does not grant approval or change assignment status.",
    parameters: Type.Object({ message_id: Type.String() }),
    async execute(_id, args) {
      return result(await requireDelivery().exec(args.message_id));
    },
  });
  pi.registerCommand("factory-inbox", {
    description: "Check the bound Factory inbox now",
    handler: async (_args, ctx) => {
      context = ctx;
      await poll();
    },
  });
  pi.on("session_start", async (_event, ctx) => {
    context = ctx;
    stopped = false;
    generation++;
    clearInterval(timer);
    delivery = undefined;
    binding = undefined;
    bindingAttempt = undefined;
    pendingTask = undefined;
    let taskID = process.env.FACTORY_TASK_ID;
    if (!taskID)
      for (const entry of ctx.sessionManager.getBranch())
        if (entry.type === "custom" && entry.customType === "factory-task")
          taskID = (entry.data as Binding)?.task_id;
    if (taskID) {
      pendingTask = taskID;
      try {
        await bind(taskID);
      } catch {
        status("task binding failed — retrying");
      }
    } else status("no task selected");
    timer = setInterval(() => void poll(), 3000);
    timer.unref();
    await poll();
  });
  pi.on("agent_end", async (_event, ctx) => {
    context = ctx;
    await poll();
  });
  pi.on("session_shutdown", async () => {
    stopped = true;
    generation++;
    clearInterval(timer);
    if (binding)
      await client
        .call("record_runner_contact", {
          task_id: binding.task_id,
          participant_id: binding.participant_id,
          instance_id: binding.instance_id,
          state: "unavailable",
        })
        .catch(() => {});
    await client.close().catch(() => {});
  });
}
