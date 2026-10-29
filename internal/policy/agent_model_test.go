package policy

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestSharedModelAccountAllowsClaudeAndCodex(t *testing.T) {
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoMCP: {Kind: "mcp", Host: "mcp.test", Origin: "https://mcp.test", Paths: []string{"/mcp"}, Tools: []string{"x"}},
	})
	account := contracts.AccountInfo{
		ID: "m1", Kind: "model", ProfileID: ProfileCustom,
		Host: "gateway.example.test", BasePath: "/anthropic", Model: "shared-model",
		Agents: []string{"claude", "codex"},
	}
	dir := t.TempDir()
	for _, agent := range []string{"claude", "codex"} {
		snap, err := b.Build(context.Background(), PolicyRequest{
			Workspace: dir, AgentID: agent, MountMode: "clone", Model: account,
		})
		if err != nil {
			t.Fatalf("agent=%s: %v", agent, err)
		}
		if agent == "claude" && snap.Model.BasePath != "/anthropic" {
			t.Fatalf("claude basePath %+v", snap.Model)
		}
		if agent == "codex" {
			// Responses 不走 Anthropic BasePath
			if snap.Model.Origin != "https://gateway.example.test" {
				t.Fatalf("codex origin %+v", snap.Model)
			}
		}
	}
	if _, err := b.Build(context.Background(), PolicyRequest{
		Workspace: dir, AgentID: "dsh", MountMode: "clone", Model: account,
	}); err == nil {
		t.Fatal("未勾选 dsh 时不应通过")
	}
}

func TestNormalizeModelAgentsDefaults(t *testing.T) {
	if got := NormalizeModelAgents(ProfileCodexModel, nil); len(got) != 1 || got[0] != "codex" {
		t.Fatalf("%v", got)
	}
	if got := NormalizeModelAgents(ProfileCustom, []string{"codex", "claude", "codex"}); len(got) != 2 {
		t.Fatalf("%v", got)
	}
}
