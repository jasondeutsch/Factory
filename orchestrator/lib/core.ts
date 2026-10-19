import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import { fileURLToPath, pathToFileURL } from "node:url";
import { createHash, randomUUID } from "node:crypto";
import { spawn, spawnSync } from "node:child_process";
import lockfile from "proper-lockfile";
import { Comms, type RecordValue } from "./client.ts";
export const ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
export const id = () => randomUUID().replaceAll("-", "");
export const hash = (value: string | Buffer) =>
  createHash("sha256").update(value).digest("hex");
export const json = (file: string): RecordValue =>
  JSON.parse(fs.readFileSync(file, "utf8"));
export const object = (value: unknown): value is RecordValue =>
  value !== null && typeof value === "object" && !Array.isArray(value);
export const inside = (root: string, file: string) =>
  !path.relative(root, file).startsWith(".." + path.sep) &&
  path.relative(root, file) !== ".." &&
  !path.isAbsolute(path.relative(root, file));
export function writeJSON(file: string, data: unknown) {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  const tmp = file + "." + id();
  try {
    fs.writeFileSync(tmp, JSON.stringify(data, null, 2) + "\n", {
      mode: 0o600,
    });
    fs.renameSync(tmp, file);
  } finally {
    fs.rmSync(tmp, { force: true });
  }
}
export function merge(base: RecordValue, overlay: RecordValue): RecordValue {
  const result = structuredClone(base);
  for (const [key, value] of Object.entries(overlay))
    result[key] =
      object(result[key]) && object(value)
        ? merge(result[key], value)
        : structuredClone(value);
  return result;
}
export function strings(value: unknown, name: string): string[] {
  if (
    !Array.isArray(value) ||
    value.some((v) => typeof v !== "string" || !v.trim())
  )
    throw new Error(`${name} must list nonempty strings`);
  return value;
}
export function projectRoot() {
  if (path.basename(ROOT) !== ".factory")
    throw new Error("Copy the toolkit into the application .factory directory");
  return fs.realpathSync(path.dirname(ROOT));
}
export function runtimeRoot() {
  const value =
    process.env.FACTORY_RUNTIME_DIR ||
    path.join(os.homedir(), ".factory/runtime");
  if (!path.isAbsolute(value))
    throw new Error("FACTORY_RUNTIME_DIR must be absolute");
  return value;
}
export function factoryDefinition(name: string) {
  if (!/^[a-z][a-z0-9_-]*$/.test(name)) throw new Error("Invalid factory name");
  const base = path.join(ROOT, "factories", name),
    definition = json(path.join(base, "factory.json"));
  if (
    !object(definition) ||
    definition.name !== name ||
    !Array.isArray(definition.agents) ||
    !definition.agents.length
  )
    throw new Error("Invalid factory.json name or agents");
  if (
    definition.terminal_host !== undefined &&
    !["herdr", "terminal", "iterm"].includes(definition.terminal_host)
  )
    throw new Error("Invalid terminal_host");
  for (const field of ["providers", "mcps"])
    for (const entry of Object.values(definition[field] || {}))
      if (!object(entry)) throw new Error(`${field} entries must be objects`);
  for (const field of Object.keys(definition))
    if (
      ![
        "name",
        "owner",
        "defaults",
        "providers",
        "mcps",
        "extensions",
        "agents",
        "terminal_host",
      ].includes(field)
    )
      throw new Error(`Unknown factory field: ${field}`);
  for (const key of ["defaults", "providers", "mcps", "extensions"])
    if (key in definition && !object(definition[key]))
      throw new Error(`${key} must be an object`);
  const names = new Set();
  for (const member of definition.agents) {
    if (
      !object(member) ||
      typeof member.name !== "string" ||
      !/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(member.name) ||
      names.has(member.name)
    )
      throw new Error("Invalid or duplicate agent name");
    names.add(member.name);
    for (const field of Object.keys(member))
      if (
        ![
          "name",
          "path",
          "config",
          "skills",
          "mcps",
          "extensions",
          "handoff",
        ].includes(field)
      )
        throw new Error(`Unknown member field: ${field}`);
    if (typeof member.path !== "string" || path.isAbsolute(member.path))
      throw new Error("Agent path must be relative");
    const directory = fs.realpathSync(path.resolve(base, member.path));
    if (
      !inside(ROOT, directory) ||
      !fs.existsSync(
        path.join(
          directory,
          member.name === "coordinator" ? "SKILL.md" : "agent.md",
        ),
      )
    )
      throw new Error("Agent path must reference instructions inside toolkit");
    if ("config" in member && !object(member.config))
      throw new Error("Member config must be an object");
    if ("handoff" in member && !object(member.handoff))
      throw new Error("handoff must be an object");
    for (const field of [
      "require_artifacts",
      "require_checks",
      "require_limitations",
    ])
      if (
        field in (member.handoff || {}) &&
        typeof member.handoff[field] !== "boolean"
      )
        throw new Error(`${field} must be boolean`);
    for (const field of ["artifacts", "checks"])
      if (field in (member.handoff || {}))
        strings(member.handoff[field], field);
    const resolved = merge(definition.defaults || {}, member.config || {});
    for (const field of ["skills", "mcps", "extensions"])
      if (field in member) resolved[field] = member[field];
    if (
      resolved.env !== undefined &&
      (!object(resolved.env) ||
        Object.values(resolved.env).some((v) => typeof v !== "string" || !v))
    )
      throw new Error("env must map environment variable names");
    if (
      resolved.runner !== undefined &&
      !["pi", "command"].includes(resolved.runner)
    )
      throw new Error("Invalid runner");
    if (
      resolved.delivery !== undefined &&
      !["native", "manual"].includes(resolved.delivery)
    )
      throw new Error("delivery must be native or manual");
    if ((resolved.runner || "pi") === "pi" && resolved.delivery === "manual")
      throw new Error("Pi uses native delivery");
    const supported = [
      "runner",
      "delivery",
      "provider",
      "model",
      "thinking",
      "tools",
      "skills",
      "mcps",
      "extensions",
      "pi",
      "args",
      "env",
      "executable",
      "command",
      "auth_file_env",
      "mcp_auth_file_env",
      "check_timeout_seconds",
    ];
    for (const key of Object.keys(resolved))
      if (!supported.includes(key))
        throw new Error(`Unsupported agent config field: ${key}`);
    for (const field of ["provider", "model", "thinking"])
      if (
        field in resolved &&
        (typeof resolved[field] !== "string" || !resolved[field])
      )
        throw new Error(`${field} must be a nonempty string`);
    for (const field of [
      "skills",
      "mcps",
      "extensions",
      "tools",
      "args",
      "command",
    ])
      if (field in resolved) strings(resolved[field], field);
    for (const field of ["mcps", "extensions"])
      for (const ref of resolved[field] || [])
        if (!(ref in (definition[field] || {})))
          throw new Error(`Unknown ${field}: ${ref}`);
    for (const ref of resolved.skills || []) {
      const file = fs.realpathSync(path.resolve(base, ref));
      if (!inside(ROOT, file))
        throw new Error("Skill must stay inside toolkit");
      if (
        !fs.existsSync(
          fs.statSync(file).isDirectory() ? path.join(file, "SKILL.md") : file,
        )
      )
        throw new Error("Skill missing");
    }
    if (
      resolved.pi &&
      (!object(resolved.pi) ||
        Object.keys(resolved.pi).some((k) => k !== "settings") ||
        !object(resolved.pi.settings || {}))
    )
      throw new Error(
        "pi accepts a settings object; use provider/MCP/extension catalogs",
      );
    for (const key of [
      "defaultProvider",
      "defaultModel",
      "defaultThinkingLevel",
      "defaultTools",
      "packages",
    ])
      if (key in (resolved.pi?.settings || {}))
        throw new Error(
          `Use explicit configuration instead of pi.settings.${key}`,
        );
    for (const ref of resolved.mcps || [])
      if (!object(definition.mcps[ref]))
        throw new Error("MCP definition must be an object");
    if (
      member.handoff &&
      (!object(member.handoff) ||
        Object.keys(member.handoff).some(
          (k) =>
            ![
              "require_artifacts",
              "require_checks",
              "require_limitations",
              "artifacts",
              "checks",
            ].includes(k),
        ))
    )
      throw new Error("Invalid handoff requirements");
  }
  if (!names.has(definition.owner))
    throw new Error("Factory owner must name a member");
  return definition;
}
export function configuration(role: string, factory: string) {
  if (!factory) throw new Error("Select --factory or an explicit task");
  const definition = factoryDefinition(factory),
    member = definition.agents.find((a: RecordValue) => a.name === role);
  if (!member) throw new Error(`Agent ${role} is not in ${factory}`);
  const config = merge(definition.defaults || {}, member.config || {});
  for (const key of ["skills", "mcps", "extensions"])
    if (key in member) config[key] = member[key];
  return {
    definition,
    member,
    config,
    directory: path.resolve(ROOT, "factories", factory, member.path),
    base: path.join(ROOT, "factories", factory),
  };
}
export function readTask(value: string) {
  const directory = fs.realpathSync(value);
  if (path.dirname(directory) !== path.join(ROOT, "tasks"))
    throw new Error(
      "Task must be a direct child of this project .factory/tasks",
    );
  const metadata = json(path.join(directory, "task.json"));
  if (
    !object(metadata) ||
    metadata.schema_version !== 2 ||
    metadata.id !== path.basename(directory) ||
    metadata.project !== projectRoot() ||
    typeof metadata.brief !== "string"
  )
    throw new Error("Invalid task.json identity");
  for (const key of ["id", "comms_id", "project", "name", "factory", "owner"])
    if (typeof metadata[key] !== "string" || !metadata[key])
      throw new Error(`task.json missing ${key}`);
  return { directory, metadata };
}
export async function remoteTask(client: Comms, metadata: RecordValue) {
  const task = await client.call("get_task", { task_id: metadata.comms_id });
  if (task.project !== metadata.project || task.owner !== metadata.owner)
    throw new Error("Task service identity mismatch");
  return task;
}
export async function event(
  client: Comms,
  metadata: RecordValue,
  actor: string,
  type: string,
  data: RecordValue = {},
) {
  return client.call("record_event", {
    task_id: metadata.comms_id,
    actor,
    kind: [
      "decision",
      "note",
      "check",
      "finding",
      "failure",
      "progress",
    ].includes(type)
      ? type
      : type.endsWith("failed")
        ? "failure"
        : "note",
    message: data.text || type,
    data: { event_type: type, ...data },
    request_key: id(),
  });
}
export const sleep = (ms: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, ms));
export async function locked<T>(
  file: string,
  action: () => Promise<T>,
): Promise<T> {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  fs.closeSync(fs.openSync(file, "a", 0o600));
  const release = await lockfile.lock(file, {
    retries: { retries: 40, minTimeout: 100, maxTimeout: 500 },
    stale: 10000,
  });
  try {
    return await action();
  } finally {
    await release();
  }
}
export function run(
  argv: string[],
  options: {
    cwd?: string;
    env?: NodeJS.ProcessEnv;
    timeout?: number;
    input?: string;
  } = {},
) {
  const result = spawnSync(argv[0], argv.slice(1), {
    cwd: options.cwd,
    env: options.env || process.env,
    encoding: "utf8",
    timeout: options.timeout || 30000,
    input: options.input,
    maxBuffer: 8 * 1024 * 1024,
  });
  if (result.error || result.status !== 0) {
    const diagnostic = path.join(runtimeRoot(), "operations", id() + ".json");
    writeJSON(diagnostic, {
      executable: path.basename(argv[0]),
      status: result.status,
      error: result.error?.message,
      stdout: result.stdout,
      stderr: result.stderr,
    });
    throw new Error(
      `${path.basename(argv[0])} failed (${result.status ?? result.error?.message}); private diagnostics: ${diagnostic}`,
    );
  }
  return result.stdout;
}
export function interactive(
  argv: string[],
  cwd: string,
  env: NodeJS.ProcessEnv,
) {
  return new Promise<number>((resolve, reject) => {
    const child = spawn(argv[0], argv.slice(1), { cwd, env, stdio: "inherit" });
    const stop = () => child.kill("SIGTERM");
    process.once("SIGTERM", stop);
    child.once("error", reject);
    child.once("exit", (code) => {
      process.removeListener("SIGTERM", stop);
      resolve(code ?? 1);
    });
  });
}
export const isMain = (url: string) =>
  !!process.argv[1] &&
  pathToFileURL(fs.realpathSync(process.argv[1])).href === url;
export async function cli(fn: () => Promise<unknown>) {
  process.umask(0o077);
  try {
    await fn();
  } catch (error) {
    console.error(
      `Factory: ${error instanceof Error ? error.message : String(error)}`,
    );
    process.exitCode = 1;
  }
}
