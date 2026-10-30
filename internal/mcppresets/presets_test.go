package mcppresets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogHasFirstBatch(t *testing.T) {
	c := Catalog()
	if len(c) < 3 {
		t.Fatalf("%d", len(c))
	}
	ids := map[string]bool{}
	for _, p := range c {
		ids[p.ID] = true
	}
	for _, id := range []string{"harbor-time", "harbor-filesystem", "context7"} {
		if !ids[id] {
			t.Fatal(id)
		}
	}
}

func TestPrepareTime(t *testing.T) {
	dest := t.TempDir()
	launch, err := Prepare("harbor-time", dest, "")
	if err != nil {
		t.Fatal(err)
	}
	if launch.Transport != "stdio" || launch.Command == "" || len(launch.Args) != 1 {
		t.Fatalf("%+v", launch)
	}
	if _, err := os.Stat(launch.Args[0]); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareFilesystemNeedsRoot(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0600)
	dest := t.TempDir()
	launch, err := Prepare("harbor-filesystem", dest, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(launch.Args) != 2 || launch.Args[1] != root {
		t.Fatalf("%+v", launch)
	}
}

func TestEnsureSampleWorkspace(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ws")
	if err := ensureSampleWorkspace(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareContext7(t *testing.T) {
	launch, err := Prepare("context7", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !launch.NeedsAPIKey || launch.Origin == "" {
		t.Fatalf("%+v", launch)
	}
}
