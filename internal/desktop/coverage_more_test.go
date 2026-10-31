package desktop

import (
	"context"
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
