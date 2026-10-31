package gate

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
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func findPython3(t *testing.T) string {
	t.Helper()
	for _, cand := range []string{"/usr/bin/python3", "/usr/local/bin/python3", "python3"} {
		if path, err := exec.LookPath(cand); err == nil {
			if abs, aerr := filepath.Abs(path); aerr == nil {
				return abs
			}
			return path
		}
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(cand)
			return abs
		}
	}
	t.Skip("python3 not found")
	return ""
}

type stdioKeeper struct {
	id     string
	bridge *mcpstdio.Bridge
}

func (k *stdioKeeper) AdoptStdioBridge(accountID string, bridge *mcpstdio.Bridge) {
	k.id = accountID
	k.bridge = bridge
}

func TestConnectStdioMCPPersistsLoopbackAccount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stdio fixture uses python3")
	}
	pyFixture, err := filepath.Abs(filepath.Join("..", "mcpstdio", "testdata", "fake_mcp.py"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pyFixture); err != nil {
		t.Fatal(err)
	}
	python := findPython3(t)
	store := secrets.NewStore()
	keeper := &stdioKeeper{}
	conn := NewConnections(store, policy.NewBuilder(nil)).WithStdioKeeper(keeper)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	acc, err := conn.ConnectMcp(ctx, contracts.ConnectInput{
		Name:      "stdio-fixture",
		Transport: "stdio",
		Command:   python,
		Args:      []string{pyFixture},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if keeper.bridge != nil {
			_ = keeper.bridge.Close()
		}
	})
	if acc.ID == "" || acc.Transport != "stdio" || acc.AuthMode != "stdio" {
		t.Fatalf("account %+v", acc)
	}
	if !strings.HasPrefix(acc.Host, "127.0.0.1:") {
		t.Fatalf("host want loopback got %q", acc.Host)
	}
	if len(acc.Tools) != 1 || acc.Tools[0] != "echo" {
		t.Fatalf("tools %+v", acc)
	}
	if store.Count() != 1 {
		t.Fatalf("store count %d", store.Count())
	}
	if keeper.id != acc.ID || keeper.bridge == nil {
		t.Fatalf("keeper id=%q bridge=%v", keeper.id, keeper.bridge != nil)
	}
	if !strings.Contains(keeper.bridge.Origin, "127.0.0.1") {
		t.Fatalf("bridge origin %q", keeper.bridge.Origin)
	}
}

func TestConnectStdioMCPRejectsEmptyCommand(t *testing.T) {
	store := secrets.NewStore()
	conn := NewConnections(store, policy.NewBuilder(nil))
	_, err := conn.ConnectMcp(context.Background(), contracts.ConnectInput{
		Name:      "x",
		Transport: "stdio",
		Command:   "",
	})
	if err == nil {
		t.Fatal("expected invalid input")
	}
	if store.Count() != 0 {
		t.Fatalf("must not store on failure: %d", store.Count())
	}
}
