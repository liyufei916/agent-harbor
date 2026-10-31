package main

import (
	"context"
	"embed"
	"log"
	"net/url"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[contracts.Event](contracts.HarborEventName)
	application.RegisterEvent[contracts.EngineProgressPayload](contracts.EngineProgressEventName)
}

func main() {
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	host := desktop.NewDesktopApp()
	if err := host.AttachOrchestrator(appCtx); err != nil {
		log.Fatal(err)
	}
	api := desktop.NewDesktopAPI(host)
	live, err := desktop.MaybeStartHTTPLive(api)
	if err != nil {
		log.Fatal(err)
	}
	if live != nil {
		defer func() { _ = live.Close() }()
		q := url.Values{}
		q.Set("transport", "http-live")
		q.Set("token", live.Token())
		q.Set("bridge", live.URL())
		log.Printf("http-live bridge %s", live.URL())
		log.Printf("browser: http://127.0.0.1:5173/?%s", q.Encode())
	}
	options := desktopOptions(host, api, cancelApp)
	app := application.New(options)
	newDesktopWindow(app, options.ShouldQuit)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// desktopOptions 由正式入口和原生桌面冒烟测试共用。
func desktopOptions(host *desktop.App, api *desktop.DesktopAPI, cancelApp context.CancelFunc) application.Options {
	if api == nil {
		api = desktop.NewDesktopAPI(host)
	}
	gate := newExitGate(host.Shutdown, func() { application.Get().Quit() }, func() {
		application.Get().Dialog.Error().SetTitle("清理未完成").SetMessage("应用已保留窗口。请重试停止或退出，确认隔离资源清理完成后再关闭。").Show()
	})
	return application.Options{
		Name:        "Agent Harbor",
		Description: "密钥留在本机 · Agent 隔离运行",
		Services: []application.Service{
			application.NewService(api),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		ShouldQuit: gate.request,
		OnShutdown: cancelApp,
	}
}

func newDesktopWindow(app *application.App, mayClose func() bool) *application.WebviewWindow {
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Agent Harbor",
		Width:  1100,
		Height: 720,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBarDefault,
		},
		BackgroundColour: application.NewRGB(24, 26, 32),
		URL:              "/",
	})
	win.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !mayClose() {
			event.Cancel()
		}
	})
	return win
}
