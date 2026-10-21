import fs from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";
import {
  ROOT,
  cli,
  isMain,
  hash,
  json,
  writeJSON,
  locked,
  inside,
  id,
} from "../lib/core.ts";
const roots = new Set([
  "README.md",
  "Makefile",
  ".gitignore",
]);
const managed = new Set(["orchestrator", "comms", "docs"]);
const excluded = new Set([
  ".git",
  "node_modules",
  "__pycache__",
  "git",
  "npm",
  ".ralph",
  "tasks",
  "auth.json",
  "mcp-auth.json",
  "trust.json",
  ".DS_Store",
  ".env",
]);
const safe = (name: string) =>
  !name.split(path.sep).some((p) => excluded.has(p)) &&
  !path.basename(name).startsWith(".env.") &&
  !/\.(local\.json|pyc|log)$/.test(name);
export const allowed = (name: string) =>
  !path.isAbsolute(name) &&
  !name.split(path.sep).includes("..") &&
  safe(name) &&
  (managed.has(name.split(path.sep)[0]) || roots.has(name));
function files(source: string, prefix = ""): Map<string, string> {
  const result = new Map<string, string>();
  for (const entry of fs.readdirSync(path.join(source, prefix), {
    withFileTypes: true,
  })) {
    const ref = path.join(prefix, entry.name);
    if (!safe(ref)) continue;
    const included = allowed(ref);
    if (entry.isSymbolicLink() && included)
      throw new Error(`Source symlink: ${ref}`);
    if (entry.isDirectory()) {
      for (const [key, value] of files(source, ref)) result.set(key, value);
    } else if (entry.isFile() && included)
      result.set(ref, path.join(source, ref));
  }
  return result;
}
function destination(root: string, ref: string) {
  const file = path.join(root, ref);
  let current = file;
  while (current !== root) {
    if (fs.existsSync(current) && fs.lstatSync(current).isSymbolicLink())
      throw new Error("Destination contains symlink");
    const parent = path.dirname(current);
    if (parent === current) throw new Error("Invalid destination");
    current = parent;
  }
  return file;
}
function digest(file: string) {
  if (!fs.existsSync(file)) return null;
  if (!fs.lstatSync(file).isFile())
    throw new Error(`Expected regular file: ${file}`);
  return hash(fs.readFileSync(file));
}
function copy(source: string, target: string) {
  fs.mkdirSync(path.dirname(target), { recursive: true });
  const temp = target + "." + id();
  try {
    fs.copyFileSync(source, temp);
    fs.chmodSync(temp, fs.statSync(source).mode & 0o777);
    fs.renameSync(temp, target);
  } finally {
    fs.rmSync(temp, { force: true });
  }
}
export function plan(source: string, target: string) {
  const manifestPath = destination(target, ".toolkit.json"),
    manifest = fs.existsSync(manifestPath)
      ? json(manifestPath)
      : { version: 1, files: {} };
  if (
    manifest.version !== 1 ||
    !manifest.files ||
    typeof manifest.files !== "object" ||
    Array.isArray(manifest.files)
  )
    throw new Error("Invalid .toolkit.json");
  for (const [name, value] of Object.entries(manifest.files))
    if (
      path.isAbsolute(name) ||
      name.split(path.sep).includes("..") ||
      !safe(name) ||
      typeof value !== "string" ||
      !/^[a-f0-9]{64}$/.test(value)
    )
      throw new Error("Invalid baseline");
  const incoming = files(source),
    updated = Object.fromEntries(
      Object.entries(manifest.files).filter(([name]) => allowed(name)),
    ),
    actions: { path: string; action: string }[] = [],
    conflicts: string[] = [],
    preserved: string[] = [];
  for (const name of new Set([...Object.keys(updated), ...incoming.keys()])) {
    const current = digest(destination(target, name)),
      wanted = incoming.has(name) ? digest(incoming.get(name)!) : null,
      old = manifest.files[name] ?? null;
    if (current === wanted) {
      if (wanted === null) delete updated[name];
      else updated[name] = wanted;
    } else if (current === old) {
      actions.push({ path: name, action: wanted === null ? "remove" : "copy" });
      if (wanted === null) delete updated[name];
      else updated[name] = wanted;
    } else if (wanted === old) preserved.push(name);
    else conflicts.push(name);
  }
  return {
    actions,
    conflicts,
    preserved_local_edits: preserved,
    manifest: { version: 1, files: updated },
  };
}
export async function synchronize(
  source: string,
  target: string,
  apply = false,
) {
  if (
    path.basename(target) !== ".factory" ||
    inside(source, target) ||
    (fs.existsSync(target) && fs.lstatSync(target).isSymbolicLink())
  )
    throw new Error("Target must be a separate project .factory directory");
  if (!fs.existsSync(path.join(source, "orchestrator/session.ts")))
    throw new Error("Source is not a complete toolkit");
  if (!fs.existsSync(target)) {
    const incoming = files(source);
    if (apply) {
      if (!fs.existsSync(path.dirname(target)))
        throw new Error("Project directory must exist");
      const staging = fs.mkdtempSync(
        path.join(path.dirname(target), ".factory-install-"),
      );
      try {
        for (const [name, file] of incoming)
          copy(file, path.join(staging, name));
        writeJSON(path.join(staging, ".toolkit.json"), {
          version: 1,
          files: Object.fromEntries(
            [...incoming]
              .filter(([name]) => allowed(name))
              .map(([name, file]) => [name, digest(file)]),
          ),
        });
        fs.renameSync(staging, target);
      } finally {
        fs.rmSync(staging, { recursive: true, force: true });
      }
    }
    return {
      mode: apply ? "installed" : "preview-install",
      files: incoming.size,
      conflicts: [],
    };
  }
  const execute = async () => {
    const { manifest, ...report } = plan(source, target);
    if (apply && !report.conflicts.length) {
      for (const action of report.actions) {
        const file = destination(target, action.path);
        if (action.action === "remove") fs.unlinkSync(file);
        else copy(path.join(source, action.path), file);
      }
      writeJSON(path.join(target, ".toolkit.json"), manifest);
    }
    return {
      mode: report.conflicts.length ? "blocked" : apply ? "updated" : "preview",
      ...report,
    };
  };
  return apply
    ? locked(path.join(target, ".toolkit.lock"), execute)
    : execute();
}
export async function main(args = process.argv.slice(2)) {
  const { values, positionals } = parseArgs({
    args,
    allowPositionals: true,
    options: { apply: { type: "boolean" } },
  });
  if (positionals.length !== 1)
    throw new Error("sync_toolkit.ts <project/.factory> [--apply]");
  const target = path.resolve(positionals[0]),
    report = await synchronize(ROOT, target, values.apply);
  console.log(JSON.stringify(report, null, 2));
  if (report.conflicts.length) process.exitCode = 1;
}
if (isMain(import.meta.url)) void cli(() => main());
