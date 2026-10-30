package desktop

import (
	"context"
	"os"
	"strings"
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
	// dockerDesktopInstalled is environment-dependent; just ensure field is populated path runs.
	_ = got.DockerDesktopInstalled
	if got.ComponentsInstalled {
		t.Fatal("expected components missing without bins")
	}
}

func TestStopContainerEngineSystemDockerNoOpDesktop(t *testing.T) {
	app := NewApp()
	app.setPrefs(contracts.UIPrefs{
		IsolationBackend: "docker",
		EngineSource:     engine.SourceSystemDocker,
	})
	api := &DesktopAPI{app: app}
	got := api.StopContainerEngine()
	if got.Ready {
		t.Fatalf("%+v", got)
	}
	if got.Message == "" || !strings.Contains(got.Message, "Docker Desktop") {
		t.Fatalf("want desktop notice, got %+v", got)
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

func TestEnsureEngineForDockerRuntimeNoDownloadOnAttach(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "0")
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
	app.setPrefs(contracts.UIPrefs{
		IsolationBackend: "docker",
		EngineSource:     engine.SourceHarborColima,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Must return quickly: Attach must not download/start Colima.
	app.ensureEngineForDockerRuntime(ctx)
	if os.Getenv("DOCKER_HOST") != "" {
		t.Fatalf("unready engine must not set DOCKER_HOST=%s", os.Getenv("DOCKER_HOST"))
	}
}
