import path from "node:path";
import {
  cli,
  isMain,
  configuration,
  runtimeRoot,
  id,
  interactive,
} from "../lib/core.ts";
import { prepareProfile, materialize } from "../lib/pi.ts";
export async function main(args = process.argv.slice(2)) {
  if (args.length !== 2)
    throw new Error("install_extensions.ts <factory> <agent>");
  const { config, definition } = configuration(args[1], args[0]);
  if ((config.runner || "pi") !== "pi") throw new Error("Pi member required");
  const env = {
    ...process.env,
    FACTORY_COMMS_URL:
      process.env.FACTORY_COMMS_URL || "http://127.0.0.1:8090/mcp",
  };
  const profile = prepareProfile(
    config,
    definition,
    process.cwd(),
    path.join(runtimeRoot(), "dependencies", args[0], args[1], id()),
    env,
  );
  materialize(profile);
  console.error(`Factory: dependency profile ${profile.target}`);
  process.exitCode = await interactive(
    [config.executable || "pi", "update", "--extensions", "--no-approve"],
    process.cwd(),
    env,
  );
}
if (isMain(import.meta.url)) void cli(() => main());
