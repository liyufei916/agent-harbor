//go:build darwin && harbor_desktop

package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// 原生 GUI 必须运行在主线程；单独的进程避免污染其他 Go 测试。
func TestMain(m *testing.M) {
	if os.Getenv("HARBOR_DESKTOP_SMOKE") != "" {
		runDesktopSmoke()
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestNativeDesktopBindingsAndShutdown(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "result")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable)
	cmd.Env = append(os.Environ(), "HARBOR_DESKTOP_SMOKE="+report)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("原生桌面验证失败：%v\n%s", err, output)
	}
	result, err := os.ReadFile(report)
	if err != nil || string(result) != "pass" {
		t.Fatalf("原生桌面证据缺失：%v %q\n%s", err, result, output)
	}
}

func runDesktopSmoke() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := desktop.NewApp()
	if err := host.AttachOrchestrator(ctx); err != nil {
		panic(err)
	}
	options := desktopOptions(host, desktop.NewDesktopAPI(host), cancel)
	base := options.Assets.Handler
	var passed atomic.Bool
	var win *application.WebviewWindow
	options.Assets.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_harbor_smoke/event":
			host.EmitHarborEvent(contracts.Event{SessionID: "desktop-smoke", Seq: 1,
				Type: contracts.EventTypeOutput, Output: &contracts.OutputData{Stream: "stdout", Text: "中文事件"}})
			w.WriteHeader(http.StatusNoContent)
		case "/_harbor_smoke/pass":
			passed.Store(true)
			w.WriteHeader(http.StatusNoContent)
			// 关闭最后一个窗口，验证正式 MacOptions 与 OnShutdown 路径。
			go win.Close()
		case "/_harbor_smoke/fail":
			fmt.Println("WebView 验证失败：", r.URL.Query().Get("reason"))
			w.WriteHeader(http.StatusNoContent)
			go win.Close()
		default:
			base.ServeHTTP(w, r)
		}
	})
	options.PostShutdown = func() {
		if passed.Load() && ctx.Err() != nil {
			if err := os.WriteFile(os.Getenv("HARBOR_DESKTOP_SMOKE"), []byte("pass"), 0600); err == nil {
				os.Exit(0)
			}
		}
		os.Exit(1)
	}
	app := application.New(options)
	win = newDesktopWindow(app, options.ShouldQuit)
	chunks, _ := fs.Glob(assets, "frontend/dist/assets/live-api-*.js")
	if len(chunks) != 1 {
		panic("请先执行 wails3 build，生成唯一的 live transport 产物")
	}
	chunk := "/assets/" + filepath.Base(chunks[0])
	win.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		win.ExecJS(fmt.Sprintf(`(async () => {
  const check = (ok, message) => { if (!ok) throw new Error(message); };
  // 桌面 live：顶栏不再展示「真实绑定」（对用户无信息量），syncBadge 清空文案并加 class live。
  const liveReady = () => {
    const b = document.getElementById('mode-badge');
    if (!b) return false;
    if (b.textContent === '真实绑定') return true;
    return b.classList.contains('live') && b.textContent !== '模拟模式';
  };
  for (let i = 0; i < 200 && !liveReady(); i++)
    await new Promise(resolve => setTimeout(resolve, 100));
  check(liveReady(), '桌面没有进入 live');
  const { createLiveTransport } = await import(%q);
  const api = createLiveTransport();
  const agents = await api.listAgents();
  check(!agents.error && agents.agents.length === 3 && agents.agents[0].id === 'claude' && agents.agents[1].id === 'codex' && agents.agents[2].id === 'dsh' && agents.agents[1].modelProfileId === 'codex-model', 'ListAgents');
  const missing = await api.getSession('desktop-smoke-unknown');
  check(missing.session === null && missing.error?.code === 'SESSION_NOT_FOUND', 'GetSession 错误 DTO');
  const stop = await api.stopSession('desktop-smoke-unknown');
  check(stop.error?.code === 'SESSION_NOT_FOUND', 'StopSession');
  const invalid = await api.createSession({workspace:'', agentId:'claude', modelAccountId:'', mcpIds:[], task:''});
  check(invalid.error?.code === 'INVALID_ARGUMENT', 'CreateSession 输入校验');
  for (const connect of [api.connectModel, api.connectMcp]) {
    const result = await connect({profileId:'', name:'', apiKey:''});
    check(result.account === null && result.error?.code === 'INVALID_ARGUMENT', '连接错误 DTO');
  }
  const off = api.onEvent(async event => {
    if (event.sessionId !== 'desktop-smoke') return;
    try {
      check(event.seq === 1 && event.output?.text === '中文事件', '事件载荷');
      off();
      await fetch('/_harbor_smoke/pass');
    } catch (error) { await fetch('/_harbor_smoke/fail?reason=' + encodeURIComponent(error.message)); }
  });
  check(typeof off === 'function', '取消事件订阅');
  await fetch('/_harbor_smoke/event');
})().catch(error => fetch('/_harbor_smoke/fail?reason=' + encodeURIComponent(error.message)));`, chunk))
	})
	if err := app.Run(); err != nil {
		panic(err)
	}
}
