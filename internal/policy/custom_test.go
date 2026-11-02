package policy

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestCustomModelProfileHTTPS(t *testing.T) {
	p, err := CustomModelProfile("https://api.openai.com", "gpt-test")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != ProfileCustom || p.Host != "api.openai.com" || !p.ValidConnection() {
		t.Fatalf("%+v", p)
	}
}

func TestCustomModelProfileDeepSeekAnthropicPath(t *testing.T) {
	p, err := CustomModelProfile("https://api.deepseek.com/anthropic", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != "https://api.deepseek.com" || p.BasePath != "/anthropic" || p.Model != "deepseek-chat" {
		t.Fatalf("%+v", p)
	}
	cands, err := CustomModelProbeCandidates("https://api.deepseek.com", "")
	if err != nil || len(cands) < 2 {
		t.Fatalf("cands=%v err=%v", cands, err)
	}
	if cands[0].Model != "deepseek-chat" {
		t.Fatalf("want deepseek-chat first, got %+v", cands[0])
	}
}

func TestCustomModelProfileAllowsLocalLoopbackOnly(t *testing.T) {
	for _, origin := range []string{"http://127.0.0.1:4000", "http://localhost:4000"} {
		p, err := CustomModelProfile(origin, "gpt-6.1-sol")
		if err != nil || !p.AllowInsecureLoopback || !p.ValidConnection() {
			t.Fatalf("origin=%s profile=%+v err=%v", origin, p, err)
		}
	}
	if _, err := CustomModelProfile("http://0.0.0.0:4000", "gpt-6.1-sol"); err == nil {
		t.Fatal("unspecified bind address must not be accepted as a connection target")
	}
}

func TestBuildAcceptsCustomModelForClaude(t *testing.T) {
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoMCP: {
			ID: ProfileDemoMCP, Kind: "mcp", Protocol: "streamable-http",
			Origin: "https://mcp.example.test", Host: "mcp.example.test",
			Paths: []string{"/mcp"}, TestPath: "/mcp", Tools: []string{"x"},
		},
	})
	dir := t.TempDir()
	snap, err := b.Build(nil, PolicyRequest{
		Workspace: dir,
		AgentID:   "claude",
		MountMode: "clone",
		Model: contracts.AccountInfo{
			ID: "m1", Kind: "model", ProfileID: ProfileCustom, Host: "api.openai.com",
		},
		MCP: contracts.AccountInfo{
			ID: "t1", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "mcp.example.test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Model.Origin != "https://api.openai.com" {
		t.Fatalf("%+v", snap.Model)
	}
}
