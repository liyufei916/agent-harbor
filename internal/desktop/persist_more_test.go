package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPersistedStateIgnoresInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui-state.json")
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	app := NewAppWithDataDir(dir)
	if len(app.listRemembered("")) != 0 {
		t.Fatal("invalid state loaded")
	}
	if err := os.WriteFile(path, []byte(`{"prefs":{"sidebarCollapsed":true},"hosts":[{"host":""},{"host":"api.example","workspace":"/ws"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	app = NewAppWithDataDir(dir)
	if len(app.listRemembered("/ws")) != 1 || !app.getPrefs().SidebarCollapsed {
		t.Fatalf("persisted state: %#v %#v", app.listRemembered("/ws"), app.getPrefs())
	}
}
