import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import { ROOT, object, writeJSON, inside } from "./core.ts";
import type { RecordValue } from "./client.ts";
export interface Profile {
  target: string;
  files: RecordValue;
  auth: Record<string, string>;
  summary: RecordValue;
  required: string[];
}
function references(value: unknown): string[] {
  const text = JSON.stringify(value);
  return [
    ...text.matchAll(
      /(?<!\$)\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))/g,
    ),
  ].map((m) => m[1] || m[2]);
}
export function prepareProfile(
  config: RecordValue,
  definition: RecordValue,
  project: string,
  session: string,
  env: NodeJS.ProcessEnv,
): Profile {
  const target = path.join(session, "pi");
  const settings = structuredClone(config.pi?.settings || {});
  for (const [field, native] of Object.entries({
    provider: "defaultProvider",
    model: "defaultModel",
    thinking: "defaultThinkingLevel",
    tools: "defaultTools",
  }))
    if (field in config) settings[native] = config[field];
  settings.cacheWarming ??= "off";
  settings.packages = (config.extensions || []).map(
    (name: string) => definition.extensions[name],
  );
  settings.extensions = [
    path.join(ROOT, "orchestrator/extensions/delivery.ts"),
    ...(settings.extensions || []),
  ];
  const providers =
    config.provider in (definition.providers || {})
      ? { [config.provider]: definition.providers[config.provider] }
      : {};
  const servers: RecordValue = {};
  for (const name of config.mcps || [])
    servers[name] = structuredClone(definition.mcps[name]);
  if (!servers.comms || servers.comms.enabled === false)
    throw new Error("Pi sessions require the comms MCP");
  servers.comms.url = env.FACTORY_COMMS_URL;
  delete servers.comms.command;
  delete servers.comms.args;
  if (env.COMMS_AUTH_TOKEN)
    servers.comms.headers = {
      ...(servers.comms.headers || {}),
      Authorization: "Bearer ${COMMS_AUTH_TOKEN}",
    };
  const enabled: string[] = [],
    required: string[] = [];
  for (const [name, server] of Object.entries(servers)) {
    if (
      !object(server) ||
      (server.headers && !object(server.headers)) ||
      (server.cwd && typeof server.cwd !== "string")
    )
      throw new Error(`Invalid MCP ${name}`);
    if (server.enabled === false) continue;
    enabled.push(name);
    required.push(...references(server));
    if (server.command) server.cwd = path.resolve(project, server.cwd || ".");
  }
  const auth: Record<string, string> = {};
  for (const [field, name] of [
    ["auth_file_env", "auth.json"],
    ["mcp_auth_file_env", "mcp-auth.json"],
  ])
    if (config[field]) {
      const ref = env[config[field]];
      if (!ref || !path.isAbsolute(ref))
        throw new Error(
          `${field} must reference an absolute private auth path`,
        );
      const file = fs.realpathSync(ref);
      if (inside(ROOT, file) || !fs.statSync(file).isFile())
        throw new Error("Authentication file must be outside toolkit");
      auth[name] = file;
    }
  env.PI_CODING_AGENT_DIR = target;
  delete env.PI_CODING_AGENT_SESSION_DIR;
  return {
    target,
    files: { settings, models: { providers }, mcp: { mcpServers: servers } },
    auth,
    required: [...new Set(required)],
    summary: {
      provider: config.provider,
      model: config.model,
      thinking: config.thinking,
      tools: config.tools || [],
      extensions: ["factory-delivery", ...(config.extensions || [])],
      enabled_mcps: enabled,
      runtime_dir: target,
      configured: !!(config.provider && config.model),
      required_environment: [...new Set(required)],
    },
  };
}
export function materialize(profile: Profile) {
  fs.mkdirSync(profile.target, { recursive: true, mode: 0o700 });
  for (const [name, value] of Object.entries(profile.files))
    writeJSON(path.join(profile.target, name + ".json"), value);
  for (const [name, source] of Object.entries(profile.auth))
    fs.symlinkSync(source, path.join(profile.target, name));
}
export function capture(
  argv: string[],
  cwd: string,
  env: NodeJS.ProcessEnv,
  timeout: number,
  input?: string,
): Promise<RecordValue> {
  return new Promise((resolve, reject) => {
    const process = spawn(argv[0], argv.slice(1), {
      cwd,
      env,
      detached: true,
      stdio: "pipe",
    });
    let stdout = "",
      stderr = "",
      timedOut = false;
    const kill = () => {
      try {
        globalThis.process.kill(-process.pid!, "SIGKILL");
      } catch {}
    };
    const timer = setTimeout(() => {
      timedOut = true;
      kill();
    }, timeout);
    process.stdout.on("data", (data) => {
      stdout += data;
      if (stdout.length > 8 * 1024 * 1024) kill();
    });
    process.stderr.on("data", (data) => {
      stderr += data;
      if (stderr.length > 8 * 1024 * 1024) kill();
    });
    process.on("error", (error) => {
      clearTimeout(timer);
      reject(error);
    });
    process.on("close", (code) => {
      clearTimeout(timer);
      resolve({ exit_code: code, stdout, stderr, timed_out: timedOut });
    });
    process.stdin.end(input || "");
  });
}
export async function check(
  profile: Profile,
  executable: string,
  env: NodeJS.ProcessEnv,
  cwd: string,
  timeout: number,
  loop = false,
) {
  if (!profile.summary.provider || !profile.summary.model)
    throw new Error(
      "Set provider and model in the factory defaults or member config",
    );
  for (const key of profile.required)
    if (!env[key]) throw new Error(`Missing environment variable: ${key}`);
  const report: RecordValue = { ...profile.summary, checks: {} },
    file = path.join(profile.target, "../startup-checks.json");
  const commands: Record<string, string[]> = {
    credentials: [
      executable,
      "auth",
      "check",
      "--provider",
      profile.summary.provider,
      "--model",
      profile.summary.model,
      "--json",
      "--no-refresh",
    ],
    mcp: [executable, "mcp", "list", "--json"],
    extensions: [
      executable,
      "--mode",
      "rpc",
      "--no-session",
      "--no-skills",
      "--no-approve",
    ],
  };
  for (const [name, argv] of Object.entries(commands)) {
    const output = await capture(
      argv,
      cwd,
      { ...env, PI_OFFLINE: "1" },
      timeout,
      name === "extensions"
        ? JSON.stringify({ type: "get_commands", id: "factory-startup" }) + "\n"
        : undefined,
    );
    report.checks[name] = output;
    writeJSON(file, report);
    if (output.exit_code !== 0 || output.timed_out)
      throw new Error(
        `Pi ${name} startup check failed; private diagnostics: ${file}`,
      );
    if (name === "extensions") {
      const responses = output.stdout.split("\n").flatMap((line: string) => {
        try {
          return [JSON.parse(line)];
        } catch {
          return [];
        }
      });
      const response = responses.find(
          (r: RecordValue) => r.id === "factory-startup",
        ),
        names = (response?.data?.commands || []).map(
          (v: RecordValue) => v.name,
        );
      for (const command of [
        "factory-inbox",
        ...(loop
          ? ["ralph-loop", "ralph-status", "ralph-stop", "ralph-resume"]
          : []),
      ])
        if (!names.includes(command))
          throw new Error(
            `Pi command unavailable: ${command}; diagnostics: ${file}`,
          );
    } else {
      const data = JSON.parse(output.stdout);
      if (name === "credentials" && data.status !== "ready")
        throw new Error(`Pi credentials not ready: ${file}`);
      if (name === "mcp") {
        if (!Array.isArray(data.servers) || data.errors?.length)
          throw new Error(`Invalid MCP readiness: ${file}`);
        for (const name of profile.summary.enabled_mcps)
          if (
            !data.servers.some(
              (s: RecordValue) => s.name === name && s.state === "connected",
            )
          )
            throw new Error(`MCP ${name} not connected: ${file}`);
        report.mcps = data.servers;
      }
    }
  }
  report.ready = true;
  writeJSON(file, report);
  return { ...profile.summary, ready: true, report: file };
}
