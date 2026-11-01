package desktop

import (
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeUIPrefsBranches(t *testing.T) {
	for _, p := range []contracts.UIPrefs{{Theme: "light", ClearOnStopDefault: "clear", SecurityTier: "strict", IsolationBackend: "docker", EngineSource: "system"}, {Theme: "bad", ClearOnStopDefault: "bad", SecurityTier: "bad", IsolationBackend: "bad", EngineSource: "bad"}, {IsolationBackend: "p2", EngineSource: "colima"}} {
		got := normalizeUIPrefs(p)
		if got.SecurityTier == "" || got.IsolationBackend == "" || got.EngineSource == "" {
			t.Fatalf("%+v", got)
		}
	}
}
func TestPersistStateLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	a := NewAppWithDataDir(dir)
	a.prefs.Theme = "dark"
	a.hosts[hostMapKey("/w", "example.com")] = contracts.RememberedHost{Workspace: "/w", Host: "example.com"}
	a.savePersistedState()
	b := NewAppWithDataDir(dir)
	if b.prefs.Theme != "dark" || len(b.hosts) != 1 {
		t.Fatalf("prefs=%+v hosts=%v", b.prefs, b.hosts)
	}
	if err := os.WriteFile(filepath.Join(dir, "ui-state.json"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	c := NewAppWithDataDir(dir)
	if c == nil {
		t.Fatal("nil app")
	}
}

func TestFinishSaveTextFileBranches(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.txt")
	var revealed string
	if got := finishSaveTextFile("", nil, "x", false, nil); !got.Cancelled {
		t.Fatal(got)
	}
	if got := finishSaveTextFile(p, nil, "hello", true, func(s string) { revealed = s }); got.Path != p || revealed != p {
		t.Fatalf("save=%+v reveal=%q", got, revealed)
	}
	if got := finishSaveTextFile(filepath.Join(t.TempDir(), "missing", "x"), nil, "x", false, nil); got.Error == nil {
		t.Fatal("write error missing")
	}
	if got := finishSaveTextFile("", os.ErrClosed, "x", false, nil); got.Error == nil {
		t.Fatal("dialog error missing")
	}
}
