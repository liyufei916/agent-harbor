package desktop

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestDesktopAPIForwardsSessionOperationsAndHandlesUnavailable(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	if got := api.GetSession("missing"); got.Error == nil || got.Error.Code != contracts.ErrOrchestratorUnavailable {
		t.Fatalf("GetSession before wire: %#v", got)
	}
	if got := api.StopSession("missing"); got.Error == nil {
		t.Fatalf("StopSession before wire: %#v", got)
	}
	if got := api.DestroySession("missing"); got.Error == nil {
		t.Fatalf("DestroySession before wire: %#v", got)
	}
	if got := api.SyncBackWorkspace("missing"); got.Error == nil {
		t.Fatalf("SyncBack before wire: %#v", got)
	}
	if got := api.GetRuntimeHealth(); got.Available || got.Error == nil {
		t.Fatalf("health without runtime: %#v", got)
	}
	app.SetSessionService(&recordingSession{})
	if got := api.GetSession("id"); got.Error != nil {
		t.Fatalf("GetSession: %#v", got)
	}
	if got := api.StopSession("id"); got.Error != nil || got.Session == nil {
		t.Fatalf("StopSession: %#v", got)
	}
	if got := api.DestroySession("id"); got.Error != nil || got.Session == nil {
		t.Fatalf("DestroySession: %#v", got)
	}
	if got := api.SyncBackWorkspace("id"); got.Error != nil || got.Session == nil {
		t.Fatalf("SyncBack: %#v", got)
	}
	if got := api.WritePty(contracts.PtyInput{SessionID: "id", Data: "x"}); got.Error == nil || got.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("WritePty placeholder: %#v", got)
	}
	if got := api.ResizePty(contracts.PtyInput{SessionID: "id"}); got.Error == nil || got.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("ResizePty placeholder: %#v", got)
	}
}

func TestDesktopAccountsHostsAndPermissionOverview(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	if got := api.RemoveAccount("missing"); got.Error == nil || got.Error.Code != contracts.ErrAccountNotFound {
		t.Fatalf("missing account: %#v", got)
	}
	if got := api.RevokeRememberedHost(contracts.RevokeHostInput{}); got.Error == nil || got.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("empty host: %#v", got)
	}
	// 未知 host 成功幂等（CONTRACT）；同时清 Gate 会话扩权
	if got := api.RevokeRememberedHost(contracts.RevokeHostInput{Workspace: "/ws", Host: "api.example"}); got.Error != nil {
		t.Fatalf("unknown host should be idempotent: %#v", got)
	}
	app.rememberHost(contracts.RememberedHost{Workspace: "/ws", Host: "API.EXAMPLE", Scope: "project"})
	if got := api.ListRememberedHosts("/ws"); len(got.Hosts) != 1 {
		t.Fatalf("remembered hosts: %#v", got)
	}
	if got := api.RevokeRememberedHost(contracts.RevokeHostInput{Workspace: "/ws", Host: "api.example"}); got.Error != nil {
		t.Fatalf("revoke host: %#v", got)
	}
	if got := api.ListRememberedHosts("/ws"); len(got.Hosts) != 0 {
		t.Fatalf("after revoke: %#v", got)
	}
	if got := api.GetPermissionOverview(""); got.Error != nil || got.Overview == nil || len(got.Overview.DeniedPathCategories) == 0 {
		t.Fatalf("permission overview: %#v", got)
	}
	app.SetSessionService(&recordingSession{})
	if got := api.GetPermissionOverview("session"); got.Error == nil || got.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("unsupported session overview: %#v", got)
	}
}
