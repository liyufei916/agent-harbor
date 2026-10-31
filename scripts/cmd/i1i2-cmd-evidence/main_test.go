package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceHelpers(t *testing.T) {
	if !contains([]string{"a", "b"}, "b") || contains([]string{"a"}, "z") {
		t.Fatal("contains")
	}
	p := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !fileExists(p) || fileExists(filepath.Join(t.TempDir(), "none")) {
		t.Fatal("fileExists")
	}
}
