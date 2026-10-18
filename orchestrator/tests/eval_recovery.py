#!/usr/bin/env python3
"""Produce an automated coordination recovery report, without real model work."""
import argparse
from datetime import datetime, timezone
import html
import io
import json
from pathlib import Path
import platform
import time
import unittest

SCENARIOS = {
    "dispatch_retry_creates_one_pane_assignment_and_notification": "One pane, one assignment, one notification across retries",
    "dispatch_uncertain_launch_reconciles_actual_participant": "Recover a lost launch response through the actual registered instance",
    "dispatch_uncertain_notification_and_failed_start_are_not_repeated": "Uncertain notification stays visible without duplicate input",
    "dispatch_uncertain_split_requires_explicit_recovery": "Lost split response requires explicit recovery of the actual pane",
    "dispatch_startup_failure_never_assigns_or_relaunches": "Failed startup creates no assignment and is not silently relaunched",
    "attachment_failure_concurrency_and_retry_preserve_documents": "Failed and concurrent attachment preserves a single complete artifact directory",
    "launcher_records_immutable_public_snapshot": "Loaded configuration remains inspectable after source changes",
    "factory_check_reports_all_members_without_registering_or_chat": "Factory check reports member failures without starting model work",
    "loop_rechecks_documents_after_wait_and_records_registered_failure": "Changed approval documents stop the initial autonomous iteration",
}

class Results(unittest.TextTestResult):
    def startTest(self, test):
        self.started = time.monotonic()
        super().startTest(test)

    def stopTest(self, test):
        failures = {case.id() for case, _ in self.failures + self.errors}
        skipped = {case.id() for case, _ in self.skipped}
        self.measurements.append({"test": test.id(), "status": "failed" if test.id() in failures else "skipped" if test.id() in skipped else "passed",
                                  "seconds": round(time.monotonic() - self.started, 3)})
        super().stopTest(test)

    def __init__(self, *args):
        super().__init__(*args)
        self.measurements = []


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="directory for JSON and HTML evidence")
    args = parser.parse_args()
    names = ["test_toolbox.ToolboxTest.test_" + name for name in SCENARIOS]
    suite = unittest.defaultTestLoader.loadTestsFromNames(names)
    log = io.StringIO()
    start = time.monotonic()
    result = unittest.TextTestRunner(stream=log, verbosity=2, resultclass=Results).run(suite)
    report = {"generated_at": datetime.now(timezone.utc).isoformat(), "environment": platform.platform(),
              "python": platform.python_version(), "mode": "real isolated comms service; fixture Pi and Herdr; no model inference",
              "seconds": round(time.monotonic() - start, 3), "successful": result.wasSuccessful(),
              "scenarios": result.measurements, "live_herdr": "not tested", "model_usage": "not applicable",
              "limitations": ["Does not evaluate real idle-pane wake-up, reasoning, product correctness, or human approval handling.",
                              "Separate Go tests cover database restarts, delivery leases, instance replacement and contact expiry."]}
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / "report.json").write_text(json.dumps(report, indent=2) + "\n")
    (args.output / "tests.txt").write_text(log.getvalue())
    rows = "".join(f"<tr><td>{html.escape(SCENARIOS.get(item['test'].split('test_')[-1], item['test']))}</td><td>{item['status']}</td><td>{item['seconds']}</td></tr>" for item in result.measurements)
    (args.output / "report.html").write_text(f'''<!doctype html><html lang="en"><meta charset="utf-8"><title>Factory recovery eval</title>
<style>body{{font:16px system-ui;max-width:1000px;margin:3rem auto;padding:1rem;color:#182635}}table{{border-collapse:collapse;width:100%}}td,th{{padding:.7rem;text-align:left;border-bottom:1px solid #ccd4dc}}small{{color:#52616e}}</style>
<h1>Coordination recovery eval</h1><p>{len(result.measurements)} automated scenarios · {report['seconds']} seconds · {'passed' if result.wasSuccessful() else 'failed'}</p>
<p>Real isolated MCP service with fixture agents and Herdr. No model inference or product testing.</p><table><thead><tr><th>Scenario</th><th>Result</th><th>Seconds</th></tr></thead><tbody>{rows}</tbody></table>
<h2>Verification still required</h2><p>Live Herdr wake-up, agent behavior, operator decisions and interruption recovery. Automated results do not establish these outcomes.</p><small>{html.escape(report['generated_at'])}</small></html>''')
    print(json.dumps(report, indent=2))
    return 0 if result.wasSuccessful() else 1

if __name__ == "__main__":
    raise SystemExit(main())
