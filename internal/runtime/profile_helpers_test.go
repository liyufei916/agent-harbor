package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileHelpersValidateInputs(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{`abc`, `"abc"`},
		{`a\\b"c`, `"a\\\\b\"c"`},
	} {
		got, err := sbQuote(tc.value)
		if err != nil || got != tc.want {
			t.Errorf("sbQuote(%q) = %q, %v", tc.value, got, err)
		}
	}
	if _, err := sbQuote("bad\x00value"); err == nil {
		t.Fatal("control character accepted")
	}
	for _, tc := range []struct {
		address string
		ok      bool
	}{
		{"127.0.0.1:1", true}, {"127.0.0.1:65535", true}, {"localhost:1", false}, {"127.0.0.1:0", false}, {"127.0.0.1:65536", false}, {"127.0.0.1:01", false}, {"bad", false},
	} {
		_, err := endpointPort(tc.address)
		if (err == nil) != tc.ok {
			t.Errorf("endpointPort(%q) error = %v", tc.address, err)
		}
	}
	if !containsPath([]string{"a", "b"}, "b") || containsPath([]string{"a"}, "c") {
		t.Fatal("containsPath mismatch")
	}
	if !within("/tmp/a", "/tmp/a/file") || within("/tmp/a", "/tmp/ab") {
		t.Fatal("within mismatch")
	}
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(file)
	if got, err := canonicalPath(file); err != nil || got != resolved {
		t.Fatalf("canonicalPath = %q, %v", got, err)
	}
	if _, err := canonicalPath("relative"); err == nil {
		t.Fatal("relative path accepted")
	}
	if _, err := canonicalPath(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
}

func TestCloneWorkspaceRejectsInvalidSourceAndCancellation(t *testing.T) {
	if root, err := sessionRoot("direct", ""); err != nil {
		t.Fatal(err)
	} else {
		_ = os.RemoveAll(root)
	}
	if root, err := sessionRoot("clone", t.TempDir()); err != nil {
		t.Fatal(err)
	} else {
		_ = os.RemoveAll(root)
	}
	if err := cloneWorkspace(context.Background(), filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "dest")); err == nil {
		t.Fatal("missing source accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	if err := cloneWorkspace(ctx, src, dst); err == nil {
		t.Fatal("cancelled clone succeeded")
	}
}
