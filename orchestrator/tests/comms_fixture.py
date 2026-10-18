"""Run the real comms service with an isolated disposable database for tests."""
from __future__ import annotations

import os
import socket
import subprocess
import tempfile
import time
from pathlib import Path
from urllib.request import urlopen

_build = None
_binary = None


def server_binary():
    global _build, _binary
    if _binary is not None:
        return _binary
    configured = os.environ.get("FACTORY_TEST_COMMS")
    if configured:
        _binary = Path(configured).resolve()
    else:
        _build = tempfile.TemporaryDirectory(prefix="factory-comms-tests-")
        _binary = Path(_build.name) / "comms"
        subprocess.run(["go", "build", "-o", str(_binary), "./cmd/comms"],
                       cwd=Path(__file__).resolve().parents[2] / "orchestrator" / "comms", check=True, timeout=120)
    return _binary


class CommsService:
    def __enter__(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="factory-comms-db-")
        self.directory = Path(self.temporary.name)
        with socket.socket() as reservation:
            reservation.bind(("127.0.0.1", 0))
            self.port = reservation.getsockname()[1]
        self.url = f"http://127.0.0.1:{self.port}"
        self.environment = {"FACTORY_COMMS_URL": self.url + "/mcp", "COMMS_AUTH_TOKEN": "local-comms-fixture-token"}
        self.log = (self.directory / "server.log").open("w+")
        self.process = subprocess.Popen([str(server_binary()), "-addr", f"127.0.0.1:{self.port}",
                                         "-db", str(self.directory / "state.db")],
                                        env={**os.environ, **self.environment}, stdout=self.log, stderr=self.log)
        try:
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                try:
                    with urlopen(self.url + "/healthz", timeout=0.2) as response:
                        if response.status == 200:
                            return self
                except OSError:
                    if self.process.poll() is not None:
                        break
                    time.sleep(0.02)
            raise RuntimeError("comms fixture did not start")
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def __exit__(self, *args):
        self.process.terminate()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(timeout=5)
        self.log.close()
        self.temporary.cleanup()


def bridge_binary():
    server = server_binary()
    binary = server.parent / "bridge"
    if not binary.exists():
        subprocess.run(["go", "build", "-o", str(binary), "./cmd/bridge"],
                       cwd=Path(__file__).resolve().parents[1] / "comms", check=True, timeout=120)
    return binary
