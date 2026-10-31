import unittest
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).parent))
from filter_coverage import filter_profile

class FilterCoverageTest(unittest.TestCase):
    def test_keeps_mode_and_core(self):
        raw = "mode: set\n" "github.com/x/core.go:1.1,1.2 1 1\n" "github.com/x/foo_docker.go:1.1,1.2 1 0\n" "github.com/x/internal/gate/connections.go:1.1,1.2 1 0\n"
        self.assertEqual(filter_profile(raw), "mode: set\ngithub.com/x/core.go:1.1,1.2 1 1\n")

    def test_empty_profile(self):
        self.assertEqual(filter_profile(""), "")

if __name__ == "__main__":
    unittest.main()
