"""Opt-in native Pi checks with fixture credentials and a local stdio MCP server."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


from comms_fixture import CommsService


@unittest.skipUnless(os.environ.get("FACTORY_TEST_PI"), "set FACTORY_TEST_PI to an installed Pi executable")
class NativePiTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.comms = CommsService().__enter__()
        cls.addClassCleanup(cls.comms.__exit__)

    def test_native_credentials_and_mcp_without_model_inference(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary).resolve() / "project"
            project.mkdir()
            toolbox = project / ".factory"
            shutil.copytree(Path(__file__).resolve().parents[1], toolbox, ignore=shutil.ignore_patterns("__pycache__"))
            server = project / "fixture_mcp.py"
            server.write_text(
                "import json, sys\n"
                "for line in sys.stdin:\n"
                "    message = json.loads(line)\n"
                "    if 'id' not in message: continue\n"
                "    method = message.get('method')\n"
                "    if method == 'initialize':\n"
                "        result = {'protocolVersion': message['params']['protocolVersion'], 'capabilities': {'tools': {}}, 'serverInfo': {'name': 'factory-fixture', 'version': '1'}}\n"
                "    elif method == 'tools/list':\n"
                "        result = {'tools': [{'name': 'inspect_fixture', 'description': 'Local connection test', 'inputSchema': {'type': 'object', 'properties': {}}}]}\n"
                "    else: result = {}\n"
                "    print(json.dumps({'jsonrpc': '2.0', 'id': message['id'], 'result': result}), flush=True)\n"
            )
            native = toolbox / "agents" / "architect" / "pi"
            settings = json.loads((native / "settings.json").read_text())
            settings.update(defaultProvider="factory-fixture", defaultModel="fixture-model")
            (native / "settings.json").write_text(json.dumps(settings))
            (native / "models.json").write_text(json.dumps({"providers": {"factory-fixture": {
                "api": "openai-completions", "baseUrl": "http://127.0.0.1:1/v1",
                "apiKey": "${FACTORY_FIXTURE_KEY}", "models": [{"id": "fixture-model"}],
            }}}))
            (native / "mcp.json").write_text(json.dumps({"mcpServers": {"comms": {"url": self.comms.url + "/mcp"}, "fixture": {
                "command": sys.executable, "args": [str(server)], "exposure": "codemode",
            }}}))
            (toolbox / "agents" / "architect" / "config.local.json").write_text(json.dumps({"executable": os.environ["FACTORY_TEST_PI"]}))
            env = {**os.environ, **self.comms.environment, "FACTORY_RUNTIME_DIR": str(project / "private-runtime"), "FACTORY_FIXTURE_KEY": "local-fixture-only"}
            created = subprocess.run([sys.executable, str(toolbox / "scripts/task.py"), "new", "--factory", "golang", "--name", "Native startup", "--brief", "Check native configuration only."],
                                     cwd=project, env=env, text=True, capture_output=True, timeout=10)
            self.assertEqual(created.returncode, 0, created.stderr)
            checked = subprocess.run([str(toolbox / "agents/architect/start.sh"), created.stdout.strip(), "--check"],
                                     cwd=project, env=env, text=True, capture_output=True, timeout=50)
            self.assertEqual(checked.returncode, 0, checked.stdout + checked.stderr)
            result = json.loads(checked.stdout)
            self.assertTrue(result["ready"])
            self.assertEqual(next(server for server in result["mcps"] if server["name"] == "fixture")["tools"], ["inspect_fixture"])
            self.assertEqual(result["mcps"][0]["exposure"], "codemode")
            self.assertEqual(json.loads((native / "settings.json").read_text()), settings)
            self.assertFalse(list((project / "private-runtime").rglob("session.jsonl")))


if __name__ == "__main__":
    unittest.main()
