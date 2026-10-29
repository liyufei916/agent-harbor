package policy

import (
	"context"
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"testing"
)

func TestDSHProfileAndModelCannotBeInterchanged(t *testing.T) {
	builder := NewBuilder(map[string]FixedProfile{
		ProfileDemoModel:  {Kind: "model", Host: "model.test"},
		ProfileDSHModel:   {Kind: "model", Host: "model.test", BasePath: "/anthropic"},
		ProfileCodexModel: {Kind: "model", Host: "api.openai.com"},
		ProfileDemoMCP:    {Kind: "mcp", Host: "tools.test"},
	})
	req := PolicyRequest{Workspace: t.TempDir(), AgentID: "dsh", Model: contracts.AccountInfo{ID: "m", Kind: "model", ProfileID: ProfileDemoModel, Host: "model.test"}, MCP: contracts.AccountInfo{ID: "t", Kind: "mcp", ProfileID: ProfileDemoMCP, Host: "tools.test"}}
	if _, err := builder.Build(context.Background(), req); err == nil {
		t.Fatal("dsh 使用 Claude profile 未拒绝")
	}
	req.Model.ProfileID = ProfileDSHModel
	snapshot, err := builder.Build(context.Background(), req)
	if err != nil || snapshot.Model.BasePath != "/anthropic" {
		t.Fatalf("dsh 策略异常 %+v %v", snapshot, err)
	}
	req.AgentID = "claude"
	if _, err := builder.Build(context.Background(), req); err == nil {
		t.Fatal("Claude 使用 dsh profile 未拒绝")
	}
	req.AgentID = "codex"
	req.Model.ProfileID = ProfileDemoModel
	if _, err := builder.Build(context.Background(), req); err == nil {
		t.Fatal("codex 使用 Claude profile 未拒绝")
	}
	req.Model.ProfileID = ProfileCodexModel
	req.Model.Host = "api.openai.com"
	if _, err := builder.Build(context.Background(), req); err != nil {
		t.Fatalf("codex + codex-model 应通过: %v", err)
	}
	req.Model.ProfileID = ProfileCustom
	if _, err := builder.Build(context.Background(), req); err == nil {
		t.Fatal("codex 使用 custom 模型未拒绝")
	}
}

func TestModelBasePathRejectsAmbiguousRoutes(t *testing.T) {
	for _, path := range []string{"//evil.test", "/../escape", "/x/./y", "/x%2fy", "/x?y", "/x#y", "/x\\y", "/x/", "/ x", "/x\n"} {
		if ValidBasePath(path) {
			t.Fatalf("允许非法路径 %q", path)
		}
	}
	for _, path := range []string{"", "/anthropic", "/api/anthropic"} {
		if !ValidBasePath(path) {
			t.Fatalf("拒绝合法路径 %q", path)
		}
	}
}
