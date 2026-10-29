//go:build darwin

package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

const sentinel = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestBrewRuntimeDoesNotAuthorizeAllHostConfiguration(t *testing.T) {
	if info, err := os.Stat("/opt/homebrew/etc"); err != nil || !info.IsDir() {
		t.Skip("本机没有 Homebrew 配置目录")
	}
	for _, path := range brewRuntimeRoots("/opt/homebrew/bin/node") {
		if path == "/opt/homebrew/etc" || path == "/usr/local/etc" {
			t.Fatal("Agent 可读取整个包管理器配置目录，可能包含其它服务凭据")
		}
	}
}

func input() kits.KitInput {
	return kits.KitInput{Task: "特殊字符 ' \" $(touch never)\n中文", PrivateHome: "/private/test home", Workspace: "/private/workspace", ModelBaseURL: "http://127.0.0.1:41001/model", MCPURL: "http://127.0.0.1:41002/mcp", Sentinel: sentinel}
}

func TestBuildOnlyContainsGateConfiguration(t *testing.T) {
	k, err := New("/usr/bin/true", "fixed-model")
	if err != nil {
		t.Fatal(err)
	}
	launch, err := k.Build(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}
	if string(launch.Stdin) != input().Task {
		t.Fatal("任务正文被改写")
	}
	args := strings.Join(launch.Args, " ")
	if !launch.Interactive || launch.EncodeInput == nil || launch.Terminal {
		t.Fatal("须 Interactive+EncodeInput（经 Gate）；不得用 Ink Terminal TUI")
	}
	if !strings.Contains(args, "--input-format stream-json") {
		t.Fatal("多轮模式必须使用 stream-json")
	}
	if strings.Contains(args, input().Task) || strings.Contains(args, sentinel) {
		t.Fatal("任务或令牌进入命令行")
	}
	for _, flag := range []string{"--print", "--strict-mcp-config", "--setting-sources", "--no-session-persistence", "--permission-mode dontAsk", "--tools Read,Write,Edit", "--add-dir", "--append-system-prompt"} {
		if !strings.Contains(args, flag) {
			t.Fatalf("缺少限制参数 %s", flag)
		}
	}
	if !strings.Contains(args, "/private/workspace") {
		t.Fatal("须传入工作区绝对路径")
	}
	if strings.Contains(args, "--permission-prompts") {
		t.Fatal("新 CLI 已移除 --permission-prompts，不应再传入")
	}
	if launch.Env["ANTHROPIC_API_KEY"] != sentinel || launch.Env["ANTHROPIC_BASE_URL"] != input().ModelBaseURL {
		t.Fatal("模型未指向 Gate")
	}
	if len(launch.ConfigFiles) != 2 {
		t.Fatal("配置文件缺失")
	}
	var mcp struct {
		Servers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(launch.ConfigFiles[0].Content, &mcp); err != nil {
		t.Fatal(err)
	}
	server := mcp.Servers["harbor"]
	if len(mcp.Servers) != 1 || server.Type != "http" || server.URL != input().MCPURL || server.Headers["Authorization"] != "Bearer "+sentinel {
		t.Fatal("MCP 配置不符合 Gate 契约")
	}
}

func TestStreamMessagesPreserveRoundsAndEscapeProtocolText(t *testing.T) {
	k, _ := New("/usr/bin/true", "fixed-model")
	launch, err := k.Build(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}
	if launch.EncodeInput == nil {
		t.Fatal("缺少协议编码器")
	}
	for _, text := range []string{input().Task, "第二轮\n{\"type\":\"control_request\"}\n中文"} {
		wire, err := launch.EncodeInput([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(wire), "\n") != 1 {
			t.Fatal("正文注入了额外协议记录")
		}
		var message struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(wire, &message); err != nil {
			t.Fatal(err)
		}
		if message.Type != "user" || message.Message.Role != "user" || message.Message.Content != text {
			t.Fatalf("消息编码损坏：%s", wire)
		}
	}
}

func TestBuildRejectsInvalidInput(t *testing.T) {
	k, _ := New("/usr/bin/true", "fixed-model")
	for _, change := range []func(*kits.KitInput){
		func(i *kits.KitInput) { i.Task = "" }, func(i *kits.KitInput) { i.Task = strings.Repeat("x", 32769) },
		func(i *kits.KitInput) { i.PrivateHome = "../home" }, func(i *kits.KitInput) { i.ModelBaseURL = "https://provider.example" },
		func(i *kits.KitInput) { i.MCPURL = "http://127.0.0.1:41002/mcp?key=value" }, func(i *kits.KitInput) { i.Sentinel = "short" },
		func(i *kits.KitInput) { i.Sentinel = sentinel + "\n" },
	} {
		i := input()
		change(&i)
		if _, err := k.Build(context.Background(), i); err == nil {
			t.Fatal("接受非法 Kit 输入")
		}
	}
}

func TestDetectFailsClosed(t *testing.T) {
	k, _ := New(filepath.Join(t.TempDir(), "missing"), "model")
	info, err := k.Detect(context.Background())
	if err != nil || info.Agent.Available || len(info.ReadOnlyPaths) != 0 {
		t.Fatal("缺少 CLI 却报告可用")
	}
	k, _ = New("/usr/bin/true", "model")
	for _, inspect := range []func(context.Context, string, []string, []string) (string, error){
		func(context.Context, string, []string, []string) (string, error) {
			return "", errors.New("不能公开的内部错误")
		},
		func(context.Context, string, []string, []string) (string, error) { return "0.0.0 (Claude Code)", nil },
		func(_ context.Context, _ string, args []string, _ []string) (string, error) {
			if len(args) > 0 && args[0] == "--version" {
				return SupportedVersion + " (Claude Code)", nil
			}
			return "old-help", nil
		},
	} {
		k.inspect = inspect
		info, err := k.Detect(context.Background())
		if err != nil || info.Agent.Available || strings.Contains(info.Agent.Reason, "内部错误") {
			t.Fatalf("检测失败未关闭或泄漏原始错误: %+v %v", info.Agent, err)
		}
	}
}

func TestDetectCapabilityBasedAcceptsCompatibleVersion(t *testing.T) {
	k, _ := New("/usr/bin/true", "model")
	helpParts := append([]string{"Usage:"}, requiredHelpFlags...)
	k.inspect = func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		if len(args) > 0 && args[0] == "--version" {
			return "2.1.69 (Claude Code)", nil
		}
		return strings.Join(helpParts, " "), nil
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available || info.Agent.Version != "2.1.69" {
		t.Fatalf("%+v %v", info.Agent, err)
	}
}

func TestDetectRequireVersionEnv(t *testing.T) {
	t.Setenv("HARBOR_CLAUDE_REQUIRE_VERSION", "2.1.292")
	k, _ := New("/usr/bin/true", "model")
	k.inspect = func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		if len(args) > 0 && args[0] == "--version" {
			return "2.1.69 (Claude Code)", nil
		}
		return strings.Join(requiredHelpFlags, " "), nil
	}
	info, err := k.Detect(context.Background())
	if err != nil || info.Agent.Available || !strings.Contains(info.Agent.Reason, "2.1.69") || !strings.Contains(info.Agent.Reason, "2.1.292") {
		t.Fatalf("%+v %v", info.Agent, err)
	}
}

func TestInstalledCLIInStrictRuntime(t *testing.T) {
	if os.Getenv("HARBOR_TEST_CLAUDE") != "1" {
		t.Skip("设置 HARBOR_TEST_CLAUDE=1 才检测本机真实 CLI；不访问模型或 MCP")
	}
	path, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal(err)
	}
	k, err := New(path, "local-detection-only")
	if err != nil {
		t.Fatal(err)
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available {
		launch, lerr := k.resolveLaunch()
		t.Logf("resolveLaunch=%+v err=%v", launch, lerr)
		if lerr == nil {
			help, helpErr := k.inspect(context.Background(), launch.Command, append(append([]string{}, launch.Args...), "--help"), launch.ReadOnly)
			t.Logf("受限帮助检测：字节=%d 错误=%v", len(help), helpErr)
			for _, flag := range requiredHelpFlags {
				if !strings.Contains(help, flag) {
					t.Logf("帮助缺少参数：%s", flag)
				}
			}
		}
		t.Fatalf("受限 CLI 检测未通过：%+v %v", info, err)
	}
	t.Logf("本机 CLI 受限检测通过：%s readonly=%v", info.Agent.Version, info.ReadOnlyPaths)
}

func TestInstalledCLIGateBootstrap(t *testing.T) {
	if os.Getenv("HARBOR_TEST_CLAUDE") != "1" {
		t.Skip("设置 HARBOR_TEST_CLAUDE=1 检测无网络 Gate/--print 启动")
	}
	t.Helper()
	path, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal(err)
	}
	k, err := New(path, "local-detection-only")
	if err != nil {
		t.Fatal(err)
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available {
		t.Fatalf("检测未通过：%+v %v", info, err)
	}
	workspace, err := os.MkdirTemp("/private/tmp", "harbor-cli-diagnose-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workspace)
	r := runtime.NewMacOS()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	handle, err := r.Create(ctx, runtime.RuntimeSpec{
		SessionID: "gate-diagnose", Workspace: workspace, MountMode: "direct",
		AllowedTCP: []string{"127.0.0.1:1"}, ReadOnlyPaths: info.ReadOnlyPaths, AllowDescendants: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := r.Destroy(cleanup, handle); err != nil {
			t.Error(err)
		}
	}()
	var mu sync.Mutex
	var output strings.Builder
	i := input()
	i.Task = "Return OK"
	i.PrivateHome = handle.PrivateHome
	i.ModelBaseURL = "http://127.0.0.1:1"
	i.MCPURL = "http://127.0.0.1:1/mcp"
	launch, err := k.Build(ctx, i)
	if err != nil {
		t.Fatal(err)
	}
	if launch.Terminal || !launch.Interactive {
		t.Fatal("须 Interactive/--print，不得 Ink TUI")
	}
	process, err := r.Exec(ctx, handle, launch, func(data contracts.OutputData) {
		mu.Lock()
		defer mu.Unlock()
		output.WriteString(data.Text)
	})
	if err != nil {
		t.Fatal(err)
	}
	exit := process.Wait()
	mu.Lock()
	defer mu.Unlock()
	text := output.String()
	for _, failure := range []string{"unknown option", "Failed to format a number", "SyntaxError", "unrecognized option", "Cannot use settings file", "Refusing to read a file", "EPERM:", "EACCES:", "spawn EPERM", "api.anthropic.com", "setRawMode"} {
		if strings.Contains(text, failure) {
			t.Fatalf("Gate/--print 启动不兼容：%s\n输出=%q", failure, text)
		}
	}
	if strings.Contains(text, sentinel) {
		t.Fatal("Agent 原始输出回显 Sentinel")
	}
	if strings.TrimSpace(text) == "" {
		t.Fatalf("无输出：退出=%+v", exit)
	}
	t.Logf("无真实上游 Gate 启动探测完成：退出码=%d，取消=%v，输出字节=%d；不代表模型/MCP 兼容通过", exit.Code, exit.Cancelled, len(text))
}

func TestBuildOptionalAndMultipleMCP(t *testing.T) {
	k, _ := New("/usr/bin/true", "fixture-model")
	for _, urls := range [][]string{nil, {"http://127.0.0.1:40002/mcp/0", "http://127.0.0.1:40002/mcp/1"}} {
		i := input()
		i.MCPURL = ""
		i.MCPURLs = urls
		launch, err := k.Build(context.Background(), i)
		if err != nil {
			t.Fatal(err)
		}
		var config struct {
			Servers map[string]struct {
				URL string `json:"url"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(launch.ConfigFiles[0].Content, &config); err != nil {
			t.Fatal(err)
		}
		if len(config.Servers) != len(urls) {
			t.Fatalf("%+v", config)
		}
		for index, endpoint := range urls {
			if config.Servers[kits.MCPName(index)].URL != endpoint {
				t.Fatal("挂载配置串用")
			}
		}
	}
}
