import fs from "node:fs";
import path from "node:path";
import { run } from "./core.ts";
export type TerminalHost = "herdr" | "terminal" | "iterm";
export function terminalHost(
  config?: string,
  environment = process.env,
): TerminalHost {
  const selected =
    environment.FACTORY_TERMINAL_HOST ||
    config ||
    (environment.HERDR_ENV === "1"
      ? "herdr"
      : process.platform === "darwin"
        ? "terminal"
        : "");
  if (!["herdr", "terminal", "iterm"].includes(selected))
    throw new Error("Set FACTORY_TERMINAL_HOST to herdr, terminal, or iterm");
  if (selected === "herdr" && environment.HERDR_ENV !== "1")
    throw new Error(
      "Herdr requires a Herdr-managed pane; do not spoof HERDR_ENV",
    );
  if (selected !== "herdr" && process.platform !== "darwin")
    throw new Error("terminal and iterm currently support macOS");
  return selected as TerminalHost;
}
export const shellQuote = (value: string) =>
  "'" + value.replaceAll("'", "'\\''") + "'";
export function createPane(
  host: TerminalHost,
  project: string,
  direction: string,
) {
  if (host !== "herdr") return undefined;
  return JSON.parse(
    run([
      "herdr",
      "pane",
      "split",
      "--current",
      "--direction",
      direction,
      "--cwd",
      project,
      "--no-focus",
    ]),
  ).result.pane.pane_id as string;
}
export function launchWindow(
  host: TerminalHost,
  argv: string[],
  project: string,
  receiptDir: string,
  pane?: string,
) {
  const command = argv.map(shellQuote).join(" ");
  if (host === "herdr") {
    if (!pane) throw new Error("Missing pane");
    run(["herdr", "pane", "run", pane, command]);
    return;
  }
  // A .command document uses macOS's native terminal integration and leaves the
  // window visible for feedback/errors. It contains paths, never resolved keys.
  const file = path.join(receiptDir, "agent.command");
  fs.mkdirSync(receiptDir, { recursive: true, mode: 0o700 });
  fs.writeFileSync(
    file,
    `#!/bin/sh\ncd ${shellQuote(project)} || exit 1\n${command}\nresult=$?\nprintf '\\nAgent exited (%s). Press Enter to close.\\n' "$result"\nread answer\nexit "$result"\n`,
    { mode: 0o700 },
  );
  run(["open", "-a", host === "terminal" ? "Terminal" : "iTerm", file]);
}
