#!/bin/sh
# Go coder tool: run from the application module root using its lint configuration.
set -eu

command -v go >/dev/null 2>&1 || { echo 'Go is not installed or not on PATH.' >&2; exit 1; }
command -v golangci-lint >/dev/null 2>&1 || { echo 'golangci-lint is not installed or not on PATH.' >&2; exit 1; }

if [ ! -f go.mod ]; then
    echo 'Run from the Go module root, or use the project-specific checks for a multi-module repository.' >&2
    exit 1
fi

python3 - <<'PY'
import pathlib
import subprocess
import sys

unformatted = []
for path in pathlib.Path('.').rglob('*.go'):
    if any(part in ('.git', '.factory', 'vendor') for part in path.parts):
        continue
    result = subprocess.run(['gofmt', '-l', str(path)], check=True, capture_output=True, text=True)
    if result.stdout.strip():
        unformatted.append(str(path))
if unformatted:
    print('Files needing gofmt:', *unformatted, sep='\n', file=sys.stderr)
    sys.exit(1)
PY
go test ./...
golangci-lint run "$@"
