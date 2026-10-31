package gate

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestStartSeedsExtraHosts(t *testing.T) {
	store := secrets.NewStore()
	ctx := context.Background()
	modelID, err := store.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "m", ProfileID: policy.ProfileDemoModel, Host: "api.example.test",
	}, []byte("k"))
	if err != nil {
		t.Fatal(err)
	}
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {
			ID: policy.ProfileDemoModel, Kind: "model", Origin: "https://api.example.test", Paths: []string{"/"}, Host: "api.example.test",
		},
	})
	h := New(store, pol)
	asked := 0
	_, err = h.Start(ctx, GateSpec{
		SessionID: "seed-extra", ModelAccountID: modelID,
		ExtraHosts: map[string]string{"cdn.example.test": "project"},
		Policy:     policy.PolicySnapshot{Model: policy.EndpointRule{Origin: "https://api.example.test", Paths: []string{"/"}}},
		RequestPermission: func(context.Context, contracts.PermissionNeeded) (string, error) {
			asked++
			return "deny", nil
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "seed-extra")
	if !h.HasExtraHost("seed-extra", "cdn.example.test") {
		t.Fatal("启动应注入 ExtraHosts")
	}
	if asked != 0 {
		t.Fatal("已记住主机不应再询问")
	}
	// project 应保留
	if !h.HasExtraHost("seed-extra", "cdn.example.test") {
		t.Fatal("project 主机应可重复命中")
	}
}

func TestHasExtraHostOnceConsumes(t *testing.T) {
	h := New(secrets.NewStore(), policy.NewBuilder(nil))
	h.sessions["s-extra"] = &sessionRuntime{
		ctx:        context.Background(),
		extraHosts: make(map[string]string),
	}

	if err := h.AllowExtraHost("missing", "cdn.example.test", "once"); err == nil {
		t.Fatal("expected error for missing session")
	}
	if err := h.AllowExtraHost("s-extra", "CDN.Example.TEST", "once"); err != nil {
		t.Fatal(err)
	}
	if !h.HasExtraHost("s-extra", "cdn.example.test") {
		t.Fatal("expected once allow")
	}
	if h.HasExtraHost("s-extra", "cdn.example.test") {
		t.Fatal("once should be consumed")
	}

	if err := h.AllowExtraHost("s-extra", "cdn.example.test", "project"); err != nil {
		t.Fatal(err)
	}
	if !h.HasExtraHost("s-extra", "cdn.example.test") {
		t.Fatal("project allow")
	}
	if !h.HasExtraHost("s-extra", "cdn.example.test") {
		t.Fatal("project should remain")
	}

	h.RevokeExtraHost("s-extra", "cdn.example.test")
	if h.HasExtraHost("s-extra", "cdn.example.test") {
		t.Fatal("revoke should clear project host for same session")
	}
	_ = h.AllowExtraHost("s-extra", "cdn.example.test", "project")
	h.RevokeExtraHostAll("CDN.Example.TEST")
	if h.HasExtraHost("s-extra", "cdn.example.test") {
		t.Fatal("RevokeExtraHostAll should clear")
	}
}
