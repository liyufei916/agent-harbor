package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeCoverageArchive(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload.tgz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, item := range []struct {
		name, body string
		mode       int64
	}{{"bin/tool", "#!/bin/sh\n", 0755}, {"docs/readme", "ok", 0644}} {
		h := &tar.Header{Name: item.name, Mode: item.mode, Size: int64(len(item.body))}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(item.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractTarGzHelpers(t *testing.T) {
	archive := makeCoverageArchive(t)
	dest := t.TempDir()
	if err := extractTarGzAll(archive, dest); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "bin", "tool")); err != nil || string(data) != "#!/bin/sh\n" {
		t.Fatalf("all=%q %v", data, err)
	}
	member := filepath.Join(t.TempDir(), "tool")
	if err := extractTarGzMember(archive, "./bin/tool", member); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(member); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzMember(archive, "missing", filepath.Join(t.TempDir(), "x")); err == nil {
		t.Fatal("missing member accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad")
	if err := os.WriteFile(bad, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzAll(bad, t.TempDir()); err == nil {
		t.Fatal("invalid archive accepted")
	}
}

func TestColimaStopAndEnsureImageFailures(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "0")
	c := NewColima()
	c.Root = t.TempDir()
	c.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureImage(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "缺少") {
		t.Fatalf("ensure image=%v", err)
	}
}
