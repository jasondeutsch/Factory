import { run } from "node:test";
import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import { parseArgs } from "node:util";
import { ROOT, cli, isMain } from "../lib/core.ts";
export async function main() {
  const { values } = parseArgs({ options: { output: { type: "string" } } });
  if (!values.output) throw new Error("--output <directory> is required");
  const files = ["delivery.test.ts", "dispatch.test.ts", "runtime.test.ts"].map(
    (name) => path.join(ROOT, "orchestrator/tests", name),
  );
  const scenarios: unknown[] = [],
    log: string[] = [];
  let failed = false;
  for await (const event of run({ files })) {
    if (event.type === "test:pass" || event.type === "test:fail") {
      const result = event.data;
      scenarios.push({
        test: result.name,
        status: result.skip
          ? "skipped"
          : event.type === "test:pass"
            ? "passed"
            : "failed",
        milliseconds: result.details.duration_ms,
      });
      if (event.type === "test:fail") failed = true;
    }
    if (event.type === "test:stderr" || event.type === "test:stdout")
      log.push(event.data.message);
  }
  const report = {
    generated_at: new Date().toISOString(),
    environment: `${os.platform()} ${os.arch()}`,
    node: process.version,
    successful: !failed,
    scenarios,
    mode: "Isolated comms and fixture terminals; native Pi only if explicitly enabled; no external model inference",
    live_terminals: "not tested",
  };
  const escape = (value: string) =>
    value
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;");
  fs.mkdirSync(values.output, { recursive: true });
  fs.writeFileSync(
    path.join(values.output, "report.json"),
    JSON.stringify(report, null, 2) + "\n",
  );
  fs.writeFileSync(path.join(values.output, "tests.txt"), log.join(""));
  fs.writeFileSync(
    path.join(values.output, "report.html"),
    `<!doctype html><html lang="en"><meta charset="utf-8"><title>Factory recovery checks</title><style>body{font:16px system-ui;max-width:1000px;margin:3rem auto;padding:1rem}pre{white-space:pre-wrap}</style><h1>Factory recovery checks</h1><p>${failed ? "Failed" : "Passed"} · Live terminal acceptance remains separate.</p><pre>${escape(JSON.stringify(report, null, 2))}</pre></html>`,
  );
  console.log(JSON.stringify(report, null, 2));
  if (failed) process.exitCode = 1;
}
if (isMain(import.meta.url)) void cli(main);
