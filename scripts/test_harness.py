"""Behavior tests for the runner's failure reporting and unborn Git support."""
from contextlib import redirect_stdout
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import harness


class HarnessTests(unittest.TestCase):
    def test_unborn_repository_includes_untracked_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            (root / "new.txt").write_text("user work\n")
            snapshot = harness.git_snapshot(root)
            self.assertIsNone(snapshot["head"])
            self.assertIn("?? new.txt", snapshot["status"])

    def run_command(self, command, require_empty=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = io.StringIO()
            with redirect_stdout(output):
                result = harness.run_step(root, root, ("probe", command, ".", require_empty, {}))
            log = (root / "probe.log").read_text()
            return result, output.getvalue(), log

    def test_failure_preserves_log_and_limits_console_output(self):
        command = [sys.executable, "-c", "print(*range(100), sep=chr(10)); raise SystemExit(7)"]
        result, output, log = self.run_command(command)
        self.assertFalse(result["passed"])
        self.assertEqual(result["exit_code"], 7)
        self.assertEqual(len(log.splitlines()), 100)
        self.assertLess(len(output.splitlines()), 45)
        self.assertIn("99", output)

    def test_formatter_output_is_failure_even_with_zero_exit(self):
        result, _, _ = self.run_command([sys.executable, "-c", "print('unformatted.go')"], True)
        self.assertFalse(result["passed"])

    def test_missing_tool_is_not_a_pass(self):
        result, output, _ = self.run_command(["/definitely-missing-avari-tool"])
        self.assertFalse(result["passed"])
        self.assertIn("FAIL", output)

    def test_success_keeps_verbose_output_out_of_context(self):
        result, output, log = self.run_command([sys.executable, "-c", "print('verbose details')"])
        self.assertTrue(result["passed"])
        self.assertNotIn("verbose details", output)
        self.assertIn("verbose details", log)

    @unittest.skipUnless(os.name == "posix", "Process groups are a macOS/Linux guarantee")
    def test_timeout_stops_child_holding_a_lock_and_fails(self):
        import fcntl

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            child = [sys.executable, "-c", "import fcntl,time; from pathlib import Path; "
                     "lock=open('held.lock','w'); fcntl.flock(lock,fcntl.LOCK_EX); "
                     "Path('ready').touch(); time.sleep(60)"]
            command = [sys.executable, "-c", f"import subprocess,time; subprocess.Popen({child!r}); time.sleep(60)"]
            with redirect_stdout(io.StringIO()):
                result = harness.run_step(root, root, ("timeout", command, ".", False, {}), timeout=2)
            self.assertFalse(result["passed"])
            self.assertEqual(result["exit_code"], 124)
            self.assertTrue((root / "ready").exists(), "Child must acquire the lock before timeout")
            with (root / "held.lock").open() as lock:
                # This fails if only the immediate parent was stopped.
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)


if __name__ == "__main__":
    unittest.main()
