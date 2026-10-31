package dsh

import (
	"context"
	"encoding/json"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureKit(t *testing.T) *Kit {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh", "lib")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{"lib/bin.js": "fixture", "package.json": `{"name":"@deepseek-ai/dsh","version":"0.2.1-rc.2"}`} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), name), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	k, err := New("/usr/bin/true", root, "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func validInput() kits.KitInput {
	return kits.KitInput{Task: "task-is-untrusted", PrivateHome: "/private/tmp/harbor-private", ModelBaseURL: "http://127.0.0.1:40001", MCPURL: "http://127.0.0.1:40002/mcp", Sentinel: strings.Repeat("a", 64)}
}

func TestBuildIsolatedDSHProfile(t *testing.T) {
	k := fixtureKit(t)
	input := validInput()
	launch, err := k.Build(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if string(launch.Stdin) != input.Task || strings.Contains(strings.Join(launch.Args, " "), input.Task) {
		t.Fatal("任务必须只通过 stdin")
	}
	if launch.Interactive {
		t.Fatal("headless 单任务模式必须关闭 stdin，不能宣称支持多轮")
	}
	if launch.Env["DSH_HOME"] != filepath.Join(input.PrivateHome, "dsh") || launch.Env["DEEPSEEK_API_KEY"] != input.Sentinel || launch.Env["DSH_TELEMETRY_DISABLED"] != "1" {
		t.Fatal("环境不符合隔离要求")
	}
	for _, file := range launch.ConfigFiles {
		if strings.Contains(string(file.Content), input.Task) || !filepath.IsLocal(file.RelativePath) {
			t.Fatal("任务或路径污染配置")
		}
	}
	var patches []struct {
		Insert []struct {
			ID     string         `json:"id"`
			Name   string         `json:"name"`
			Config map[string]any `json:"config"`
		} `json:"insert"`
	}
	if err := json.Unmarshal(launch.ConfigFiles[2].Content, &patches); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, row := range patches[0].Insert {
		found[row.ID] = true
		for _, bad := range []string{"bash", "subprocess", "plugin-manager", "web-app", "credentials-local", "subagent", "pwsh", "otel", "hmr"} {
			if strings.Contains(row.Name, bad) {
				t.Fatalf("启用了非授权能力 %s", row.Name)
			}
		}
		if row.ID == "connection" && (row.Config["failOnStartupError"] != false || row.Config["url"] != input.MCPURL) {
			t.Fatal("MCP 非必需或路径不符")
		}
		if row.ID == "llm-deepseek" && row.Config["baseURL"] != input.ModelBaseURL {
			t.Fatal("模型未通过 Gate")
		}
	}
	for _, id := range []string{"connection", "headless-runner", "llm-deepseek", "fs-sandbox", "sandbox-policy"} {
		if !found[id] {
			t.Fatalf("缺少 %s", id)
		}
	}
}

func TestDSHRejectsInvalidInputAndVersion(t *testing.T) {
	k := fixtureKit(t)
	for _, change := range []func(*kits.KitInput){func(i *kits.KitInput) { i.Task = " " }, func(i *kits.KitInput) { i.Sentinel = "short" }, func(i *kits.KitInput) { i.ModelBaseURL = "https://api.deepseek.com" }, func(i *kits.KitInput) { i.PrivateHome = "../escape" }, func(i *kits.KitInput) { i.MCPURL = "http://localhost:12/mcp" }} {
		i := validInput()
		change(&i)
		if _, err := k.Build(context.Background(), i); err == nil {
			t.Fatal("非法输入未拒绝")
		}
	}
	meta := filepath.Join(k.installation, "node_modules", "@deepseek-ai", "dsh", "package.json")
	os.WriteFile(meta, []byte(`{"name":"@deepseek-ai/dsh","version":"unverified"}`), 0600)
	if _, err := k.Build(context.Background(), validInput()); err == nil {
		t.Fatal("未知版本未拒绝")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := k.Build(ctx, validInput()); err == nil {
		t.Fatal("忽略取消")
	}
}

func TestDSHRejectsEscapedInstallationFiles(t *testing.T) {
	for _, name := range []string{"package.json", "lib/bin.js"} {
		t.Run(name, func(t *testing.T) {
			k := fixtureKit(t)
			path := filepath.Join(k.installation, "node_modules", "@deepseek-ai", "dsh", name)
			outside := filepath.Join(t.TempDir(), "outside")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(outside, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			if _, err = k.Build(context.Background(), validInput()); err == nil {
				t.Fatal("安装文件指向根目录外部时必须拒绝")
			}
		})
	}
}

func TestBuildOptionalAndMultipleMCP(t *testing.T) {
	k := fixtureKit(t)
	for _, urls := range [][]string{nil, {"http://127.0.0.1:40002/mcp/0", "http://127.0.0.1:40002/mcp/1"}} {
		input := validInput()
		input.MCPURL = ""
		input.MCPURLs = urls
		launch, err := k.Build(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		var patches []struct {
			Insert []struct {
				Name   string         `json:"name"`
				Config map[string]any `json:"config"`
			} `json:"insert"`
		}
		if err := json.Unmarshal(launch.ConfigFiles[2].Content, &patches); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, row := range patches[0].Insert {
			if row.Name == "@deepseek-ai/dsh-mcp-client" {
				if row.Config["url"] != urls[found] || row.Config["serverName"] != kits.MCPServerName(input, found) {
					t.Fatal("挂载配置串用")
				}
				found++
			}
		}
		if found != len(urls) {
			t.Fatalf("配置挂载数=%d", found)
		}
	}
}
