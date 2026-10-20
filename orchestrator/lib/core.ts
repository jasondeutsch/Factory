import fs from "node:fs";
import path from "node:path";
import {
  toolkitRoot,
  projectRoot,
  runtimeRoot,
  tasksRoot,
  loadFactory,
  selectFactory,
} from "./project.ts";
export {
  projectRoot,
  runtimeRoot,
  tasksRoot,
  selectFactory,
} from "./project.ts";
import { pathToFileURL } from "node:url";
import { createHash, randomUUID } from "node:crypto";
import { spawn, spawnSync } from "node:child_process";
import lockfile from "proper-lockfile";
import { Comms, type RecordValue } from "./client.ts";
export const ROOT = toolkitRoot;
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
export function factoryDefinition(name?: string) {
  if (name) selectFactory(name);
  const { definition } = loadFactory();
  const base = projectRoot();
  if (
    !object(definition) ||
    typeof definition.name !== "string" ||
    !/^[a-z][a-z0-9_-]*$/.test(definition.name) ||
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
        "artifacts",
        "runtime_dir",
        "recipe",
      ].includes(field)
    )
      throw new Error(`Unknown factory field: ${field}`);
  for (const key of ["defaults", "providers", "mcps", "extensions"])
    if (key in definition && !object(definition[key]))
      throw new Error(`${key} must be an object`);
  if ("artifacts" in definition && !object(definition.artifacts))
    throw new Error(
      "artifacts must map output filenames to HTML template paths",
    );
  for (const [name, template] of Object.entries(definition.artifacts || {})) {
    if (
      !/^[A-Za-z0-9][A-Za-z0-9_.-]*\.html$/.test(name) ||
      typeof template !== "string"
    )
      throw new Error("artifacts require HTML filenames and template paths");
    const file = fs.realpathSync(path.resolve(base, template));
    if (!fs.statSync(file).isFile())
      throw new Error("Artifact template must be a file");
  }
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
          "entrypoint",
          "config",
          "skills",
          "mcps",
          "extensions",
          "handoff",
        ].includes(field)
      )
        throw new Error(`Unknown member field: ${field}`);
    if (typeof member.path !== "string")
      throw new Error("Agent path is required");
    const directory = fs.realpathSync(path.resolve(base, member.path));
    if (
      !fs.existsSync(
        path.join(
          directory,
          member.name === "coordinator" ? "SKILL.md" : "agent.md",
        ),
      )
    )
      throw new Error("Agent path must reference its instructions");
    if (member.entrypoint !== undefined) {
      if (typeof member.entrypoint !== "string" || !member.entrypoint.trim())
        throw new Error(
          "entrypoint must be a nonempty path relative to the agent bundle",
        );
      const entrypoint = fs.realpathSync(
        path.resolve(directory, member.entrypoint),
      );
      if (!inside(directory, entrypoint) || !fs.statSync(entrypoint).isFile())
        throw new Error("entrypoint must be a file inside the agent bundle");
      fs.accessSync(entrypoint, fs.constants.X_OK);
    }
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
    if (member.entrypoint && (resolved.runner || "pi") !== "pi")
      throw new Error(
        "entrypoint is for Pi runners; command runners use config.command",
      );
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
      "loop",
    ];
    if (resolved.loop !== undefined) {
      const loop = resolved.loop;
      if (
        !object(loop) ||
        Object.keys(loop).some(
          (k) => !["prompt", "approval_artifacts", "commands"].includes(k),
        ) ||
        typeof loop.prompt !== "string" ||
        !loop.prompt.includes("{iterations}")
      )
        throw new Error("loop requires a prompt containing {iterations}");
      strings(loop.approval_artifacts, "loop.approval_artifacts");
      strings(loop.commands, "loop.commands");
      if (
        !loop.approval_artifacts.length ||
        loop.approval_artifacts.some(
          (name: string) => !Object.hasOwn(definition.artifacts || {}, name),
        )
      )
        throw new Error("loop approvals must name configured artifacts");
    }
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
    directory: member.path,
    entrypoint: member.entrypoint
      ? path.resolve(member.path, member.entrypoint)
      : undefined,
    base: projectRoot(),
    source: loadFactory().source,
  };
}
export function readTask(value: string) {
  const directory = fs.realpathSync(value);
  const metadata = json(path.join(directory, "task.json"));
  if (metadata.project !== projectRoot())
    throw new Error("Task belongs to another project");
  if (
    path.dirname(directory) !==
    fs.realpathSync(tasksRoot(metadata.factory_config))
  )
    throw new Error(
      "Task must be a direct child of this project runtime tasks directory",
    );
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
