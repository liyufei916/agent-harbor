package desktop

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type busyListSession struct {
	recordingSession
}

func (b *busyListSession) List(context.Context) ([]contracts.SessionSummary, error) {
	return []contracts.SessionSummary{
		{SessionID: "s1", State: contracts.StateRunning},
	}, nil
}

func TestRestartAppBusyWithoutForce(t *testing.T) {
	app := NewAppWithDataDir(t.TempDir())
	app.SetSessionService(&busyListSession{})
	api := NewDesktopAPI(app)
	res := api.RestartApp(contracts.RestartAppInput{Force: false})
	if res.Restarting || res.Error == nil || res.Error.Code != contracts.ErrSessionBusy {
		t.Fatalf("%+v", res)
	}
	if res.BusySessions != 1 {
		t.Fatalf("busy=%d", res.BusySessions)
	}
}

func TestRestartAppForceRelaunches(t *testing.T) {
	prevR, prevQ := relaunchFn, quitFn
	t.Cleanup(func() {
		relaunchFn, quitFn = prevR, prevQ
	})
	var launched, quit atomic.Bool
	relaunchFn = func() error {
		launched.Store(true)
		return nil
	}
	quitFn = func() { quit.Store(true) }

	app := NewAppWithDataDir(t.TempDir())
	app.SetSessionService(&busyListSession{})
	api := NewDesktopAPI(app)
	res := api.RestartApp(contracts.RestartAppInput{Force: true})
	if res.Error != nil || !res.Restarting || !launched.Load() {
		t.Fatalf("%+v launched=%v", res, launched.Load())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !quit.Load() {
		time.Sleep(20 * time.Millisecond)
	}
	if !quit.Load() {
		t.Fatal("expected quitFn")
	}
}
