package policy

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestBuildRejectsInvalidCombinations(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel: {ID: ProfileDemoModel, Kind: "model", Origin: "https://model.example", Host: "model.example", Paths: []string{"/v1/messages"}, TestPath: "/v1/messages", Protocol: "anthropic", Model: "m"},
		ProfileDemoMCP:   {ID: ProfileDemoMCP, Kind: "mcp", Origin: "https://mcp.example", Host: "mcp.example", Paths: []string{"/mcp"}, TestPath: "/mcp", Protocol: "streamable-http", Tools: []string{"read"}},
	})
	base := PolicyRequest{Workspace: dir, AgentID: "claude", Model: contracts.AccountInfo{Kind: "model", ProfileID: ProfileDemoModel, Host: "model.example"}}
	cases := []struct {
		name string
		edit func(*PolicyRequest)
	}{
		{"missing agent", func(r *PolicyRequest) { r.AgentID = "" }},
		{"wrong model kind", func(r *PolicyRequest) { r.Model.Kind = "mcp" }},
		{"unknown profile", func(r *PolicyRequest) { r.Model.ProfileID = "missing" }},
		{"dsh mismatch", func(r *PolicyRequest) { r.AgentID = "dsh" }},
		{"mcp kind", func(r *PolicyRequest) {
			r.MCP = contracts.AccountInfo{ID: "x", Kind: "model", ProfileID: ProfileDemoMCP, Host: "mcp.example"}
		}},
		{"duplicate mcp", func(r *PolicyRequest) {
			r.MCPs = []contracts.AccountInfo{{ID: "x", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "mcp.example"}, {ID: "x", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "mcp.example"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.edit(&req)
			if _, err := b.Build(context.Background(), req); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}
