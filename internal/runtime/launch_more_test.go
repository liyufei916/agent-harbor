package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLaunchEnvironmentAndConfigValidation(t *testing.T) {
	home := t.TempDir()
	if env, err := launchEnvironment(home, "", map[string]string{"ANTHROPIC_API_KEY": "key"}); err != nil || len(env) == 0 {
		t.Fatalf("environment: %v", err)
	}
	if env, err := launchEnvironment(home, "", map[string]string{"CODEX_HOME": filepath.Join(home, "codex")}); err != nil || len(env) == 0 {
		t.Fatalf("Codex environment: %v", err)
	}
	for _, extra := range []map[string]string{{"PATH": "/tmp"}, {"DSH_HOME": "/tmp"}, {"ANTHROPIC_API_KEY": "bad\x00value"}} {
		if _, err := launchEnvironment(home, "", extra); err == nil {
			t.Fatalf("invalid environment accepted: %#v", extra)
		}
	}
	for _, files := range [][]ConfigFile{
		{{RelativePath: "../escape", Content: []byte("x")}},
		{{RelativePath: "a", Content: []byte("x")}, {RelativePath: "a", Content: []byte("y")}},
		{{RelativePath: "dir", Content: nil}, {RelativePath: "dir/file", Content: []byte("x")}},
	} {
		if err := validateConfigFiles(files); err == nil {
			t.Fatalf("invalid config accepted: %#v", files)
		}
	}
	dir := t.TempDir()
	if err := writeConfigFiles(dir, []ConfigFile{{RelativePath: "nested/file", Content: []byte("ok")}}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "nested/file")); err != nil || string(got) != "ok" {
		t.Fatalf("written config: %q %v", got, err)
	}
	if err := writeConfigFiles(dir, []ConfigFile{{RelativePath: "nested/file", Content: []byte("again")}}); err == nil {
		t.Fatal("existing config overwritten")
	}
}

func TestSyncWorkspaceInvalidAndCancelled(t *testing.T) {
	if err := SyncWorkspace(context.Background(), "", t.TempDir()); err == nil {
		t.Fatal("empty source accepted")
	}
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "dst")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SyncWorkspace(ctx, src, dst); err == nil {
		t.Fatal("cancelled sync succeeded")
	}
	if err := SyncWorkspace(context.Background(), filepath.Join(t.TempDir(), "missing"), dst); err == nil {
		t.Fatal("missing source accepted")
	}
}
