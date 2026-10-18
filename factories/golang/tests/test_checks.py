"""Go factory check-script behavior with isolated tool fixtures."""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]


class GoChecksTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.project = Path(temporary.name).resolve()
        self.env = os.environ.copy()

    def test_go_check_script_runs_existing_tools_and_stops_on_failure(self):
        binaries = self.project / "bin"
        binaries.mkdir()
        capture = self.project / "checks.jsonl"
        stub = (
            f"#!{sys.executable}\nimport json, os, pathlib, sys\n"
            "name = pathlib.Path(sys.argv[0]).name\n"
            "with open(os.environ['CAPTURE'], 'a') as stream:\n"
            "    stream.write(json.dumps([name, *sys.argv[1:]]) + '\\n')\n"
            "if name == 'go' and os.environ.get('FAIL_GO') == '1':\n"
            "    sys.exit(2)\n"
        )
        for name in ("go", "gofmt", "golangci-lint"):
            executable = binaries / name
            executable.write_text(stub)
            executable.chmod(0o755)
        (self.project / "go.mod").write_text("module example.invalid/test\n")
        (self.project / "file with spaces.go").write_text("package example\n")
        env = {**self.env, "PATH": str(binaries) + os.pathsep + os.environ.get("PATH", ""), "CAPTURE": str(capture)}
        script = ROOT / "factories" / "golang" / "agents" / "coder_golang" / "scripts" / "check-go.sh"
        result = subprocess.run(["sh", str(script), "--config", "team.yml"], cwd=self.project, env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(line) for line in capture.read_text().splitlines()]
        self.assertEqual(calls, [["gofmt", "-l", "file with spaces.go"], ["go", "test", "./..."], ["golangci-lint", "run", "--config", "team.yml"]])
        capture.unlink()
        result = subprocess.run(["sh", str(script)], cwd=self.project, env={**env, "FAIL_GO": "1"}, capture_output=True, text=True, timeout=15)
        self.assertNotEqual(result.returncode, 0)
        calls = [json.loads(line) for line in capture.read_text().splitlines()]
        self.assertFalse(any(call[0] == "golangci-lint" for call in calls))

