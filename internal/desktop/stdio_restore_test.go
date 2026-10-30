package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/mcpstdio"
	"github.com/liyufei916/agent-harbor/internal/policy"
)

func TestRestoreStdioBridgesReloadsHostPort(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stdio fixture uses python3")
	}
	py, err := filepath.Abs(filepath.Join("..", "mcpstdio", "testdata", "fake_mcp.py"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(py); err != nil {
		t.Fatal(err)
	}
	python := findAbsPython3(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	app := NewApp()
	secret, err := mcpstdio.EncodeSecret(mcpstdio.Spec{Command: python, Args: []string{py}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := app.secrets.Put(ctx, contracts.AccountInfo{
		Kind: "mcp", Name: "local-echo", ProfileID: policy.ProfileCustom,
		Host: "127.0.0.1:1", Transport: "stdio", Command: python, AuthMode: "stdio",
		Tools: []string{"echo"},
	}, secret)
	if err != nil {
		t.Fatal(err)
	}

	n := app.RestoreStdioBridges(ctx)
	if n != 1 {
		t.Fatalf("restored=%d want 1", n)
	}
	info, err := app.secrets.Lookup(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if info.Host == "" || info.Host == "127.0.0.1:1" || !strings.HasPrefix(info.Host, "127.0.0.1:") {
		t.Fatalf("host not refreshed: %q", info.Host)
	}
	if len(info.Tools) != 1 || info.Tools[0] != "echo" {
		t.Fatalf("tools=%v", info.Tools)
	}

	// 模拟二次重启：关掉桥后再 Restore，端口应再次变化或至少仍可达。
	oldHost := info.Host
	app.closeAllStdioBridges()
	n = app.RestoreStdioBridges(ctx)
	if n != 1 {
		t.Fatalf("second restore=%d", n)
	}
	info2, err := app.secrets.Lookup(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if info2.Host == "" || !strings.HasPrefix(info2.Host, "127.0.0.1:") {
		t.Fatalf("second host=%q", info2.Host)
	}
	// 端口通常不同；若碰巧相同也接受（极少）
	_ = oldHost

	if err := app.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	app.mu.Lock()
	left := len(app.stdioBridges)
	app.mu.Unlock()
	if left != 0 {
		t.Fatalf("bridges after shutdown: %d", left)
	}
}

func findAbsPython3(t *testing.T) string {
	t.Helper()
	for _, cand := range []string{"/usr/bin/python3", "/usr/local/bin/python3", "python3"} {
		if path, err := exec.LookPath(cand); err == nil {
			if abs, absErr := filepath.Abs(path); absErr == nil {
				return abs
			}
			return path
		}
		if filepath.IsAbs(cand) {
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
		}
	}
	t.Skip("python3 not found")
	return ""
}
