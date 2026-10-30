package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/engine"
)

func normalizeEngineSource(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case engine.SourceSystemDocker, "system", "docker", "desktop":
		return engine.SourceSystemDocker
	default:
		return engine.SourceHarborColima
	}
}

func (a *App) engineRef() engine.Engine {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.eng != nil {
		return a.eng
	}
	src := normalizeEngineSource(a.prefs.EngineSource)
	a.eng = engine.NewSelected(src)
	return a.eng
}

func (a *App) resetEngine() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.eng = nil
}

func dockerIsolationBackend(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "docker", "p2", "container":
		return true
	default:
		return false
	}
}

func (a *App) setEngineProgress(percent int, label string, installing bool) {
	a.mu.Lock()
	a.engProgress = percent
	a.engProgressLabel = label
	a.engInstalling = installing
	a.mu.Unlock()
	payload := contracts.EngineProgressPayload{
		Percent: percent,
		Label:   label,
		Done:    !installing,
	}
	if percent >= 100 && !installing {
		payload.Done = true
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(contracts.EngineProgressEventName, payload)
	}
}

func (a *App) engineProgressSnapshot() (installing bool, percent int, label string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.engInstalling, a.engProgress, a.engProgressLabel
}

func (a *App) beginEngineInstall() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	a.mu.Lock()
	if a.engInstallCancel != nil {
		a.engInstallCancel()
	}
	a.engInstallCancel = cancel
	a.mu.Unlock()
	return ctx, cancel
}

func (a *App) endEngineInstall(cancel context.CancelFunc) {
	a.mu.Lock()
	a.engInstallCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) cancelEngineInstall() bool {
	a.mu.Lock()
	cancel := a.engInstallCancel
	a.engInstallCancel = nil
	a.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

// applyEngineDockerHost 在隔离后端为 docker 且 Harbor Colima 已就绪时，把 DOCKER_HOST 接到本进程（供 Runtime 使用）。
func (a *App) applyEngineDockerHost(ctx context.Context) {
	if !dockerIsolationBackend(a.getPrefs().IsolationBackend) {
		return
	}
	eng := a.engineRef()
	c, ok := eng.(*engine.Colima)
	if !ok {
		return
	}
	st, err := eng.Health(ctx)
	if err != nil || !st.Ready {
		return
	}
	_ = c.ApplyDockerHost()
}

// ensureEngineForDockerRuntime 启动时只挂已就绪引擎的 DOCKER_HOST，绝不在此下载/启 VM
// （否则 prefs=docker 时应用会卡在闪屏；下载仅由用户点「一键安装」触发）。
func (a *App) ensureEngineForDockerRuntime(ctx context.Context) {
	a.applyEngineDockerHost(ctx)
}

func mapEngineStatus(st engine.Status) contracts.ContainerEngineHealthResponse {
	return contracts.ContainerEngineHealthResponse{
		Ready:      st.Ready,
		Source:     st.Source,
		Message:    st.Message,
		DockerHost: st.DockerHost,
		Error:      nil,
	}
}

func enrichEngineHealth(app *App, resp contracts.ContainerEngineHealthResponse) contracts.ContainerEngineHealthResponse {
	resp.DockerDesktopInstalled = engine.DockerDesktopInstalled()
	if c, ok := app.engineRef().(*engine.Colima); ok {
		resp.ComponentsInstalled = c.ComponentsPresent()
	} else if resp.Source == engine.SourceSystemDocker {
		resp.ComponentsInstalled = resp.Ready
	}
	installing, pct, label := app.engineProgressSnapshot()
	resp.Installing = installing
	resp.ProgressPercent = pct
	resp.ProgressLabel = label
	return resp
}

// GetContainerEngineHealth 探测 P2 容器引擎（不启停）。
func (d *DesktopAPI) GetContainerEngineHealth() contracts.ContainerEngineHealthResponse {
	eng := d.app.engineRef()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st, err := eng.Health(ctx)
	if err != nil {
		ae := mapError(err)
		return enrichEngineHealth(d.app, contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: ae.Message,
			Error:   ae,
		})
	}
	return enrichEngineHealth(d.app, mapEngineStatus(st))
}

// EnsureContainerEngine 用户显式一键安装：下载/启动，并尽量构建 runtime 镜像；推送进度。
func (d *DesktopAPI) EnsureContainerEngine() contracts.ContainerEngineHealthResponse {
	eng := d.app.engineRef()
	ctx, cancel := d.app.beginEngineInstall()
	defer d.app.endEngineInstall(cancel)

	d.app.setEngineProgress(0, "开始安装 Harbor 容器引擎…", true)
	defer func() {
		installing, pct, _ := d.app.engineProgressSnapshot()
		if installing {
			d.app.setEngineProgress(pct, "安装结束", false)
		}
	}()

	if c, ok := eng.(*engine.Colima); ok {
		c.OnProgress = func(p engine.Progress) {
			d.app.setEngineProgress(p.Percent, p.Label, true)
		}
	}

	fail := func(err error) contracts.ContainerEngineHealthResponse {
		ae := mapError(err)
		msg := ae.Message
		if ctx.Err() != nil {
			msg = "安装已取消"
			ae = contracts.NewAppError(contracts.ErrCancelled, msg, true)
		}
		d.app.setEngineProgress(0, msg, false)
		if app := application.Get(); app != nil {
			app.Event.Emit(contracts.EngineProgressEventName, contracts.EngineProgressPayload{
				Percent: 0, Label: msg, Done: true, Error: msg,
			})
		}
		return enrichEngineHealth(d.app, contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: msg,
			Error:   ae,
		})
	}

	if err := eng.Ensure(ctx); err != nil {
		return fail(err)
	}
	if err := eng.Start(ctx); err != nil {
		return fail(err)
	}
	if c, ok := eng.(*engine.Colima); ok {
		if dir := resolveDockerfileDir(); dir != "" {
			_ = c.EnsureImage(ctx, dir) // best-effort
		}
	}
	st, err := eng.Health(ctx)
	if err != nil {
		return fail(err)
	}
	d.app.setEngineProgress(100, "安装完成", false)
	return enrichEngineHealth(d.app, mapEngineStatus(st))
}

// StopContainerEngine 取消进行中的一键安装，并停止 Harbor 托管 Colima（不关本机 Docker Desktop）。
func (d *DesktopAPI) StopContainerEngine() contracts.ContainerEngineHealthResponse {
	cancelled := d.app.cancelEngineInstall()
	if cancelled {
		d.app.setEngineProgress(0, "正在取消安装…", false)
	}
	eng := d.app.engineRef()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	src := normalizeEngineSource(d.app.getPrefs().EngineSource)
	if src == engine.SourceSystemDocker {
		msg := "本机 Docker 由系统管理，Harbor 不会关闭 Docker Desktop。"
		if cancelled {
			msg = "已取消安装。" + msg
		}
		d.app.setEngineProgress(0, msg, false)
		return enrichEngineHealth(d.app, contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  src,
			Message: msg,
			Error:   nil,
		})
	}
	if err := eng.Stop(ctx); err != nil {
		ae := mapError(err)
		d.app.setEngineProgress(0, ae.Message, false)
		return enrichEngineHealth(d.app, contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  src,
			Message: ae.Message,
			Error:   ae,
		})
	}
	msg := "已停止 Harbor Colima。"
	if cancelled {
		msg = "已取消安装并停止 Harbor Colima。"
	}
	d.app.setEngineProgress(0, msg, false)
	st, _ := eng.Health(ctx)
	out := mapEngineStatus(st)
	out.Message = msg
	out.Ready = false
	return enrichEngineHealth(d.app, out)
}

func resolveDockerfileDir() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKERFILE_DIR")); v != "" {
		if st, err := os.Stat(filepath.Join(v, "Dockerfile")); err == nil && !st.IsDir() {
			return v
		}
	}
	candidates := []string{
		"deploy/docker",
		filepath.Join("docs", "agent-harbor", "deploy", "docker"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "deploy", "docker"),
			filepath.Join(wd, "..", "deploy", "docker"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "Dockerfile")); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}
