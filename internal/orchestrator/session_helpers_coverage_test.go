package orchestrator

import (
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"os"
	"path/filepath"
	"testing"
)

func TestHasActiveSessionAndCanonicalWorkspaceBranches(t *testing.T) {
	o := &Service{sessions: map[string]*session{}}
	if hasActiveSessionLocked(o) {
		t.Fatal("empty service has active session")
	}
	if got, err := canonicalWorkspace(""); err == nil || got != "" {
		t.Fatalf("empty workspace accepted: %q %v", got, err)
	}
	if got, err := canonicalWorkspace("bad\x00path"); err == nil || got != "" {
		t.Fatalf("NUL workspace accepted: %q %v", got, err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	got, err := canonicalWorkspace(missing)
	if err != nil || got != missing {
		t.Fatalf("missing workspace=%q err=%v", got, err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !sameWorkspace(file, file) || sameWorkspace(file, filepath.Join(filepath.Dir(file), "other")) {
		t.Fatal("sameWorkspace result incorrect")
	}
	o.sessions["active"] = &session{snapshot: contracts.SessionSnapshot{CleanupComplete: false}}
	if !hasActiveSessionLocked(o) {
		t.Fatal("active session not detected")
	}
	o.sessions["active"].snapshot.CleanupComplete = true
	if hasActiveSessionLocked(o) {
		t.Fatal("cleaned session reported active")
	}
}
