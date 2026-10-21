#!/bin/sh
# Run from the project directory; all Go dependencies are vendored.
set -eu
comms_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../comms" && pwd)
exec go -C "$comms_dir" run -mod=vendor ./cmd/monitor -project "$PWD" "$@"
