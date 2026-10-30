package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// applyEngineDockerHost 在隔离后端为 docker 且 Harbor Colima 已就绪时，把 DOCKER_HOST 接到本进程（供 Runtime 使用）。
func (a *App) applyEngineDockerHost(ctx context.Context) {
	backend := strings.ToLower(strings.TrimSpace(a.getPrefs().IsolationBackend))
	if backend != "docker" && backend != "p2" && backend != "container" {
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

func mapEngineStatus(st engine.Status) contracts.ContainerEngineHealthResponse {
	return contracts.ContainerEngineHealthResponse{
		Ready:      st.Ready,
		Source:     st.Source,
		Message:    st.Message,
		DockerHost: st.DockerHost,
		Error:      nil,
	}
}

// GetContainerEngineHealth 探测 P2 容器引擎（不启停）。
func (d *DesktopAPI) GetContainerEngineHealth() contracts.ContainerEngineHealthResponse {
	eng := d.app.engineRef()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st, err := eng.Health(ctx)
	if err != nil {
		ae := mapError(err)
		return contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: ae.Message,
			Error:   ae,
		}
	}
	return mapEngineStatus(st)
}

// EnsureContainerEngine 确保引擎可用：下载/启动，并尽量构建 runtime 镜像。
func (d *DesktopAPI) EnsureContainerEngine() contracts.ContainerEngineHealthResponse {
	eng := d.app.engineRef()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	if err := eng.Ensure(ctx); err != nil {
		ae := mapError(err)
		return contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: ae.Message,
			Error:   ae,
		}
	}
	if err := eng.Start(ctx); err != nil {
		ae := mapError(err)
		return contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: ae.Message,
			Error:   ae,
		}
	}
	if c, ok := eng.(*engine.Colima); ok {
		if dir := resolveDockerfileDir(); dir != "" {
			_ = c.EnsureImage(ctx, dir) // best-effort; Health below reports daemon readiness
		}
	}
	st, err := eng.Health(ctx)
	if err != nil {
		ae := mapError(err)
		return contracts.ContainerEngineHealthResponse{
			Ready:   false,
			Source:  normalizeEngineSource(d.app.getPrefs().EngineSource),
			Message: ae.Message,
			Error:   ae,
		}
	}
	return mapEngineStatus(st)
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
