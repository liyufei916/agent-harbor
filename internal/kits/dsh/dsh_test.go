package dsh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

func TestResolvePathsKeepsExplicit(t *testing.T) {
	node, inst := ResolvePaths("/custom/bin/node", "/custom/dsh-root")
	if node != "/custom/bin/node" || inst != "/custom/dsh-root" {
		t.Fatalf("explicit paths changed: %q %q", node, inst)
	}
}

func TestResolvePathsDefaultsWhenEmpty(t *testing.T) {
	node, inst := ResolvePaths("", "")
	wantInst := DefaultInstallation()
	if inst != wantInst {
		t.Fatalf("installation=%q want %q", inst, wantInst)
	}
	bundled := DefaultNode(wantInst)
	if _, err := os.Stat(bundled); err == nil {
		if node != bundled {
			t.Fatalf("node=%q want bundled %q", node, bundled)
		}
		return
	}
	if node != "node" {
		t.Fatalf("without install, node=%q want node", node)
	}
}

func TestNewUsesDefaultInstallLayout(t *testing.T) {
	bundled := DefaultNode(DefaultInstallation())
	if _, err := os.Stat(bundled); err != nil {
		t.Skip("本机未装默认 dsh 布局")
	}
	k, err := New("", "", "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	if k.node != bundled || k.installation != DefaultInstallation() {
		t.Fatalf("got node=%q install=%q", k.node, k.installation)
	}
}

func TestInstalledDSHBootstrap(t *testing.T) {
	if os.Getenv("HARBOR_TEST_DSH") != "1" {
		t.Skip("需要明确启用本机 dsh 检测")
	}
	k, _ := New(os.Getenv("HARBOR_DSH_NODE"), os.Getenv("HARBOR_DSH_INSTALLATION"), "fixture-model")
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available {
		t.Fatalf("检测失败: %+v %v", info, err)
	}
	ws, err := os.MkdirTemp("/private/tmp", "harbor-dsh-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(ws)
	r := runtime.NewMacOS()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	h, err := r.Create(ctx, runtime.RuntimeSpec{SessionID: "dsh-test", Workspace: ws, MountMode: "direct", AllowedTCP: []string{"127.0.0.1:1"}, ReadOnlyPaths: info.ReadOnlyPaths})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Destroy(context.Background(), h); err != nil {
			t.Error(err)
		}
	}()
	launch, err := k.Build(ctx, kits.KitInput{Task: "Return OK", PrivateHome: h.PrivateHome, ModelBaseURL: "http://127.0.0.1:1", MCPURL: "http://127.0.0.1:1/mcp", Sentinel: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ws, "output.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := r.Exec(ctx, h, launch, func(d contracts.OutputData) { f.WriteString(d.Text) })
	if err != nil {
		t.Fatal(err)
	}
	exit := p.Wait()
	raw, _ := os.ReadFile(path)
	t.Logf("退出=%+v\n%s", exit, raw)
	if exit.Code == 0 {
		t.Fatal("没有上游不能成功")
	}
	if !strings.Contains(string(raw), "connection (required)") || strings.Contains(string(raw), `"type":"session"`) {
		t.Fatal("MCP 不可用时必须拒绝启动任务")
	}
}
