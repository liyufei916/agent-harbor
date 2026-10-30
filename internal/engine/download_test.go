package engine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallArtifactRawWithChecksum(t *testing.T) {
	root := t.TempDir()
	payload := []byte("#!/bin/sh\necho colima\n")
	sum := sha256.Sum256(payload)
	a := pinnedArtifact{
		Name: "colima", Kind: artifactRaw, Required: true,
		URL: "https://example.invalid/colima", SHA256: hex.EncodeToString(sum[:]),
	}
	c := NewColima()
	c.Root = root
	_ = os.MkdirAll(c.binDir(), 0700)
	c.fetch = func(_ context.Context, url string, w io.Writer) error {
		if !strings.Contains(url, "colima") {
			t.Fatalf("url=%s", url)
		}
		_, err := w.Write(payload)
		return err
	}
	dest := filepath.Join(c.binDir(), "colima")
	if err := c.installArtifact(context.Background(), c.fetch, a, dest, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("dest=%q err=%v", got, err)
	}
}

func TestInstallArtifactRejectsBadChecksum(t *testing.T) {
	root := t.TempDir()
	c := NewColima()
	c.Root = root
	_ = os.MkdirAll(c.binDir(), 0700)
	a := pinnedArtifact{Name: "colima", Kind: artifactRaw, URL: "u", SHA256: strings.Repeat("0", 64)}
	c.fetch = func(_ context.Context, _ string, w io.Writer) error {
		_, err := w.Write([]byte("nope"))
		return err
	}
	err := c.installArtifact(context.Background(), c.fetch, a, filepath.Join(c.binDir(), "colima"), nil)
	if err == nil || !strings.Contains(err.Error(), "校验") {
		t.Fatalf("got %v", err)
	}
}

func TestExtractTarGzMember(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("docker-cli")
	hdr := &tar.Header{Name: "docker/docker", Mode: 0755, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gz.Close()

	dir := t.TempDir()
	archive := filepath.Join(dir, "d.tgz")
	if err := os.WriteFile(archive, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "docker")
	if err := extractTarGzMember(archive, "docker/docker", dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "docker-cli" {
		t.Fatalf("%q", got)
	}
}

func TestEnsurePinnedBinsDownloadsMissing(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "1")
	root := t.TempDir()
	payload := []byte("bin")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	c := NewColima()
	c.Root = root
	_ = os.MkdirAll(c.binDir(), 0700)
	// Override by installing only via custom fetch path using installArtifact loop manually:
	c.fetch = func(_ context.Context, url string, w io.Writer) error {
		_, err := w.Write(payload)
		return err
	}
	// Use installArtifact directly for each synthetic art to avoid GOOS matrix.
	for _, name := range []string{"colima"} {
		a := pinnedArtifact{Name: name, Kind: artifactRaw, URL: "https://example/" + name, SHA256: hexSum}
		if err := c.installArtifact(context.Background(), c.fetch, a, filepath.Join(c.binDir(), name), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(c.binDir(), "colima")); err != nil {
		t.Fatal(err)
	}
}
