import fs from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";
import { Comms, type RecordValue } from "../lib/client.ts";
import {
  tasksRoot,
  selectFactory,
  cli,
  isMain,
  projectRoot,
  factoryDefinition,
  readTask,
  remoteTask,
  id,
  hash,
  json,
  writeJSON,
  locked,
  event,
  inside,
} from "../lib/core.ts";
const escapeHTML = (text: string) =>
  text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
export async function attachRecord(
  client: Comms,
  record: RecordValue,
  factory: string,
) {
  const definition = factoryDefinition(factory);
  if (record.project !== projectRoot() || record.owner !== definition.owner)
    throw new Error("Task must match selected project and factory owner");
  await client.call("set_task_factory", {
    task_id: record.id,
    factory: {
      name: definition.name,
      owner: definition.owner,
      agents: definition.agents.map((m: RecordValue) => ({
        name: m.name,
        path: m.path,
      })),
    },
  });
  const root = tasksRoot();
  fs.mkdirSync(root, { recursive: true });
  return locked(path.join(root, ".attach.lock"), async () => {
    let target: string | undefined;
    for (const entry of fs.readdirSync(root, { withFileTypes: true }))
      if (entry.isDirectory() && !entry.name.startsWith(".")) {
        const dir = path.join(root, entry.name),
          pointer = path.join(dir, "task.json");
        if (!fs.existsSync(pointer)) continue;
        const metadata = readTask(dir).metadata;
        if (metadata.comms_id === record.id) {
          if (metadata.factory !== factory)
            throw new Error("Task already attached to another factory");
          target = dir;
          for (const file of [
            "reviews",
            ...Object.keys(definition.artifacts || {}),
          ])
            if (!fs.existsSync(path.join(dir, file)))
              throw new Error(
                "Incomplete attachment; restore missing documents",
              );
          break;
        }
      }
    if (!target) {
      const slug =
        record.name
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, "-")
          .replace(/^-|-$/g, "")
          .slice(0, 64) || "task";
      const stamp = new Date().toISOString().replace(/[-:.]/g, "");
      target = path.join(root, stamp + "-" + slug);
      if (fs.existsSync(target)) target += "-" + id().slice(0, 8);
      const staging = fs.mkdtempSync(path.join(root, ".attaching-"));
      try {
        fs.mkdirSync(path.join(staging, "reviews"));
        for (const [document, templatePath] of Object.entries(
          definition.artifacts || {},
        )) {
          const template = fs.readFileSync(templatePath as string, "utf8");
          const text = template.replace(
            /\$\$|\$\{?([a-zA-Z_][a-zA-Z_0-9]*)\}?/g,
            (match, key) => {
              if (match === "$$") return "$";
              if (!["title", "brief"].includes(key))
                throw new Error(`Unknown template variable: ${key}`);
              return escapeHTML(key === "title" ? record.name : record.brief);
            },
          );
          fs.writeFileSync(path.join(staging, document), text);
        }
        writeJSON(path.join(staging, "task.json"), {
          schema_version: 2,
          id: path.basename(target),
          comms_id: record.id,
          project: record.project,
          name: record.name,
          factory,
          factory_config: process.env.FACTORY_CONFIG,
          brief: record.brief,
          owner: record.owner,
          created_at: record.created_at,
        });
        fs.renameSync(staging, target);
      } finally {
        fs.rmSync(staging, { recursive: true, force: true });
      }
    }
    try {
      await client.call("record_event", {
        task_id: record.id,
        actor: record.owner,
        kind: "note",
        message: "artifacts_attached",
        data: {
          event_type: "artifacts_attached",
          artifact_dir: target,
          factory,
        },
        request_key: "artifacts-" + hash(target),
      });
    } catch {
      throw new Error(
        `Artifacts saved at ${target}; retry attach to record location`,
      );
    }
    return target;
  });
}
export async function main(args = process.argv.slice(2)) {
  const { values: v, positionals: p } = parseArgs({
    args,
    allowPositionals: true,
    options: {
      factory: { type: "string" },
      project: { type: "string" },
      name: { type: "string" },
      brief: { type: "string" },
      "brief-file": { type: "string" },
      text: { type: "string" },
      "text-file": { type: "string" },
      "request-key": { type: "string" },
      state: { type: "string" },
      summary: { type: "string" },
      from: { type: "string" },
      to: { type: "string" },
      kind: { type: "string" },
      for: { type: "string" },
      all: { type: "boolean" },
      by: { type: "string" },
      outcome: { type: "string" },
      artifact: { type: "string", multiple: true },
    },
  });
  if (v.project) process.env.FACTORY_PROJECT = fs.realpathSync(v.project);
  selectFactory(v.factory);
  const input = (field: "text" | "brief") => {
    const file = v[`${field}-file`];
    if (file && v[field]) throw new Error("Choose text or file");
    const text = file
      ? fs.readFileSync(file === "-" ? 0 : file, "utf8")
      : v[field];
    if (!text?.trim()) throw new Error(`${field} required`);
    return text.trim();
  };
  const client = new Comms();
  try {
    if (p[0] === "new") {
      if (!v.name?.trim()) throw new Error("new requires --name");
      const def = factoryDefinition(v.factory);
      const record = await client.call("create_task", {
        project: projectRoot(),
        name: v.name,
        brief: input("brief"),
        owner: def.owner,
        request_key: v["request-key"] || id(),
      });
      console.log(await attachRecord(client, record, def.name));
      return;
    }
    if (p[0] === "attach") {
      if (!p[1]) throw new Error("attach <task-id> required");
      console.log(
        await attachRecord(
          client,
          await client.call("get_task", { task_id: p[1] }),
          factoryDefinition().name,
        ),
      );
      return;
    }
    if (!p[1]) throw new Error("Specify command and task directory");
    const { directory, metadata } = readTask(p[1]);
    let result: unknown;
    if (p[0] === "status") {
      const record = await remoteTask(client, metadata);
      result = v.state
        ? await client.call("update_task", {
            task_id: record.id,
            actor: record.owner,
            expected_version: record.version,
            status: v.state,
            summary: v.summary || "",
          })
        : record;
    } else if (p[0] === "send")
      result = await client.call("send_message", {
        task_id: metadata.comms_id,
        sender: v.from,
        recipient: v.to,
        kind: v.kind,
        text: input("text"),
        request_key: v["request-key"] || id(),
      });
    else if (p[0] === "inbox")
      result = await client.pages("read_inbox", "messages", {
        task_id: metadata.comms_id,
        recipient: v.for,
        include_acknowledged: !!v.all,
      });
    else if (p[0] === "ack" || p[0] === "exec")
      result = await client.call(
        p[0] === "ack" ? "acknowledge_message" : "execute_message",
        { task_id: metadata.comms_id, message_id: p[2], actor: v.by },
      );
    else if (p[0] === "note" || p[0] === "decide") {
      const artifacts = (v.artifact || []).map((ref) => {
        const file = fs.realpathSync(path.resolve(directory, ref));
        if (!inside(directory, file))
          throw new Error("Decision artifact must be inside task");
        return {
          path: path.relative(directory, file),
          sha256: hash(fs.readFileSync(file)),
        };
      });
      if (!v.by) throw new Error("--by required");
      if (
        p[0] === "decide" &&
        !["approve", "reject", "request_changes"].includes(v.outcome || "")
      )
        throw new Error("Invalid decision outcome");
      result = await event(
        client,
        metadata,
        v.by,
        p[0] === "decide" ? "decision" : "note",
        {
          text: input("text"),
          ...(p[0] === "decide" ? { outcome: v.outcome, artifacts } : {}),
        },
      );
    } else
      throw new Error(
        "Commands: new, attach, status, send, inbox, ack, exec, decide, note",
      );
    console.log(JSON.stringify(result, null, 2));
  } finally {
    await client.close();
  }
}
if (isMain(import.meta.url)) void cli(() => main());
