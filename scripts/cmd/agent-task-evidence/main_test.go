package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceHelpers(t *testing.T) {
	if got := splitCSV(" a, b,,c "); len(got) != 3 {
		t.Fatalf("%v", got)
	}
	if envOr("AGENT_HARBOR_TEST_MISSING", "fallback") != "fallback" {
		t.Fatal("env")
	}
	if !contains([]string{"a"}, "a") || agentAvailable(nil, "x") {
		t.Fatal("helpers")
	}
	p := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !fileExists(p) || fileExists(filepath.Join(t.TempDir(), "none")) {
		t.Fatal("file")
	}
}
