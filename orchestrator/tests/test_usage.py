import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec=importlib.util.spec_from_file_location("pi_usage",Path(__file__).resolve().parents[1]/"adapters/pi/import_usage.py")
module=importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class UsageTest(unittest.TestCase):
    def test_cache_is_input_reasoning_not_added_and_cost_is_unknown(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/"session.jsonl"
            rows=[{"type":"session","version":3,"id":"session"},{"type":"message","id":"reply","message":{"role":"assistant","provider":"fixture","model":"model","stopReason":"stop","usage":{"input":10,"output":4,"cacheRead":6,"cacheWrite":2,"totalTokens":22,"reasoning":3,"cost":{"total":1.23}}}}]
            path.write_text("".join(json.dumps(row)+"\n" for row in rows))
            identity,usage=list(module.measurements(path))[0]
            self.assertEqual(identity,"session:reply")
            self.assertEqual(usage["input_tokens"],18)
            self.assertEqual(usage["output_tokens"],4)
            self.assertNotIn("cost_usd",usage)
            rows[1]["message"]["usage"]["totalTokens"]=999
            path.write_text("".join(json.dumps(row)+"\n" for row in rows))
            with self.assertRaisesRegex(ValueError,"totals disagree"):list(module.measurements(path))
    def test_partial_file_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/"session.jsonl"
            path.write_text('{"type":"session","version":3,"id":"session"}\n{"type":')
            with self.assertRaisesRegex(ValueError,"partial"):list(module.measurements(path))
