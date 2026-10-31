package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcceptanceHelpers(t *testing.T) {
	if short("abcdef") != "abcdef" || short("abcdefghijklmnopqrstuvwxyz") != "abcdefgh…" {
		t.Fatal("short")
	}
	if !contains([]string{"x"}, "x") || contains([]string{"x"}, "y") {
		t.Fatal("contains")
	}
	if !looksLikeFullKey("sk-abcdefghijklmnopqrstuvwxyz1234567890") || containsSecretish("api_key=secret") {
		t.Fatal("secret")
	}
	p := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(p, 0700); err != nil {
		t.Fatal(err)
	}
	if !dirExists(p) || fileExists(p) {
		t.Fatal("paths")
	}
}
