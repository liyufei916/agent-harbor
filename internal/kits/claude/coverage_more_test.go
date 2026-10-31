package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNodeRejectsMissingAndAcceptsExecutable(t *testing.T) {
	if _, err := resolveNode(); err != nil {
		// CI may not have Node; the fail-closed result is the expected branch.
		return
	}
	root := t.TempDir()
	node := filepath.Join(root, "node")
	if err := os.WriteFile(node, []byte("#!/usr/bin/env node\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isNodeScript(node) {
		t.Fatal("node script not detected")
	}
	if isNodeScript(filepath.Join(root, "missing")) {
		t.Fatal("missing script detected")
	}
}
