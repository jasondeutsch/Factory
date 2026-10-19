#!/usr/bin/env python3
"""Install one member's declared Pi extensions into a private generated profile."""
import argparse
import os
from pathlib import Path
import subprocess
import sys
import uuid

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from session import configuration
from adapters.pi import profile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("factory")
    parser.add_argument("agent")
    args = parser.parse_args()
    try:
        directory, config = configuration(args.agent, args.factory)
        if config.get("runner", "pi") != "pi":
            raise ValueError("extension installation is only supported for Pi members")
        runtime = Path(os.environ.get("FACTORY_RUNTIME_DIR", str(Path.home() / ".factory/runtime"))).expanduser()
        if not runtime.is_absolute():
            raise ValueError("FACTORY_RUNTIME_DIR must be absolute")
        os.umask(0o077)
        environment = {**os.environ, "FACTORY_COMMS_URL": os.environ.get("FACTORY_COMMS_URL", "http://127.0.0.1:8090/mcp")}
        prepared = profile.load(directory, config, Path.cwd(), runtime / "dependencies" / args.factory / args.agent / uuid.uuid4().hex, environment)
        profile.materialize(prepared)
        print(f"Factory: installing declared extensions in {prepared['target']}", file=sys.stderr)
        return subprocess.call([config.get("executable", "pi"), "update", "--extensions", "--no-approve"], env=environment)
    except (ValueError, OSError) as error:
        print(f"factory extensions: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
