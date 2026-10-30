package desktop

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/engine"
)

func TestNormalizeEngineSource(t *testing.T) {
	if got := normalizeEngineSource(""); got != engine.SourceHarborColima {
		t.Fatal(got)
	}
	if got := normalizeEngineSource("system-docker"); got != engine.SourceSystemDocker {
		t.Fatal(got)
	}
}

func TestDockerIsolationBackend(t *testing.T) {
	if !dockerIsolationBackend("docker") || !dockerIsolationBackend("P2") {
		t.Fatal("want docker true")
	}
	if dockerIsolationBackend("seatbelt") || dockerIsolationBackend("") {
		t.Fatal("want seatbelt false")
	}
}

func TestGetContainerEngineHealthFailClosedWithoutBins(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "0")
	app := NewApp()
	app.setPrefs(contracts.UIPrefs{EngineSource: engine.SourceHarborColima})
	api := &DesktopAPI{app: app}
	got := api.GetContainerEngineHealth()
	if got.Ready {
		t.Fatalf("%+v", got)
	}
	if got.Source != engine.SourceHarborColima {
		t.Fatalf("%+v", got)
	}
	if got.Message == "" {
		t.Fatal("want message")
	}
}

func TestEnsureEngineForDockerRuntimeSeatbeltNoop(t *testing.T) {
	prevHost := os.Getenv("DOCKER_HOST")
	_ = os.Unsetenv("DOCKER_HOST")
	t.Cleanup(func() {
		if prevHost == "" {
			_ = os.Unsetenv("DOCKER_HOST")
		} else {
			_ = os.Setenv("DOCKER_HOST", prevHost)
		}
	})
	app := NewApp()
	app.setPrefs(contracts.UIPrefs{IsolationBackend: "seatbelt"})
	app.ensureEngineForDockerRuntime(context.Background())
	if os.Getenv("DOCKER_HOST") != "" {
		t.Fatalf("seatbelt must not set DOCKER_HOST=%s", os.Getenv("DOCKER_HOST"))
	}
}

func TestEnsureEngineForDockerRuntimeNoPanicWithoutDownload(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "0")
	app := NewApp()
	app.setPrefs(contracts.UIPrefs{
		IsolationBackend: "docker",
		EngineSource:     engine.SourceHarborColima,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	app.ensureEngineForDockerRuntime(ctx) // 无 Desktop / 缺 bin 时 fail-closed，不 panic
}
