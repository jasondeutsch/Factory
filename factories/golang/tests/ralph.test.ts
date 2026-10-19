import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import http from "node:http";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { fixture } from "../../../orchestrator/tests/fixture.ts";
import { json, writeJSON, sleep } from "../../../orchestrator/lib/core.ts";
import type { RecordValue } from "../../../orchestrator/lib/client.ts";

test(
  "native Ralph retains role instructions across fresh iterations",
  {
    skip: !process.env.FACTORY_TEST_PI || !process.env.FACTORY_TEST_RALPH_DIR,
    timeout: 60000,
  },
  async () => {
    const f = await fixture();
    const requests: RecordValue[] = [];
    const model = http.createServer(async (req, res) => {
      let body = "";
      for await (const chunk of req) body += chunk;
      requests.push(JSON.parse(body));
      res.writeHead(200, { "Content-Type": "text/event-stream" });
      const tag = requests.length === 1 ? "NEXT" : "COMPLETE";
      for (const choice of [
        {
          delta: {
            role: "assistant",
            content: `Fixture iteration.\n<promise>${tag}</promise>`,
          },
          finish_reason: null,
        },
        { delta: {}, finish_reason: "stop" },
      ])
        res.write(
          "data: " +
            JSON.stringify({
              id: "fixture",
              object: "chat.completion.chunk",
              created: 1,
              model: "fixture-model",
              choices: [{ index: 0, ...choice }],
            }) +
            "\n\n",
        );
      res.end("data: [DONE]\n\n");
    });
    model.listen(0, "127.0.0.1");
    await once(model, "listening");
    const address = model.address() as { port: number };
    let child: ReturnType<typeof spawn> | undefined;
    try {
      const configPath = path.join(f.toolkit, "factories/golang/factory.json"),
        config = json(configPath);
      Object.assign(config.defaults, {
        provider: "fixture",
        model: "fixture-model",
        thinking: "off",
        executable: process.env.FACTORY_TEST_PI,
        pi: { settings: { retry: { enabled: false } } },
      });
      config.providers = {
        fixture: {
          api: "openai-completions",
          baseUrl: `http://127.0.0.1:${address.port}/v1`,
          apiKey: "${FACTORY_FIXTURE_KEY}",
          models: [{ id: "fixture-model" }],
        },
      };
      config.extensions.ralph = {
        source: process.env.FACTORY_TEST_RALPH_DIR,
        skills: [],
        prompts: [],
        themes: [],
      };
      config.agents.find((m: RecordValue) => m.name === "coder_golang").mcps = [
        "comms",
      ];
      writeJSON(configPath, config);
      const directory = (
        await f.cli("orchestrator/scripts/task.ts", [
          "new",
          "--factory",
          "golang",
          "--name",
          "Ralph fixture",
          "--brief",
          "Deterministic loop test",
        ])
      ).stdout.trim();
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
        "Fixture approval only",
      ]);
      const env = {
        ...f.env,
        FACTORY_FIXTURE_KEY: "fixture-only",
        PI_OFFLINE: "1",
        RALPH_NEXT_ITERATION_DELAY_SECONDS: "0",
      };
      const checked = JSON.parse(
        (
          await f.cli(
            "orchestrator/session.ts",
            ["coder_golang", directory, "--loop", "2", "--check"],
            env,
          )
        ).stdout,
      );
      assert.equal(requests.length, 0);
      const preview = JSON.parse(
        (
          await f.cli(
            "orchestrator/session.ts",
            ["coder_golang", directory, "--loop", "2", "--dry-run"],
            env,
          )
        ).stdout,
      );
      const session = path.dirname(checked.runtime_dir);
      child = spawn(
        process.env.FACTORY_TEST_PI!,
        [
          "--mode",
          "rpc",
          "--no-skills",
          "--no-approve",
          "--append-system-prompt",
          path.join(session, "instructions.md"),
          "--session",
          path.join(session, "session.jsonl"),
        ],
        {
          cwd: f.project,
          env: { ...env, PI_CODING_AGENT_DIR: checked.runtime_dir },
          stdio: "pipe",
        },
      );
      let output = "";
      child.stdout!.on("data", (data) => (output += data));
      child.stderr!.on("data", (data) => (output += data));
      child.stdin!.write(
        JSON.stringify({ type: "prompt", message: preview.command.at(-1) }) +
          "\n",
      );
      const stateFile = path.join(f.project, ".ralph/loop.md");
      let state = "";
      for (let i = 0; i < 600; i++) {
        if (fs.existsSync(stateFile))
          state = fs.readFileSync(stateFile, "utf8");
        if (
          state.includes('stop_reason: "complete"') &&
          state.includes("running: false")
        )
          break;
        await sleep(50);
      }
      assert.match(state, /stop_reason: "complete"/, output);
      assert.equal(requests.length, 2, output);
      for (const request of requests) {
        const content = JSON.stringify(request.messages);
        assert.match(content, /# Go coder/);
        assert.match(content, /conventional-commits/);
        assert.match(content, /bounded-iteration/);
        assert.ok(content.includes(directory));
        assert.ok(
          !request.messages.some(
            (message: RecordValue) => message.role === "assistant",
          ),
        );
      }
      assert.ok(
        fs.readdirSync(session).filter((file) => file.endsWith(".jsonl"))
          .length >= 2,
      );
    } finally {
      if (child && child.exitCode === null) {
        child.kill();
        await once(child, "exit");
      }
      model.closeAllConnections();
      await new Promise<void>((resolve) => model.close(() => resolve()));
      await f.close();
    }
  },
);
