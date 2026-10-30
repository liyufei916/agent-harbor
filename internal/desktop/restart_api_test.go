package desktop

import (
	"context"
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
	var launched, quit bool
	relaunchFn = func() error {
		launched = true
		return nil
	}
	quitFn = func() { quit = true }

	app := NewAppWithDataDir(t.TempDir())
	app.SetSessionService(&busyListSession{})
	api := NewDesktopAPI(app)
	res := api.RestartApp(contracts.RestartAppInput{Force: true})
	if res.Error != nil || !res.Restarting || !launched {
		t.Fatalf("%+v launched=%v", res, launched)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !quit {
		time.Sleep(20 * time.Millisecond)
	}
	if !quit {
		t.Fatal("expected quitFn")
	}
}
