import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

export const toolkitRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
type Config = Record<string, any>;
export function projectRoot() {
  return fs.realpathSync(process.env.FACTORY_PROJECT || process.cwd());
}
export function resolveReference(base: string, value: string): string {
  if (typeof value !== "string" || !value.trim())
    throw new Error("Path must be a nonempty string");
  if (value === "@factory" || value.startsWith("@factory/")) {
    const resolved = path.resolve(
      toolkitRoot,
      value === "@factory" ? "." : value.slice("@factory/".length),
    );
    if (
      resolved !== toolkitRoot &&
      !resolved.startsWith(toolkitRoot + path.sep)
    )
      throw new Error("Invalid @factory path");
    return resolved;
  }
  return path.resolve(
    base,
    value.startsWith("~/") ? path.join(os.homedir(), value.slice(2)) : value,
  );
}
function mergeConfig(base: Config, overlay: Config): Config {
  const result = structuredClone(base);
  for (const [key, value] of Object.entries(overlay)) {
    if (key === "agents" && Array.isArray(value)) {
      const names = new Set<string>();
      for (const member of value) {
        if (
          !member ||
          typeof member.name !== "string" ||
          names.has(member.name)
        )
          throw new Error("Invalid or duplicate agent name");
        names.add(member.name);
      }
      result.agents = [...(result.agents || [])];
      for (const member of value) {
        const index = result.agents.findIndex(
          (existing: Config) => existing.name === member.name,
        );
        if (index < 0) result.agents.push(member);
        else result.agents[index] = mergeConfig(result.agents[index], member);
      }
    } else if (
      value &&
      typeof value === "object" &&
      !Array.isArray(value) &&
      result[key] &&
      typeof result[key] === "object" &&
      !Array.isArray(result[key])
    ) {
      result[key] = mergeConfig(result[key], value);
    } else result[key] = value;
  }
  return result;
}
function normalize(config: Config, base: string) {
  const resolve = (value: string) => resolveReference(base, value);
  const normalizeAgent = (agent: Config) => {
    if (agent.skills !== undefined) {
      if (!Array.isArray(agent.skills))
        throw new Error("skills must be an array");
      agent.skills = agent.skills.map(resolve);
    }
    if (agent.executable?.startsWith(".") || agent.executable?.startsWith("~/"))
      agent.executable = resolve(agent.executable);
    if (agent.pi?.settings?.extensions)
      agent.pi.settings.extensions = agent.pi.settings.extensions.map(resolve);
  };
  if (config.runtime_dir !== undefined)
    config.runtime_dir = resolve(config.runtime_dir);
  if (config.recipe !== undefined) config.recipe = resolve(config.recipe);
  if (config.defaults) normalizeAgent(config.defaults);
  if (config.agents !== undefined && !Array.isArray(config.agents))
    throw new Error("agents must be an array");
  for (const member of config.agents || []) {
    if (member.path !== undefined) member.path = resolve(member.path);
    normalizeAgent(member);
    if (member.config) normalizeAgent(member.config);
  }
  for (const [name, template] of Object.entries(config.artifacts || {}))
    config.artifacts[name] = resolve(template as string);
  for (const server of Object.values(config.mcps || {}) as Config[]) {
    if (server.cwd !== undefined) server.cwd = resolve(server.cwd);
    if (server.command?.startsWith(".") || server.command?.startsWith("~/"))
      server.command = resolve(server.command);
  }
  for (const extension of Object.values(config.extensions || {}) as Config[]) {
    if (extension.source?.startsWith(".") || extension.source?.startsWith("~/"))
      extension.source = resolve(extension.source);
  }
  return config;
}
function readConfig(file: string, ancestors = new Set<string>()): Config {
  file = fs.realpathSync(file);
  if (ancestors.has(file)) throw new Error(`Circular factory extends: ${file}`);
  const next = new Set(ancestors).add(file);
  const config = JSON.parse(fs.readFileSync(file, "utf8"));
  if (!config || typeof config !== "object" || Array.isArray(config))
    throw new Error("factory.json must be an object");
  const base = path.dirname(file);
  const inherited =
    config.extends === undefined
      ? {}
      : readConfig(configFile(resolveReference(base, config.extends)), next);
  delete config.extends;
  return mergeConfig(inherited, normalize(config, base));
}
function configFile(value: string) {
  return fs.statSync(value).isDirectory()
    ? path.join(value, "factory.json")
    : value;
}
export function selectFactory(reference?: string) {
  const projectFile = path.join(projectRoot(), "factory.json");
  let source =
    process.env.FACTORY_CONFIG ||
    (fs.existsSync(projectFile) ? projectFile : undefined);
  if (reference) {
    const candidate = resolveReference(projectRoot(), reference);
    if (fs.existsSync(candidate)) source = configFile(candidate);
    else if (!source && reference === "coordinator") return undefined;
    else if (!source)
      throw new Error(
        `Factory configuration not found: ${reference}. Use a path to factory.json or create project factory.json.`,
      );
    else if ((readConfig(source).name || "coordinator") !== reference)
      throw new Error(
        `Factory selection conflicts with configuration: ${reference}`,
      );
  }
  if (source) {
    source = fs.realpathSync(source);
    process.env.FACTORY_CONFIG = source;
  }
  return source;
}
export function loadFactory(source = selectFactory()) {
  const nativeFile = path.join(os.homedir(), ".pi/agent/settings.json");
  const native =
    !source && fs.existsSync(nativeFile)
      ? JSON.parse(fs.readFileSync(nativeFile, "utf8"))
      : {};
  const baseline = normalize(
    {
      name: "coordinator",
      owner: "coordinator",
      defaults: {
        runner: "pi",
        mcps: ["comms"],
        ...(native.defaultProvider ? { provider: native.defaultProvider } : {}),
        ...(native.defaultModel ? { model: native.defaultModel } : {}),
      },
      mcps: {
        comms: { url: "http://127.0.0.1:8090/mcp", exposure: "codemode" },
      },
      agents: [
        {
          name: "coordinator",
          path: "@factory/orchestrator",
          skills: ["@factory/orchestrator/comms/skills/task-comms"],
        },
      ],
    },
    toolkitRoot,
  );
  const selected = source ? readConfig(source) : {};
  // A team coordinator replaces the framework fallback role instead of
  // inheriting its skill selection.
  if (
    selected.agents?.some(
      (member: Config) => member.name === "coordinator" && member.path,
    )
  )
    baseline.agents = baseline.agents.filter(
      (member: Config) => member.name !== "coordinator",
    );
  return { source, definition: mergeConfig(baseline, selected) };
}
export function runtimeRoot(source?: string) {
  const project = projectRoot();
  const suffix = createHash("sha256")
    .update(project)
    .digest("hex")
    .slice(0, 12);
  const configured = loadFactory(source).definition.runtime_dir;
  const value =
    process.env.FACTORY_RUNTIME_DIR ||
    configured ||
    path.join(
      os.homedir(),
      ".factory",
      `${path.basename(project)}-${suffix}`,
      "runtime",
    );
  if (!path.isAbsolute(value))
    throw new Error("FACTORY_RUNTIME_DIR must be absolute");
  return value;
}
export const tasksRoot = (source?: string) =>
  path.join(runtimeRoot(source), "tasks");
