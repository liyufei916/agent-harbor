package desktop

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestAccountsPrefsAndRememberedHosts(t *testing.T) {
	app := NewApp()
	app.prefsPath = t.TempDir() + "/ui-prefs.json"
	app.SetSessionService(&recordingSession{})
	api := NewDesktopAPI(app)
	id, err := app.secrets.Put(context.Background(), contracts.AccountInfo{
		Kind: "model", Name: "Demo", ProfileID: "demo-model", Host: "api.example.test",
	}, []byte("secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	listed := api.ListAccounts("model")
	if listed.Error != nil || len(listed.Accounts) != 1 || listed.Accounts[0].ID != id {
		t.Fatalf("%+v", listed)
	}
	created := api.CreateSession(contracts.SessionSpec{
		Workspace: "/demo/project", AgentID: "claude", ModelAccountID: id, MCPIDs: []string{"mcp1"}, Task: "hello",
	})
	if created.Error != nil {
		t.Fatal(created.Error)
	}
	busy := api.RemoveAccount(id)
	if busy.Error == nil || busy.Error.Code != contracts.ErrSessionBusy {
		t.Fatalf("%+v", busy)
	}
	missing := api.RemoveAccount("missing")
	if missing.Error == nil || missing.Error.Code != contracts.ErrAccountNotFound {
		t.Fatalf("%+v", missing)
	}

	prefs := api.GetUIPrefs()
	if prefs.Error != nil || prefs.Prefs == nil || prefs.Prefs.AutoCollapseWorkbench != true {
		t.Fatalf("%+v", prefs)
	}
	saved := api.SetUIPrefs(contracts.UIPrefs{SidebarCollapsed: true, AutoCollapseWorkbench: false})
	if saved.Error != nil || !saved.Prefs.SidebarCollapsed || saved.Prefs.AutoCollapseWorkbench {
		t.Fatalf("%+v", saved)
	}

	if err := app.policy.RememberHost("docs.example.test", "/demo/project", "project"); err != nil {
		t.Fatal(err)
	}
	hosts := api.ListRememberedHosts("/demo/project")
	if hosts.Error != nil || len(hosts.Hosts) != 1 {
		t.Fatalf("%+v", hosts)
	}
	if revoked := api.RevokeRememberedHost(contracts.RevokeHostInput{Host: "missing.example", Workspace: "/demo/project"}); revoked.Error != nil {
		t.Fatal(revoked.Error)
	}
	overview := api.GetPermissionOverview("")
	if overview.Error != nil || overview.Overview == nil || overview.Overview.Workspace != "" || len(overview.Overview.DeniedPathCategories) == 0 {
		t.Fatalf("%+v", overview)
	}
	for _, host := range overview.Overview.AllowedHosts {
		if host == "" {
			t.Fatal("empty host")
		}
	}

	if sessions := api.ListSessions(); sessions.Error == nil || sessions.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("%+v", sessions)
	}
	if pty := api.WritePty(contracts.PtyInput{SessionID: "s", Data: "hi"}); pty.Error == nil || pty.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("%+v", pty)
	}
	t.Setenv("HARBOR_MCP_OAUTH_AUTHORIZE_URL", "")
	oauth := api.StartMcpOAuth(contracts.StartMcpOAuthInput{Name: "tool", Origin: "https://mcp.example.test"})
	if oauth.Error == nil {
		t.Fatal("missing oauth config should fail closed")
	}
	if cancelled := api.CancelMcpOAuth("none"); cancelled.Error != nil {
		t.Fatal(cancelled.Error)
	}
}
