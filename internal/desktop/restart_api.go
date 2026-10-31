package desktop

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 测试可替换，避免 go test 把自身可执行文件再拉起一份。
var (
	relaunchFn = spawnRelaunch
	quitFn     = func() {
		if app := application.Get(); app != nil {
			app.Quit()
		}
	}
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
	if err := relaunchFn(); err != nil {
		return contracts.RestartAppResponse{
			Error: contracts.NewAppError(contracts.ErrInternal, "无法拉起新进程，请手动重启应用。", true),
		}
	}
	// 在启动 goroutine 前捕获函数，避免测试替身或运行时重绑定与异步读取竞争。
	quit := quitFn
	// 异步触发退出，先让本响应回到前端。
	go func() {
		time.Sleep(250 * time.Millisecond)
		quit()
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
