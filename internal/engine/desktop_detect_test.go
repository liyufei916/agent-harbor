package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComponentsPresentEmpty(t *testing.T) {
	c := NewColima()
	c.Root = t.TempDir()
	c.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if c.ComponentsPresent() {
		t.Fatal("expected missing")
	}
}

func TestComponentsPresentLocalBin(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	_ = os.MkdirAll(bin, 0700)
	for _, name := range []string{"colima", "docker", "limactl"} {
		_ = os.WriteFile(filepath.Join(bin, name), []byte("x"), 0755)
	}
	c := NewColima()
	c.Root = root
	c.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if !c.ComponentsPresent() {
		t.Fatal("expected present")
	}
}

func TestDockerDesktopInstalledDoesNotPanic(t *testing.T) {
	_ = DockerDesktopInstalled()
}
