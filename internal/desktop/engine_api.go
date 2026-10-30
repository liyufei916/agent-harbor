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
		Done:    !installing && percent >= 100,
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

// ensureEngineForDockerRuntime M4：容器隔离偏好下启动时自动 Ensure/Start（无感）；失败不 panic，由 Runtime Health fail-closed。
func (a *App) ensureEngineForDockerRuntime(ctx context.Context) {
	if !dockerIsolationBackend(a.getPrefs().IsolationBackend) {
		return
	}
	eng := a.engineRef()
	st, err := eng.Health(ctx)
	if err == nil && st.Ready {
		a.applyEngineDockerHost(ctx)
		return
	}
	_ = eng.Ensure(ctx)
	_ = eng.Start(ctx)
	st2, err2 := eng.Health(ctx)
	if err2 != nil || !st2.Ready {
		return
	}
	if c, ok := eng.(*engine.Colima); ok {
		_ = c.ApplyDockerHost()
		if dir := resolveDockerfileDir(); dir != "" {
			_ = c.EnsureImage(ctx, dir)
		}
	}
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

// EnsureContainerEngine 确保引擎可用：下载/启动，并尽量构建 runtime 镜像；推送安装进度百分比。
func (d *DesktopAPI) EnsureContainerEngine() contracts.ContainerEngineHealthResponse {
	eng := d.app.engineRef()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

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
		d.app.setEngineProgress(0, ae.Message, false)
		if app := application.Get(); app != nil {
			app.Event.Emit(contracts.EngineProgressEventName, contracts.EngineProgressPayload{
				Percent: 0, Label: ae.Message, Done: true, Error: ae.Message,
			})
		}
		return enrichEngineHealth(d.app, contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: ae.Message,
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
