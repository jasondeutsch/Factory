import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execute } from "../../../orchestrator/tests/fixture.ts";
import { ROOT } from "../../../orchestrator/lib/core.ts";
test("Go checks run configured tools and stop before lint after test failure", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "factory-go-check-"));
  try {
    const bin = path.join(root, "bin"),
      log = path.join(root, "calls");
    fs.mkdirSync(bin);
    for (const name of ["gofmt", "go", "golangci-lint"])
      fs.writeFileSync(
        path.join(bin, name),
        `#!${process.execPath}\nconst fs=require('node:fs');fs.appendFileSync(process.env.CAPTURE,JSON.stringify([${JSON.stringify(name)},...process.argv.slice(2)])+'\\n');if(${JSON.stringify(name)}==='go'&&process.env.FAIL_GO)process.exit(2);`,
        { mode: 0o700 },
      );
    fs.writeFileSync(path.join(root, "go.mod"), "module fixture\n");
    fs.writeFileSync(
      path.join(root, "file with spaces.go"),
      "package fixture\n",
    );
    const script = path.join(
        ROOT,
        "factories/golang/agents/coder_golang/scripts/check-go.sh",
      ),
      env = {
        ...process.env,
        PATH: bin + path.delimiter + process.env.PATH,
        CAPTURE: log,
      };
    await execute("sh", [script, "--config", "team.yml"], { cwd: root, env });
    assert.deepEqual(
      fs
        .readFileSync(log, "utf8")
        .trim()
        .split("\n")
        .map((line) => JSON.parse(line)),
      [
        ["gofmt", "-l", "file with spaces.go"],
        ["go", "test", "./..."],
        ["golangci-lint", "run", "--config", "team.yml"],
      ],
    );
    fs.rmSync(log);
    await assert.rejects(
      execute("sh", [script], { cwd: root, env: { ...env, FAIL_GO: "1" } }),
    );
    assert.ok(!fs.readFileSync(log, "utf8").includes("golangci-lint"));
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
