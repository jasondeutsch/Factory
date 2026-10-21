import { once } from "node:events";
import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fixture } from "./fixture.ts";
import { json, writeJSON, hash, merge } from "../lib/core.ts";
import { prepareProfile } from "../lib/pi.ts";
import { terminalHost, shellQuote } from "../lib/terminal.ts";
import { measurements } from "../adapters/pi/import_usage.ts";
import { synchronize } from "../scripts/sync_toolkit.ts";
let f: Awaited<ReturnType<typeof fixture>>;
before(async () => {
  f = await fixture();
});
after(async () => {
  await f?.close();
});
test("task attachment is idempotent, preserves documents and records exact approval hashes", async () => {
  const created = await f.cli("orchestrator/scripts/task.ts", [
    "new",
    "--factory",
    "sample",
    "--name",
    "Hello <Go>",
    "--brief",
    "Say hello",
    "--request-key",
    "hello",
  ]);
  const directory = created.stdout.trim(),
    metadata = json(path.join(directory, "task.json"));
  assert.match(
    fs.readFileSync(path.join(directory, "spec.html"), "utf8"),
    /Hello &lt;Go&gt;/,
  );
  const spec = path.join(directory, "spec.html");
  fs.writeFileSync(spec, "Approved spec");
  const attached = await Promise.all(
    [1, 2].map(() =>
      f.cli("orchestrator/scripts/task.ts", [
        "attach",
        metadata.comms_id,
        "--factory",
        "sample",
      ]),
    ),
  );
  assert.ok(attached.every((r) => r.stdout.trim() === directory));
  assert.equal(fs.readFileSync(spec, "utf8"), "Approved spec");
  await f.cli("orchestrator/scripts/task.ts", [
    "decide",
    directory,
    "--by",
    "human",
    "--outcome",
    "approve",
    "--text",
    "Approved",
    "--artifact",
    "spec.html",
  ]);
  const events = await f.client.pages("list_events", "events", {
    task_id: metadata.comms_id,
  });
  assert.equal(
    events.find((e) => e.kind === "decision")!.data.artifacts[0].sha256,
    hash("Approved spec"),
  );
  const preview = JSON.parse(
    (
      await f.cli("orchestrator/session.ts", [
        "designer",
        directory,
        "--dry-run",
      ])
    ).stdout,
  );
  assert.equal(preview.cwd, f.project);
  assert.equal(preview.manifest.delivery, "native");
  assert.ok(
    preview.skills.some((s: string) => s.endsWith("fixture-design/SKILL.md")),
  );
  assert.equal(preview.runner_configuration.thinking, "high");
  assert.ok(!fs.existsSync(path.join(f.env.FACTORY_RUNTIME_DIR, "sessions")));
  await assert.rejects(
    f.cli("orchestrator/session.ts", [
      "designer",
      directory,
      "--factory",
      "minimal",
      "--dry-run",
    ]),
    /conflicts/,
  );
});
test("configuration merges member overrides and only materializes selected resources", () => {
  assert.deepEqual(
    merge({ a: { x: 1, y: 2 }, list: ["a"] }, { a: { x: 3 }, list: [] }),
    { a: { x: 3, y: 2 }, list: [] },
  );
  const env = {
    FACTORY_COMMS_URL: "http://127.0.0.1:8090/mcp",
    COMMS_AUTH_TOKEN: "do-not-materialize",
  };
  const profile = prepareProfile(
    { provider: "selected", model: "test", mcps: ["comms"], extensions: [] },
    {
      providers: {
        selected: { apiKey: "${KEY}" },
        other: { apiKey: "unused" },
      },
      mcps: { comms: { url: "ignored" }, other: { command: "missing" } },
      extensions: {},
    },
    f.project,
    path.join(f.root, "profile"),
    env,
  );
  assert.deepEqual(Object.keys(profile.files.models.providers), ["selected"]);
  assert.deepEqual(Object.keys(profile.files.mcp.mcpServers), ["comms"]);
  assert.match(profile.files.settings.extensions[0], /delivery.ts$/);
  assert.ok(!JSON.stringify(profile.files).includes("do-not-materialize"));
});
test("invalid configuration is rejected before launching a process", async () => {
  const file = path.join(f.toolkit, "factories/minimal/factory.json"),
    original = fs.readFileSync(file);
  try {
    const config = json(file);
    config.defaults.args = ["--no-extensions"];
    writeJSON(file, config);
    await assert.rejects(
      f.cli("orchestrator/session.ts", [
        "writer",
        "--factory",
        "minimal",
        "--dry-run",
      ]),
      /must not override/,
    );
  } finally {
    fs.writeFileSync(file, original);
  }
});
test("terminal selection is explicit and quoting preserves shell metacharacters", () => {
  assert.equal(terminalHost(undefined, { HERDR_ENV: "1" }), "herdr");
  assert.throws(() => terminalHost("herdr", {}), /Herdr-managed/);
  assert.throws(() => terminalHost("unknown", {}), /Set FACTORY/);
  assert.equal(shellQuote("a'$(danger)"), "'a'\\''$(danger)'");
  if (process.platform === "darwin")
    assert.equal(
      terminalHost("herdr", { FACTORY_TERMINAL_HOST: "iterm" }),
      "iterm",
    );
});
test("usage measurements reject partial or inconsistent records and account for cached tokens", () => {
  const header = { type: "session", version: 3, id: "s", cwd: f.project };
  const row = {
    type: "message",
    id: "m",
    message: {
      role: "assistant",
      provider: "fixture",
      model: "m",
      stopReason: "stop",
      usage: {
        input: 10,
        output: 3,
        cacheRead: 5,
        cacheWrite: 2,
        totalTokens: 20,
      },
    },
  };
  const text = JSON.stringify(header) + "\n" + JSON.stringify(row) + "\n";
  const parsed = measurements(text);
  assert.equal(parsed.values[0].usage!.input_tokens, 17);
  assert.equal(parsed.values[0].usage!.output_tokens, 3);
  assert.throws(() => measurements(text.trimEnd()), /partial/);
  row.message.usage.totalTokens = 30;
  assert.throws(
    () =>
      measurements(JSON.stringify(header) + "\n" + JSON.stringify(row) + "\n"),
    /disagree/,
  );
});
test("toolkit updates preserve factories and refuse conflicting local edits", async () => {
  const source = path.join(f.root, "source"),
    target = path.join(f.root, "installation/.factory");
  fs.mkdirSync(path.join(source, "orchestrator"), { recursive: true });
  fs.mkdirSync(path.join(source, "factories"), { recursive: true });
  fs.mkdirSync(path.dirname(target));
  fs.writeFileSync(path.join(source, "orchestrator/session.ts"), "first");
  fs.writeFileSync(path.join(source, "factories/custom.json"), "custom");
  await synchronize(source, target, true);
  assert.ok(!fs.existsSync(path.join(target, "factories")));
  fs.mkdirSync(path.join(target, "factories"));
  fs.writeFileSync(path.join(target, "factories/custom.json"), "user");
  for (const directory of ["agents", "skills", "resources"]) {
    fs.mkdirSync(path.join(target, directory));
    fs.writeFileSync(path.join(target, directory, "custom.md"), "user");
  }
  const baseline = json(path.join(target, ".toolkit.json"));
  baseline.files["resources/custom.md"] = hash("user");
  writeJSON(path.join(target, ".toolkit.json"), baseline);
  fs.writeFileSync(path.join(source, "orchestrator/session.ts"), "second");
  await synchronize(source, target, true);
  assert.equal(
    fs.readFileSync(path.join(target, "orchestrator/session.ts"), "utf8"),
    "second",
  );
  assert.equal(
    fs.readFileSync(path.join(target, "factories/custom.json"), "utf8"),
    "user",
  );
  for (const directory of ["agents", "skills", "resources"])
    assert.equal(
      fs.readFileSync(path.join(target, directory, "custom.md"), "utf8"),
      "user",
    );
  fs.writeFileSync(path.join(target, "orchestrator/session.ts"), "local");
  fs.writeFileSync(path.join(source, "orchestrator/session.ts"), "third");
  const report = await synchronize(source, target, true);
  assert.deepEqual(report.conflicts, ["orchestrator/session.ts"]);
  assert.equal(
    fs.readFileSync(path.join(target, "orchestrator/session.ts"), "utf8"),
    "local",
  );
});
test(
  "installed Pi loads the delivery extension in RPC without model inference",
  { skip: !process.env.FACTORY_TEST_PI },
  async () => {
    const file = path.join(f.toolkit, "factories/minimal/factory.json"),
      original = fs.readFileSync(file);
    try {
      const config = json(file);
      Object.assign(config.defaults, {
        provider: "fixture",
        model: "fixture-model",
        executable: process.env.FACTORY_TEST_PI,
        check_timeout_seconds: 15,
      });
      config.providers = {
        fixture: {
          api: "openai-completions",
          baseUrl: "http://127.0.0.1:1/v1",
          apiKey: "${FACTORY_FIXTURE_KEY}",
          models: [{ id: "fixture-model" }],
        },
      };
      writeJSON(file, config);
      const check = JSON.parse(
        (
          await f.cli(
            "orchestrator/session.ts",
            ["writer", "--factory", "minimal", "--check"],
            { FACTORY_FIXTURE_KEY: "fixture-not-a-secret" },
          )
        ).stdout,
      );
      assert.equal(check.ready, true);
      assert.ok(check.extensions.includes("factory-delivery"));
    } finally {
      fs.writeFileSync(file, original);
    }
  },
);

test("Configured loop entry guard requires current spec and plan approvals", async () => {
  const directory = (
    await f.cli("orchestrator/scripts/task.ts", [
      "new",
      "--factory",
      "sample",
      "--name",
      "Approval guard",
      "--brief",
      "Guard only",
    ])
  ).stdout.trim();
  const preview = ["builder", directory, "--loop", "2", "--dry-run"];
  await assert.rejects(f.cli("orchestrator/session.ts", preview), /approval/);
  await f.cli("orchestrator/scripts/task.ts", [
    "decide",
    directory,
    "--by",
    "human",
    "--outcome",
    "approve",
    "--artifact",
    "spec.html",
    "--artifact",
    "plan.html",
    "--text",
    "Approved for fixture",
  ]);
  assert.ok(
    JSON.parse((await f.cli("orchestrator/session.ts", preview)).stdout)
      .command.at(-1)
      .includes("/fixture-loop"),
  );
  fs.appendFileSync(path.join(directory, "plan.html"), "Changed");
  await assert.rejects(f.cli("orchestrator/session.ts", preview), /approval/);
  await assert.rejects(
    f.cli("orchestrator/session.ts", [
      "designer",
      directory,
      "--loop",
      "2",
      "--dry-run",
    ]),
    /Loop requires/,
  );
});

test("usage importer records a measured request once across retries", async () => {
  const directory = (
    await f.cli("orchestrator/scripts/task.ts", [
      "new",
      "--factory",
      "sample",
      "--name",
      "Usage",
      "--brief",
      "Fixture",
    ])
  ).stdout.trim();
  const { comms_id: task_id } = json(path.join(directory, "task.json"));
  const owner = await f.client.call("register_participant", {
    task_id,
    role: "coordinator",
    instance_id: "usage-owner",
    routing_identity: "coordinator",
    request_key: "owner",
  });
  const worker = await f.client.call("register_participant", {
    task_id,
    role: "coder",
    instance_id: "usage-worker",
    routing_identity: "coder",
    request_key: "worker",
  });
  const assignment = await f.client.call("create_assignment", {
    task_id,
    assigning_participant_id: owner.id,
    assignee_id: worker.id,
    title: "Fixture",
    instructions: "Fixture",
    expected_output: ["result"],
    stage: "build",
    attempt: 1,
    request_key: "assignment",
  });
  const session = path.join(f.root, "usage-worker/session.jsonl");
  fs.mkdirSync(path.dirname(session));
  fs.writeFileSync(
    session,
    [
      { type: "session", version: 3, id: "session", cwd: f.project },
      {
        type: "message",
        id: "m",
        message: {
          role: "assistant",
          provider: "fixture",
          model: "fixture",
          stopReason: "stop",
          usage: {
            input: 10,
            output: 2,
            cacheRead: 3,
            cacheWrite: 0,
            totalTokens: 15,
          },
        },
      },
    ]
      .map((row) => JSON.stringify(row))
      .join("\n") + "\n",
  );
  const args = [
    directory,
    "--session",
    session,
    "--participant",
    worker.id,
    "--assignment",
    assignment.id,
  ];
  await f.cli("orchestrator/adapters/pi/import_usage.ts", args);
  await f.cli("orchestrator/adapters/pi/import_usage.ts", args);
  const events = await f.client.pages("list_events", "events", { task_id });
  assert.equal(events.filter((e) => e.kind === "usage").length, 1);
});

test(
  "real Pi binds an explicit task and records native contact without a model call",
  { skip: !process.env.FACTORY_TEST_PI, timeout: 20000 },
  async () => {
    const { spawn } = await import("node:child_process");
    const { materialize } = await import("../lib/pi.ts");
    const { sleep } = await import("../lib/core.ts");
    const task = await f.client.call("create_task", {
      project: f.project,
      name: "Native contact",
      brief: "No model work",
      owner: "coordinator",
      request_key: "native-contact",
    });
    await f.client.call("set_task_factory", {
      task_id: task.id,
      factory: {
        name: "sample",
        owner: "coordinator",
        agents: [{ name: "coordinator", path: "../../orchestrator" }],
      },
    });
    const session = path.join(f.root, "native-contact");
    const env = {
      ...f.env,
      FACTORY_PROJECT: f.project,
      FACTORY_NAME: "sample",
      FACTORY_ROLE: "coordinator",
      FACTORY_TASK_ID: task.id,
      FACTORY_INSTANCE_ID: "native-fixture",
      FACTORY_SESSION_MANIFEST: path.join(session, "manifest.json"),
      PI_OFFLINE: "1",
    };
    const profile = prepareProfile(
      { provider: "fixture", model: "m", mcps: ["comms"] },
      {
        providers: {
          fixture: {
            api: "openai-completions",
            baseUrl: "http://127.0.0.1:1/v1",
            apiKey: "fixture",
            models: [{ id: "m" }],
          },
        },
        mcps: { comms: { url: f.env.FACTORY_COMMS_URL } },
        extensions: {},
      },
      f.project,
      session,
      env,
    );
    materialize(profile);
    writeJSON(env.FACTORY_SESSION_MANIFEST, {
      runner: "pi",
      delivery: "native",
      skills: [],
      mcps: ["comms"],
      configuration_fingerprint: "a".repeat(64),
      prepared_at: new Date().toISOString(),
      startup_checked: true,
    });
    const child = spawn(
      process.env.FACTORY_TEST_PI!,
      [
        "--mode",
        "rpc",
        "--no-skills",
        "--no-approve",
        "--session",
        path.join(session, "session.jsonl"),
      ],
      { cwd: f.project, env, stdio: "pipe" },
    );
    let diagnostics = "";
    child.stdout.on("data", (chunk) => (diagnostics += chunk));
    child.stderr.on("data", (chunk) => (diagnostics += chunk));
    try {
      let active = false;
      for (let i = 0; i < 100; i++) {
        const participants = await f.client.pages(
          "list_participants",
          "participants",
          { task_id: task.id },
        );
        if (participants.length) {
          const snapshot = await f.client
            .call("get_session", {
              task_id: task.id,
              participant_id: participants[0].id,
            })
            .catch(() => ({ contact_state: "pending" }));
          if (snapshot.contact_state === "idle") {
            active = true;
            break;
          }
        }
        await sleep(100);
      }
      assert.ok(active, diagnostics);
      assert.ok(!diagnostics.includes("127.0.0.1:1/v1"));
    } finally {
      if (child.exitCode === null) {
        child.kill();
        await once(child, "exit");
      }
    }
  },
);

test("all declared members load their selected instructions and skills", async () => {
  for (const factory of ["sample", "minimal"])
    for (const member of json(
      path.join(f.toolkit, "factories", factory, "factory.json"),
    ).agents) {
      const preview = JSON.parse(
        (
          await f.cli("orchestrator/session.ts", [
            member.name,
            "--factory",
            factory,
            "--dry-run",
          ])
        ).stdout,
      );
      assert.equal(preview.role, member.name);
      assert.ok(
        preview.instructions.includes(
          "Source directory for relative references:",
        ),
      );
      assert.ok(preview.instructions.includes("task-comms"));
      assert.ok(
        preview.skills.every((file: string) => file.startsWith(f.toolkit)),
      );
      assert.ok(
        preview.runner_configuration.extensions.includes("factory-delivery"),
      );
    }
});

test("invalid member fields and catalog references fail before task creation", async () => {
  const file = path.join(f.toolkit, "factories/minimal/factory.json"),
    original = fs.readFileSync(file);
  const cases = [
    (c: any) => {
      c.agents[0].unknown = true;
    },
    (c: any) => {
      c.agents[0].mcps = ["missing"];
    },
    (c: any) => {
      c.agents[0].path = "/tmp";
    },
    (c: any) => {
      c.agents.push(c.agents[0]);
    },
    (c: any) => {
      c.terminal_host = "missing";
    },
    (c: any) => {
      c.agents[0].handoff = { require_checks: "yes" };
    },
    (c: any) => {
      c.defaults.env = [];
    },
  ];
  try {
    for (const change of cases) {
      const config = JSON.parse(original.toString());
      change(config);
      writeJSON(file, config);
      await assert.rejects(
        f.cli("orchestrator/scripts/task.ts", [
          "new",
          "--factory",
          "minimal",
          "--name",
          "Invalid",
          "--brief",
          "Must not exist",
        ]),
      );
    }
  } finally {
    fs.writeFileSync(file, original);
  }
  const tasks = await f.client.call("list_tasks", { project: f.project });
  assert.ok(!tasks.tasks.some((task: any) => task.name === "Invalid"));
});

test("macOS hosts open an executable command document without copying secrets", async () => {
  const { launchWindow } = await import("../lib/terminal.ts");
  const { capture } = await import("../lib/pi.ts");
  const bin = path.join(f.root, "open-fixture"),
    record = path.join(f.root, "open-args");
  fs.mkdirSync(bin);
  fs.writeFileSync(
    path.join(bin, "open"),
    `#!${process.execPath}\nrequire('node:fs').writeFileSync(${JSON.stringify(record)},JSON.stringify(process.argv.slice(2)));`,
    { mode: 0o700 },
  );
  const original = process.env.PATH;
  process.env.PATH = bin + path.delimiter + original;
  try {
    for (const host of ["terminal", "iterm"] as const) {
      const receipt = path.join(f.root, host),
        output = path.join(f.root, host + ".txt"),
        argument = "space ' $(never-run)";
      launchWindow(
        host,
        [
          process.execPath,
          "-e",
          'require("node:fs").writeFileSync(process.argv[1],process.argv[2])',
          output,
          argument,
        ],
        f.project,
        receipt,
      );
      const commandFile = path.join(receipt, "agent.command");
      assert.deepEqual(json(record), [
        "-a",
        host === "terminal" ? "Terminal" : "iTerm",
        commandFile,
      ]);
      assert.equal(fs.statSync(commandFile).mode & 0o777, 0o700);
      assert.ok(
        !fs.readFileSync(commandFile, "utf8").includes(f.env.COMMS_AUTH_TOKEN),
      );
      // EOF lets the final read finish without a human. This runs only our fixture argv.
      const outcome = await capture(
        ["sh", commandFile],
        f.project,
        process.env,
        3000,
        "\n",
      );
      assert.equal(outcome.exit_code, 0);
      assert.equal(fs.readFileSync(output, "utf8"), argument);
    }
  } finally {
    process.env.PATH = original;
  }
});

test("artifact scaffolding and loop guards follow configuration rather than factory names", async () => {
  const file = path.join(f.toolkit, "factories/minimal/factory.json"),
    original = fs.readFileSync(file);
  try {
    const config = json(file);
    fs.mkdirSync(path.join(path.dirname(file), "templates"));
    fs.writeFileSync(
      path.join(path.dirname(file), "templates/proposal.html"),
      "<h1>$title</h1><p>$brief</p>",
    );
    config.artifacts = { "proposal.html": "templates/proposal.html" };
    config.agents.find((m: any) => m.name === "writer").config.loop = {
      prompt: "/review-loop {iterations} {task_dir}",
      approval_artifacts: ["proposal.html"],
      commands: ["review-loop"],
    };
    writeJSON(file, config);
    const directory = (
      await f.cli("orchestrator/scripts/task.ts", [
        "new",
        "--factory",
        "minimal",
        "--name",
        "Configured document",
        "--brief",
        "A proposal",
      ])
    ).stdout.trim();
    assert.ok(fs.existsSync(path.join(directory, "proposal.html")));
    assert.ok(!fs.existsSync(path.join(directory, "spec.html")));
    await assert.rejects(
      f.cli("orchestrator/session.ts", [
        "writer",
        directory,
        "--loop",
        "2",
        "--dry-run",
      ]),
      /approval/,
    );
    await f.cli("orchestrator/scripts/task.ts", [
      "decide",
      directory,
      "--by",
      "human",
      "--outcome",
      "approve",
      "--artifact",
      "proposal.html",
      "--text",
      "Fixture approval",
    ]);
    const preview = JSON.parse(
      (
        await f.cli("orchestrator/session.ts", [
          "writer",
          directory,
          "--loop",
          "2",
          "--dry-run",
        ])
      ).stdout,
    );
    assert.ok(preview.command.at(-1).startsWith("/review-loop 2 "));
  } finally {
    fs.writeFileSync(file, original);
  }
});

test("project config composes external resources and routes artifacts outside the repository", async () => {
  const projectFile = path.join(f.project, "factory.json");
  const custom = path.join(f.root, "project-runs");
  const skill = path.join(f.root, "project-skill");
  fs.mkdirSync(skill);
  fs.writeFileSync(path.join(skill, "SKILL.md"), "# Project-specific skill");
  writeJSON(projectFile, {
    extends: "../framework/factories/sample/factory.json",
    runtime_dir: "../project-runs",
    agents: [
      {
        name: "designer",
        config: { model: "project-model" },
        skills: ["../project-skill"],
      },
    ],
  });
  try {
    const env = { FACTORY_RUNTIME_DIR: "" };
    const preview = JSON.parse(
      (await f.cli("orchestrator/session.ts", ["designer", "--dry-run"], env))
        .stdout,
    );
    assert.equal(preview.cwd, f.project);
    assert.equal(preview.runner_configuration.model, "project-model");
    assert.equal(preview.runner_configuration.thinking, "high");
    assert.deepEqual(preview.skills, [path.join(skill, "SKILL.md")]);
    assert.ok(
      preview.runner_configuration.runtime_dir.startsWith(custom + path.sep),
    );
    const directory = (
      await f.cli(
        "orchestrator/scripts/task.ts",
        ["new", "--name", "External", "--brief", "Configuration test"],
        env,
      )
    ).stdout.trim();
    assert.equal(path.dirname(directory), path.join(custom, "tasks"));
    assert.ok(fs.existsSync(path.join(directory, "spec.html")));
    const worker = JSON.parse(
      (
        await f.cli(
          "orchestrator/session.ts",
          ["builder", directory, "--dry-run"],
          env,
        )
      ).stdout,
    );
    assert.equal(worker.cwd, f.project);
    assert.equal(worker.configuration_source, projectFile);
    assert.deepEqual(fs.readdirSync(f.project), ["factory.json"]);
    assert.ok(!fs.existsSync(path.join(custom, "sessions")));
  } finally {
    fs.rmSync(projectFile);
  }
});

test("circular configuration inheritance fails before runtime writes", async () => {
  const file = path.join(f.project, "factory.json");
  writeJSON(file, { extends: "./factory.json" });
  try {
    await assert.rejects(
      f.cli("orchestrator/session.ts", ["coordinator", "--dry-run"]),
      /Circular factory extends/,
    );
  } finally {
    fs.rmSync(file);
  }
});

test("no project config can launch the built-in coordinator preview", async () => {
  const preview = JSON.parse(
    (await f.cli("orchestrator/session.ts", ["coordinator", "--dry-run"]))
      .stdout,
  );
  assert.equal(preview.configuration_source, "built-in coordinator");
  assert.equal(preview.cwd, f.project);
  assert.equal(
    preview.command[0],
    path.join(f.toolkit, "orchestrator/agent/start.sh"),
  );
  assert.equal(preview.runner_configuration.provider, "openrouter");
  assert.equal(
    preview.runner_configuration.model,
    "deepseek/deepseek-v4-flash",
  );
  assert.equal(
    preview.skills.filter((file: string) =>
      file.endsWith("/orchestrator/SKILL.md"),
    ).length,
    1,
  );
  assert.ok(preview.instructions.includes("task-comms"));
  assert.deepEqual(fs.readdirSync(f.project), []);
});

test("default runtime separates projects with identical names and --project selects the workspace", async () => {
  const previews = [];
  for (const parent of ["one", "two"]) {
    const project = path.join(f.root, parent, "same-name");
    fs.mkdirSync(project, { recursive: true });
    const preview = JSON.parse(
      (
        await f.cli(
          "orchestrator/session.ts",
          ["coordinator", "--project", project, "--dry-run"],
          { FACTORY_RUNTIME_DIR: "" },
        )
      ).stdout,
    );
    assert.equal(preview.cwd, project);
    assert.match(
      preview.runner_configuration.runtime_dir,
      /\.factory\/same-name-[a-f0-9]{12}\/runtime\/sessions\//,
    );
    assert.deepEqual(fs.readdirSync(project), []);
    previews.push(preview);
  }
  assert.notEqual(
    previews[0].runner_configuration.runtime_dir.split("/sessions/")[0],
    previews[1].runner_configuration.runtime_dir.split("/sessions/")[0],
  );
});

test("an external command worker runs in the application with its resolved skills and runtime", async () => {
  const bundle = path.join(f.root, "external-agent");
  fs.mkdirSync(bundle);
  fs.writeFileSync(path.join(bundle, "agent.md"), "# External worker");
  const runner = path.join(bundle, "run.cjs");
  fs.writeFileSync(
    runner,
    `const fs = require('node:fs'); console.log(JSON.stringify({ cwd: process.cwd(), instructions: fs.readFileSync(process.argv[2], 'utf8'), project: process.env.FACTORY_PROJECT, config: process.env.FACTORY_CONFIG, runtime: process.env.FACTORY_RUNTIME_DIR }));`,
  );
  const config = path.join(f.project, "factory.json");
  writeJSON(config, {
    name: "external",
    agents: [
      {
        name: "worker",
        path: "../external-agent",
        config: {
          runner: "command",
          command: [process.execPath, runner, "{instructions_file}"],
        },
      },
    ],
  });
  try {
    const output = JSON.parse(
      (await f.cli("orchestrator/session.ts", ["worker"])).stdout,
    );
    assert.equal(output.cwd, f.project);
    assert.equal(output.project, f.project);
    assert.equal(output.config, config);
    assert.equal(output.runtime, f.env.FACTORY_RUNTIME_DIR);
    assert.ok(output.instructions.includes("# External worker"));
    assert.deepEqual(fs.readdirSync(f.project), ["factory.json"]);
  } finally {
    fs.rmSync(config);
  }
});
