package dsh

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDSHPathsValidation(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh")
	if err := os.MkdirAll(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(pkg, "lib", "bin.js")
	if err := os.MkdirAll(filepath.Dir(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env node\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "package.json"), []byte(`{"name":"@deepseek-ai/dsh","version":"0.2.1-rc.2"}`), 0600); err != nil {
		t.Fatal(err)
	}
	k, err := New("/usr/bin/true", root, "model")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, _ := filepath.EvalSymlinks(root)
	canonicalScript, _ := filepath.EvalSymlinks(script)
	node, resolvedRoot, resolvedScript, err := k.paths()
	if err != nil || node != "/usr/bin/true" || resolvedRoot != canonicalRoot || resolvedScript != canonicalScript {
		t.Fatalf("paths = %q %q %q, %v", node, resolvedRoot, resolvedScript, err)
	}
	bad := *k
	bad.installation = filepath.Join(root, "missing")
	if _, _, _, err := bad.paths(); err == nil {
		t.Fatal("missing installation accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := k.Detect(ctx); err == nil {
		t.Fatal("cancelled Detect succeeded")
	}
}
