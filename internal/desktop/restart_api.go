package desktop

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 测试可替换，避免 go test 把自身可执行文件再拉起一份 / os.Exit。
var (
	relaunchFn = spawnRelaunch
	quitFn     = func() {
		if app := application.Get(); app != nil {
			app.Quit()
		}
	}
	exitFn = func() { os.Exit(0) }
)

// RestartApp 拉起同路径新进程后退出本进程（经退出门清理会话）。
// Force=false 且存在进行中会话时返回 SESSION_BUSY，由前端确认后再 Force=true。
func (d *DesktopAPI) RestartApp(in contracts.RestartAppInput) contracts.RestartAppResponse {
	busy := d.countBusySessions()
	if !in.Force && busy > 0 {
		return contracts.RestartAppResponse{
			BusySessions: busy,
			Error: contracts.NewAppError(
				contracts.ErrSessionBusy,
				"有进行中的会话；确认后将停止并重启应用。",
				true,
			),
		}
	}
	// http-live：先放端口再拉起子进程，避免子进程 bind 失败。
	if d.app != nil {
		if hub := d.app.httpLiveRef(); hub != nil {
			_ = hub.Close()
		}
	}
	if err := relaunchFn(); err != nil {
		return contracts.RestartAppResponse{
			Error: contracts.NewAppError(contracts.ErrInternal, "无法拉起新进程，请手动重启应用。", true),
		}
	}
	// 异步触发退出，先让本响应回到前端。
	go func() {
		time.Sleep(250 * time.Millisecond)
		if application.Get() != nil {
			quitFn()
			return
		}
		// 无 Wails 壳（http-live-serve）：退出本进程。
		exitFn()
	}()
	return contracts.RestartAppResponse{Restarting: true}
}

func (d *DesktopAPI) countBusySessions() int {
	if d == nil || d.app == nil {
		return 0
	}
	ctxResp := d.ListSessions()
	if ctxResp.Error != nil {
		return 0
	}
	n := 0
	for _, s := range ctxResp.Sessions {
		switch strings.ToLower(strings.TrimSpace(s.State)) {
		case contracts.StateRunning, contracts.StateProvisioning, contracts.StateStopping:
			n++
		}
	}
	return n
}

func spawnRelaunch() error {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return err
	}
	args := []string{}
	if len(os.Args) > 1 {
		args = append(args, os.Args[1:]...)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Start()
}
