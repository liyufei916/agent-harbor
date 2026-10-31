package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsurePinnedBinsSkipsInstalledAndHonorsDisabledDownload(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "0")
	root := t.TempDir()
	c := NewColima()
	c.Root = root
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"colima", "docker", "limactl"} {
		if err := os.WriteFile(filepath.Join(root, "bin", name), []byte("x"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.ensurePinnedBins(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(root, "bin", "docker"))
	if err := c.ensurePinnedBins(context.Background()); err == nil {
		t.Fatal("disabled download succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.ensurePinnedBins(ctx); err == nil {
		t.Fatal("cancelled download succeeded")
	}
}

func TestInstallArtifactRawAndErrors(t *testing.T) {
	root := t.TempDir()
	c := NewColima()
	c.Root = root
	payload := []byte("binary")
	sum := sha256.Sum256(payload)
	a := pinnedArtifact{Name: "tool", URL: "test://tool", SHA256: hex.EncodeToString(sum[:]), Kind: artifactRaw, ExpectedBytes: int64(len(payload))}
	fetch := func(_ context.Context, _ string, w io.Writer) error { _, err := w.Write(payload); return err }
	if err := c.installArtifact(context.Background(), fetch, a, filepath.Join(root, "tool"), nil); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "tool")); err != nil || string(got) != string(payload) {
		t.Fatalf("installed=%q %v", got, err)
	}

	bad := a
	bad.SHA256 = "bad"
	if err := c.installArtifact(context.Background(), func(_ context.Context, _ string, w io.Writer) error { _, _ = w.Write(payload); return nil }, bad, filepath.Join(root, "bad"), nil); err == nil {
		t.Fatal("hash mismatch accepted")
	}
	if err := c.installArtifact(context.Background(), func(context.Context, string, io.Writer) error { return errors.New("network") }, a, filepath.Join(root, "fail"), nil); err == nil {
		t.Fatal("fetch failure accepted")
	}
	archive := makeCoverageArchive(t)
	archiveData, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	sumArchive := sha256.Sum256(archiveData)
	tarArtifact := pinnedArtifact{Name: "tool", URL: "test://archive", SHA256: hex.EncodeToString(sumArchive[:]), Kind: artifactTarGz, ExtractRoot: "lima"}
	if err := c.installArtifact(context.Background(), func(_ context.Context, _ string, w io.Writer) error { _, err := w.Write(archiveData); return err }, tarArtifact, filepath.Join(root, "bin", "tool"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "lima", "bin", "tool")); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultFetchResponsesAndProgressWriter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		if r.URL.Path == "/ok" {
			_, _ = w.Write([]byte("payload"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := defaultFetch(context.Background(), server.URL+"/ok", &out); err != nil || out.String() != "payload" {
		t.Fatalf("fetch=%q %v", out.String(), err)
	}
	if err := defaultFetch(context.Background(), server.URL+"/missing", &out); err == nil {
		t.Fatal("404 fetch succeeded")
	}
	var seen []int64
	pw := &progressWriter{w: &out, on: func(n int64) { seen = append(seen, n) }}
	_, _ = pw.Write(make([]byte, 256*1024))
	if len(seen) == 0 {
		t.Fatal("progress not reported")
	}
}

func TestColimaEnsureImageWithLocalBinaries(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"colima", "docker", "limactl"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("x"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	c := NewColima()
	c.Root = root
	c.lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	c.run = func(_ context.Context, name string, args []string, _ []string) (string, error) {
		if len(args) > 0 && args[0] == "info" {
			return "27.0", nil
		}
		if len(args) > 0 && args[0] == "image" {
			return "id", nil
		}
		return "", nil
	}
	if err := c.EnsureImage(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
