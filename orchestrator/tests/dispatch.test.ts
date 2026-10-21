import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fixture } from "./fixture.ts";
import { json, writeJSON } from "../lib/core.ts";
let f: Awaited<ReturnType<typeof fixture>>;
before(async () => {
  f = await fixture();
});
after(async () => {
  await f?.close();
});
test("dispatch retries reconcile the actual instance without duplicate launches or assignments", async () => {
  const directory = (
    await f.cli("orchestrator/scripts/task.ts", [
      "new",
      "--factory",
      "sample",
      "--name",
      "Dispatch",
      "--brief",
      "Fixture",
    ])
  ).stdout.trim();
  const metadata = json(path.join(directory, "task.json"));
  const failedInstance = "f".repeat(32);
  await assert.rejects(
    f.cli("orchestrator/session.ts", [
      "missing",
      directory,
      "--factory",
      "sample",
      "--instance-id",
      failedInstance,
    ]),
  );
  const startupEvent = (
    await f.client.pages("list_events", "events", {
      task_id: metadata.comms_id,
    })
  ).find((item) => item.data?.instance_id === failedInstance);
  assert.equal(startupEvent?.kind, "failure");
  assert.equal(startupEvent?.data?.event_type, "session_startup_failed");
  assert.match(startupEvent?.message || "", /Worker startup failed/);
  const owner = await f.client.call("register_participant", {
    task_id: metadata.comms_id,
    role: "coordinator",
    instance_id: "owner",
    routing_identity: "coordinator",
    request_key: "owner",
  });
  const bin = path.join(f.root, "bin");
  fs.mkdirSync(bin);
  const log = path.join(f.root, "terminal.log");
  fs.writeFileSync(
    path.join(bin, "herdr"),
    `#!${process.execPath}\nimport fs from 'node:fs';
import {Comms} from ${JSON.stringify(path.join(f.toolkit, "orchestrator/lib/client.ts"))};
const args=process.argv.slice(2);fs.appendFileSync(${JSON.stringify(log)},JSON.stringify(args)+'\\n');
if(args[1]==='split')console.log(JSON.stringify({result:{pane:{pane_id:'fixture-pane'}}}));
if(args[1]==='run'){
 const script=fs.readFileSync(args[3].match(/'([^']+)'/)[1],'utf8');
 const marker=script.match(/> '([^']+launch.started)'/)[1];
 if(!process.env.SKIP_LAUNCH_MARKER)fs.writeFileSync(marker,'started\\n');
 const words=[...script.matchAll(/'([^']*)'/g)].map(m=>m[1]);const instance=words[words.indexOf('--instance-id')+1],identity=words[words.indexOf('--routing-identity')+1];
 const c=new Comms();try{const p=await c.call('register_participant',{task_id:${JSON.stringify(metadata.comms_id)},role:'designer',instance_id:instance,routing_identity:identity,request_key:'launch-'+instance});await c.call('record_session',{task_id:${JSON.stringify(metadata.comms_id)},participant_id:p.id,manifest:{runner:'pi',delivery:'native',skills:[],mcps:[],prepared_at:new Date().toISOString(),configuration_fingerprint:'a'.repeat(64),startup_checked:true},request_key:'snapshot-'+instance});if(!process.env.SKIP_CONTACT)await c.call('record_runner_contact',{task_id:${JSON.stringify(metadata.comms_id)},participant_id:p.id,instance_id:instance,state:'idle'});}finally{await c.close();}
 if(process.env.FAIL_LAUNCH_RESPONSE)process.exitCode=1;
}
`,
    { mode: 0o700 },
  );
  const request = path.join(f.root, "request.json");
  writeJSON(request, {
    role: "designer",
    title: "Draft",
    instructions: "Draft only",
    stage: "design",
    attempt: 1,
    expected_output: ["spec.html"],
  });
  const args = [
    directory,
    "--request",
    request,
    "--key",
    "design-1",
    "--owner",
    owner.id,
    "--timeout",
    "2",
  ];
  const env = {
    HERDR_ENV: "1",
    PATH: bin + path.delimiter + process.env.PATH,
    FACTORY_TERMINAL_HOST: "herdr",
  };
  await assert.rejects(
    f.cli("orchestrator/dispatch.ts", args, {
      ...env,
      FAIL_LAUNCH_RESPONSE: "1",
      SKIP_CONTACT: "1",
      SKIP_LAUNCH_MARKER: "1",
    }),
  );
  const dispatchRoot = path.join(f.env.FACTORY_RUNTIME_DIR, "dispatch");
  const receiptDirectory = fs
    .readdirSync(dispatchRoot)
    .map((name) => path.join(dispatchRoot, name))
    .find((candidate) => fs.existsSync(path.join(candidate, "receipt.json")))!;
  await assert.rejects(
    f.cli("orchestrator/dispatch.ts", args, env),
    /pane did not start/,
  );
  assert.equal(
    (
      await f.client.pages("list_assignments", "assignments", {
        task_id: metadata.comms_id,
      })
    ).length,
    0,
  );
  fs.writeFileSync(path.join(receiptDirectory, "launch.started"), "started\n");
  await assert.rejects(
    f.cli("orchestrator/dispatch.ts", args, env),
    /no fresh contact/,
  );
  assert.equal(
    (
      await f.client.pages("list_assignments", "assignments", {
        task_id: metadata.comms_id,
      })
    ).length,
    0,
  );
  const worker = (
    await f.client.pages("list_participants", "participants", {
      task_id: metadata.comms_id,
    })
  ).find((participant) => participant.role === "designer")!;
  await f.client.call("record_event", {
    task_id: metadata.comms_id,
    actor: "designer",
    kind: "failure",
    message: "Pi credentials startup check failed",
    data: {
      event_type: "session_startup_failed",
      instance_id: worker.instance_id,
    },
  });
  await assert.rejects(
    f.cli("orchestrator/dispatch.ts", args, env),
    /Worker startup failed: Pi credentials startup check failed/,
  );
  await f.client.call("record_runner_contact", {
    task_id: metadata.comms_id,
    participant_id: worker.id,
    instance_id: worker.instance_id,
    state: "idle",
  });
  const recovered = JSON.parse(
    (await f.cli("orchestrator/dispatch.ts", args, env)).stdout,
  );
  assert.equal(recovered.phase, "native_delivery_ready");
  const retried = JSON.parse(
    (await f.cli("orchestrator/dispatch.ts", args, env)).stdout,
  );
  assert.equal(retried.assignment_id, recovered.assignment_id);
  const commands = fs
    .readFileSync(log, "utf8")
    .trim()
    .split("\n")
    .map((l) => JSON.parse(l));
  assert.equal(commands.filter((c) => c[1] === "split").length, 1);
  assert.equal(commands.filter((c) => c[1] === "run").length, 1);
  const assignments = await f.client.pages("list_assignments", "assignments", {
    task_id: metadata.comms_id,
  });
  assert.equal(assignments.length, 1);
  writeJSON(request, { ...json(request), instructions: "Changed" });
  await assert.rejects(
    f.cli("orchestrator/dispatch.ts", args, env),
    /different arguments/,
  );
});
