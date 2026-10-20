import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import net from "node:net";
import { spawn, execFile } from "node:child_process";
import { promisify } from "node:util";
import { once } from "node:events";
import { ROOT, sleep } from "../lib/core.ts";
import { seedConfigurations } from "./configuration-fixture.ts";
import { Comms } from "../lib/client.ts";
export const execute = promisify(execFile);
export async function fixture(contentRoot?: string) {
  const root = fs.realpathSync(
    fs.mkdtempSync(path.join(os.tmpdir(), "factory-test-")),
  );
  const project = path.join(root, "project"),
    toolkit = path.join(project, ".factory");
  fs.mkdirSync(project);
  fs.cpSync(ROOT, toolkit, {
    recursive: true,
    filter: (source) =>
      !["node_modules", ".git", "__pycache__"].includes(path.basename(source)),
  });
  if (contentRoot) {
    for (const directory of ["factories", "agents", "skills"])
      fs.cpSync(
        path.join(contentRoot, directory),
        path.join(toolkit, directory),
        { recursive: true },
      );
  } else seedConfigurations(toolkit);
  fs.symlinkSync(
    path.join(ROOT, "node_modules"),
    path.join(toolkit, "node_modules"),
    "dir",
  );
  const binary = process.env.FACTORY_TEST_COMMS || path.join(root, "comms");
  if (!process.env.FACTORY_TEST_COMMS)
    await execute("go", ["build", "-o", binary, "./cmd/comms"], {
      cwd: path.join(ROOT, "orchestrator/comms"),
    });
  const reservation = net.createServer();
  reservation.listen(0, "127.0.0.1");
  await once(reservation, "listening");
  const port = (reservation.address() as net.AddressInfo).port;
  await new Promise<void>((resolve) => reservation.close(() => resolve()));
  const env = {
    ...process.env,
    FACTORY_COMMS_URL: `http://127.0.0.1:${port}/mcp`,
    COMMS_AUTH_TOKEN: "fixture-only",
    FACTORY_RUNTIME_DIR: path.join(root, "runtime"),
    FACTORY_TEST_PROJECT: project,
    FACTORY_TEST_TOOLKIT: toolkit,
  };
  const server = spawn(
    binary,
    ["-addr", `127.0.0.1:${port}`, "-db", path.join(root, "state.db")],
    { env, stdio: "ignore" },
  );
  for (let i = 0; ; i++) {
    try {
      if ((await fetch(`http://127.0.0.1:${port}/healthz`)).ok) break;
    } catch {}
    if (i > 100) throw new Error("Fixture failed to start");
    await sleep(50);
  }
  const client = new Comms(env);
  return {
    root,
    project,
    toolkit,
    env,
    client,
    async cli(file: string, args: string[], overrides: NodeJS.ProcessEnv = {}) {
      return execute(process.execPath, [path.join(toolkit, file), ...args], {
        cwd: project,
        env: { ...env, ...overrides },
        timeout: 20000,
        maxBuffer: 4 * 1024 * 1024,
      });
    },
    async close() {
      await client.close();
      server.kill();
      await once(server, "exit");
      fs.rmSync(root, { recursive: true, force: true });
    },
  };
}
