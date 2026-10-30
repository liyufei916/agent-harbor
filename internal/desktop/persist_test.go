package desktop

import (
	"path/filepath"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestUIPrefsAndHostsPersist(t *testing.T) {
	dir := t.TempDir()
	a := NewAppWithDataDir(dir)
	a.setPrefs(contracts.UIPrefs{
		SidebarCollapsed: true, AutoCollapseWorkbench: false,
		Theme: "light", AskClearOnStop: false, ClearOnStopDefault: "clear",
	})
	a.rememberHost(contracts.RememberedHost{
		Workspace: "/tmp/ws", Host: "cdn.example.test", Scope: "project",
	})

	a2 := NewAppWithDataDir(dir)
	prefs := a2.getPrefs()
	if !prefs.SidebarCollapsed || prefs.AutoCollapseWorkbench {
		t.Fatalf("prefs=%+v", prefs)
	}
	if prefs.Theme != "light" || prefs.AskClearOnStop || prefs.ClearOnStopDefault != "clear" {
		t.Fatalf("theme/clear prefs=%+v", prefs)
	}
	hosts := a2.listRemembered("/tmp/ws")
	if len(hosts) != 1 || hosts[0].Host != "cdn.example.test" {
		t.Fatalf("hosts=%+v", hosts)
	}
	if a2.statePath() != filepath.Join(dir, "ui-state.json") {
		t.Fatalf("path=%s", a2.statePath())
	}
}
