#!/bin/sh
# Go coder tool: run from the application module root using its lint configuration.
set -eu

command -v go >/dev/null 2>&1 || { echo 'Go is not installed or not on PATH.' >&2; exit 1; }
command -v golangci-lint >/dev/null 2>&1 || { echo 'golangci-lint is not installed or not on PATH.' >&2; exit 1; }

if [ ! -f go.mod ]; then
    echo 'Run from the Go module root, or use the project-specific checks for a multi-module repository.' >&2
    exit 1
fi

node --input-type=module <<'JS'
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const unformatted = [];
function check(directory) {
    for (const entry of fs.readdirSync(directory, {withFileTypes: true})) {
        if (['.git', '.factory', 'vendor'].includes(entry.name)) continue;
        const file = path.join(directory, entry.name);
        if (entry.isDirectory()) check(file);
        else if (entry.isFile() && file.endsWith('.go') && execFileSync('gofmt', ['-l', file], {encoding: 'utf8'}).trim()) unformatted.push(file);
    }
}
check('.');
if (unformatted.length) {
    console.error('Files needing gofmt:\n' + unformatted.join('\n'));
    process.exit(1);
}
JS

go test ./...
golangci-lint run "$@"
