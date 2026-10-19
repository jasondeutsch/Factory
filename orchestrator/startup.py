#!/usr/bin/env python3
"""Start coordinator dependencies and supervise its host delivery bridge."""
from __future__ import annotations

import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent / "scripts"))
from factorylib import Client, ROOT, installed_project


def compose_command():
    if shutil.which("docker") and subprocess.run(
        ["docker", "compose", "version"], stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL, timeout=10,
    ).returncode == 0:
        return ["docker", "compose"]
    if shutil.which("docker-compose"):
        return ["docker-compose"]
    raise ValueError("install Docker Compose or set FACTORY_COMMS_URL to an existing service")


def ensure_comms():
    # Explicit endpoints belong to the operator, including local/custom ports.
    if os.environ.get("FACTORY_COMMS_URL"):
        Client()
        return "external endpoint connected"
    port = os.environ.get("COMMS_PORT", "8090")
    if not port.isdigit() or not 1 <= int(port) <= 65535:
        raise ValueError("COMMS_PORT must be a port number")
    os.environ["FACTORY_COMMS_URL"] = f"http://127.0.0.1:{port}/mcp"
    command = [*compose_command(), "-f", str(ROOT / "orchestrator/comms/compose.yaml"), "up", "--build", "-d"]
    subprocess.run(command, check=True)
    deadline = time.monotonic() + 60
    while True:
        try:
            Client()
            return "local Compose service connected"
        except ValueError:
            if time.monotonic() >= deadline:
                raise ValueError("comms did not become ready; inspect Compose logs and COMMS_AUTH_TOKEN") from None
            time.sleep(1)


def stop_process(process):
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


def run_coordinator(command, project, runtime):
    bridge = None
    coordinator = None
    # Build an owned executable instead of supervising `go run`, whose child
    # could outlive it. The temporary directory and bridge die with this launch.
    with tempfile.TemporaryDirectory(prefix="coordinator-", dir=runtime) as directory:
        log_path = runtime / (Path(directory).name + "-delivery.log")
        with log_path.open("w") as log:
            try:
                if os.environ.get("HERDR_ENV") == "1":
                    if not shutil.which("herdr"):
                        raise ValueError("Herdr delivery requires herdr on PATH")
                    binary = str(Path(directory) / "bridge")
                    subprocess.run(["go", "build", "-o", binary, "./cmd/bridge"],
                                   cwd=ROOT / "orchestrator/comms", check=True)
                    bridge = subprocess.Popen([binary, "--project", str(project)], stdout=log, stderr=log)
                    print(f"Factory: delivery bridge started; diagnostics: {log_path}", file=sys.stderr)
                else:
                    print("Factory: outside Herdr; automatic pane notifications unavailable. Use comms inbox polling.", file=sys.stderr)
                print("Factory: task identity and pane binding remain pending until a task is selected and its actual session is registered/bound.", file=sys.stderr)
                coordinator = subprocess.Popen(command)
                while coordinator.poll() is None:
                    if bridge is not None and bridge.poll() is not None:
                        raise ValueError(f"delivery bridge stopped; inspect {log_path}; coordinator stopped to avoid silent notification loss")
                    time.sleep(0.2)
                return coordinator.returncode
            finally:
                stop_process(coordinator)
                stop_process(bridge)


def main(argv=None):
    arguments = sys.argv[1:] if argv is None else argv
    command = [sys.executable, str(ROOT / "orchestrator/session.py"), "coordinator", *arguments]
    # Existing inspection commands retain their behavior, including no writes
    # or connections for --dry-run. Factory checking has no positional role.
    if "--check-factory" in arguments:
        command.remove("coordinator")
    if any(flag in arguments for flag in ("--dry-run", "--check", "--check-factory", "--help", "-h")):
        return subprocess.call(command)
    try:
        project = installed_project()
        # Validate arguments, selected factory, task and configuration before
        # building images or launching any service. Native checks run later.
        subprocess.run([*command, "--dry-run"], stdout=subprocess.DEVNULL, check=True)
        os.umask(0o077)
        runtime = Path(os.environ.get("FACTORY_RUNTIME_DIR", str(Path.home() / ".factory/runtime"))).expanduser()
        if not runtime.is_absolute():
            raise ValueError("FACTORY_RUNTIME_DIR must be absolute")
        runtime.mkdir(parents=True, exist_ok=True)
        print(f"Factory: {ensure_comms()}", file=sys.stderr)
        return run_coordinator(command, project, runtime)
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f"factory startup: {error}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(143))
    sys.exit(main())
