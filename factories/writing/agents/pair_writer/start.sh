#!/bin/sh
# Load this role through the shared launcher; caller arguments remain unchanged.
set -eu
role_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
exec node "$role_dir/../../../../orchestrator/session.ts" pair_writer "$@"
