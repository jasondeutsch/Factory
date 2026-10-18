# Task records and recovery

[Comms](../comms/README.md) is the authoritative store for task metadata, status, inbox messages, acknowledgements, decisions, events, and usage. Agents share its [task-comms skill](../comms/skills/task-comms/SKILL.md). The coordinator owns the Go task's status; `pair_writer` owns a writing task. Documents and application code remain local.

## Storage boundaries

| Location | Contents |
| --- | --- |
| Comms container `/data/` | Embedded SQLite database and its WAL/SHM files. No host-home database. |
| `.factory/tasks/<UTC-millisecond-timestamp>-<slug>/` | Local task pointer, HTML documents, progress, and reviews. |
| Application source directories | Requested implementation, with one source-editing owner at a time. |
| `$HOME/.factory/runtime/<project>/<task>/<role>/<session>/` | Private assembled instructions, writable Pi configuration, startup diagnostics, and transcripts. |
| `<project>/.ralph/loop.md` | Upstream plugin's single workspace loop state and replayed prompt. |

The default Compose deployment has **no volume**. A restart retains the same container's database; removal/recreation loses it. Explicitly mount `/data` when persistence is needed. Local documents and transcripts do not reconstruct a lost database. See [storage options](../comms/README.md#container-local-data-by-default).

`FACTORY_RUNTIME_DIR` selects another absolute private session root. It does not change database storage. Nothing automatically removes local documents or private sessions. Archive them deliberately after retaining needed evidence and stopping sessions.

## Local task artifacts

```text
.factory/tasks/<UTC-millisecond-timestamp>-<slug>/
  task.json       # Schema 2: comms task ID, project, recipe, and initial display context
  reviews/
  spec.html       # Go draft, refined by the architect
  plan.html       # Go draft, refined by the architect
  progress.md     # Created when implementation is assigned
```

There are no local status, mailbox, acknowledgement, or event files for new tasks. `task.json` links the artifact directory to `comms_id`; its `id` identifies the local directory. Initial name/brief fields support offline configuration previews. Actual launch reads current metadata from comms. The project identifier is the resolved application root containing `.factory/`, not a path resolved inside the server container.

Task names may contain spaces and uppercase letters; only the directory slug is normalized. Millisecond timestamps order local directories, with a suffix on collision. Service IDs are distinct and include randomness. Each session receives both identities explicitly. There is no shared current-task pointer or implicit newest-task selection.

Go tasks scaffold draft `spec.html` and `plan.html`, not finished requirements. The spec defines behavior and acceptance criteria; the plan defines approach, stable item IDs, dependencies, and verification. Execution belongs in `progress.md` and comms events, not edits to approved plans. Writing tasks choose their format and have no mandatory spec/plan.

## Messages, decisions, and status

Saving a message, notifying a live pane, acknowledging reading, and completing work are separate actions. Comms does not wake idle sessions; Herdr controls can notify them after a successful store. Preserve message IDs after notification failures and avoid duplicate assignments.

Owner status uses `pending`, `working`, `needs_input`, `blocked`, `complete`, or `cancelled`, with a version check against concurrent changes. These labels do not control processes. Cancellation requires the coordinator to stop active sessions through native controls as well.

Decisions are comms events with actual deciding identity, rationale, outcome, and artifact-relative paths/SHA-256 hashes. See the [decision convention](../comms/skills/task-comms/SKILL.md). The loop launcher reads all event pages and requires the latest decision for both current documents to approve their exact hashes. An unscoped rejection/change request blocks both until resolved. A message saying “approved” is not that event. The helper can hash and publish an actual conversational decision.

The launch check is not continuous enforcement or authenticated human authorization. Agents reconcile approvals and feedback on each iteration. Role names, event evidence, and delegated authority are declared by clients. Shared-token access is not per-role identity or an enterprise approval boundary.

## Resume

Ask the coordinator to resume the named task. It resolves the reference through project-scoped `list_tasks`, finds the matching local pointer, and reads status, inboxes, all relevant event pages, progress, and reviews. When documents have not yet been scaffolded, the artifact helper can attach the existing service task. It must not recreate a remote task simply because a pane is new.

Reuse an existing role session where practical. `session.py` prepares a fresh session, not native transcript resume. Use the runner's resume controls with its associated private profile when restoring a transcript. A fresh session can reconstruct context from comms and local artifacts, but does not inherit another pane's conversation or authentication.

For Ralph, inspect `.ralph/loop.md` and confirm its task and assignment. Reconcile partial work, approval hashes, and the last iteration outcome before resuming. A limit requires reassessment, not endless restart. See [Ralph recovery](../orchestrator/references/ralph.md#state-and-resume).

Old schema-1 file-only tasks are not silently imported. Keep any needed historical evidence; the current launcher requires a service-linked task. Moving a checkout changes its path-based project identifier and private runtime grouping. There is no automatic cross-machine remapping.

## Retention and limits

The supplied ignore rules exclude local task artifacts, overlays, and auth filenames. In a Git project, setup also excludes root `.ralph/` from version control. Teams can choose to retain suitable evidence explicitly. Do not include credentials in documents, messages, or events; automatic redaction is not provided.

The report presents submitted evidence and measured usage; it does not prove all agent activity was captured. Missing measurements remain unknown. SQLite transactions protect service records, but this prototype provides no tamper-evident audit store, per-role authorization, automated scheduling, or workload sandbox. Private runtime file permissions do not encrypt contents.
