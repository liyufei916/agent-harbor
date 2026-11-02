"""会话诊断的目录发现测试，不访问真实凭据或模型。"""
import importlib.util
import json
import pathlib
import tempfile
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location(
    "probe_model_tools", pathlib.Path(__file__).with_name("probe-model-tools.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class GateDiscoveryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.home = pathlib.Path(self.tmp.name)
        self.roots = self.home / "Library/Application Support/AgentHarbor/sessions"
        self.patch = patch.object(pathlib.Path, "home", return_value=self.home)
        self.patch.start()
        self.addCleanup(self.patch.stop)

    def fixture(self, name):
        folder = self.roots / name
        (folder / "home").mkdir(parents=True)
        (folder / "sandbox.sb").write_text(
            '(allow network-outbound (remote tcp4 "localhost:50123"))\n'
            '(allow network-outbound (remote tcp4 "localhost:50124"))')
        (folder / "home/harbor-mcp.json").write_text(json.dumps({
            "mcpServers": {"fixture": {"headers": {
                "Authorization": "Bearer harbor_sentinel_fixture-only"}}}}))

    def test_explicit_session_uses_model_port(self):
        self.fixture("session-123")
        self.assertEqual(probe.gate_connection("session-123"), (
            "http://127.0.0.1:50123", "harbor_sentinel_fixture-only"))

    def test_missing_profile_has_actionable_message(self):
        with self.assertRaisesRegex(ValueError, "缺少 sandbox.sb"):
            probe.gate_connection("session-123")

    def test_rejects_path_traversal(self):
        with self.assertRaises(ValueError):
            probe.gate_connection("../outside")

    @patch.object(probe.socket, "create_connection", return_value=MagicMock())
    def test_auto_discovers_unique_session(self, connect):
        self.fixture("session-123")
        # 排除宿主机临时目录，保持测试结果独立于本地进程。
        original = pathlib.Path.glob
        def local_glob(path, pattern):
            return [] if str(path) == "/private/tmp" else original(path, pattern)
        with patch.object(pathlib.Path, "glob", local_glob):
            self.assertEqual(probe.gate_connection("auto")[0], "http://127.0.0.1:50123")
        connect.assert_called_once_with(("127.0.0.1", 50123), timeout=0.3)

    @patch.object(probe.socket, "create_connection", return_value=MagicMock())
    def test_auto_requires_choice_for_multiple_sessions(self, connect):
        self.fixture("session-123")
        self.fixture("session-456")
        original = pathlib.Path.glob
        def local_glob(path, pattern):
            return [] if str(path) == "/private/tmp" else original(path, pattern)
        with patch.object(pathlib.Path, "glob", local_glob):
            with self.assertRaisesRegex(ValueError, "存在多个活动会话"):
                probe.gate_connection("auto")


if __name__ == "__main__":
    unittest.main()
