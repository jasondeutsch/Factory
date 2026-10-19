"""Coordinator startup boundaries; no real services or model requests."""
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location("startup", Path(__file__).parents[1] / "startup.py")
startup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(startup)


class StartupTests(unittest.TestCase):
    def test_external_endpoint_never_starts_compose(self):
        with patch.dict(os.environ, {"FACTORY_COMMS_URL": "https://comms.example/mcp"}), patch.object(startup, "Client") as client, patch.object(startup, "compose_command") as compose:
            self.assertEqual(startup.ensure_comms(), "external endpoint connected")
            client.assert_called_once()
            compose.assert_not_called()

    def test_local_compose_waits_for_mcp_and_respects_port(self):
        with patch.dict(os.environ, {"COMMS_PORT": "8123"}, clear=True), patch.object(startup, "Client", side_effect=[ValueError("starting"), Mock()]) as client, patch.object(startup, "compose_command", return_value=["docker-compose"]), patch.object(startup.subprocess, "run") as run, patch.object(startup.time, "sleep"):
            startup.ensure_comms()
            self.assertEqual(os.environ["FACTORY_COMMS_URL"], "http://127.0.0.1:8123/mcp")
            self.assertEqual(client.call_count, 2)
            self.assertEqual(run.call_args.args[0][-3:], ["up", "--build", "-d"])

    def test_check_modes_do_not_start_services(self):
        for flag in ("--check", "--dry-run", "--check-factory", "--help"):
            with self.subTest(flag=flag), patch.object(startup.subprocess, "call", return_value=0) as call, patch.object(startup, "ensure_comms") as comms:
                self.assertEqual(startup.main(["--factory", "golang", flag]), 0)
                comms.assert_not_called()
                self.assertEqual("coordinator" in call.call_args.args[0], flag != "--check-factory")

    def test_invalid_configuration_prevents_service_startup(self):
        with patch.object(startup, "installed_project", return_value=Path("/project")), patch.object(startup.subprocess, "run", side_effect=startup.subprocess.CalledProcessError(1, "fixture")), patch.object(startup, "ensure_comms") as comms:
            self.assertEqual(startup.main(["--factory", "missing"]), 1)
            comms.assert_not_called()

    def test_bad_external_endpoint_does_not_fall_back_to_local_service(self):
        with patch.dict(os.environ, {"FACTORY_COMMS_URL": "https://comms.example/mcp"}), patch.object(startup, "Client", side_effect=ValueError("unauthorized")), patch.object(startup, "compose_command") as compose:
            with self.assertRaisesRegex(ValueError, "unauthorized"):
                startup.ensure_comms()
            compose.assert_not_called()

    def test_bridge_failure_stops_coordinator(self):
        bridge = Mock()
        bridge.poll.return_value = 1
        coordinator = Mock()
        coordinator.poll.return_value = None
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {"HERDR_ENV": "1"}), patch.object(startup.shutil, "which", return_value="/fixture/herdr"), patch.object(startup.subprocess, "run"), patch.object(startup.subprocess, "Popen", side_effect=[bridge, coordinator]):
            with self.assertRaisesRegex(ValueError, "delivery bridge stopped"):
                startup.run_coordinator(["fake-agent"], Path(directory), Path(directory))
            coordinator.terminate.assert_called_once()

    def test_normal_exit_stops_owned_bridge(self):
        bridge = Mock()
        bridge.poll.return_value = None
        coordinator = Mock(returncode=0)
        coordinator.poll.return_value = 0
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {"HERDR_ENV": "1"}), patch.object(startup.shutil, "which", return_value="/fixture/herdr"), patch.object(startup.subprocess, "run"), patch.object(startup.subprocess, "Popen", side_effect=[bridge, coordinator]):
            self.assertEqual(startup.run_coordinator(["fake-agent"], Path(directory), Path(directory)), 0)
            bridge.terminate.assert_called_once()

    def test_outside_herdr_does_not_start_bridge(self):
        coordinator = Mock(returncode=0)
        coordinator.poll.return_value = 0
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {}, clear=True), patch.object(startup.subprocess, "Popen", return_value=coordinator) as launch, patch.object(startup.subprocess, "run") as build:
            self.assertEqual(startup.run_coordinator(["fake-agent"], Path(directory), Path(directory)), 0)
            launch.assert_called_once_with(["fake-agent"])
            build.assert_not_called()
