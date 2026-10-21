import fs from "node:fs";
import path from "node:path";
import { isDeepStrictEqual, parseArgs } from "node:util";
import { Comms, type RecordValue } from "./lib/client.ts";
import {
  ROOT,
  cli,
  isMain,
  readTask,
  remoteTask,
  factoryDefinition,
  selectFactory,
  configuration,
  strings,
  hash,
  id,
  json,
  writeJSON,
  locked,
  runtimeRoot,
  sleep,
} from "./lib/core.ts";
import { terminalHost, createPane, launchWindow } from "./lib/terminal.ts";
import {
  assignmentRequest,
  position,
  type HistoryEvent,
  type Workflow,
} from "./lib/workflow.ts";
export async function main(argv = process.argv.slice(2)) {
  const { values: v, positionals } = parseArgs({
    args: argv,
    allowPositionals: true,
    options: {
      request: { type: "string" },
      key: { type: "string" },
      owner: { type: "string" },
      "reuse-participant": { type: "string" },
      direction: { type: "string", default: "right" },
      "recover-pane": { type: "string" },
      timeout: { type: "string", default: "60" },
    },
  });
  if (positionals.length !== 1 || !v.request || !v.key?.trim() || !v.owner)
    throw new Error(
      "dispatch.ts <task-dir> --request <json> --key <retry-key> --owner <participant-id>",
    );
  if (!["right", "down"].includes(v.direction!))
    throw new Error("Invalid split direction");
  const timeout = Number(v.timeout);
  if (!(timeout > 0 && timeout <= 300))
    throw new Error("Timeout must be 1–300 seconds");
  const task = readTask(positionals[0]);
  selectFactory(task.metadata.factory_config);
  const definition = factoryDefinition(task.metadata.factory),
    request = json(v.request);
  for (const field of ["role", "title", "instructions", "stage"])
    if (typeof request[field] !== "string" || !request[field].trim())
      throw new Error(`Request missing ${field}`);
  if (
    !Number.isInteger(request.attempt) ||
    request.attempt < 1 ||
    !Array.isArray(request.expected_output) ||
    !request.expected_output.length
  )
    throw new Error("Request needs positive attempt and expected_output");
  for (const field of Object.keys(request))
    if (
      ![
        "role",
        "title",
        "instructions",
        "inputs",
        "expected_output",
        "stage",
        "attempt",
        "handoff",
      ].includes(field)
    )
      throw new Error(`Unknown request field: ${field}`);
  strings(request.expected_output, "expected_output");
  if (request.inputs) strings(request.inputs, "inputs");
  const { member, config } = configuration(request.role, task.metadata.factory);
  request.handoff ??= member.handoff || {};
  const host = terminalHost(definition.terminal_host),
    key = hash(task.metadata.comms_id + "\0" + v.key),
    directory = path.join(runtimeRoot(), "dispatch", key),
    receiptPath = path.join(directory, "receipt.json");
  const fingerprint = hash(
    JSON.stringify({
      request,
      owner: v.owner,
      reuse: v["reuse-participant"],
      host,
      direction: v.direction,
    }),
  );
  const client = new Comms();
  try {
    if (task.metadata.workflow) {
      const workflow = task.metadata.workflow as Workflow;
      const step = workflow.steps.find((entry) => entry.id === request.stage);
      if (!step || step.type !== "agent")
        throw new Error(
          `Stage ${request.stage} is not an agent step in this task's workflow`,
        );
      const expected = assignmentRequest(step, request.attempt);
      if (
        Object.entries(expected).some(
          ([field, value]) => !isDeepStrictEqual(request[field], value),
        ) ||
        !isDeepStrictEqual(request.handoff, expected.handoff || {})
      )
        throw new Error(
          `Dispatch request differs from workflow step ${step.id}; generate it with workflow.ts request`,
        );
      const events = await client.pages("list_events", "events", {
        task_id: task.metadata.comms_id,
      });
      const current = position(
        workflow,
        events as HistoryEvent[],
        task.directory,
      );
      if (
        !fs.existsSync(receiptPath) &&
        (current.state !== "ready" ||
          current.step?.id !== step.id ||
          current.attempt !== request.attempt)
      )
        throw new Error(
          `Workflow is ${current.state} at ${current.step?.id || "end"}; cannot dispatch ${step.id}`,
        );
    }
    const record = await remoteTask(client, task.metadata);
    if (["complete", "cancelled"].includes(record.status))
      throw new Error("Task is terminal");
    const participants = () =>
      client.pages("list_participants", "participants", { task_id: record.id });
    const known = await participants();
    if (
      !known.some(
        (p) =>
          p.id === v.owner &&
          p.routing_identity === record.owner &&
          !p.retired_at,
      )
    )
      throw new Error("--owner must be the active task owner participant");
    await locked(
      path.join(runtimeRoot(), "dispatch", key + ".lock"),
      async () => {
        const receipt: RecordValue = fs.existsSync(receiptPath)
          ? json(receiptPath)
          : {
              fingerprint,
              phase: "prepared",
              instance_id: id(),
              task_id: record.id,
              host,
              receipt: receiptPath,
            };
        if (receipt.fingerprint !== fingerprint)
          throw new Error("Retry key already used with different arguments");
        const save = (phase: string) => {
          receipt.phase = phase;
          writeJSON(receiptPath, receipt);
        };
        const startupFailure = async () => {
          const events = await client.pages("list_events", "events", {
            task_id: record.id,
          });
          const failure = events.findLast(
            (entry) =>
              ["session_startup_failed", "session_runner_failed"].includes(
                entry.data?.event_type,
              ) && entry.data?.instance_id === receipt.instance_id,
          );
          return failure?.message as string | undefined;
        };
        try {
          let participant: RecordValue | undefined;
          if (v["reuse-participant"]) {
            participant = known.find(
              (p) =>
                p.id === v["reuse-participant"] &&
                !p.retired_at &&
                p.role === request.role,
            );
            if (!participant)
              throw new Error(
                "Reuse requires active participant with requested role",
              );
            const snapshot = await client.call("get_session", {
              task_id: record.id,
              participant_id: participant.id,
            });
            if (
              snapshot.manifest?.delivery !== "native" ||
              snapshot.contact_state !== "idle"
            )
              throw new Error(
                "Reuse requires fresh idle native runner contact",
              );
          } else {
            if (receipt.phase === "prepared") {
              save("window_pending");
              receipt.pane_id = createPane(host, record.project, v.direction!);
              save("window_created");
            }
            if (receipt.phase === "window_pending") {
              if (host !== "herdr" || !v["recover-pane"])
                throw new Error(
                  "Window creation uncertain; inspect before retry. For Herdr supply --recover-pane for the actual empty pane.",
                );
              receipt.pane_id = v["recover-pane"];
              save("window_created");
            }
            if (receipt.phase === "window_created") {
              save("launch_pending");
              launchWindow(
                host,
                [
                  process.execPath,
                  path.join(ROOT, "orchestrator/session.ts"),
                  "--factory",
                  task.metadata.factory_config || task.metadata.factory,
                  request.role,
                  task.directory,
                  "--instance-id",
                  receipt.instance_id,
                  "--routing-identity",
                  "factory-" + key.slice(0, 16),
                ],
                record.project,
                directory,
                receipt.pane_id,
              );
              save("launch_submitted");
            }
            if (
              host === "herdr" &&
              ["launch_pending", "launch_submitted"].includes(receipt.phase)
            ) {
              const marker = path.join(directory, "launch.started");
              const deadline = Date.now() + timeout * 1000;
              while (!fs.existsSync(marker)) {
                if (Date.now() > deadline)
                  throw new Error(
                    "Herdr accepted the command, but the pane did not start it. Inspect the pane and receipt before retrying; dispatch will not send a second command automatically.",
                  );
                await sleep(250);
              }
              save("launch_confirmed");
            }
            const deadline = Date.now() + timeout * 1000;
            let polls = 0;
            do {
              participant = (await participants()).find(
                (p) => p.instance_id === receipt.instance_id && !p.retired_at,
              );
              if (participant) break;
              if (++polls % 4 === 0) {
                const failure = await startupFailure();
                if (failure)
                  throw new Error(`Worker startup failed: ${failure}`);
              }
              if (Date.now() > deadline)
                throw new Error(
                  "No actual participant registered. Inspect the visible session; retry never resends its launch.",
                );
              await sleep(250);
            } while (true);
          }
          receipt.participant_id = participant!.id;
          const nativeDelivery =
            (config.runner || "pi") === "pi" || config.delivery === "native";
          if (nativeDelivery) {
            const deadline = Date.now() + timeout * 1000;
            let polls = 0;
            for (;;) {
              const snapshot = await client
                .call("get_session", {
                  task_id: record.id,
                  participant_id: participant!.id,
                })
                .catch((): RecordValue => ({}));
              if (
                snapshot.manifest?.delivery === "native" &&
                (snapshot.contact_state === "idle" ||
                  (receipt.assignment_id &&
                    snapshot.contact_state === "working"))
              )
                break;
              if (++polls % 2 === 0) {
                const failure = await startupFailure();
                if (failure)
                  throw new Error(`Worker startup failed: ${failure}`);
              }
              if (Date.now() > deadline)
                throw new Error(
                  "Worker registered but its native inbox has no fresh contact; no assignment was created. Inspect the session and retry this dispatch.",
                );
              await sleep(500);
            }
          }
          const { role, ...assignmentRequest } = request;
          const assignment = await client.call("create_assignment", {
            task_id: record.id,
            assigning_participant_id: v.owner,
            assignee_id: participant!.id,
            ...assignmentRequest,
            request_key: "dispatch-" + key,
          });
          receipt.assignment_id = assignment.id;
          save("assigned");
          if (nativeDelivery) {
            save("native_delivery_ready");
          } else {
            receipt.notice =
              "Assignment stored. This runner requires its own delivery adapter or explicit inbox polling.";
            save("manual_delivery");
          }
          if (receipt.attention_id) {
            const inbox = await client.call("get_coordinator_inbox", {
              task_id: record.id,
            });
            const item = inbox.items.find(
              (i: RecordValue) => i.id === receipt.attention_id,
            );
            if (item)
              await client.call("resolve_attention", {
                task_id: record.id,
                attention_id: item.id,
                actor_participant_id: v.owner,
                expected_version: item.version,
                resolution: "recovered",
                explanation:
                  "Same dispatch recovered; native runner contact checked",
                request_key: "dispatch-resolved-" + key,
              });
          }
          delete receipt.error;
          writeJSON(receiptPath, receipt);
          console.log(JSON.stringify(receipt, null, 2));
        } catch (error) {
          receipt.error = String(error);
          try {
            const attention = await client.call("request_attention", {
              task_id: record.id,
              actor_participant_id: v.owner,
              recipient: record.owner,
              kind: "failure",
              title: "Dispatch needs inspection",
              detail:
                "Inspect the saved receipt and terminal session. Retry does not automatically relaunch.",
              references: [receiptPath],
              request_key: "dispatch-attention-" + key,
            });
            receipt.attention_id = attention.id;
          } catch {}
          writeJSON(receiptPath, receipt);
          console.log(JSON.stringify(receipt, null, 2));
          throw error;
        }
      },
    );
  } finally {
    await client.close();
  }
}
if (isMain(import.meta.url)) void cli(() => main());
