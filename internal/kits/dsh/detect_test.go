package dsh

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeDSHStubInstall(t *testing.T, dir, nodeVersion, pkgVersion string) (node, root string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	node = filepath.Join(dir, "node")
	root = filepath.Join(dir, "install")
	pkgDir := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh")
	script := filepath.Join(pkgDir, "lib", "bin.js")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '%s\\n' '" + nodeVersion + "'; exit 0; fi\n" +
		"case \"$2\" in\n" +
		"  --version) printf '%s\\n' '" + pkgVersion + "'; exit 0 ;;\n" +
		"  --help) printf '%s\\n' 'usage --profile harbor headless'; exit 0 ;;\n" +
		"esac\n" +
		"echo unexpected \"$@\" >&2; exit 1\n"
	if err := os.WriteFile(node, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("// stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]string{"name": "@deepseek-ai/dsh", "version": pkgVersion})
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	return node, root
}

func TestDetectStaysUnavailableWhenNodeVersionWrong(t *testing.T) {
	node, root := writeDSHStubInstall(t, t.TempDir(), "v0.0.0", SupportedVersion)
	k, err := New(node, root, "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	info, err := k.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Agent.Available {
		t.Fatalf("wrong node version must be unavailable: %+v", info)
	}
}

func TestPathsRejectsBrokenManifest(t *testing.T) {
	dir := t.TempDir()
	node, root := writeDSHStubInstall(t, dir, SupportedNodeVersion, "0.0.0-not-supported")
	k, err := New(node, root, "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = k.paths()
	if err == nil {
		t.Fatal("unsupported package version must fail paths()")
	}
}

func TestPathsAcceptsSupportedLayout(t *testing.T) {
	node, root := writeDSHStubInstall(t, t.TempDir(), SupportedNodeVersion, SupportedVersion)
	k, err := New(node, root, "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	gotNode, gotRoot, script, err := k.paths()
	if err != nil {
		t.Fatal(err)
	}
	if gotNode == "" || gotRoot == "" || script == "" {
		t.Fatalf("empty paths: %q %q %q", gotNode, gotRoot, script)
	}
}

func TestDetectAvailableWithInjectedProbe(t *testing.T) {
	node, root := writeDSHStubInstall(t, t.TempDir(), SupportedNodeVersion, SupportedVersion)
	k, err := New(node, root, "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	k.probe = func(_ context.Context, command string, args, _ []string) (string, error) {
		if len(args) == 1 && args[0] == "--version" {
			return SupportedNodeVersion + "\n", nil
		}
		if len(args) >= 2 && args[1] == "--version" {
			return SupportedVersion + "\n", nil
		}
		if len(args) >= 2 && args[1] == "--help" {
			return "usage --profile harbor headless\n", nil
		}
		return "", errors.New("unexpected probe " + command)
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available || info.Agent.Version != SupportedVersion {
		t.Fatalf("%+v err=%v", info, err)
	}
	if !info.NeedsDescendants || len(info.ReadOnlyPaths) < 2 {
		t.Fatalf("readonly: %+v", info)
	}
}
