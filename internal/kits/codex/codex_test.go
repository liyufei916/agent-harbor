package codex

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/kits"
)

func TestPackageRootForScriptIncludesNpmPackageNotJustBin(t *testing.T) {
	script := filepath.Join(string(filepath.Separator), "opt", "homebrew", "lib", "node_modules", "@openai", "codex", "bin", "codex.js")
	got := packageRootForScript(script)
	want := filepath.Join(string(filepath.Separator), "opt", "homebrew", "lib", "node_modules", "@openai", "codex")
	if got != want {
		t.Fatalf("packageRoot=%q want %q", got, want)
	}
	if packageRootForScript("/tmp/codex.js") != "/tmp" {
		t.Fatalf("非 bin 布局应退回脚本目录")
	}
}

func validInput() kits.KitInput {
	return kits.KitInput{
		Task:         "读取 README 并总结",
		PrivateHome:  "/private/tmp/harbor-home",
		ModelBaseURL: "http://127.0.0.1:41001",
		MCPURL:       "http://127.0.0.1:41002/mcp",
		Sentinel:     strings.Repeat("s", 64),
		Workspace:    "/private/tmp/harbor-ws",
	}
}

func TestBuildUsesResponsesProviderAndPrivateCodexHome(t *testing.T) {
	k, err := New("/bin/sh", "gpt-5-codex")
	if err != nil {
		t.Fatal(err)
	}
	launch, err := k.Build(context.Background(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	if launch.Command != "/bin/sh" || launch.Env["CODEX_HOME"] != "/private/tmp/harbor-home/codex" || launch.Env["OPENAI_API_KEY"] != strings.Repeat("s", 64) {
		t.Fatalf("Codex 启动环境错误: %+v", launch)
	}
	if len(launch.ConfigFiles) != 1 || launch.ConfigFiles[0].RelativePath != "codex/config.toml" {
		t.Fatalf("Codex 配置路径错误: %+v", launch.ConfigFiles)
	}
	config := string(launch.ConfigFiles[0].Content)
	for _, required := range []string{"wire_api = \"responses\"", "env_key = \"OPENAI_API_KEY\"", "[mcp_servers.harbor]", "http://127.0.0.1:41002/mcp", `base_url = "http://127.0.0.1:41001/v1"`, `sandbox_mode = "danger-full-access"`, `approval_policy = "never"`} {
		if !strings.Contains(config, required) {
			t.Fatalf("Codex 配置缺少 %q: %s", required, config)
		}
	}
	if strings.Contains(config, "workspace-write") {
		t.Fatal("外层已 Seatbelt 时不应再启用 Codex workspace-write 嵌套沙箱")
	}
	if strings.Contains(config, `base_url = "http://127.0.0.1:41001"`) && !strings.Contains(config, `base_url = "http://127.0.0.1:41001/v1"`) {
		t.Fatal("Codex base_url 缺少 /v1，会打到 Gate 未放行的 /responses")
	}
	if strings.Contains(config, "api_key") {
		t.Fatal("Codex 配置写入了 api_key 字段")
	}
	joined := strings.Join(launch.Args, " ")
	if !strings.Contains(joined, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("缺少外部沙箱 bypass 旗标: %v", launch.Args)
	}
	if launch.Interactive {
		t.Fatal("无 app-server 的替身二进制不应启用 turn_bridge")
	}
}

func TestNativeAppServerUsesTurnBridge(t *testing.T) {
	k, err := New("/bin/sh", "gpt-5-codex")
	if err != nil {
		t.Fatal(err)
	}
	k.lookupNode = func() (string, error) { return "/tmp/harbor-node", nil }
	k.inspect = func(_ context.Context, _ string, args []string) (string, error) {
		if len(args) >= 1 && args[0] == "app-server" {
			return "Usage: codex app-server\n--listen <URL> stdio://\n", nil
		}
		if len(args) == 1 && args[0] == "--version" {
			return "codex-cli 0.155.0", nil
		}
		return "--json --ephemeral --skip-git-repo-check", nil
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available || !containsPath(info.ReadOnlyPaths, "/tmp/harbor-node") || !containsPath(info.ReadOnlyPaths, "/bin/sh") {
		t.Fatalf("检测未放行 node 与原生二进制: %+v %v", info, err)
	}
	launch, err := k.Build(context.Background(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	if launch.Command != "/tmp/harbor-node" || !launch.Interactive || launch.EncodeInput == nil {
		t.Fatalf("原生 app-server 应走 turn_bridge: %+v", launch)
	}
	joined := strings.Join(launch.Args, " ")
	if !strings.Contains(joined, "--native") || !strings.Contains(joined, "--codex /bin/sh") || strings.Contains(joined, " exec ") {
		t.Fatalf("桥参数错误: %v", launch.Args)
	}
	if len(launch.ConfigFiles) != 2 || launch.ConfigFiles[1].RelativePath != "codex/harbor-turn.mjs" {
		t.Fatalf("缺少常驻桥脚本: %+v", launch.ConfigFiles)
	}
}

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

func TestEncodePromptJSONLine(t *testing.T) {
	got, err := encodePrompt([]byte("下一轮\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\"下一轮\"\n" {
		t.Fatalf("encode=%q", got)
	}
}

func TestDetectRequiresCodexExecFlags(t *testing.T) {
	k, err := New("/bin/sh", "gpt-5-codex")
	if err != nil {
		t.Fatal(err)
	}
	k.inspect = func(_ context.Context, _ string, args []string) (string, error) {
		if len(args) == 1 && args[0] == "--version" {
			return "codex-cli 0.163.0", nil
		}
		return "--json --ephemeral --skip-git-repo-check", nil
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available || info.Agent.ID != "codex" || info.Agent.ModelProfileID != "codex-model" || !info.NeedsChildProcesses {
		t.Fatalf("Codex 检测失败: %+v %v", info, err)
	}
}

func TestBuildRejectsNonGateURLs(t *testing.T) {
	k, _ := New("/bin/sh", "gpt-5-codex")
	input := validInput()
	input.ModelBaseURL = "https://api.openai.com/v1"
	if _, err := k.Build(context.Background(), input); err == nil {
		t.Fatal("Codex 接受了非 Gate 地址")
	}
}

func TestBuildAllowsZeroMCP(t *testing.T) {
	k, _ := New("/bin/sh", "gpt-5-codex")
	input := validInput()
	input.MCPURL = ""
	input.MCPURLs = nil
	launch, err := k.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("无 MCP 时应可启动 Codex: %v", err)
	}
	config := string(launch.ConfigFiles[0].Content)
	if strings.Contains(config, "[mcp_servers.") {
		t.Fatalf("无 MCP 不应写入 mcp_servers: %s", config)
	}
	if !strings.Contains(config, "wire_api = \"responses\"") {
		t.Fatalf("缺少 Responses: %s", config)
	}
}

func TestBuildUsesMCPURLs(t *testing.T) {
	k, _ := New("/bin/sh", "gpt-5-codex")
	input := validInput()
	input.MCPURL = ""
	input.MCPURLs = []string{"http://127.0.0.1:41002/mcp/0", "http://127.0.0.1:41003/mcp/1"}
	launch, err := k.Build(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	config := string(launch.ConfigFiles[0].Content)
	if !strings.Contains(config, "[mcp_servers.harbor]") || !strings.Contains(config, "[mcp_servers.harbor-2]") {
		t.Fatalf("多 MCP 配置错误: %s", config)
	}
}
