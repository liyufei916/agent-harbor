package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(p, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := hashFile(p); got == "" {
		t.Fatal("empty hash")
	}
	if hashFile(filepath.Join(t.TempDir(), "missing")) != "" {
		t.Fatal("missing hash")
	}
}
