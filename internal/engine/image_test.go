package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureImageBuildsWhenMissing(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	_ = os.MkdirAll(bin, 0700)
	dockerBin := filepath.Join(bin, "docker")
	colimaBin := filepath.Join(bin, "colima")
	_ = os.WriteFile(dockerBin, []byte("x"), 0755)
	_ = os.WriteFile(colimaBin, []byte("x"), 0755)
	_ = os.WriteFile(filepath.Join(bin, "limactl"), []byte("x"), 0755)

	dfDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dfDir, "Dockerfile"), []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var sawBuild bool
	c := NewColima()
	c.Root = root
	c.lookPath = func(string) (string, error) { return "", errors.New("x") }
	c.run = func(_ context.Context, name string, args []string, _ []string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "image inspect") {
			return "", errors.New("missing")
		}
		if strings.Contains(joined, "info") {
			return "24.0.0", nil
		}
		if name == dockerBin && args[0] == "build" {
			sawBuild = true
			return "", nil
		}
		return "", errors.New("unexpected " + joined)
	}
	if err := c.EnsureImage(context.Background(), dfDir); err != nil {
		t.Fatal(err)
	}
	if !sawBuild {
		t.Fatal("expected docker build")
	}
}
