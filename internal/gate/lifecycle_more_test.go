package gate

import (
	"context"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestGateLifecycleWithoutMCP(t *testing.T) {
	store := secrets.NewStore()
	id, err := store.Put(context.Background(), contracts.AccountInfo{Kind: "model", Name: "model", ProfileID: policy.ProfileDemoModel, Host: "example.com"}, []byte("key"))
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, policy.NewBuilder(map[string]policy.FixedProfile{}))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	binding, err := h.Start(ctx, GateSpec{SessionID: "s", ModelAccountID: id}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ModelBaseURL == "" || binding.Sentinel == "" || len(binding.AllowedTCP) != 1 || len(binding.MCPURLs) != 0 || binding.MCPURL != "" {
		t.Fatalf("binding: %#v", binding)
	}
	if h.Health() != "ok" {
		t.Fatal("health failed")
	}
	if _, ok := h.getSession("s"); !ok {
		t.Fatal("session not ready")
	}
	if _, err := h.Start(ctx, GateSpec{SessionID: "s", ModelAccountID: id}, nil, nil); err != ErrSessionExists {
		t.Fatalf("duplicate start error: %v", err)
	}
	if err := h.Revoke(ctx, "s"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.getSession("s"); ok {
		t.Fatal("revoked session still available")
	}
	if err := h.Revoke(ctx, "missing"); err != nil {
		t.Fatal("missing revoke should be idempotent: ", err)
	}
}

func TestGateStartRejectsIncompleteSpec(t *testing.T) {
	h := New(secrets.NewStore(), policy.NewBuilder(nil))
	if _, err := h.Start(context.Background(), GateSpec{}, nil, nil); err == nil {
		t.Fatal("incomplete spec accepted")
	}
}

func TestGateLifecycleWithMCP(t *testing.T) {
	store := secrets.NewStore()
	modelID, err := store.Put(context.Background(), contracts.AccountInfo{Kind: "model", Name: "model", ProfileID: policy.ProfileDemoModel, Host: "example.com"}, []byte("model-key"))
	if err != nil {
		t.Fatal(err)
	}
	mcpID, err := store.Put(context.Background(), contracts.AccountInfo{Kind: "mcp", Name: "mcp", ProfileID: policy.ProfileCustom, Host: "mcp.example"}, []byte("mcp-key"))
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, policy.NewBuilder(nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	binding, err := h.Start(ctx, GateSpec{SessionID: "mcp-session", ModelAccountID: modelID, Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{{AccountID: mcpID, Endpoint: policy.EndpointRule{Origin: "https://mcp.example", Paths: []string{"/mcp"}}, Tools: []string{"read"}}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if binding.MCPURL == "" || len(binding.MCPURLs) != 1 || len(binding.AllowedTCP) != 2 {
		_ = h.Revoke(context.Background(), "mcp-session")
		t.Fatalf("MCP binding: %#v", binding)
	}
	if err := h.Revoke(ctx, "mcp-session"); err != nil {
		t.Fatal(err)
	}
}
