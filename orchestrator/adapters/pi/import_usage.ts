import fs from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";
import { Comms, type RecordValue } from "../../lib/client.ts";
import {
  cli,
  isMain,
  readTask,
  remoteTask,
  hash,
  object,
} from "../../lib/core.ts";
export function measurements(text: string) {
  if (!text.endsWith("\n")) throw new Error("Session ends in a partial record");
  const rows = text
    .trimEnd()
    .split("\n")
    .map((line) => JSON.parse(line));
  const header = rows.shift();
  if (
    !object(header) ||
    header.type !== "session" ||
    header.version !== 3 ||
    !header.id
  )
    throw new Error("Expected Pi session v3");
  const seen = new Set<string>(),
    values: { key: string; usage: RecordValue | null }[] = [];
  for (const row of rows) {
    if (!object(row)) throw new Error("Invalid session row");
    const message = row.message;
    if (row.type !== "message" || message?.role !== "assistant") continue;
    if (typeof row.id !== "string" || seen.has(row.id))
      throw new Error("Missing/duplicate entry identity");
    seen.add(row.id);
    if (!message.usage) {
      values.push({ key: row.id, usage: null });
      continue;
    }
    if (message.stopReason === "pending")
      throw new Error("Measurement still pending");
    const counts = ["input", "output", "cacheRead", "cacheWrite"].map(
      (k) => message.usage[k],
    );
    if (
      counts.some((v) => !Number.isSafeInteger(v) || v < 0) ||
      !message.provider ||
      !message.model
    )
      throw new Error("Invalid measurement or model attribution");
    if (
      message.usage.totalTokens !== undefined &&
      message.usage.totalTokens !== counts.reduce((a, b) => a + b, 0)
    )
      throw new Error("Token totals disagree");
    values.push({
      key: header.id + ":" + row.id,
      usage: {
        provider: message.provider,
        model: message.responseModel || message.model,
        source: "runtime",
        input_tokens: counts[0] + counts[2] + counts[3],
        output_tokens: counts[1],
        cache_read_tokens: counts[2],
        cache_write_tokens: counts[3],
      },
    });
  }
  return { header, values };
}
export async function main(args = process.argv.slice(2)) {
  const { values: v, positionals: p } = parseArgs({
    args,
    allowPositionals: true,
    options: {
      session: { type: "string" },
      participant: { type: "string" },
      assignment: { type: "string" },
    },
  });
  if (p.length !== 1 || !v.session || !v.participant || !v.assignment)
    throw new Error("Specify task, --session, --participant, --assignment");
  const { metadata } = readTask(p[0]),
    client = new Comms();
  try {
    await remoteTask(client, metadata);
    const assignments = await client.pages("list_assignments", "assignments", {
      task_id: metadata.comms_id,
      assignee_id: v.participant,
    });
    if (assignments.length !== 1 || assignments[0].id !== v.assignment)
      throw new Error("Ambiguous assignment attribution");
    const assignment = assignments[0];
    const participant = (
      await client.pages("list_participants", "participants", {
        task_id: metadata.comms_id,
      })
    ).find((p) => p.id === v.participant);
    if (
      !participant ||
      path.basename(path.dirname(fs.realpathSync(v.session))) !==
        participant.instance_id
    )
      throw new Error("Session does not belong to this instance");
    const { header, values } = measurements(fs.readFileSync(v.session, "utf8"));
    if (header.cwd !== metadata.project)
      throw new Error("Session project mismatch");
    let imported = 0;
    for (const row of values)
      if (row.usage) {
        await client.call("record_usage", {
          task_id: metadata.comms_id,
          actor: participant.routing_identity,
          stage: assignment.stage,
          attempt: assignment.attempt,
          usage: row.usage,
          request_key: "pi-" + hash(v.participant + ":" + row.key),
        });
        imported++;
      }
    console.log(
      JSON.stringify({
        imported,
        missing: values.length - imported,
        cost: "unknown",
        coverage: "Assistant messages only; excludes compaction and tool usage",
      }),
    );
  } finally {
    await client.close();
  }
}
if (isMain(import.meta.url)) void cli(() => main());
