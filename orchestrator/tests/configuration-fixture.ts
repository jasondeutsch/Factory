// Synthetic configurations exercise the framework without shipping a real factory.
import fs from "node:fs";
import path from "node:path";
import { writeJSON } from "../lib/core.ts";
export function seedConfigurations(toolkit: string) {
  const write = (ref: string, text: string) => {
    const file = path.join(toolkit, ref);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, text);
  };
  for (const name of ["designer", "critic", "builder", "reviewer", "writer"])
    write(
      `agents/${name}/agent.md`,
      `# Test role ${name}\nFixture instructions only.\n`,
    );
  write("agents/designer/skills/fixture-design/SKILL.md", "# Fixture skill\n");
  for (const name of ["sample", "minimal"]) {
    const roles =
      name === "sample"
        ? ["coordinator", "designer", "critic", "builder", "reviewer"]
        : ["writer", "coordinator"];
    const agents = roles.map((role) => ({
      name: role,
      path:
        role === "coordinator" ? "../../orchestrator" : `../../agents/${role}`,
      skills: [
        "../../comms/skills/task-comms",
        ...(role === "designer"
          ? ["../../agents/designer/skills/fixture-design"]
          : []),
      ],
      config: {
        thinking: role === "designer" ? "high" : "medium",
        ...(role === "builder"
          ? {
              loop: {
                prompt: "/fixture-loop {iterations} {task_dir}",
                approval_artifacts: ["spec.html", "plan.html"],
                commands: ["fixture-loop"],
              },
            }
          : {}),
      },
    }));
    write(`factories/${name}/README.md`, "# Synthetic test recipe\n");
    const artifacts =
      name === "sample"
        ? {
            "spec.html": "templates/spec.html",
            "plan.html": "templates/plan.html",
          }
        : {};
    for (const file of Object.keys(artifacts))
      write(
        `factories/${name}/templates/${file}`,
        "<html><h1>$title</h1><p>$brief</p></html>",
      );
    writeJSON(path.join(toolkit, "factories", name, "factory.json"), {
      name,
      owner: name === "sample" ? "coordinator" : "writer",
      defaults: {
        runner: "pi",
        provider: "fixture",
        model: "model",
        mcps: ["comms"],
      },
      providers: {},
      mcps: { comms: { url: "http://127.0.0.1:8090/mcp" } },
      extensions: {},
      artifacts,
      agents,
    });
  }
}
