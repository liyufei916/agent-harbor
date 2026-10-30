package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncWorkspaceOverwritesAndKeepsExtra(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(src, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dst, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "a.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "nested", "a.txt"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "only-dst.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SyncWorkspace(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "nested", "a.txt")); err != nil || string(data) != "new" {
		t.Fatalf("未覆盖：%q %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "only-dst.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("不应删除 destination 多余文件：%q %v", data, err)
	}
}

func TestSyncWorkspaceSkipsSymlinks(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "ok.txt"), []byte("harbor-syncback-ok"), 0600); err != nil {
		t.Fatal(err)
	}
	debug := filepath.Join(src, ".claude", "debug")
	if err := os.MkdirAll(debug, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(debug, "log.txt")
	if err := os.WriteFile(target, []byte("dbg"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(debug, "latest")); err != nil {
		t.Fatal(err)
	}
	if err := SyncWorkspace(context.Background(), src, dst); err != nil {
		t.Fatalf("导回不应因符号链接失败: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "ok.txt")); err != nil || string(data) != "harbor-syncback-ok" {
		t.Fatalf("常规文件未导回: %q %v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(dst, ".claude", "debug", "latest")); !os.IsNotExist(err) {
		t.Fatalf("符号链接不应被复制: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, ".claude", "debug", "log.txt")); err != nil || string(data) != "dbg" {
		t.Fatalf("链接目标常规文件应导回: %q %v", data, err)
	}
}

func TestSyncWorkspaceRejectsSameAndNested(t *testing.T) {
	dir := t.TempDir()
	if err := SyncWorkspace(context.Background(), dir, dir); err == nil {
		t.Fatal("同路径应拒绝")
	}
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := SyncWorkspace(context.Background(), dir, child); err == nil {
		t.Fatal("嵌套目标应拒绝")
	}
}
