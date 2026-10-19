# Team customization and toolkit updates

Keep one `.factory/` directory in the application repository. No upstream subdirectory or toolkit registry is needed. Node dependencies are installed separately with npm.

| Area | Ownership |
| --- | --- |
| `factories/` | Team-owned recipes, member configuration, custom agents and skills. Seeded once on install; never overwritten by toolkit updates. |
| `orchestrator/`, `resources/`, `docs/`, root README/Makefile/gitignore/package files/tsconfig | Toolkit-owned defaults. Updates replace only files whose local version matches the recorded baseline. |
| Auth/trust files, environment files, `*.local.json` | Private files excluded from source copying and managed updates; not configuration layers. |
| `tasks/`, runtime directories and databases | Execution data; never synchronized. |
| `.toolkit.json` | Hash baseline for managed files; retain it with the copied toolkit. No configuration or credentials. |

Customize member model/provider/MCP/tool settings through its `factory.json` entry. Put a team-specific agent/skill under the team's factory and reference its relative path. Shared bundled code remains editable, but simultaneous upstream/local changes require explicit reconciliation.

## Install

From the source Factory checkout, run `npm ci --omit=dev`, then preview and apply:

```sh
node orchestrator/scripts/sync_toolkit.ts /path/to/project/.factory
node orchestrator/scripts/sync_toolkit.ts /path/to/project/.factory --apply
```

The project directory must already exist. A new install copies supplied factories once and publishes a complete staged directory. Source Git metadata, dependency caches, known auth/trust files, `.env*` files, local JSON overrides and task data are excluded. Do not embed credential values in portable configuration. The script does not install dependencies or start services. Run `npm ci --omit=dev` in the installed `.factory/` before launching it, and again after package-lock changes.

## Update

Run the same commands from the updated source checkout. The preview lists copies, removals, preserved local edits and conflicts. Applying recomputes the plan under a local lock:

- Unchanged managed files can receive upstream updates/removals.
- Local-only edits remain when upstream did not change that file.
- Both sides changed: report a conflict and apply nothing.
- Untracked local files and all team factories remain untouched.
- Repeated/interrupted updates can be retried against the same source. Each file replacement is atomic; an I/O failure can leave a partially applied update, so retain the baseline and retry after fixing the error.

Inspect conflicts and reconcile the affected source/destination files deliberately, then preview again. There is no force-overwrite switch. To adopt an upstream example-factory improvement, compare its files with your team-owned factory and make the desired edit separately; synchronization does not silently change your workflow.

For an existing `cp -R` installation without `.toolkit.json`, run the updater against the source revision it was copied from **before** pulling toolkit changes. Matching files establish the baseline. Existing differing files are conflicts rather than assumed safe to overwrite. Keep `.toolkit.json`; deleting it loses the information needed to distinguish edits from upstream changes.

Restart affected services and launch fresh agent sessions to use updated code/configuration. Updating files does not modify a running session's prepared configuration. `make update` still updates external dependencies, not this toolkit copy.
