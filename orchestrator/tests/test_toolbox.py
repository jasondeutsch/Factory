"""Behavior checks using temporary projects; no live agents or product tests."""

from __future__ import annotations

import concurrent.futures
import hashlib
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import Mock, patch
from pathlib import Path

from comms_fixture import CommsService, bridge_binary

SOURCE = Path(__file__).resolve().parents[2]


class ToolboxTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.comms = CommsService().__enter__()
        cls.addClassCleanup(cls.comms.__exit__)

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.project = Path(self.temporary.name).resolve() / "Project With Spaces"
        self.project.mkdir()
        self.toolbox = self.project / ".factory"
        shutil.copytree(SOURCE, self.toolbox, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
        self.runtime = Path(self.temporary.name).resolve() / "private-runtime"
        self.env = {**os.environ, **self.comms.environment, "FACTORY_RUNTIME_DIR": str(self.runtime)}
        self.env.pop("PYTHONDONTWRITEBYTECODE", None)
        self.env.pop("HERDR_ENV", None)
        for factory in ("golang", "writing"):
            path = self.toolbox / "factories" / factory / "factory.json"
            definition = json.loads(path.read_text())
            definition["defaults"].update(provider="fixture", model="test-model")
            path.write_text(json.dumps(definition))
        self.task = self.create_task()

    def configure(self, role, config=None, factory="golang", **fields):
        path = self.toolbox / "factories" / factory / "factory.json"
        definition = json.loads(path.read_text())
        member = next(item for item in definition["agents"] if item["name"] == role)
        if config is not None:
            member.setdefault("config", {}).update(config)
        member.update(fields)
        path.write_text(json.dumps(definition))

    def define(self, section, key, value, factory="golang"):
        path = self.toolbox / "factories" / factory / "factory.json"
        definition = json.loads(path.read_text())
        definition.setdefault(section, {})[key] = value
        path.write_text(json.dumps(definition))

    def command(self, script, *args, expected=0, env=None, cwd=None, input=None):
        result = subprocess.run(
            [sys.executable, str(self.toolbox / script), *map(str, args)],
            cwd=cwd or self.project, env=env or self.env,
            text=True, input=input, capture_output=True, timeout=15,
        )
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        return result

    def task_command(self, *args, **kwargs):
        return self.command("orchestrator/scripts/task.py", *args, **kwargs)

    def create_task(self, factory="golang", name="Mixed CASE Task", brief="Keep it small."):
        result = self.task_command("new", "--factory", factory, "--name", name, "--brief", brief)
        path = Path(result.stdout.strip())
        return path if path.is_absolute() else self.project / path

    def fake_pi(self):
        binaries = self.project / "bin"
        binaries.mkdir(exist_ok=True)
        capture = self.project / "launch.json"
        fake = binaries / "pi"
        fake.write_text(
            f"#!{sys.executable}\n"
            "import json, os, pathlib, sys, time\n"
            "args = sys.argv[1:]\n"
            "agent_dir = pathlib.Path(os.environ['PI_CODING_AGENT_DIR'])\n"
            "if '--mode' in args and args[args.index('--mode') + 1] == 'rpc':\n"
            "    request = json.loads(sys.stdin.readline())\n"
            "    commands = [] if os.environ.get('MISSING_RALPH') else [{'name': name} for name in ('ralph-loop', 'ralph-status', 'ralph-stop', 'ralph-resume')]\n"
            "    print(json.dumps({'type': 'response', 'id': request['id'], 'success': True, 'data': {'commands': commands}}))\n"
            "    sys.exit(0)\n"
            "check = 'credentials' if args[:2] == ['auth', 'check'] else 'mcp' if args[:2] == ['mcp', 'list'] else None\n"
            "if check:\n"
            "    if os.environ.get('HANG_CHECK') == check: time.sleep(30)\n"
            "    if os.environ.get('FAIL_CHECK') == check:\n"
            "        print('simulated native failure ' + os.environ.get('SECRET_SENTINEL', ''))\n"
            "        sys.exit(1)\n"
            "    if check == 'credentials':\n"
            "        print(json.dumps({'status': 'ready'}))\n"
            "    else:\n"
            "        servers = json.loads((agent_dir / 'mcp.json').read_text())['mcpServers']\n"
            "        print(json.dumps({'servers': [{'name': key, 'state': 'connected', 'tools': ['inspect'], 'exposure': value.get('exposure', 'codemode')} for key, value in servers.items() if value.get('enabled', True)], 'errors': []}))\n"
            "    sys.exit(0)\n"
            "instructions = args[args.index('--append-system-prompt') + 1]\n"
            "settings = json.loads((agent_dir / 'settings.json').read_text())\n"
            "record = {'cwd': os.getcwd(), 'args': args, 'instructions': pathlib.Path(instructions).read_text(), 'private_config': str(agent_dir), 'settings': settings, 'certs': os.environ.get('NODE_EXTRA_CA_CERTS')}\n"
            "record['identity'] = {'role': os.environ.get('FACTORY_ROLE'), 'project': os.environ.get('FACTORY_PROJECT'), 'task_id': os.environ.get('FACTORY_TASK_ID'), 'task_dir': os.environ.get('FACTORY_TASK_DIR')}\n"
            "pathlib.Path(os.environ['CAPTURE']).write_text(json.dumps(record))\n"
        )
        fake.chmod(0o755)
        env = {**self.env, "PATH": str(binaries) + os.pathsep + os.environ.get("PATH", ""), "CAPTURE": str(capture)}
        return capture, env

    def test_attachment_failure_concurrency_and_retry_preserve_documents(self):
        import sys
        sys.path.insert(0, str(SOURCE / "orchestrator/comms"))
        from client import Client
        client = Client(self.env)
        record = client.call("create_task", project=str(self.project), owner="coordinator", name="Atomic", brief="Small")
        template = self.toolbox / "factories/golang/templates/plan.html"
        original = template.read_text()
        template.write_text("$missing_variable")
        failed = self.task_command("attach", record["id"], "--factory", "golang", expected=1)
        self.assertFalse(any(json.loads(p.read_text())["comms_id"] == record["id"] for p in (self.toolbox / "tasks").glob("*/task.json")))
        template.write_text(original)
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            results = list(pool.map(lambda _: self.task_command("attach", record["id"], "--factory", "golang").stdout.strip(), range(2)))
        self.assertEqual(results[0], results[1])
        path = Path(results[0])
        if not path.is_absolute(): path = self.project / path
        (path / "spec.html").write_text("Authored content")
        self.task_command("attach", record["id"], "--factory", "golang")
        self.assertEqual((path / "spec.html").read_text(), "Authored content")
        events = [event for event in client.events(record["id"]) if (event.get("data") or {}).get("event_type") == "artifacts_attached"]
        self.assertEqual(len(events), 1)

    def test_factory_check_reports_all_members_without_registering_or_chat(self):
        capture, env = self.fake_pi()
        report = json.loads(self.command("orchestrator/session.py", "--check-factory", "--factory", "golang", env=env).stdout)
        self.assertTrue(report["ready"])
        self.assertEqual(len(report["members"]), 5)
        self.assertFalse(capture.exists())
        for member in report["members"]:
            self.assertTrue(Path(member["diagnostics"]).exists())
        definition = self.toolbox / "factories/golang/factory.json"
        config = json.loads(definition.read_text())
        config["agents"][-1].setdefault("config", {})["executable"] = "missing-factory-runner"
        definition.write_text(json.dumps(config))
        report = json.loads(self.command("orchestrator/session.py", "--check-factory", "--factory", "golang", env=env, expected=1).stdout)
        self.assertFalse(report["ready"])
        self.assertEqual(sum(member["ready"] for member in report["members"]), 4)

    def test_launcher_records_immutable_public_snapshot(self):
        capture, env = self.fake_pi()
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        sys.path.insert(0, str(SOURCE / "orchestrator/comms"))
        from client import Client
        client = Client(env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        participant = client.call("list_participants", task_id=task_id)["participants"][0]
        snapshot = client.call("get_session", task_id=task_id, participant_id=participant["id"])
        self.assertTrue(snapshot["manifest"]["startup_checked"])
        self.assertEqual(snapshot["contact_state"], "unknown")
        original = snapshot["manifest"]["skills"][0]["sha256"]
        (self.toolbox / "resources/agents/architect/agent.md").write_text("changed after launch")
        again = client.call("get_session", task_id=task_id, participant_id=participant["id"])
        self.assertEqual(original, again["manifest"]["skills"][0]["sha256"])

    def dispatch_fixture(self):
        capture, env = self.fake_pi()
        binary = self.project / "bin/herdr"
        binary.write_text("#!" + sys.executable + "\n" + (SOURCE / "orchestrator/tests/fixtures/herdr.py").read_text().split("\n", 1)[1])
        binary.chmod(0o755)
        root = self.project / "host-fixture"
        root.mkdir()
        env.update(HERDR_ENV="1", HOST_FIXTURE=str(root))
        bridge = self.toolbox / "orchestrator/start-delivery.sh"
        import shlex
        bridge.write_text("#!/bin/sh\nexec " + shlex.quote(str(bridge_binary())) + ' "$@"\n')
        sys.path.insert(0, str(SOURCE / "orchestrator/comms"))
        from client import Client
        client = Client(env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        owner = client.call("register_participant", task_id=task_id, role="coordinator", routing_identity="coordinator", instance_id="owner", request_key="owner")
        request = self.project / "assignment.json"
        request.write_text(json.dumps({"role":"architect", "title":"Draft", "instructions":"Draft approved scope", "expected_output":["spec.html"], "stage":"design", "attempt":1}))
        arguments = [self.task, "--request", request, "--key", "dispatch-1", "--owner", owner["id"], "--timeout", "1"]
        return env, root, client, task_id, arguments

    def test_dispatch_retry_creates_one_pane_assignment_and_notification(self):
        env, root, client, task_id, args = self.dispatch_fixture()
        first = json.loads(self.command("orchestrator/dispatch.py", *args, env=env).stdout)
        second = json.loads(self.command("orchestrator/dispatch.py", *args, env=env).stdout)
        self.assertEqual(first["assignment_id"], second["assignment_id"])
        calls = [json.loads(line)[:2] for line in (root / "calls.jsonl").read_text().splitlines()]
        for command in (["pane","split"], ["pane","run"], ["agent","prompt"]):
            self.assertEqual(calls.count(command), 1)
        assignments = client.call("list_assignments", task_id=task_id)["assignments"]
        self.assertEqual(len(assignments), 1)
        self.assertEqual(assignments[0]["state"], "offered")
        request = self.project / "assignment.json"
        data = json.loads(request.read_text()); data["instructions"] = "different"; request.write_text(json.dumps(data))
        self.assertIn("different arguments", self.command("orchestrator/dispatch.py", *args, env=env, expected=1).stderr)

    def test_dispatch_uncertain_launch_reconciles_actual_participant(self):
        env, root, client, task_id, args = self.dispatch_fixture()
        failed = json.loads(self.command("orchestrator/dispatch.py", *args, env={**env,"UNCERTAIN_LAUNCH":"1"}, expected=1).stdout)
        self.assertEqual(failed["phase"], "launch_pending")
        resumed = json.loads(self.command("orchestrator/dispatch.py", *args, env=env).stdout)
        self.assertEqual(resumed["phase"], "notification_checked")
        calls = [json.loads(line)[:2] for line in (root / "calls.jsonl").read_text().splitlines()]
        self.assertEqual(calls.count(["pane","run"]), 1)

    def test_dispatch_uncertain_notification_and_failed_start_are_not_repeated(self):
        env, root, client, task_id, args = self.dispatch_fixture()
        first = json.loads(self.command("orchestrator/dispatch.py", *args, env={**env,"UNCERTAIN_PROMPT":"1"}, expected=1).stdout)
        self.assertEqual(first["phase"], "assigned")
        self.command("orchestrator/dispatch.py", *args, env=env, expected=1)
        calls = [json.loads(line)[:2] for line in (root / "calls.jsonl").read_text().splitlines()]
        self.assertEqual(calls.count(["agent","prompt"]), 1)
        self.assertEqual(len(client.call("list_assignments",task_id=task_id)["assignments"]), 1)

    def test_dispatch_uncertain_split_requires_explicit_recovery(self):
        env, root, client, task_id, args = self.dispatch_fixture()
        first = json.loads(self.command("orchestrator/dispatch.py", *args, env={**env,"UNCERTAIN_SPLIT":"1"}, expected=1).stdout)
        self.assertEqual(first["phase"], "split_pending")
        self.command("orchestrator/dispatch.py", *args, env=env, expected=1)
        recovered = json.loads(self.command("orchestrator/dispatch.py", *args, "--recover-pane", "fixture-pane", env=env).stdout)
        self.assertEqual(recovered["phase"], "notification_checked")
        calls = [json.loads(line)[:2] for line in (root / "calls.jsonl").read_text().splitlines()]
        self.assertEqual(calls.count(["pane","split"]), 1)
        self.assertEqual(calls.count(["pane","run"]), 1)

    def test_dispatch_startup_failure_never_assigns_or_relaunches(self):
        env, root, client, task_id, args = self.dispatch_fixture()
        self.command("orchestrator/dispatch.py", *args, env={**env,"FAIL_CHECK":"credentials"}, expected=1)
        resumed = json.loads(self.command("orchestrator/dispatch.py", *args, env=env, expected=1).stdout)
        self.assertIn("no actual participant", resumed["error"])
        calls = [json.loads(line)[:2] for line in (root / "calls.jsonl").read_text().splitlines()]
        self.assertEqual(calls.count(["pane","run"]), 1)
        self.assertEqual(client.call("list_assignments",task_id=task_id)["assignments"], [])

    def test_usage_import_is_attributed_and_idempotent(self):
        sys.path.insert(0, str(SOURCE / "orchestrator/comms"))
        from client import Client
        client = Client(self.env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        def register(role):
            return client.call("register_participant", task_id=task_id, role=role, routing_identity=role, instance_id="usage-"+role, request_key=role)
        owner,worker=register("coordinator"),register("coder")
        assignment=client.call("create_assignment",task_id=task_id,assigning_participant_id=owner["id"],assignee_id=worker["id"],title="Build",instructions="Scope",expected_output=["code"],stage="build",attempt=1,request_key="assign")
        path=self.runtime/worker["instance_id"]/"session.jsonl"
        path.parent.mkdir(parents=True)
        rows=[{"type":"session","version":3,"id":"session", "cwd":str(self.project)}, {"type":"message","id":"response","message":{"role":"assistant","provider":"fixture","model":"model","stopReason":"stop","usage":{"input":2,"output":3,"cacheRead":1,"cacheWrite":0,"totalTokens":6}}}]
        path.write_text("".join(json.dumps(row)+"\n" for row in rows))
        for _ in range(2):
            self.command("orchestrator/adapters/pi/import_usage.py",self.task,"--session",path,"--participant",worker["id"],"--assignment",assignment["id"])
        report=client.call("get_task_report",task_id=task_id)
        self.assertEqual(report["usage"]["records"],1)
        self.assertEqual(report["usage"]["total_tokens"],6)
        self.assertIsNone(report["usage"]["cost_usd"])

    def test_task_location_name_resolution_and_html_escaping(self):
        self.assertEqual(self.task.parent, self.toolbox / "tasks")
        self.assertRegex(self.task.name, r"^\d{8}T\d{9}Z-mixed-case-task")
        self.assertEqual(json.loads((self.task / "task.json").read_text())["name"], "Mixed CASE Task")
        self.assertFalse((self.project.parent / "tasks").exists())
        hostile = '<script>alert("x")</script> & words'
        task = self.create_task(name=hostile, brief=hostile)
        document = (task / "spec.html").read_text()
        self.assertNotIn("<script>", document)
        self.assertIn("&lt;script&gt;", document)
        self.assertTrue((task / "plan.html").is_file())

    def test_writing_task_has_no_spec_or_plan(self):
        task = self.create_task("writing")
        self.assertFalse((task / "spec.html").exists())
        self.assertFalse((task / "plan.html").exists())
        self.assertEqual(json.loads((task / "task.json").read_text())["owner"], "pair_writer")

    def test_long_brief_from_stdin_and_invocation_outside_project(self):
        brief = "long brief\n" * 1000
        result = self.task_command(
            "new", "--factory", "golang", "--name", "A", "--brief-file", "-",
            input=brief, cwd=self.project.parent,
        )
        path = self.project.parent / result.stdout.strip()
        self.assertEqual(path.parent, self.toolbox / "tasks")
        self.assertEqual(json.loads((path / "task.json").read_text())["brief"], brief.strip())

    def test_peer_messages_and_acknowledgement(self):
        payload = 'A question\nwith "quotes", $(touch SHOULD_NOT_EXIST), and `literal text`.'
        result = self.task_command(
            "send", self.task, "--from", "architect", "--to", "architecture_reviewer",
            "--kind", "question", "--text-file", "-", input=payload,
        )
        message_id = json.loads(result.stdout)["message_id"]
        inbox = json.loads(self.task_command("inbox", self.task, "--for", "architecture_reviewer").stdout)
        self.assertEqual(inbox[0]["text"], payload)
        self.assertFalse((self.project / "SHOULD_NOT_EXIST").exists())
        self.task_command("ack", self.task, message_id, "--by", "coder_golang", expected=1)
        self.task_command("ack", self.task, message_id, "--by", "architecture_reviewer")
        first_ack = json.loads(self.task_command("inbox", self.task, "--for", "architecture_reviewer", "--all").stdout)[0]["acknowledged_at"]
        self.task_command("ack", self.task, message_id, "--by", "architecture_reviewer")
        self.assertEqual(json.loads(self.task_command("inbox", self.task, "--for", "architecture_reviewer", "--all").stdout)[0]["acknowledged_at"], first_ack)
        self.assertEqual(json.loads(self.task_command("inbox", self.task, "--for", "architecture_reviewer").stdout), [])
        self.assertEqual(len(json.loads(self.task_command("inbox", self.task, "--for", "architecture_reviewer", "--all").stdout)), 1)

    def test_concurrent_senders_do_not_lose_messages(self):
        def send(index):
            return self.task_command(
                "send", self.task, "--from", f"worker-{index}", "--to", "coordinator",
                "--kind", "progress", "--text", f"Result {index}",
            )
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            list(pool.map(send, range(12)))
        messages = json.loads(self.task_command("inbox", self.task, "--for", "coordinator").stdout)
        self.assertEqual(len(messages), 12)
        self.assertEqual(len({message["id"] for message in messages}), 12)

    def test_failed_notification_preserves_message(self):
        result = self.task_command(
            "send", self.task, "--from", "architect", "--to", "coordinator",
            "--kind", "result", "--text", "Ready", "--notify", "some-coordinator", expected=1,
        )
        receipt = json.loads(result.stdout)
        self.assertTrue(receipt["stored"])
        self.assertEqual(receipt["notification"], "failed")
        self.assertEqual(len(json.loads(self.task_command("inbox", self.task, "--for", "coordinator").stdout)), 1)

    def test_notification_delegates_to_verified_bridge_with_argv(self):
        binaries = self.project / "bin"
        binaries.mkdir()
        capture = self.project / "notification.json"
        fake = self.toolbox / "orchestrator/start-delivery.sh"
        fake.write_text(
            f"#!{sys.executable}\nimport json, os, sys\n"
            "open(os.environ['CAPTURE'], 'w').write(json.dumps(sys.argv[1:]))\n"
        )
        fake.chmod(0o755)
        env = {**self.env, "HERDR_ENV": "1", "PATH": str(binaries) + os.pathsep + os.environ.get("PATH", ""), "CAPTURE": str(capture)}
        target = 'name; $(touch BAD)'
        result = self.task_command(
            "send", self.task, "--from", "architect", "--to", "coordinator",
            "--kind", "result", "--text", "Ready", "--notify", target, env=env,
        )
        argv = json.loads(capture.read_text())
        self.assertEqual(argv[-4:], ["--notify", "coordinator", "--agent", target])
        self.assertEqual(argv[:2], ["--project", str(self.project)])
        self.assertEqual(json.loads(result.stdout)["notification"], "checked")
        self.assertFalse((self.project / "BAD").exists())

    def test_decision_records_exact_document_versions(self):
        result = self.task_command(
            "decide", self.task, "--by", "human", "--outcome", "approve",
            "--artifact", "spec.html", "--artifact", "plan.html", "--text", "Approved",
        )
        record = json.loads(result.stdout)
        self.assertEqual(record["data"]["artifacts"][0]["sha256"], hashlib.sha256((self.task / "spec.html").read_bytes()).hexdigest())
        (self.task / "spec.html").write_text("Changed")
        self.assertNotEqual(record["data"]["artifacts"][0]["sha256"], hashlib.sha256((self.task / "spec.html").read_bytes()).hexdigest())
        self.task_command("decide", self.task, "--by", "human", "--outcome", "approve", "--artifact", "../../../outside", "--text", "x", expected=1)

    def test_status_and_explicit_task_boundaries(self):
        self.task_command("status", self.task, "--state", "needs_input", "--summary", "Awaiting decision")
        result = json.loads(self.task_command("status", self.task).stdout)
        self.assertEqual(result["task"]["status"], "needs_input")
        self.task_command("status", self.project, expected=1)

    def test_all_roles_dry_run_without_runtime_files(self):
        roles = ["coordinator", "architect", "architecture_reviewer", "coder_golang", "code_reviewer_golang", "pair_writer"]
        writing = self.create_task(factory="writing", name="Writing preview")
        before = sorted(str(path.relative_to(self.project)) for path in self.project.rglob("*"))
        for role in roles:
            selected = writing if role == "pair_writer" else self.task
            result = self.command("orchestrator/session.py", role, selected, "--dry-run")
            plan = json.loads(result.stdout)
            self.assertEqual(plan["cwd"], str(self.project))
            self.assertIn("task-comms", plan["instructions"])
            self.assertIn("comms", plan["runner_configuration"]["enabled_mcps"])
            self.assertIn("--session", plan["command"])
            self.assertNotIn("--continue", plan["command"])
            self.assertNotIn("--print", plan["command"])
        self.assertFalse(self.runtime.exists())
        self.assertEqual(before, sorted(str(path.relative_to(self.project)) for path in self.project.rglob("*")))

    def test_independent_role_models_and_missing_environment_reference(self):
        self.configure("architect", {"model": "architect-model", "thinking": "high"})
        architect = json.loads(self.command("orchestrator/session.py", "architect", self.task, "--dry-run").stdout)
        coder = json.loads(self.command("orchestrator/session.py", "coder_golang", self.task, "--dry-run").stdout)
        self.assertEqual(architect["runner_configuration"]["model"], "architect-model")
        self.assertEqual(coder["runner_configuration"]["model"], "test-model")
        self.assertEqual(architect["runner_configuration"]["thinking"], "high")
        self.assertNotIn("--model", architect["command"])
        self.configure("architect", {"env": {"NODE_EXTRA_CA_CERTS": "NONEXISTENT_FACTORY_TEST_ENV"}})
        result = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", expected=1)
        self.assertIn("NONEXISTENT_FACTORY_TEST_ENV", result.stderr)
        self.assertFalse(self.runtime.exists())

    def test_native_pi_launch_uses_project_root_and_private_home_session(self):
        capture, env = self.fake_pi()
        self.configure("architect", {"env": {"NODE_EXTRA_CA_CERTS": "FACTORY_ARCHITECT_CERTS"}})
        env["FACTORY_ARCHITECT_CERTS"] = "/some/private/certificates.pem"
        source = self.toolbox / "factories/golang/factory.json"
        original = source.read_bytes()
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        record = json.loads(capture.read_text())
        self.assertEqual(record["cwd"], str(self.project))
        self.assertTrue(Path(record["private_config"]).is_relative_to(self.runtime))
        self.assertEqual(record["certs"], "/some/private/certificates.pem")
        self.assertEqual(source.read_bytes(), original)
        self.assertEqual(record["settings"]["defaultProvider"], "fixture")
        self.assertNotIn("--tools", record["args"])
        self.assertIn("--no-approve", record["args"])
        self.assertIn("# Spec and plan", record["instructions"])
        session_file = Path(record["args"][record["args"].index("--session") + 1])
        self.assertTrue(session_file.is_relative_to(self.runtime))
        instructions = session_file.parent / "instructions.md"
        self.assertEqual(instructions.stat().st_mode & 0o777, 0o600)

    def test_loop_requires_current_approvals_and_supported_role(self):
        self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "0", "--dry-run", expected=1)
        self.command("orchestrator/session.py", "architect", self.task, "--loop", "5", "--dry-run", expected=1)
        result = self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "5", "--dry-run", expected=1)
        self.assertIn("approval of the current spec.html", result.stderr)
        self.task_command("decide", self.task, "--by", "human", "--outcome", "approve", "--artifact", "spec.html", "--artifact", "plan.html", "--text", "Approved")
        plan = json.loads(self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "5", "--dry-run").stdout)
        self.assertTrue(plan["command"][-1].startswith("/ralph-loop --max-iterations=5 "))
        self.assertIn(str(self.task), plan["command"][-1])
        self.assertIn("conventional-commits", plan["instructions"])
        self.assertFalse(self.runtime.exists())
        (self.task / "plan.html").write_text("A changed plan")
        result = self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "5", "--dry-run", expected=1)
        self.assertIn("approval of the current plan.html", result.stderr)
        self.task_command("decide", self.task, "--by", "human", "--outcome", "approve", "--artifact", "plan.html", "--text", "Approved revision")
        self.task_command("decide", self.task, "--by", "human", "--outcome", "request_changes", "--artifact", "spec.html", "--text", "Revise behavior")
        self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "5", "--dry-run", expected=1)

    def test_loop_checks_real_command_registration_before_launch(self):
        capture, env = self.fake_pi()
        self.task_command("decide", self.task, "--by", "human", "--outcome", "approve", "--artifact", "spec.html", "--artifact", "plan.html", "--text", "Approved")
        result = self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "3", "--check", env=env)
        self.assertIn("ralph-loop", json.loads(result.stdout)["commands"])
        self.assertFalse(capture.exists())
        sys.path.insert(0, str(self.toolbox / "orchestrator/comms"))
        from client import Client
        client = Client(self.env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        owner = client.call("register_participant", task_id=task_id, instance_id="actual-owner", role="coordinator", routing_identity="coordinator", request_key="owner")
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as executor:
            launch = executor.submit(self.command, "orchestrator/session.py", "coder_golang", self.task, "--loop", "3", env=env)
            deadline = time.monotonic() + 5
            worker = None
            while time.monotonic() < deadline:
                workers = [p for p in client.call("list_participants", task_id=task_id)["participants"] if p["role"] == "coder_golang"]
                if workers:
                    worker = workers[0]
                    break
                time.sleep(0.05)
            self.assertIsNotNone(worker, "launcher must register before waiting for work")
            client.call("create_assignment", task_id=task_id, assigning_participant_id=owner["id"], assignee_id=worker["id"], title="Fixture build", instructions="Fixture only", expected_output=["fixture result"], stage="build", attempt=1, request_key="build")
            launch.result(timeout=10)
        record = json.loads(capture.read_text())
        self.assertEqual(record["cwd"], str(self.project))
        self.assertTrue(record["args"][-1].startswith("/ralph-loop --max-iterations=3 "))
        capture.unlink()
        result = self.command("orchestrator/session.py", "coder_golang", self.task, "--loop", "3", expected=1,
                              env={**env, "MISSING_RALPH": "1"})
        self.assertIn("command(s) unavailable", result.stderr)
        self.assertFalse(capture.exists())

    def test_loop_rechecks_documents_after_wait_and_records_registered_failure(self):
        capture, env = self.fake_pi()
        self.task_command("decide", self.task, "--by", "human", "--outcome", "approve", "--artifact", "spec.html", "--artifact", "plan.html", "--text", "Approved")
        sys.path.insert(0, str(self.toolbox / "orchestrator/comms"))
        from client import Client
        client = Client(self.env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        owner = client.call("register_participant", task_id=task_id, instance_id="actual-owner", role="coordinator", routing_identity="coordinator", request_key="owner")
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as executor:
            launch = executor.submit(self.command, "orchestrator/session.py", "coder_golang", self.task, "--loop", "3", env=env, expected=1)
            deadline = time.monotonic() + 5
            worker = None
            while time.monotonic() < deadline:
                workers = [p for p in client.call("list_participants", task_id=task_id)["participants"] if p["role"] == "coder_golang"]
                if workers:
                    worker = workers[0]
                    break
                time.sleep(0.05)
            self.assertIsNotNone(worker)
            (self.task / "plan.html").write_text("Changed while launcher waited for an assignment")
            client.call("create_assignment", task_id=task_id, assigning_participant_id=owner["id"], assignee_id=worker["id"], title="Fixture build", instructions="Fixture only", expected_output=["fixture result"], stage="build", attempt=1, request_key="build")
            result = launch.result(timeout=10)
        self.assertIn("approval of the current plan.html", result.stderr)
        self.assertFalse(capture.exists(), "changed approvals must prevent model launch")
        failures = [event for event in client.events(task_id) if event["kind"] == "failure"]
        self.assertEqual(len(failures), 1)
        self.assertEqual(failures[0]["data"]["participant_id"], worker["id"])

    def test_linear_is_optional_and_credentials_stay_private(self):
        capture, env = self.fake_pi()
        for role in ("architect", "architecture_reviewer", "code_reviewer_golang"):
            plan = json.loads(self.command("orchestrator/session.py", role, self.task, "--dry-run", env=env).stdout)
            self.assertNotIn("LINEAR_API_KEY", plan["runner_configuration"]["required_environment"])
            self.assertIn("linear-context", plan["instructions"])
        self.configure("architect", mcps=["comms", "linear"])
        missing_env = {key: value for key, value in env.items() if key != "LINEAR_API_KEY"}
        result = self.command("orchestrator/session.py", "architect", self.task, expected=1, env=missing_env)
        self.assertIn("LINEAR_API_KEY", result.stderr)
        self.assertFalse(capture.exists())
        env["LINEAR_API_KEY"] = "fixture-private-linear-key"
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        profile = Path(json.loads(capture.read_text())["private_config"])
        linear = json.loads((profile / "mcp.json").read_text())["mcpServers"]["linear"]
        self.assertEqual(linear["url"], "https://mcp.linear.app/mcp/readonly")
        self.assertEqual(linear["headers"]["Authorization"], "Bearer ${LINEAR_API_KEY}")
        self.assertNotIn(env["LINEAR_API_KEY"], self.command("orchestrator/session.py", "architect", self.task, "--dry-run", env=env).stdout)

    def test_mcp_overlay_and_relative_workspace_are_materialized(self):
        capture, env = self.fake_pi()
        self.define("mcps", "gopls", {"command": "gopls", "args": ["mcp"], "timeout": 15, "cwd": "."})
        self.command("orchestrator/session.py", "coder_golang", self.task, env=env)
        profile = Path(json.loads(capture.read_text())["private_config"])
        config = json.loads((profile / "mcp.json").read_text())["mcpServers"]["gopls"]
        self.assertEqual(config["command"], "gopls")
        self.assertEqual(config["timeout"], 15)
        self.assertEqual(config["cwd"], str(self.project))
        self.assertFalse((profile / "mcp.local.json").exists())
        report = json.loads((profile.parent / "startup-checks.json").read_text())
        self.assertTrue(all(server["tools"] == ["inspect"] for server in report["mcps"]))

    def test_startup_failure_is_visible_without_exposing_native_diagnostics(self):
        capture, env = self.fake_pi()
        env.update(FAIL_CHECK="mcp", SECRET_SENTINEL="secret-that-must-stay-private")
        result = self.command("orchestrator/session.py", "coder_golang", self.task, expected=1, env=env)
        self.assertFalse(capture.exists())
        self.assertIn("mcp startup check failed", result.stderr)
        self.assertNotIn(env["SECRET_SENTINEL"], result.stdout + result.stderr)
        sys.path.insert(0, str(SOURCE / "orchestrator" / "comms"))
        from client import Client
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        events = list(Client(self.env).events(task_id))
        failure = next(entry["data"] for entry in events if entry["data"].get("event_type") == "session_startup_failed")
        self.assertNotIn(env["SECRET_SENTINEL"], json.dumps(failure))
        report = Path(failure["runtime_dir"]) / "startup-checks.json"
        self.assertIn(env["SECRET_SENTINEL"], report.read_text())
        self.assertEqual(report.stat().st_mode & 0o777, 0o600)

    def test_startup_timeout_stops_before_chat(self):
        capture, env = self.fake_pi()
        env["HANG_CHECK"] = "credentials"
        self.configure("architect", {"check_timeout_seconds": 0.1})
        result = self.command("orchestrator/session.py", "architect", self.task, expected=1, env=env)
        self.assertFalse(capture.exists())
        self.assertIn("timed out", result.stderr)

    def test_native_check_prepares_bundle_without_starting_chat(self):
        capture, env = self.fake_pi()
        result = self.command("orchestrator/session.py", "architect", self.task, "--check", env=env)
        self.assertTrue(json.loads(result.stdout)["ready"])
        self.assertFalse(capture.exists())

    def test_private_auth_file_is_explicitly_linked_not_copied_into_bundle(self):
        capture, env = self.fake_pi()
        auth = self.project.parent / "private-auth.json"
        auth.write_text('{"fixture":{"type":"api_key","key":"private-fixture-key"}}')
        env["FACTORY_AUTH_FILE"] = str(auth)
        self.configure("architect", {"auth_file_env": "FACTORY_AUTH_FILE"})
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        profile = Path(json.loads(capture.read_text())["private_config"])
        self.assertTrue((profile / "auth.json").is_symlink())
        self.assertEqual((profile / "auth.json").resolve(), auth)
        self.assertFalse((self.toolbox / "resources" / "agents" / "architect" / "pi" / "auth.json").exists())

    def test_readonly_native_profile_produces_private_writable_runtime(self):
        capture, env = self.fake_pi()
        source = self.toolbox / "factories/golang/factory.json"
        source.chmod(0o444)
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        target = Path(json.loads(capture.read_text())["private_config"])
        self.assertEqual(target.stat().st_mode & 0o777, 0o700)
        self.assertEqual((target / "settings.json").stat().st_mode & 0o777, 0o600)
        self.assertEqual(source.stat().st_mode & 0o777, 0o444)

    def test_disabled_mcp_does_not_require_its_environment(self):
        capture, env = self.fake_pi()
        self.define("mcps", "optional", {"url": "https://example.invalid/mcp", "headers": {"Authorization": "Bearer ${UNAVAILABLE_OPTIONAL_MCP_KEY}"}})
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        profile = Path(json.loads(capture.read_text())["private_config"])
        self.assertNotIn("optional", json.loads((profile / "mcp.json").read_text())["mcpServers"])

    def test_mcp_auth_is_shared_only_when_explicitly_requested(self):
        capture, env = self.fake_pi()
        auth = self.project.parent / "private-mcp-auth.json"
        auth.write_text('{}')
        env["FACTORY_MCP_AUTH_FILE"] = str(auth)
        self.configure("architect", {"mcp_auth_file_env": "FACTORY_MCP_AUTH_FILE"})
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        profile = Path(json.loads(capture.read_text())["private_config"])
        self.assertEqual((profile / "mcp-auth.json").resolve(), auth)

    def test_native_provider_environment_values_do_not_appear_in_dry_run(self):
        self.define("providers", "fixture", {"baseUrl": "http://example.invalid/v1", "apiKey": "${FIXTURE_KEY}", "models": [{"id": "test-model"}]})
        env = {**self.env, "FIXTURE_KEY": "private-env-value"}
        result = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", env=env)
        self.assertNotIn(env["FIXTURE_KEY"], result.stdout)
        self.assertNotIn("apiKey", result.stdout)
        self.assertFalse(self.runtime.exists())

    def test_custom_runner_receives_complete_instructions(self):
        capture = self.project / "custom.json"
        fake = self.project / "fake-agent.py"
        fake.write_text(
            "import json, os, pathlib, sys\n"
            "pathlib.Path(sys.argv[2]).write_text(json.dumps({'cwd': os.getcwd(), 'instructions': pathlib.Path(sys.argv[1]).read_text()}))\n"
        )
        self.configure("pair_writer", {"runner": "command", "command": [sys.executable, str(fake), "{instructions_file}", str(capture)]}, factory="writing")
        writing = self.create_task(factory="writing", name="Custom runner")
        self.command("orchestrator/session.py", "pair_writer", writing)
        result = json.loads(capture.read_text())
        self.assertEqual(result["cwd"], str(self.project))
        self.assertIn("# Business writing", result["instructions"])
        self.assertIn("# Technical writing", result["instructions"])

    def test_mcp_created_task_attaches_once_and_has_no_local_mailbox(self):
        sys.path.insert(0, str(SOURCE / "orchestrator" / "comms"))
        from client import Client
        client = Client(self.env)
        record = client.call("create_task", project=str(self.project), name="MCP task", brief="Use service tools",
                             owner="coordinator", request_key="mcp-created")
        first = Path(self.task_command("attach", record["id"], "--factory", "golang").stdout.strip())
        first = first if first.is_absolute() else self.project / first
        second = Path(self.task_command("attach", record["id"], "--factory", "golang").stdout.strip())
        second = second if second.is_absolute() else self.project / second
        self.assertEqual(first, second)
        self.assertTrue((first / "spec.html").is_file())
        for name in ("status.json", "messages", "events", "acks"):
            self.assertFalse((first / name).exists())
        self.task_command("send", first, "--from", "coordinator", "--to", "architect", "--kind", "assignment", "--text", "Draft")
        messages = list(client.inbox(record["id"], "architect"))
        self.assertEqual(messages[0]["text"], "Draft")
        client.call("send_message", task_id=record["id"], sender="architect", recipient="coordinator", kind="result", text="Ready")
        self.assertEqual(json.loads(self.task_command("inbox", first, "--for", "coordinator").stdout)[0]["text"], "Ready")

    def test_direct_mcp_decisions_control_loop_and_pagination(self):
        sys.path.insert(0, str(SOURCE / "orchestrator" / "comms"))
        from client import Client
        client = Client(self.env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        artifacts = [{"path": name, "sha256": hashlib.sha256((self.task / name).read_bytes()).hexdigest()}
                     for name in ("spec.html", "plan.html")]
        client.call("record_event", task_id=task_id, actor="human", kind="decision", message="Actual approval",
                    data={"outcome": "approve", "artifacts": artifacts})
        # Approval must remain discoverable beyond the maximum 500-event page.
        for index in range(501):
            client.call("record_event", task_id=task_id, actor="coordinator", kind="note", message=str(index))
        self.command("orchestrator/session.py", "coder_golang", self.task, "--factory", "golang", "--loop", "2", "--dry-run")
        client.call("record_event", task_id=task_id, actor="human", kind="decision", message="Stop and revise",
                    data={"outcome": "request_changes"})
        self.command("orchestrator/session.py", "coder_golang", self.task, "--factory", "golang", "--loop", "2", "--dry-run", expected=1)

    def test_inbox_pagination_and_service_outage(self):
        sys.path.insert(0, str(SOURCE / "orchestrator" / "comms"))
        from client import Client
        client = Client(self.env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        for index in range(201):
            client.call("send_message", task_id=task_id, sender="architect", recipient="coordinator",
                        kind="progress", text=f"Progress {index}", request_key=f"pagination-{index}")
        messages = json.loads(self.task_command("inbox", self.task, "--for", "coordinator").stdout)
        self.assertEqual(len(messages), 201)
        self.assertEqual(messages[-1]["text"], "Progress 200")
        env = {**self.env, "FACTORY_COMMS_URL": "http://127.0.0.1:1/mcp"}
        capture, runner_env = self.fake_pi()
        result = self.command("orchestrator/session.py", "architect", self.task,
                              env={**runner_env, **env}, expected=1)
        self.assertIn("comms unavailable", result.stderr)
        self.assertFalse(capture.exists())
        self.assertFalse(self.runtime.exists())
        self.assertFalse((self.task / "messages").exists())

    def test_service_auth_failure_has_no_local_state_fallback(self):
        env = {**self.env, "COMMS_AUTH_TOKEN": "incorrect-fixture-token"}
        result = self.task_command("status", self.task, env=env, expected=1)
        self.assertIn("comms HTTP 401", result.stderr)
        self.assertNotIn(env["COMMS_AUTH_TOKEN"], result.stderr)
        self.assertFalse((self.task / "status.json").exists())

    def test_ambiguous_unbound_agent_requires_a_factory_task(self):
        source = self.toolbox / "resources/agents/architect"
        duplicate = self.toolbox / "factories/another/agents/architect"
        shutil.copytree(source, duplicate)
        factory = duplicate.parents[1]
        (factory / "README.md").write_text("# Another factory\n")
        (factory / "factory.json").write_text(json.dumps({"name": "another", "owner": "architect", "agents": [
            {"name": "architect", "path": "agents/architect"}
        ]}))
        result = self.command("orchestrator/session.py", "architect", "--dry-run", expected=1)
        self.assertIn("select a factory", result.stderr)
        self.assertFalse(self.runtime.exists())
        self.command("orchestrator/session.py", "architect", self.task, "--dry-run")

    def test_factory_membership_and_paths_control_launch(self):
        path = self.toolbox / "factories/golang/factory.json"
        definition = json.loads(path.read_text())
        source = self.toolbox / "factories/golang/agents/coder_golang"
        relocated = source.with_name("implementation")
        shutil.copytree(source, relocated)
        for member in definition["agents"]:
            if member["name"] == "coder_golang":
                member["path"] = "agents/implementation"
        path.write_text(json.dumps(definition))
        planned = self.command("orchestrator/session.py", "coder_golang", self.task, "--dry-run")
        plan = json.loads(planned.stdout)
        self.assertIn(str(relocated / "agent.md"), plan["instructions"])
        self.assertIn('"path": "agents/implementation"', plan["instructions"])
        definition["agents"] = [member for member in definition["agents"] if member["name"] != "architect"]
        path.write_text(json.dumps(definition))
        rejected = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", expected=1)
        self.assertIn("in factory golang, found 0", rejected.stderr)

    def test_invalid_factory_membership_fails_before_task_creation(self):
        path = self.toolbox / "factories/golang/factory.json"
        original = json.loads(path.read_text())
        for change, message in [
            (lambda d: d["agents"].append(d["agents"][0].copy()), "duplicate agent name"),
            (lambda d: d["agents"][0].update(path="../../../outside"), "stay inside the toolkit"),
            (lambda d: d["agents"][0].update(path="missing"), "bundle is missing"),
            (lambda d: d.update(owner="missing"), "owner must name a member"),
            (lambda d: d.update(name="wrong"), "name must match"),
            (lambda d: d["agents"][0].update(config=[]), "config must be an object"),
            (lambda d: d["agents"][0].update(config={"model": ""}), "config.model must be"),
            (lambda d: d["agents"][0].update(config={"modle": "typo"}), "unsupported agent config fields"),
            (lambda d: d["agents"][0].update(config={"pi": {"models": []}}), "config.pi accepts"),
        ]:
            with self.subTest(message=message):
                definition = json.loads(json.dumps(original))
                change(definition)
                path.write_text(json.dumps(definition))
                result = self.task_command("new", "--factory", "golang", "--name", "Invalid", "--brief", "Invalid", expected=1)
                self.assertIn(message, result.stderr)
        path.write_text(json.dumps(original))
        tasks = list((self.toolbox / "tasks").glob("*/task.json"))
        self.assertEqual(len(tasks), 1)

    def test_factory_configuration_selects_task_owner(self):
        path = self.toolbox / "factories/writing/factory.json"
        definition = json.loads(path.read_text())
        definition["owner"] = "coordinator"
        path.write_text(json.dumps(definition))
        task = self.create_task(factory="writing", name="Configured owner")
        self.assertEqual(json.loads((task / "task.json").read_text())["owner"], "coordinator")

    def test_extension_installer_uses_factory_selection_and_private_files(self):
        executable = self.project / "package-fixture.py"
        capture = self.project / "packages.json"
        executable.write_text(
            f"#!{sys.executable}\nimport json, os, pathlib, sys\n"
            "profile = pathlib.Path(os.environ['PI_CODING_AGENT_DIR'])\n"
            f"pathlib.Path({str(capture)!r}).write_text(json.dumps({{'args': sys.argv[1:], 'path': str(profile), 'settings': json.loads((profile / 'settings.json').read_text())}}))\n"
        )
        executable.chmod(0o755)
        self.configure("coder_golang", {"executable": str(executable)})
        self.command("orchestrator/scripts/install_extensions.py", "golang", "coder_golang")
        recorded = json.loads(capture.read_text())
        self.assertEqual(recorded["args"], ["update", "--extensions", "--no-approve"])
        self.assertTrue(Path(recorded["path"]).is_relative_to(self.runtime))
        self.assertIn("edxeth/pi-ralph-loop", recorded["settings"]["packages"][0]["source"])

    def test_defaults_merge_objects_but_replace_lists(self):
        self.define("defaults", "pi", {"settings": {"retry": {"enabled": True, "maxRetries": 4}}})
        self.configure("architect", {"pi": {"settings": {"retry": {"enabled": False}}}, "tools": ["read"]})
        capture, env = self.fake_pi()
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        settings = json.loads(capture.read_text())["settings"]
        self.assertEqual(settings["retry"], {"enabled": False, "maxRetries": 4})
        self.assertEqual(settings["defaultTools"], ["read"])

    def test_selected_resources_and_defaults_generate_isolated_native_files(self):
        capture, env = self.fake_pi()
        self.define("providers", "unused", {"apiKey": "${MISSING_UNUSED_KEY}"})
        self.command("orchestrator/session.py", "coder_golang", self.task, env=env)
        coder = Path(json.loads(capture.read_text())["private_config"])
        self.assertEqual(set(json.loads((coder / "mcp.json").read_text())["mcpServers"]), {"comms", "gopls"})
        self.assertEqual(len(json.loads((coder / "settings.json").read_text())["packages"]), 1)
        self.assertNotIn("unused", json.loads((coder / "models.json").read_text())["providers"])
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        architect = Path(json.loads(capture.read_text())["private_config"])
        self.assertNotEqual(architect, coder)
        self.assertEqual(set(json.loads((architect / "mcp.json").read_text())["mcpServers"]), {"comms"})
        self.assertEqual(json.loads((architect / "settings.json").read_text())["packages"], [])
        self.assertEqual(json.loads((architect / "settings.json").read_text())["defaultThinkingLevel"], "high")

    def test_stale_agent_local_files_cannot_override_factory(self):
        bundle = self.toolbox / "resources/agents/architect"
        (bundle / "config.json").write_text('{"model":"wrong"}')
        (bundle / "config.local.json").write_text('{"model":"wrong"}')
        (bundle / "pi").mkdir(exist_ok=True)
        (bundle / "pi/settings.json").write_text('{"defaultModel":"wrong"}')
        plan = json.loads(self.command("orchestrator/session.py", "architect", self.task, "--dry-run").stdout)
        self.assertEqual(plan["runner_configuration"]["model"], "test-model")

    def test_unknown_resource_names_and_missing_comms_fail_before_launch(self):
        for field in ("mcps", "extensions"):
            self.configure("architect", **{field: ["missing"]})
            result = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", expected=1)
            self.assertIn("unknown " + field, result.stderr)
            self.configure("architect", **{field: []})
        result = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", expected=1)
        self.assertIn("require the comms", result.stderr)

    def test_factory_overrides_generate_private_native_profiles(self):
        self.define("providers", "another-router", {"apiKey": "${TEAM_GATEWAY_KEY}"})
        self.configure("architect", {"provider": "another-router", "model": "architect-model", "thinking": "low", "tools": ["read"], "env": {"TEAM_GATEWAY_KEY": "FACTORY_FIXTURE_KEY"}, "pi": {"settings": {"retry": {"enabled": False}}}}, skills=["../../orchestrator/comms/skills/task-comms"])
        capture, env = self.fake_pi()
        env["FACTORY_FIXTURE_KEY"] = "fixture-secret"
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        launched = json.loads(capture.read_text())
        self.assertEqual(launched["settings"]["defaultModel"], "architect-model")
        self.assertEqual(launched["settings"]["defaultProvider"], "another-router")
        self.assertEqual(launched["settings"]["defaultTools"], ["read"])
        self.assertFalse(launched["settings"]["retry"]["enabled"])
        private = Path(launched["private_config"])
        self.assertEqual(json.loads((private / "models.json").read_text())["providers"]["another-router"]["apiKey"], "${TEAM_GATEWAY_KEY}")
        coder = json.loads(self.command("orchestrator/session.py", "coder_golang", self.task, "--dry-run", env=env).stdout)
        self.assertEqual(coder["runner_configuration"]["model"], "test-model")
        self.assertNotIn(env["FACTORY_FIXTURE_KEY"], launched["instructions"])
        rejected = self.command("orchestrator/session.py", "architect", self.task, "--factory", "writing", "--dry-run", env=env, expected=1)
        self.assertIn("conflicts with the task", rejected.stderr)

    def test_generated_instructions_identify_document_sources(self):
        root = self.toolbox
        self.command("orchestrator/session.py", "coordinator", "--dry-run", expected=1)
        selected = json.loads(self.command("orchestrator/session.py", "coordinator", "--factory", "golang", "--dry-run").stdout)
        text = selected["instructions"]
        self.assertIn(f"Toolkit root: {root}", text)
        self.assertIn(f"Source file: {root / 'orchestrator/SKILL.md'}", text)
        self.assertIn(f"Source file: {root / 'factories/golang/README.md'}", text)
        self.assertIn(f"Source directory for relative references: {root / 'factories/golang'}", text)

    def test_loop_waits_for_actual_participant_assignment(self):
        sys.path.insert(0, str(SOURCE / "orchestrator"))
        spec = importlib.util.spec_from_file_location("loop_launch_fixture", SOURCE / "orchestrator/session.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        client = Mock()
        client.call.side_effect = [{"status": "working"}, {"assignments": []}, {"status": "working"}, {"assignments": [{"state": "offered"}]}]
        with patch.object(module.time, "sleep"):
            module.wait_for_assignment(client, "task-id", "actual-participant", 10)
        self.assertEqual(client.call.call_count, 4)
        client.call.assert_called_with("list_assignments", task_id="task-id", assignee_id="actual-participant", after="", limit=200)
        client.call.side_effect = [{"status": "working"}, {"assignments": [{"state": "submitted"}]}]
        with self.assertRaisesRegex(ValueError, "no active assignment"):
            module.wait_for_assignment(client, "task-id", "actual-participant", 0)

    def test_loop_finds_later_assignments_and_stops_on_cancellation(self):
        sys.path.insert(0, str(SOURCE / "orchestrator"))
        spec = importlib.util.spec_from_file_location("loop_paging_fixture", SOURCE / "orchestrator/session.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        client = Mock()
        first_page = [{"id": str(i), "state": "submitted"} for i in range(200)]
        client.call.side_effect = [{"status": "working"}, {"assignments": first_page},
                                   {"assignments": [{"id": "200", "state": "offered"}]}]
        module.wait_for_assignment(client, "task-id", "participant", 0)
        client.call.assert_called_with("list_assignments", task_id="task-id", assignee_id="participant", after="199", limit=200)
        client.reset_mock()
        client.call.side_effect = [{"status": "cancelled"}]
        with self.assertRaisesRegex(ValueError, "cancelled"):
            module.wait_for_assignment(client, "task-id", "participant", 10)
        client.call.assert_called_once_with("get_task", task_id="task-id")

    def test_factory_native_secrets_are_not_copied_into_model_instructions(self):
        self.define("providers", "fixture", {"apiKey": "SECRET-PROVIDER-SENTINEL"})
        self.define("mcps", "private", {"url": "http://example.invalid/mcp", "headers": {"Authorization": "SECRET-MCP-SENTINEL"}})
        dry = self.command("orchestrator/session.py", "architect", self.task, "--dry-run")
        self.assertNotIn("SECRET-", dry.stdout)
        capture, env = self.fake_pi()
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        record = json.loads(capture.read_text())
        self.assertNotIn("SECRET-", record["instructions"])
        native = Path(record["private_config"])
        self.assertEqual(json.loads((native / "models.json").read_text())["providers"]["fixture"]["apiKey"], "SECRET-PROVIDER-SENTINEL")

    def test_malformed_task_and_mcp_config_report_errors_without_tracebacks(self):
        task_path = self.task / "task.json"
        original = task_path.read_text()
        for value in ([], {"schema_version": 2}, {**json.loads(original), "brief": None}):
            task_path.write_text(json.dumps(value))
            result = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", expected=1)
            self.assertIn("task.json", result.stderr)
            self.assertNotIn("Traceback", result.stderr)
        task_path.write_text(original)
        for value in ([], None, {"headers": []}, {"cwd": 42}):
            self.define("mcps", "comms", value)
            result = self.command("orchestrator/session.py", "architect", self.task, "--dry-run", expected=1)
            self.assertTrue("MCP server comms" in result.stderr or "entries must be objects" in result.stderr)
            self.assertNotIn("Traceback", result.stderr)

    def test_task_launch_registers_real_instance_and_refuses_silent_replacement(self):
        capture, env = self.fake_pi()
        self.command("orchestrator/session.py", "architect", self.task, "--check", env=env)
        sys.path.insert(0, str(self.toolbox / "orchestrator/comms"))
        from client import Client
        client = Client(self.env)
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        self.assertEqual(client.call("list_participants", task_id=task_id)["participants"], [])
        self.command("orchestrator/session.py", "architect", self.task, env=env)
        record = json.loads(capture.read_text())
        participants = client.call("list_participants", task_id=task_id)["participants"]
        self.assertEqual(len(participants), 1)
        first = participants[0]
        self.assertEqual(first["instance_id"], Path(record["private_config"]).parent.name)
        self.assertIn("Participant ID: " + first["id"], record["instructions"])
        failed = self.command("orchestrator/session.py", "architect", self.task, expected=1, env=env)
        self.assertIn("already registered", failed.stderr)
        participants = client.call("list_participants", task_id=task_id)["participants"]
        self.assertEqual(len(participants), 1)
        self.assertNotIn("retired_at", participants[0])
        self.command("orchestrator/session.py", "architect", self.task, "--replace-participant", first["id"], env=env)
        participants = client.call("list_participants", task_id=task_id)["participants"]
        self.assertEqual(len(participants), 2)
        self.assertIsNotNone(participants[0]["retired_at"])
        self.assertNotIn("retired_at", participants[1])

    def test_role_start_scripts_load_config_skills_and_unassigned_context(self):
        capture, env = self.fake_pi()
        # A new unassigned session must not inherit another session's task identity.
        env.update(FACTORY_TASK_ID="stale-task", FACTORY_TASK_DIR="/wrong/task", FACTORY_ROLE="wrong-role")
        for role in ("coordinator", "architect", "architecture_reviewer", "coder_golang", "code_reviewer_golang", "pair_writer"):
            directory = self.toolbox / "orchestrator" if role == "coordinator" else next(path for path in [self.toolbox / "resources/agents" / role, *(self.toolbox / "factories").glob(f"*/agents/{role}")] if path.is_dir())
            factory = "writing" if role == "pair_writer" else "golang"
            self.configure(role, {"model": f"{role}-model", "thinking": "high"}, factory=factory)
            script = directory / "start.sh"
            planned = subprocess.run([str(script), "--factory", factory, "--dry-run"], cwd=self.project.parent,
                                     env=env, text=True, capture_output=True, timeout=15)
            self.assertEqual(planned.returncode, 0, planned.stderr)
            plan = json.loads(planned.stdout)
            self.assertEqual(plan["role"], role)
            self.assertIsNone(plan["task"])
            self.assertEqual(plan["runner_configuration"]["model"], f"{role}-model")
            self.assertIn(str(self.toolbox / "orchestrator/comms/skills/task-comms/SKILL.md"), plan["skills"])
            self.assertIn("No task is selected", plan["instructions"])
            self.assertNotIn("stale-task", plan["instructions"])
            self.assertFalse(self.runtime.exists())
        # Exercise exec and the prepared native profile, not only shell syntax.
        script = self.toolbox / "resources/agents/architect/start.sh"
        started = subprocess.run([str(script), "--factory", "golang"], cwd=self.project.parent, env=env,
                                 text=True, capture_output=True, timeout=15)
        self.assertEqual(started.returncode, 0, started.stdout + started.stderr)
        record = json.loads(capture.read_text())
        self.assertEqual(record["cwd"], str(self.project))
        self.assertEqual(record["settings"]["defaultModel"], "architect-model")
        self.assertEqual(record["identity"], {"role": "architect", "project": str(self.project), "task_id": None, "task_dir": None})
        self.assertIn("task-comms", record["instructions"])
        loop = subprocess.run([str(self.toolbox / "factories/golang/agents/coder_golang/start.sh"), "--factory", "golang", "--loop", "2", "--dry-run"],
                              cwd=self.project, env=env, text=True, capture_output=True, timeout=15)
        self.assertNotEqual(loop.returncode, 0)
        self.assertIn("requires an explicit task", loop.stderr)

    def test_start_script_forwards_task_and_rejects_missing_skill(self):
        capture, env = self.fake_pi()
        script = self.toolbox / "resources/agents/architect/start.sh"
        started = subprocess.run([str(script), str(self.task)], cwd=self.project.parent,
                                 env=env, text=True, capture_output=True, timeout=15)
        self.assertEqual(started.returncode, 0, started.stderr)
        record = json.loads(capture.read_text())
        task_id = json.loads((self.task / "task.json").read_text())["comms_id"]
        self.assertEqual(record["identity"]["task_id"], task_id)
        self.assertEqual(record["identity"]["task_dir"], str(self.task))
        capture.unlink()
        (self.toolbox / "orchestrator/comms/skills/task-comms/SKILL.md").unlink()
        broken = subprocess.run([str(script), str(self.task)], cwd=self.project, env=env,
                                text=True, capture_output=True, timeout=15)
        self.assertNotEqual(broken.returncode, 0)
        self.assertIn("skill must exist", broken.stderr)
        self.assertFalse(capture.exists())



if __name__ == "__main__":
    unittest.main()
