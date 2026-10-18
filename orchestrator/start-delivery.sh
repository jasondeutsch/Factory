#!/bin/sh
# Host process: run inside the Herdr workspace whose agents should be notified.
set -eu
comms_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../comms" && pwd)
exec go -C "$comms_dir" run ./cmd/bridge "$@"
