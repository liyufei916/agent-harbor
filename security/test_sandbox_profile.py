"""可移植断言：生产 sandbox.sb 与探测脚本不得 fail-open。"""

import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SANDBOX_SB = ROOT / "internal" / "runtime" / "sandbox.sb"
PROBE = ROOT / "security" / "seatbelt_probe.py"


class SandboxProfileTest(unittest.TestCase):
    def test_production_profile_deny_default(self):
        text = SANDBOX_SB.read_text(encoding="utf-8")
        self.assertIn("(deny default)", text)
        self.assertNotIn("(allow default)", text)
        self.assertNotIn('import "system.sb"', text)
        self.assertIn("{{.ProcessFork}}", text)
        self.assertIn("{{.Rules}}", text)

    def test_probe_is_diagnostic_only(self):
        text = PROBE.read_text(encoding="utf-8")
        self.assertIn("不是生产 Runtime", text)
        self.assertIn("system.sb", text)


if __name__ == "__main__":
    unittest.main()
