package policy

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestBuildAcceptsStdioLoopbackMCP(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel: {
			ID: ProfileDemoModel, Kind: "model", Protocol: "anthropic", Model: "m",
			Origin: "https://api.example.test", Host: "api.example.test", Paths: []string{"/v1/messages"}, TestPath: "/v1/messages",
		},
	})
	snap, err := b.Build(context.Background(), PolicyRequest{
		Workspace: dir,
		AgentID:   "claude",
		MountMode: "clone",
		Model:     contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileDemoModel, Host: "api.example.test"},
		MCP: contracts.AccountInfo{
			ID: "t1", Kind: "mcp", ProfileID: ProfileCustom, Host: "127.0.0.1:34567",
			Transport: "stdio", Tools: []string{"echo"}, Command: "/usr/bin/python3",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.MCP.Origin != "http://127.0.0.1:34567" {
		t.Fatalf("origin=%s", snap.MCP.Origin)
	}
	if len(snap.Tools) != 1 || snap.Tools[0] != "echo" {
		t.Fatalf("tools=%v", snap.Tools)
	}
}

func TestBuildRejectsNonLoopbackStdio(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel: {
			ID: ProfileDemoModel, Kind: "model", Protocol: "anthropic", Model: "m",
			Origin: "https://api.example.test", Host: "api.example.test", Paths: []string{"/v1/messages"}, TestPath: "/v1/messages",
		},
	})
	if _, err := b.Build(context.Background(), PolicyRequest{
		Workspace: dir,
		AgentID:   "claude",
		Model:     contracts.AccountInfo{ID: "m1", Kind: "model", ProfileID: ProfileDemoModel, Host: "api.example.test"},
		MCP: contracts.AccountInfo{
			ID: "t1", Kind: "mcp", ProfileID: ProfileCustom, Host: "8.8.8.8:9",
			Transport: "stdio", Tools: []string{"echo"},
		},
	}); err == nil {
		t.Fatal("non-loopback stdio accepted")
	}
}
