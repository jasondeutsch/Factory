"""Opt-in real Pi/Ralph transitions against a local deterministic model fixture.

FACTORY_TEST_PI selects Pi; FACTORY_TEST_RALPH_DIR points to the plugin checkout.
No paid provider, application code, or live Linear workspace is used.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


from comms_fixture import CommsService


@unittest.skipUnless(os.environ.get("FACTORY_TEST_PI") and os.environ.get("FACTORY_TEST_RALPH_DIR"),
                     "set FACTORY_TEST_PI and FACTORY_TEST_RALPH_DIR for native Ralph verification")
class NativeRalphTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.comms = CommsService().__enter__()
        cls.addClassCleanup(cls.comms.__exit__)

    def test_fresh_iterations_keep_role_skills_mcp_and_private_sessions(self):
        requests = []
        task_identity = {}

        class ModelFixture(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                requests.append(json.loads(self.rfile.read(int(self.headers["Content-Length"]))))
                number = len(requests)
                if number in (1, 3):
                    delta = {"role": "assistant", "tool_calls": [{"index": 0, "id": f"call-{number}", "type": "function",
                             "function": {"name": "codemode", "arguments": json.dumps({"code": "text(await tools.mcp__fixture__inspect_fixture({})); " +
                                 "text(await tools.mcp__comms__record_event(" + json.dumps({"task_id": task_identity["id"], "actor": "coder_golang", "kind": "progress", "stage": "build", "attempt": 1, "message": f"Fixture iteration {number}", "request_key": f"fixture-iteration-{number}"}) + "));"})}}]}
                    reason = "tool_calls"
                else:
                    tag = "NEXT" if number == 2 else "COMPLETE"
                    delta = {"role": "assistant", "content": f"Fixture iteration.\n<promise>{tag}</promise>"}
                    reason = "stop"
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                for choice in (
                    {"delta": delta, "finish_reason": None},
                    {"delta": {}, "finish_reason": reason},
                ):
                    chunk = {"id": "local-fixture", "object": "chat.completion.chunk", "created": 1,
                             "model": "fixture-model", "choices": [{"index": 0, **choice}]}
                    self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()

        server = ThreadingHTTPServer(("127.0.0.1", 0), ModelFixture)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary).resolve() / "Project With Spaces"
            project.mkdir()
            toolbox = project / ".factory"
            shutil.copytree(Path(__file__).resolve().parents[1], toolbox, ignore=shutil.ignore_patterns("__pycache__"))
            mcp = project / "fixture_mcp.py"
            starts = project / "mcp-starts.txt"
            calls = project / "mcp-calls.txt"
            mcp.write_text(
                "import json, os, pathlib, sys\n"
                f"with open({str(starts)!r}, 'a') as log: log.write('start\\n')\n"
                "for line in sys.stdin:\n"
                "    message = json.loads(line)\n"
                "    if 'id' not in message: continue\n"
                "    if message.get('method') == 'initialize':\n"
                "        result = {'protocolVersion': message['params']['protocolVersion'], 'capabilities': {'tools': {}}, 'serverInfo': {'name': 'fixture', 'version': '1'}}\n"
                "    elif message.get('method') == 'tools/list':\n"
                "        result = {'tools': [{'name': 'inspect_fixture', 'description': 'Inspect a local fixture', 'inputSchema': {'type': 'object', 'properties': {}}}]}\n"
                "    elif message.get('method') == 'tools/call':\n"
                f"        with open({str(calls)!r}, 'a') as log: log.write(os.getcwd() + '\\n')\n"
                "        result = {'content': [{'type': 'text', 'text': 'Local fixture tool executed.'}]}\n"
                "    else: result = {}\n"
                "    print(json.dumps({'jsonrpc': '2.0', 'id': message['id'], 'result': result}), flush=True)\n"
            )
            native = toolbox / "agents/coder_golang/pi"
            settings = json.loads((native / "settings.json").read_text())
            settings.update(defaultProvider="factory-fixture", defaultModel="fixture-model", defaultThinkingLevel="off",
                            retry={"enabled": False}, packages=[{"source": str(Path(os.environ["FACTORY_TEST_RALPH_DIR"]).resolve()), "skills": []}])
            (native / "settings.json").write_text(json.dumps(settings))
            (native / "models.json").write_text(json.dumps({"providers": {"factory-fixture": {
                "api": "openai-completions", "baseUrl": f"http://127.0.0.1:{server.server_port}/v1",
                "apiKey": "${FACTORY_FIXTURE_KEY}", "models": [{"id": "fixture-model", "reasoning": False}],
            }}}))
            (native / "mcp.json").write_text(json.dumps({"mcpServers": {"comms": {"url": self.comms.url + "/mcp"}, "fixture": {
                "command": sys.executable, "args": [str(mcp)], "exposure": "codemode",
            }}}))
            pi = os.environ["FACTORY_TEST_PI"]
            (toolbox / "agents/coder_golang/config.local.json").write_text(json.dumps({"executable": pi}))
            env = {**os.environ, **self.comms.environment, "FACTORY_RUNTIME_DIR": str(project.parent / "runtime"),
                   "FACTORY_FIXTURE_KEY": "local-fixture-only", "PI_OFFLINE": "1",
                   "RALPH_NEXT_ITERATION_DELAY_SECONDS": "0"}

            def run(script, *args):
                result = subprocess.run([sys.executable, str(toolbox / script), *map(str, args)], cwd=project,
                                        env=env, text=True, capture_output=True, timeout=50)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                return result.stdout.strip()

            task = Path(run("scripts/task.py", "new", "--factory", "golang", "--name", "Ralph fixture", "--brief", "Local transition fixture only."))
            if not task.is_absolute():
                task = project / task
            task_identity["id"] = json.loads((task / "task.json").read_text())["comms_id"]
            run("scripts/task.py", "decide", task, "--by", "human", "--outcome", "approve", "--artifact", "spec.html",
                "--artifact", "plan.html", "--text", "Fixture approval, not real implementation.")
            run("scripts/task.py", "send", task, "--from", "coordinator", "--to", "coder_golang", "--kind", "assignment",
                "--text", "Exercise the local transition fixture only.")
            checked = json.loads(run("orchestrator/session.py", "coder_golang", task, "--loop", "2", "--check"))
            self.assertIn("ralph-loop", checked["commands"])
            self.assertEqual(requests, [])  # Preflight did not call a model.
            planned = json.loads(run("orchestrator/session.py", "coder_golang", task, "--loop", "2", "--dry-run"))
            session = Path(checked["report"]).parent
            env["PI_CODING_AGENT_DIR"] = str(session / "pi")
            command = [pi, "--mode", "rpc", "--no-skills", "--no-approve", "--append-system-prompt", str(session / "instructions.md"),
                       "--session", str(session / "session.jsonl")]
            with open(session / "fixture-rpc.log", "w+") as output:
                process = subprocess.Popen(command, cwd=project, env=env, stdin=subprocess.PIPE, stdout=output, stderr=output, text=True)
                try:
                    process.stdin.write(json.dumps({"type": "prompt", "message": planned["command"][-1]}) + "\n")
                    process.stdin.flush()
                    state_file = project / ".ralph/loop.md"
                    deadline = time.monotonic() + 35
                    state = ""
                    while time.monotonic() < deadline and process.poll() is None:
                        if state_file.exists():
                            state = state_file.read_text()
                            if 'stop_reason: "complete"' in state and "running: false" in state:
                                break
                        time.sleep(0.05)
                    output.flush()
                    diagnostics = (session / "fixture-rpc.log").read_text()
                    self.assertIn('stop_reason: "complete"', state, diagnostics)
                    self.assertEqual(len(requests), 4, diagnostics)
                    for request in requests:
                        self.assertEqual(request["model"], "fixture-model")
                        content = json.dumps(request["messages"])
                        self.assertIn("# Go coder", content)
                        self.assertIn("conventional-commits", content)
                        self.assertIn("bounded-iteration", content)
                        self.assertIn(str(task), content)
                        self.assertIn("codemode", json.dumps(request.get("tools", [])))
                    for request in (requests[0], requests[2]):
                        self.assertFalse(any(message["role"] == "assistant" for message in request["messages"]))
                    transcripts = list(session.glob("*.jsonl"))
                    self.assertGreaterEqual(len(transcripts), 2)
                    for transcript in transcripts:
                        header = json.loads(transcript.read_text().splitlines()[0])
                        self.assertEqual(header["cwd"], str(project))
                    self.assertEqual(calls.read_text().splitlines(), [str(project), str(project)])
                    sys.path.insert(0, str(toolbox / "comms"))
                    from client import Client
                    events = list(Client(env).events(task_identity["id"]))
                    progress = [entry for entry in events if entry["kind"] == "progress"]
                    self.assertEqual([entry["message"] for entry in progress], ["Fixture iteration 1", "Fixture iteration 3"])
                    self.assertFalse((project / ".git").exists())
                    self.assertEqual(json.loads((native / "settings.json").read_text()), settings)
                finally:
                    process.terminate()
                    try:
                        process.communicate(timeout=5)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.communicate(timeout=5)


if __name__ == "__main__":
    unittest.main()
