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

func TestBuildCustomOrigin(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(map[string]FixedProfile{})
	snap, err := b.Build(context.Background(), PolicyRequest{
		Workspace: dir,
		AgentID:   "claude",
		Model:     contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileCustom, Host: "api.example.test:8443"},
		MCP:       contracts.AccountInfo{ID: "t1", Kind: "mcp", ProfileID: ProfileCustom, Host: "mcp.example.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Model.Origin != "https://api.example.test:8443" || snap.MCP.Origin != "https://mcp.example.test" {
		t.Fatalf("%+v", snap)
	}
	if err := b.RememberHost("files.example.test", dir, "project"); err != nil {
		t.Fatal(err)
	}
	if got := b.ListRememberedHosts(dir); len(got) != 1 || got[0].Host != "files.example.test" {
		t.Fatalf("%+v", got)
	}
	b.RevokeRememberedHost("files.example.test", dir)
	b.RevokeRememberedHost("files.example.test", dir)
	if len(b.ListRememberedHosts("")) != 0 {
		t.Fatal("revoke should be idempotent")
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
