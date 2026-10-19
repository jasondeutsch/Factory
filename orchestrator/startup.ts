import { ROOT, cli, interactive, isMain, run, sleep } from "./lib/core.ts";
import { Comms } from "./lib/client.ts";
import path from "node:path";
export async function main(args = process.argv.slice(2)) {
  const command = [
    process.execPath,
    path.join(ROOT, "orchestrator/session.ts"),
    ...(args.includes("--check-factory") ? [] : ["coordinator"]),
    ...args,
  ];
  if (
    args.some((v) =>
      ["--check", "--check-factory", "--dry-run", "--help"].includes(v),
    )
  ) {
    process.exitCode = await interactive(command, process.cwd(), process.env);
    return;
  }
  run([...command, "--dry-run"]);
  if (!process.env.FACTORY_COMMS_URL) {
    const port = process.env.COMMS_PORT || "8090";
    if (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535)
      throw new Error("Invalid COMMS_PORT");
    process.env.FACTORY_COMMS_URL = `http://127.0.0.1:${port}/mcp`;
    let compose = ["docker", "compose"];
    try {
      run([...compose, "version"]);
    } catch {
      compose = ["docker-compose"];
      run([...compose, "version"]);
    }
    const code = await interactive(
      [
        ...compose,
        "-f",
        path.join(ROOT, "orchestrator/comms/compose.yaml"),
        "up",
        "--build",
        "-d",
      ],
      ROOT,
      process.env,
    );
    if (code) throw new Error("Compose startup failed");
  }
  const deadline = Date.now() + 60000;
  for (;;) {
    const client = new Comms();
    try {
      await client.connect();
      break;
    } catch (error) {
      if (Date.now() > deadline)
        throw new Error(
          "Comms unavailable; inspect endpoint, credentials and Compose logs",
        );
      await sleep(1000);
    } finally {
      await client.close().catch(() => {});
    }
  }
  console.error(
    "Factory: comms connected. Pi handles inbox delivery internally; terminal host controls windows/panes.",
  );
  process.exitCode = await interactive(command, process.cwd(), process.env);
}
if (isMain(import.meta.url)) void cli(() => main());
