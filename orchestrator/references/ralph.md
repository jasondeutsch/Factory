# Bounded coder loops

This is an agent implementation reference. Users ask for implementation or give feedback in conversation; the coordinator operates the session controls.

## Assignment and launch

The Go coder's native `pi/settings.json` declares [edxeth/pi-ralph-loop](https://github.com/edxeth/pi-ralph-loop), pinned to commit `108823f8d1e2089fec1f7202ca3a8905d12df574` (package version 1.3.3). Pi manages loading and installing this declared package in the private runtime profile. Git/network access is needed to obtain the plugin; application work does not require a Git repository. An already available local package can replace the source in a native settings overlay for offline use. Do not replace certificate validation to obtain it.

Before starting, the coordinator records the actual approval of current `spec.html` and `plan.html`, registers the coder instance and creates its canonical comms assignment, and confirms no other agent owns source edits. Plan items have stable IDs and verification. Create or resume the task's `progress.md` with the assigned item IDs and any relevant prior work; do not create another spec or plan.

For structured assignments, start the coder's ordinary visible session, let it register its actual instance, and create the assignment for that participant. After confirming the native Ralph commands are available in that session, submit the native loop command **to the same running coder**:

```text
/ralph-loop --max-iterations=5 Follow the loaded bounded-iteration skill for the explicit task and canonical assignment. Read get_task, get_assignment, inbox, approval events, spec.html, plan.html and progress.md each iteration. Accept offered work before implementing. Publish progress through update_assignment and hand off with submit_result. Stop on cancellation, changed approvals, blockers, or missing assignment. Use the skill's control tags.
```

Supply the actual task/assignment IDs and artifact directory as part of that session's assignment context. The orchestrator sends this through supported live session controls, not by asking the human to operate lifecycle scripts. Do not submit a slash command as an ordinary prompt when the extension is unavailable.

`start.sh <task-path> --loop N` remains a fresh-session bootstrap entrypoint with a positive iteration cap; it is not resume for an already registered worker. The launcher checks current document hashes/decisions and asks Pi to load extensions and confirm the Ralph commands before dispatch. `--dry-run` creates no files but reads comms approval events; `--check` checks startup without beginning model work.

**A new launcher invocation creates a new instance ID.** Do not assign work to an earlier preflight/ordinary instance and assume a newly launched loop owns it. The first loop iteration must resolve registration and a canonical assignment for its actual instance; without one it reports the blocker and stops. Prefer starting the loop within the already assigned coder session. A native context reset inside that process retains `FACTORY_INSTANCE_ID`; a replacement process must use explicit participant replacement and orchestrator-directed cancellation/follow-up assignments.

Pi starts in its normal interactive host and receives `/ralph-loop` as its initial native command. The repeated prompt points to this task's durable documents and assignment. The loaded role instructions and skills are supplied through `--append-system-prompt`; Pi reloads them, its native profile, and MCP configuration for replacement sessions. Fresh session transcripts remain under the same private session directory.

The [bounded-iteration skill](../../skills/bounded-iteration/SKILL.md) defines the iteration protocol. One verified item produces `NEXT`; a published implementation handoff produces `COMPLETE`. Blockers and decisions produce `STOP`. The code reviewer independently verifies the result. The coordinator owns overall task completion.

## Visibility and feedback

The pane remains interactive across iterations. Use native `/ralph-status` to inspect the loop. `/ralph-stop` requests a stop **after the current iteration**, rather than killing an active command. Pi's abort control interrupts active model work; inspect partial changes before continuing.

Direct feedback must be recorded in the task mailbox or progress file before a fresh session discards conversation history. Material scope changes go back to the coordinator and require updated approval. `WAIT` is for an actual pending asynchronous result; human decisions should stop the loop and appear as `needs_input` through the coordinator.

The coordinator checks task messages, progress, loop state, and live pane output. There is no new background observer. If the loop ends from an iteration limit, abort, or runtime error before the agent can send a handoff, the coordinator must report that outcome and preserve partial work. It must not infer success from an idle pane or automatically grant another budget.

## State and resume

The unmodified plugin uses `<project>/.ralph/loop.md`. **Only one loop can be active in a project workspace**, even if several task directories exist. Other roles remain ordinary Pi sessions. Its process ownership guard prevents a second session from driving an active loop; do not bypass it by deleting state. Do not change the coder's working directory to the task folder to avoid this constraint.

The root `.ralph/` directory is runtime state and must be excluded from version control in the application repository's root `.gitignore`; the toolbox's own `.gitignore` cannot ignore a sibling directory. During installation, the coordinator adds that single ignore entry if the project uses Git. This does not create commits or require a clean branch.

After an interruption, inspect the saved prompt, task identity, approved document hashes, progress, and partial changes. Resume only in the associated coder session/profile and only if that exact assignment remains valid. The coordinator can send `/ralph-resume` through the available native session controls. If those controls are unavailable, explain that concrete limitation. Do not resume another task's saved prompt in a new task session. `/ralph-restart` discards iteration position; it is not the default response to a failure.

Starting a new loop replaces the single workspace loop record. Before switching tasks, retain any needed prior loop record with that task's evidence. Task documents, progress, and review files remain in `.factory/tasks/<task>/`; status, messages, and decisions live in comms; Pi transcripts and assembled native profiles remain in `$HOME/.factory/runtime/`.

The launch guard reads all comms decision pages for the task ID in the local pointer. Local event files are not an approval source. Every fresh iteration reloads the shared comms skill and the native connection; unavailable service or changed approvals require stopping for the coordinator.

## Limits of the integration

- Plain-prompt completion relies on agent evidence and independent review. Ralph does not enforce our approval, progress, or verification instructions at each promise; the launch approval check is an additional entry guard, not a sandbox.
- The native package may expose its bundled planning skill through resource discovery. Our roles use Factory's HTML documents and bounded-iteration skill; they do not invoke that planner.
- Five iterations is a starting assignment budget, not a token, cost, request, or elapsed-time limit. Provider retries and recovery prompts can add work inside an iteration.
- Interactive installation of a newly copied toolbox requires provider/model selection and any required MCP credentials or installed tools. A local package override must point to the real plugin checkout; it is not copied into the toolbox automatically.
- Reviewers and the pair writer do not start automatic Ralph loops. Review cycles remain explicitly assigned and bounded by the factory recipe.

For an already available checkout, a coder `pi/settings.local.json` overlay can replace the package source:

```json
{
  "packages": [
    {"source": "/absolute/path/to/pi-ralph-loop", "skills": [], "prompts": [], "themes": []}
  ]
}
```

Use the pinned revision for equivalent behavior. This array replaces the entire package list, so retain any other packages the role needs. An absolute checkout path remains usable when the authored profile is copied to a private runtime directory; a relative path to something outside that profile may resolve differently there. Do not install the extension globally and assume isolated role profiles will inherit it.

## Maintainer verification

The normal test suite checks approval changes, role restrictions, missing plugin commands, optional Linear credentials, and launch arguments with temporary files and fake executables. To additionally exercise the real plugin without paid inference, set `FACTORY_TEST_PI` to an installed Pi executable and `FACTORY_TEST_RALPH_DIR` to the checkout of the pinned plugin, then run the same suite. The opt-in fixture serves deterministic responses on localhost, advances through two fresh iterations, calls a local MCP tool in both, and checks role/skill continuity, project working directories, and private transcript locations. It does not contact Linear or validate live Herdr pane controls.
