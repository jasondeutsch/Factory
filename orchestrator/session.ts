import fs from "node:fs";
import { position, type HistoryEvent, type Workflow } from "./lib/workflow.ts";
import path from "node:path";
import { parseArgs } from "node:util";
import { Comms, type RecordValue } from "./lib/client.ts";
import {
  ROOT,
  configuration,
  factoryDefinition,
  projectRoot,
  selectFactory,
  runtimeRoot,
  readTask,
  remoteTask,
  hash,
  id,
  writeJSON,
  event,
  interactive,
  isMain,
  cli,
  sleep,
  strings,
  inside,
} from "./lib/core.ts";
import { prepareProfile, materialize, check, type Profile } from "./lib/pi.ts";

export async function approved(
  client: Comms,
  metadata: RecordValue,
  directory: string,
  artifacts: string[],
) {
  const task = await remoteTask(client, metadata);
  if (["complete", "cancelled"].includes(task.status))
    throw new Error("Task is terminal");
  const latest: RecordValue = {};
  for (const record of await client.pages("list_events", "events", {
    task_id: metadata.comms_id,
  })) {
    if (
      record.kind !== "decision" ||
      !["approve", "reject", "request_changes"].includes(record.data?.outcome)
    )
      continue;
    const decision = record.data;
    const scoped = (metadata.workflow as Workflow | undefined)?.steps.some(
      (step) => step.type === "decision" && step.id === record.stage,
    );
    if (
      !scoped &&
      !decision.artifacts?.length &&
      decision.outcome !== "approve"
    )
      for (const name of artifacts)
        latest[name] = { outcome: decision.outcome };
    for (const artifact of decision.artifacts || [])
      latest[artifact.path] = {
        outcome: decision.outcome,
        hash: artifact.sha256,
      };
  }
  for (const name of artifacts)
    if (
      latest[name]?.outcome !== "approve" ||
      latest[name]?.hash !== hash(fs.readFileSync(path.join(directory, name)))
    )
      throw new Error(`Loop requires approval of current ${name}`);
}
export function startupFailureSummary(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  const known = [
    /^Pi (credentials|mcp|extensions) startup check failed/,
    /^Pi command unavailable: [a-zA-Z0-9_-]+/,
    /^Pi credentials not ready/,
    /^MCP [a-zA-Z0-9_-]+ not connected/,
    /^Invalid MCP readiness/,
    /^Missing environment variable: [A-Z][A-Z0-9_]*/,
    /^Environment variable required by [a-zA-Z0-9_-]+: [A-Z][A-Z0-9_]*/,
    /^No assignment arrived before loop startup timeout/,
    /^Loop requires approval of current [a-zA-Z0-9_.-]+/,
    /^Agent process exited with status [0-9]+/,
  ];
  const match = known.map((pattern) => message.match(pattern)?.[0]).find(Boolean);
  return match || "Worker startup failed; inspect its private diagnostics";
}
export function prepare(
  role: string,
  factory: string,
  task?: ReturnType<typeof readTask>,
  instance = id(),
) {
  const project = projectRoot(),
    resolved = configuration(role, factory),
    { config, directory, base, definition, source, entrypoint } = resolved;
  const session = path.join(
    runtimeRoot(),
    "sessions",
    task ? path.basename(task.directory) : "unassigned",
    role,
    instance,
  );
  const roleFile = path.join(directory, "agent.md");
  const instruction = fs.existsSync(roleFile)
    ? roleFile
    : path.join(directory, "SKILL.md");
  const skills = (config.skills || []).map((ref: string) => {
    const p = fs.realpathSync(path.resolve(base, ref));
    return fs.statSync(p).isDirectory() ? path.join(p, "SKILL.md") : p;
  });
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    FACTORY_COMMS_URL:
      process.env.FACTORY_COMMS_URL || "http://127.0.0.1:8090/mcp",
    FACTORY_PROJECT: project,
    FACTORY_CONFIG: source,
    FACTORY_TOOLKIT: ROOT,
    FACTORY_HOME: ROOT,
    FACTORY_RUNTIME_DIR: runtimeRoot(),
    FACTORY_NAME: factory,
    FACTORY_ROLE: role,
    FACTORY_INSTANCE_ID: instance,
  };
  for (const name of [
    "FACTORY_TASK_ID",
    "FACTORY_TASK_DIR",
    "FACTORY_PARTICIPANT_ID",
    "FACTORY_ROUTING_IDENTITY",
    "AGENT_INSTRUCTIONS_FILE",
    "AGENT_EXECUTABLE",
  ])
    delete env[name];
  if (task) {
    env.FACTORY_TASK_ID = task.metadata.comms_id;
    env.FACTORY_TASK_DIR = task.directory;
  }
  for (const [name, ref] of Object.entries(config.env || {})) {
    if (typeof ref !== "string" || !process.env[ref])
      throw new Error(
        `Environment variable required by ${role}: ${String(ref)}`,
      );
    if (
      name.startsWith("FACTORY_") ||
      ["PI_CODING_AGENT_DIR", "COMMS_AUTH_TOKEN"].includes(name)
    )
      throw new Error(
        "Session identity and service settings cannot be remapped",
      );
    env[name] = process.env[ref];
  }
  const membership = {
    name: definition.name,
    owner: definition.owner,
    agents: definition.agents.map((m: RecordValue) => ({
      name: m.name,
      path: m.path,
    })),
  };
  let instructions = `# Factory session\n\nRole: ${role}\nProject root: ${project}\nToolkit root: ${ROOT}\nRuntime directory: ${runtimeRoot()}\nTasks directory: ${path.join(runtimeRoot(), "tasks")}\nInstance ID: ${instance}\nSelected factory configuration: ${source || "built-in coordinator"}\n`;
  instructions += task
    ? `Task directory: ${task.directory}\nComms task ID: ${task.metadata.comms_id}\nTask owner: ${task.metadata.owner}\nBrief (JSON string): ${JSON.stringify(task.metadata.brief)}\n`
    : "No task selected. Establish an explicit task from the user request; never select the newest. After creating and attaching a task, use factory_bind_task to connect this session.\n";
  instructions +=
    "Workers wait for an explicit assignment. Startup grants no approval. Comms is authoritative for state. Use factory_read_message to retrieve inbox notifications; call factory_exec_message immediately before acting. Completion is recorded through assignment results.\n";
  instructions += `\n## Factory membership\n${JSON.stringify(membership, null, 2)}\n`;
  const workflow = task?.metadata.workflow || definition.workflow;
  if (workflow)
    instructions += `\n## Workflow definition (authoritative)\n${JSON.stringify(workflow, null, 2)}\nUse workflow.ts status before dispatching or recording a decision. Follow these steps and outcome routes; the recipe below is explanatory context.\n`;
  for (const file of [
    instruction,
    ...(role === definition.owner && definition.recipe
      ? [definition.recipe]
      : []),
    ...skills,
  ])
    instructions += `\n## Source file: ${file}\nSource directory for relative references: ${path.dirname(file)}\n\n${fs.readFileSync(file, "utf8")}\n`;
  const file = path.join(session, "instructions.md"),
    runner = config.runner || "pi";
  let profile: Profile | undefined, command: string[];
  if (runner === "pi") {
    profile = prepareProfile(config, definition, project, session, env);
    const extra = strings(config.args || [], "args"),
      reserved = [
        "--provider",
        "--model",
        "--thinking",
        "--tools",
        "--no-tools",
        "--session",
        "--session-dir",
        "--session-id",
        "--continue",
        "--resume",
        "--fork",
        "--print",
        "--mode",
        "--approve",
        "--api-key",
        "--no-extensions",
        "-e",
        "--extension",
        "--append-system-prompt",
        "--system-prompt",
        "-p",
        "-c",
        "-r",
        "-a",
        "-t",
        "-nt",
      ];
    if (extra.some((v) => reserved.includes(v.split("=")[0])))
      throw new Error(
        "args must not override prepared configuration or session ownership",
      );
    if (entrypoint) {
      env.AGENT_EXECUTABLE = config.executable || "pi";
      env.AGENT_INSTRUCTIONS_FILE = file;
    }
    command = [
      entrypoint || config.executable || "pi",
      ...(entrypoint ? [] : ["--append-system-prompt", file]),
      "--no-skills",
      "--no-approve",
      ...extra,
      "--session",
      path.join(session, "session.jsonl"),
      "--name",
      `${task?.metadata.name || "Factory"} / ${role}`,
      "Read your loaded Factory instructions. Check the explicitly selected task and inbox; stay interactive.",
    ];
  } else if (runner === "command") {
    const raw = strings(config.command || [], "command");
    if (!raw.some((v) => v.includes("{instructions_file}")))
      throw new Error("command must load {instructions_file}");
    const replacements: Record<string, string> = {
      instructions_file: file,
      project_root: project,
      task_dir: task?.directory || "",
      session_dir: session,
    };
    command = raw.map((v) =>
      v.replace(/\{([^}]+)\}/g, (_all, key) => {
        if (!(key in replacements))
          throw new Error("Unknown command placeholder");
        return replacements[key];
      }),
    );
  } else throw new Error("Unsupported runner");
  const manifest: RecordValue = {
    runner,
    delivery: runner === "pi" ? "native" : config.delivery || "manual",
    provider: runner === "pi" ? config.provider || "" : "",
    model: runner === "pi" ? config.model || "" : "",
    thinking: runner === "pi" ? config.thinking || "" : "",
    mcps: profile?.summary.enabled_mcps || [],
    skills: [instruction, ...skills].map((p: string) => ({
      path: p,
      sha256: hash(fs.readFileSync(p)),
    })),
    prepared_at: new Date().toISOString(),
    startup_checked: false,
  };
  manifest.configuration_fingerprint = hash(
    JSON.stringify({
      ...manifest,
      prepared_at: undefined,
      tools: config.tools,
      extensions: config.extensions,
    }),
  );
  env.FACTORY_SESSION_MANIFEST = path.join(session, "manifest.json");
  return {
    role,
    config,
    project,
    session,
    instance,
    skills,
    instructions,
    file,
    command,
    env,
    profile,
    manifest,
    configuration_source: source || "built-in coordinator",
  };
}
export async function main(argv = process.argv.slice(2)) {
  const { values, positionals } = parseArgs({
    args: argv,
    allowPositionals: true,
    options: {
      factory: { type: "string" },
      project: { type: "string" },
      "instance-id": { type: "string" },
      "routing-identity": { type: "string" },
      "replace-participant": { type: "string" },
      loop: { type: "string" },
      "dry-run": { type: "boolean" },
      check: { type: "boolean" },
      validate: { type: "boolean" },
      "check-factory": { type: "boolean" },
      help: { type: "boolean" },
    },
  });
  if (values.help) {
    console.log(
      "session.ts <role> [task-dir] --factory <name> [--dry-run | --check]\nsession.ts --check-factory --factory <name>",
    );
    return;
  }
  if (values.project)
    process.env.FACTORY_PROJECT = fs.realpathSync(values.project);
  selectFactory(values.factory);
  if (
    [values.check, values["dry-run"], values["check-factory"]].filter(Boolean)
      .length > 1
  )
    throw new Error("Choose one inspection mode");
  if (values["check-factory"]) {
    if (positionals.length)
      throw new Error("--check-factory accepts no positionals");
    const members = [];
    for (const member of factoryDefinition(values.factory).agents) {
      try {
        await main([member.name, "--check"]);
        members.push({ name: member.name, ready: true });
      } catch (error) {
        members.push({ name: member.name, ready: false, error: String(error) });
      }
    }
    console.log(
      JSON.stringify(
        { members, ready: members.every((m) => m.ready) },
        null,
        2,
      ),
    );
    if (members.some((m) => !m.ready)) process.exitCode = 1;
    return;
  }
  const [role, taskPath] = positionals;
  if (!role || positionals.length > 2)
    throw new Error("Specify role and optional task directory");
  const selectedName = values.factory ? factoryDefinition().name : undefined;
  const task = taskPath ? readTask(taskPath) : undefined,
    factory = task?.metadata.factory || factoryDefinition().name;
  if (task?.metadata.factory_config)
    selectFactory(task.metadata.factory_config);
  if (!factory) throw new Error("Select --factory or an explicit task");
  if (task && selectedName && selectedName !== factory)
    throw new Error("--factory conflicts with task");
  if (
    !task &&
    (values["instance-id"] ||
      values["routing-identity"] ||
      values["replace-participant"])
  )
    throw new Error("Participant options require a task");
  if (values["instance-id"] && !/^[a-f0-9]{32}$/.test(values["instance-id"]))
    throw new Error("Invalid instance ID");
  const client = new Comms();
  let participant: RecordValue | undefined;
  let runnerStarted = false;
  try {
    if (task && !values["dry-run"]) {
      const remote = await remoteTask(client, task.metadata);
      for (const key of ["name", "brief", "owner"])
        task.metadata[key] = remote[key];
    }
    const plan = prepare(role, factory, task, values["instance-id"]);
    const timeout = plan.config.check_timeout_seconds ?? 30;
    if (typeof timeout !== "number" || timeout <= 0 || timeout > 300)
      throw new Error("Invalid check_timeout_seconds");
    const loop = values.loop ? Number(values.loop) : undefined;
    let loopPolicy = plan.config.loop;
    if (loop !== undefined) {
      if (task?.metadata.workflow) {
        const events = await client.pages("list_events", "events", {
          task_id: task.metadata.comms_id,
        });
        const current = position(
          task.metadata.workflow as Workflow,
          events as HistoryEvent[],
          task.directory,
        );
        if (
          !current.step ||
          current.step.type !== "agent" ||
          current.step.agent !== role ||
          !["ready", "active"].includes(current.state)
        )
          throw new Error(
            `Loop cannot start while workflow is ${current.state} at ${current.step?.id || "end"}`,
          );
        loopPolicy = current.step.loop;
      }
      if (
        !task ||
        !loopPolicy ||
        !plan.profile ||
        !Number.isInteger(loop) ||
        loop < 1
      )
        throw new Error(
          "Loop requires an explicit task, configured loop, Pi runner and positive iterations",
        );
      await approved(
        client,
        task.metadata,
        task.directory,
        loopPolicy.approval_artifacts,
      );
      plan.command[plan.command.length - 1] = loopPolicy.prompt
        .replaceAll("{iterations}", String(loop))
        .replaceAll("{task_dir}", task.directory);
    }
    if (values.validate) return;
    if (values["dry-run"]) {
      console.log(
        JSON.stringify(
          {
            role,
            configuration_source: plan.configuration_source,
            cwd: plan.project,
            command: plan.command,
            skills: plan.skills,
            instructions: plan.instructions,
            runner_configuration: plan.profile?.summary,
            manifest: plan.manifest,
          },
          null,
          2,
        ),
      );
      return;
    }
    fs.mkdirSync(plan.session, { recursive: true, mode: 0o700 });
    fs.writeFileSync(plan.file, plan.instructions, { mode: 0o600 });
    if (plan.profile) {
      materialize(plan.profile);
      const checkEnv = { ...plan.env };
      delete checkEnv.FACTORY_TASK_ID;
      await check(
        plan.profile,
        plan.config.executable || "pi",
        checkEnv,
        plan.project,
        timeout * 1000,
        loop ? loopPolicy.commands : [],
      );
      plan.manifest.startup_checked = true;
    } else if (values.check)
      throw new Error("Command runners have no native readiness check");
    writeJSON(plan.env.FACTORY_SESSION_MANIFEST!, plan.manifest);
    if (values.check) {
      console.log(JSON.stringify({ ready: true, ...plan.profile?.summary }));
      return;
    }
    if (task) {
      participant = await client.call<RecordValue>("register_participant", {
        task_id: task.metadata.comms_id,
        instance_id: plan.instance,
        role,
        routing_identity: values["routing-identity"] || role,
        replaces_participant_id: values["replace-participant"] || "",
        request_key: "launch-" + plan.instance,
      });
      plan.env.FACTORY_PARTICIPANT_ID = participant.id;
      plan.env.FACTORY_ROUTING_IDENTITY = participant.routing_identity;
      fs.appendFileSync(
        plan.file,
        `\nRegistered participant: ${participant.id}\nRouting identity: ${participant.routing_identity}\nReuse this registration.\n`,
      );
      await client.call("record_session", {
        task_id: task.metadata.comms_id,
        participant_id: participant.id,
        manifest: plan.manifest,
        request_key: "snapshot-" + plan.instance,
      });
      if (loop) {
        const deadline = Date.now() + timeout * 1000;
        for (;;) {
          await approved(
            client,
            task.metadata,
            task.directory,
            loopPolicy.approval_artifacts,
          );
          const assignments = await client.pages(
            "list_assignments",
            "assignments",
            { task_id: task.metadata.comms_id, assignee_id: participant.id },
          );
          if (
            assignments.some((a) =>
              ["offered", "accepted", "working"].includes(a.state),
            )
          )
            break;
          if (Date.now() > deadline)
            throw new Error(
              "No assignment arrived before loop startup timeout",
            );
          await sleep(250);
        }
      }
    }
    await client.close();
    runnerStarted = true;
    const exitCode = await interactive(plan.command, plan.project, plan.env);
    if (exitCode !== 0)
      throw new Error(`Agent process exited with status ${exitCode}`);
  } catch (error) {
    if (task) {
      const summary = startupFailureSummary(error);
      const reporter = new Comms();
      await event(reporter, task.metadata, role, runnerStarted ? "session_runner_failed" : "session_startup_failed", {
        participant_id: participant?.id,
        instance_id: values["instance-id"],
        text: summary,
        error: summary,
      }).catch(() => {});
      await reporter.close().catch(() => {});
    }
    throw error;
  } finally {
    await client.close().catch(() => {});
  }
}
if (isMain(import.meta.url)) void cli(() => main());
