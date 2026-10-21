import { Comms } from "../lib/client.ts";
import { cli, isMain, readTask, remoteTask } from "../lib/core.ts";
import {
  assignmentRequest,
  position,
  type HistoryEvent,
  type Workflow,
} from "../lib/workflow.ts";

export async function taskPosition(client: Comms, taskDirectory: string) {
  const task = readTask(taskDirectory);
  const workflow = task.metadata.workflow as Workflow | undefined;
  if (!workflow) throw new Error("Task has no pinned workflow definition");
  const record = await remoteTask(client, task.metadata);
  const events = await client.pages("list_events", "events", {
    task_id: task.metadata.comms_id,
  });
  return {
    task,
    workflow,
    record,
    current: position(workflow, events as HistoryEvent[], task.directory),
  };
}

export async function main(args = process.argv.slice(2)) {
  if (args.length !== 2 || !["status", "request"].includes(args[0]))
    throw new Error("workflow.ts status|request <task-directory>");
  const client = new Comms();
  try {
    const { current, record } = await taskPosition(client, args[1]);
    if (args[0] === "status")
      console.log(
        JSON.stringify({ ...current, task_status: record.status }, null, 2),
      );
    else {
      if (["complete", "cancelled"].includes(record.status))
        throw new Error(`Task is ${record.status}`);
      if (current.state !== "ready" || current.step?.type !== "agent")
        throw new Error(`No agent step ready to dispatch (${current.state})`);
      console.log(
        JSON.stringify(
          assignmentRequest(current.step, current.attempt!),
          null,
          2,
        ),
      );
    }
  } finally {
    await client.close();
  }
}
if (isMain(import.meta.url)) await cli(() => main());
