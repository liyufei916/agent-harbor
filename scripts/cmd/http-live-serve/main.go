// 双终端 · 浏览器本机 live：loopback DesktopAPI + 可选同端口 UI（非托管 SaaS）。
//
//	go run ./scripts/cmd/http-live-serve
//
// 默认挂载 frontend/dist。打开日志 open: URL 即可（不必另开 Vite）。
// 热更新：cd frontend && npm run dev，再用打印的 5173 链接。
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/liyufei916/agent-harbor/internal/desktop"
)

func main() {
	_ = os.Setenv("HARBOR_HTTP_LIVE", "1")
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	uiDir := filepath.Join(root, "frontend", "dist")
	if _, err := os.Stat(filepath.Join(uiDir, "index.html")); err != nil {
		log.Printf("提示: 未找到 %s，先 npm run build --prefix frontend；根路径将只显示指引页", uiDir)
		uiDir = ""
	}

	host := desktop.NewDesktopApp()
	if err := host.AttachOrchestrator(appCtx); err != nil {
		log.Fatal(err)
	}
	api := desktop.NewDesktopAPI(host)
	cfg := desktop.HTTPLiveConfigFromEnv()
	cfg.UIDir = uiDir
	live, err := desktop.StartHTTPLive(api, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = live.Close() }()

	q := url.Values{}
	q.Set("transport", "http-live")
	q.Set("token", live.Token())
	q.Set("bridge", live.URL())
	fmt.Printf("http-live bridge: %s\n", live.URL())
	fmt.Printf("token: %s\n", live.Token())
	if uiDir != "" {
		fmt.Printf("open: %s/?%s\n", live.URL(), q.Encode())
		fmt.Println("(已挂载 frontend/dist，无需 Vite)")
	} else {
		fmt.Printf("open: http://127.0.0.1:5173/?%s\n", q.Encode())
		fmt.Println("(需另开: cd frontend && npm run dev)")
	}
	fmt.Println("health:", live.URL()+"/v1/health")
	fmt.Println("(Ctrl-C 退出；勿与另一编排实例同时开会话)")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	cancel()
	_ = host.Shutdown(context.Background())
}
