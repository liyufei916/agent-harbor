package gate

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestMCPMountValidationBranches(t *testing.T) {
	store := secrets.NewStore()
	id, err := store.Put(context.Background(), contracts.AccountInfo{Kind: "mcp", Name: "mcp", ProfileID: policy.ProfileCustom, Host: "example.com"}, []byte("key"))
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, policy.NewBuilder(nil))
	for name, spec := range map[string]GateSpec{
		"inconsistent": {MCPAccountID: id, Policy: policy.PolicySnapshot{MCPID: "other"}},
		"missing":      {Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{{AccountID: "missing"}}}},
		"duplicate":    {Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{{AccountID: id}, {AccountID: id}}}},
		"wrong kind":   {Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{{AccountID: "model"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := h.mcpMounts(context.Background(), spec); err == nil {
				t.Fatal("invalid mount accepted")
			}
		})
	}
	mounts, err := h.mcpMounts(context.Background(), GateSpec{MCPAccountID: id, Policy: policy.PolicySnapshot{MCP: policy.EndpointRule{Paths: []string{"/mcp"}}, MCPID: id}})
	if err != nil || len(mounts) != 1 || mounts[0].AccountID != id {
		t.Fatalf("single mount: %#v %v", mounts, err)
	}
}
