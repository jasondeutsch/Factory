#!/bin/sh
# Standalone Pi entrypoint. The caller owns the working directory and native configuration.
set -eu
agent_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
if [ -n "${AGENT_INSTRUCTIONS_FILE:-}" ]; then
  # A caller supplied the complete instruction/skill selection.
  test -f "$AGENT_INSTRUCTIONS_FILE" || { echo "Agent instructions file not found: $AGENT_INSTRUCTIONS_FILE" >&2; exit 1; }
  exec "${AGENT_EXECUTABLE:-pi}" --append-system-prompt "$AGENT_INSTRUCTIONS_FILE" "$@"
fi
# Standalone sessions use this bundle's Pi profile and linked skills.
runtime_root=${FACTORY_AGENT_RUNTIME_DIR:-"${HOME}/.factory/agents"}
mkdir -p "$runtime_root"
profile=$(mktemp -d "$runtime_root/${agent_dir##*/}.XXXXXX")
cp "$agent_dir"/pi/*.json "$profile/"
export PI_CODING_AGENT_DIR="$profile"
set -- --append-system-prompt "$agent_dir/agent.md" --no-skills \
  --skill "$agent_dir/.." \
  --skill "$agent_dir/../../comms/skills/task-comms" "$@"
for skill in "$agent_dir"/skills/*; do
  [ -f "$skill/SKILL.md" ] || continue
  skill_dir=$(CDPATH= cd -- "$skill" && pwd -P)
  set -- --skill "$skill_dir" "$@"
done
exec "${AGENT_EXECUTABLE:-pi}" "$@"
