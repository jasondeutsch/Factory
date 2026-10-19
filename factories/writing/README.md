# Interactive writing factory

Member names, bundle paths, and the task owner are defined in [factory.json](factory.json). Use `pair_writer` directly for technical documentation, proposals, executive communication, and business writing. Start a coordinator only when delegation or multiple participants would help.

An existing agent can take this role by resolving `pair_writer` from [factory.json](factory.json), then reading that bundle's `agent.md` and configured skills. It creates or resumes the writing task using the [operations reference](../../orchestrator/references/operations.md); the user does not need to start a separate session or run lifecycle commands. If a dedicated pane is requested, the coordinator prepares it. Reading role instructions does not automatically apply runner-specific model or tool settings.

Establish the audience, purpose, desired reader action, format, and constraints through conversation. Work with existing material when supplied. For a short edit, edit directly; for a substantial document, discuss its argument or outline before producing a full draft.

Keep the working document in the task directory unless the user names another output path. Use Markdown, self-contained HTML, or an available document tool according to the deliverable. Do not introduce `spec.html` or `plan.html` for writing tasks. Record meaningful decisions, open questions, and source references alongside the draft. Ask before changing a document's central position or adding claims that require evidence.

The writer stays interactive. It can ask an available architecture reviewer to examine technical arguments or the coordinator to arrange another review. Neither a second agent nor a review ceremony is mandatory. Sending a message does not grant authority to contact people or publish externally.

When the user is satisfied, report the output path and any remaining fact checks. The writer can mark a task complete when acting as its recorded owner; otherwise it sends the coordinator a completion proposal. User feedback can reopen the task.

## Dependencies

From the application root:

```bash
make -C .factory/factories/writing install
make -C .factory/factories/writing update
```

Installs the shared coordinator tools, including Pi. No additional writing-specific packages are required. Run `update` only when intending to upgrade host tools. In the toolkit checkout, omit `.factory/`. Docker with Compose, credentials, and model configuration remain separate setup. See [dependency setup](../../docs/troubleshooting.md#dependency-commands).
