package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/engine"
)

func TestEngineHelpersAndProgressState(t *testing.T) {
	for _, input := range []string{"system-docker", "system", "docker", "desktop"} {
		if got := normalizeEngineSource(input); got != engine.SourceSystemDocker {
			t.Fatalf("source %q=%q", input, got)
		}
	}
	if normalizeEngineSource("unknown") != engine.SourceHarborColima {
		t.Fatal("unknown source should use Harbor")
	}
	for _, input := range []string{"docker", "p2", "container"} {
		if !dockerIsolationBackend(input) {
			t.Fatal(input)
		}
	}
	if dockerIsolationBackend("seatbelt") {
		t.Fatal("seatbelt is not docker")
	}
	a := NewApp()
	a.setEngineProgress(120, "ready", false)
	installing, percent, label := a.engineProgressSnapshot()
	if installing || percent != 120 || label != "ready" {
		t.Fatalf("%v %d %s", installing, percent, label)
	}
	ctx, cancel := a.beginEngineInstall()
	defer a.endEngineInstall(cancel)
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	if !a.cancelEngineInstall() {
		t.Fatal("expected install cancellation")
	}
	if a.cancelEngineInstall() {
		t.Fatal("second cancellation should be false")
	}
}

func TestDesktopMappingAndUnavailableSession(t *testing.T) {
	st := mapEngineStatus(engine.Status{Ready: true, Source: "x", Message: "ok", DockerHost: "unix:///tmp/docker.sock"})
	if !st.Ready || st.Source != "x" || st.DockerHost == "" {
		t.Fatalf("%+v", st)
	}
	if got := htmlEscape(`<bad a="x"> &`); got != "&lt;bad a=&quot;x&quot;&gt; &amp;" {
		t.Fatal(got)
	}
	u := unavailableSession{}
	ctx := context.Background()
	if _, err := u.Create(ctx, contracts.SessionSpec{}); err == nil {
		t.Fatal("Create unexpectedly available")
	}
	if _, err := u.Get(ctx, "x"); err == nil {
		t.Fatal("Get unexpectedly available")
	}
	if _, err := u.Stop(ctx, "x"); err == nil {
		t.Fatal("Stop unexpectedly available")
	}
	if _, err := u.List(ctx); err == nil {
		t.Fatal("List unexpectedly available")
	}
	if _, err := u.Destroy(ctx, "x"); err == nil {
		t.Fatal("Destroy unexpectedly available")
	}
	if _, err := u.SyncBack(ctx, "x"); err == nil {
		t.Fatal("SyncBack unexpectedly available")
	}
	if err := u.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := u.WithConnection(ctx, func(context.Context) error { return nil }); err == nil {
		t.Fatal("WithConnection unexpectedly available")
	}
}

func TestMapErrorPreservesAppError(t *testing.T) {
	appErr := contracts.NewAppError("X", "message", true)
	if got := mapError(appErr); got.Code != appErr.Code || got.Message != appErr.Message {
		t.Fatalf("mapError changed AppError: %+v", got)
	}
	if got := mapError(context.Canceled); got.Code != contracts.ErrCancelled {
		t.Fatalf("cancel=%+v", got)
	}
	if mapError(nil) != nil {
		t.Fatal("nil error should remain nil")
	}
}

func TestContainerEngineFailureAndStopPaths(t *testing.T) {
	app := NewApp()
	app.eng = engine.NewColima()
	app.eng.(*engine.Colima).Root = t.TempDir()
	app.prefs.EngineSource = engine.SourceHarborColima
	api := NewDesktopAPI(app)
	res := api.EnsureContainerEngine()
	if res.Ready || res.Error == nil {
		t.Fatalf("ensure failure=%+v", res)
	}
	app.prefs.EngineSource = engine.SourceSystemDocker
	app.eng = engine.NewSystemDocker()
	res = api.StopContainerEngine()
	if res.Ready || res.Source != engine.SourceSystemDocker || res.Message == "" {
		t.Fatalf("system stop=%+v", res)
	}
	app.eng = engine.NewColima()
	app.eng.(*engine.Colima).Root = t.TempDir()
	res = api.StopContainerEngine()
	if res.Ready || res.Message == "" {
		t.Fatalf("colima stop=%+v", res)
	}
}

func TestResolveDockerfileDirOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARBOR_DOCKERFILE_DIR", dir)
	if got := resolveDockerfileDir(); got != dir {
		t.Fatalf("dir=%q", got)
	}
	t.Setenv("HARBOR_DOCKERFILE_DIR", filepath.Join(dir, "missing"))
	if got := resolveDockerfileDir(); got == filepath.Join(dir, "missing") {
		t.Fatal("invalid override accepted")
	}
}

func TestContainerEngineColimaSuccessWithFixtureBinaries(t *testing.T) {
	oldPath, oldDockerHost := os.Getenv("PATH"), os.Getenv("DOCKER_HOST")
	oldColimaHome, oldColimaProfile := os.Getenv("COLIMA_HOME"), os.Getenv("COLIMA_PROFILE")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", oldPath)
		_ = os.Setenv("DOCKER_HOST", oldDockerHost)
		_ = os.Setenv("COLIMA_HOME", oldColimaHome)
		_ = os.Setenv("COLIMA_PROFILE", oldColimaProfile)
	})
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"colima", "docker", "limactl"} {
		content := "#!/bin/sh\n"
		if name == "docker" {
			content += "echo 27.0\n"
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HARBOR_DOCKERFILE_DIR", filepath.Join(root, "missing"))
	app := NewApp()
	c := engine.NewColima()
	c.Root = root
	app.eng = c
	app.prefs.EngineSource = engine.SourceHarborColima
	res := NewDesktopAPI(app).EnsureContainerEngine()
	if !res.Ready || res.Source != engine.SourceHarborColima {
		t.Fatalf("success=%+v", res)
	}
}

func TestDesktopConstructionAndReset(t *testing.T) {
	if defaultDataDir() == "" {
		t.Fatal("default data dir empty")
	}
	app := NewApp()
	app.eng = engine.NewSystemDocker()
	app.resetEngine()
	if app.eng != nil {
		t.Fatal("engine not reset")
	}
	revealInFileManager("")
}
