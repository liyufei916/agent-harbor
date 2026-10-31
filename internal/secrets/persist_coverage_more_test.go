package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentMetadataLoadSaveBranches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	if err := os.WriteFile(path, []byte(`{"accounts":[{"id":"ok","kind":"model","name":"n"},{"id":""}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{byID: map[string]*record{}, vault: newMemoryVault(), metaPath: path}
	if err := s.loadMeta(); err != nil || len(s.byID) != 1 {
		t.Fatalf("load=%v records=%v", err, s.byID)
	}
	if err := s.saveMetaLocked(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
		t.Fatalf("saved metadata=%q err=%v", data, err)
	}
	noPath := &Store{byID: map[string]*record{}, vault: newMemoryVault()}
	if err := noPath.saveMetaLocked(); err != nil {
		t.Fatal(err)
	}
	bad := &Store{byID: map[string]*record{}, vault: newMemoryVault(), metaPath: filepath.Join(dir, "bad")}
	if err := os.WriteFile(bad.metaPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := bad.loadMeta(); err == nil {
		t.Fatal("malformed metadata accepted")
	}
}
