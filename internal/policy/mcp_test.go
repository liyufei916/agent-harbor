package policy

import (
	"context"
	"fmt"
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"testing"
)

func TestBuildOptionalAndMultipleMCP(t *testing.T) {
	profiles := map[string]FixedProfile{
		ProfileDemoModel: {Kind: "model", Host: "model.test", Origin: "https://model.test"},
		"mcp":            {Kind: "mcp", Host: "mcp.test", Origin: "https://mcp.test", Paths: []string{"/mcp"}, Tools: []string{"read"}},
	}
	b := NewBuilder(profiles)
	request := PolicyRequest{Workspace: t.TempDir(), AgentID: "claude", Model: contracts.AccountInfo{ID: "model", Kind: "model", ProfileID: ProfileDemoModel, Host: "model.test"}}
	for _, count := range []int{0, 1, 8} {
		request.MCPs = nil
		for i := 0; i < count; i++ {
			request.MCPs = append(request.MCPs, contracts.AccountInfo{ID: fmt.Sprint(i), Kind: "mcp", ProfileID: "mcp", Host: "mcp.test"})
		}
		snapshot, err := b.Build(context.Background(), request)
		if err != nil || len(snapshot.MCPMounts) != count {
			t.Fatalf("count=%d %+v %v", count, snapshot, err)
		}
	}
	request.MCPs = append(request.MCPs, request.MCPs[0])
	if _, err := b.Build(context.Background(), request); err == nil {
		t.Fatal("接受超过8个挂载")
	}
	request.MCPs = request.MCPs[:2]
	request.MCPs[1] = request.MCPs[0]
	if _, err := b.Build(context.Background(), request); err == nil {
		t.Fatal("接受重复挂载")
	}
	request.MCPs[1].ID = "other"
	request.MCPs[1].Host = "wrong.test"
	if _, err := b.Build(context.Background(), request); err == nil {
		t.Fatal("接受不匹配主机")
	}
}

func TestBuildAllowsCustomMCPWithoutTools(t *testing.T) {
	b := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel: {Kind: "model", Host: "model.test", Origin: "https://model.test"},
	})
	snap, err := b.Build(context.Background(), PolicyRequest{
		Workspace: t.TempDir(),
		AgentID:   "claude",
		Model:     contracts.AccountInfo{ID: "model", Kind: "model", ProfileID: ProfileDemoModel, Host: "model.test"},
		MCPs: []contracts.AccountInfo{{
			ID: "ctx7", Kind: "mcp", ProfileID: ProfileCustom, Host: "mcp.context7.com",
			// Tools 空：可继续但工具受限（空白名单）
		}},
	})
	if err != nil {
		t.Fatalf("want allow empty tools, got %v", err)
	}
	if len(snap.MCPMounts) != 1 || len(snap.MCPMounts[0].Tools) != 0 {
		t.Fatalf("want empty tools mount, got %+v", snap.MCPMounts)
	}
}
