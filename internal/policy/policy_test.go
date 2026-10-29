package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestBuildHappyPath(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel: {
			ID: ProfileDemoModel, Kind: "model",
			Origin: "https://api.example.test", Paths: []string{"/v1/"}, Host: "api.example.test",
		},
		ProfileDemoMCP: {
			ID: ProfileDemoMCP, Kind: "mcp",
			Origin: "https://mcp.example.test", Paths: []string{"/mcp"}, Host: "mcp.example.test",
			Tools: []string{"list_resources"},
		},
	})
	snap, err := b.Build(context.Background(), PolicyRequest{
		Workspace: dir,
		AgentID:   "claude",
		Model:     contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileDemoModel, Host: "api.example.test"},
		MCP:       contracts.AccountInfo{ID: "t1", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "mcp.example.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.MountMode != "direct" || snap.MCPID != "t1" {
		t.Fatalf("%+v", snap)
	}
	if snap.Model.Origin != "https://api.example.test" {
		t.Fatalf("origin=%s", snap.Model.Origin)
	}
	request := PolicyRequest{Workspace: dir, AgentID: "claude", MountMode: "clone", Model: contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileDemoModel, Host: "api.example.test"}, MCP: contracts.AccountInfo{ID: "t1", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "mcp.example.test"}}
	copyPolicy, err := b.Build(context.Background(), request)
	if err != nil || copyPolicy.MountMode != "clone" {
		t.Fatalf("clone 策略未保留：%+v %v", copyPolicy, err)
	}
	request.MountMode = "invalid"
	if _, err := b.Build(context.Background(), request); err == nil {
		t.Fatal("接受未知挂载模式")
	}
}

func TestDefaultModelForHostUsesExplicitAndHostDefaults(t *testing.T) {
	t.Setenv("HARBOR_CLAUDE_MODEL", "")
	if got := defaultModelForHost("api.deepseek.com", ""); got != "deepseek-chat" {
		t.Fatalf("DeepSeek 默认模型=%q", got)
	}
	if got := defaultModelForHost("api.example.test", "explicit-model"); got != "explicit-model" {
		t.Fatalf("显式模型未优先=%q", got)
	}
	t.Setenv("HARBOR_CLAUDE_MODEL", "env-model")
	if got := defaultModelForHost("api.example.test", ""); got != "env-model" {
		t.Fatalf("环境模型未生效=%q", got)
	}
}

func TestDenyHomeWorkspace(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	b := NewBuilder(DefaultProfiles())
	_, err = b.Build(context.Background(), PolicyRequest{
		Workspace: home,
		AgentID:   "claude",
		Model:     contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileDemoModel, Host: "h"},
		MCP:       contracts.AccountInfo{ID: "t1", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "h"},
	})
	if err != ErrInvalidWorkspace {
		t.Fatalf("got %v", err)
	}
}

func TestPathAllowed(t *testing.T) {
	rule := EndpointRule{Origin: "https://x", Paths: []string{"/v1/"}}
	if !PathAllowed(rule, "/v1/chat/completions") {
		t.Fatal("expected allow")
	}
	if PathAllowed(rule, "/admin") {
		t.Fatal("expected deny")
	}
	root := EndpointRule{Paths: []string{"/"}}
	if !PathAllowed(root, "/") || PathAllowed(root, "/v1/models") {
		t.Fatal("root path must not allow every path")
	}
}

func TestBuildRejectsHostMismatch(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel: {
			ID: ProfileDemoModel, Kind: "model",
			Origin: "https://api.example.test", Paths: []string{"/v1/"}, Host: "api.example.test",
		},
		ProfileDemoMCP: {
			ID: ProfileDemoMCP, Kind: "mcp",
			Origin: "https://mcp.example.test", Paths: []string{"/mcp"}, Host: "mcp.example.test",
		},
	})
	_, err := b.Build(context.Background(), PolicyRequest{
		Workspace: dir,
		AgentID:   "claude",
		Model:     contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileDemoModel, Host: "evil.example"},
		MCP:       contracts.AccountInfo{ID: "t1", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "mcp.example.test"},
	})
	if err != ErrHostMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestDefaultProfilesAreNotAllowAll(t *testing.T) {
	for id, profile := range DefaultProfiles() {
		for _, path := range profile.Paths {
			if path == "/" || path == "" {
				t.Fatalf("%s allows every path via %q", id, path)
			}
		}
	}
}

func TestNormalizeSymlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	link := filepath.Join(base, "link")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, err := NormalizeWorkspace(link)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(link)
	if got != filepath.Clean(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestFixedEndpointDoesNotAuthorizeSimilarPaths(t *testing.T) {
	rule := EndpointRule{Paths: []string{"/mcp"}}
	if !PathAllowed(rule, "/mcp") || PathAllowed(rule, "/mcp-admin") || PathAllowed(rule, "/mcp/private") {
		t.Fatal("精确端点被当作宽路径前缀")
	}
}
