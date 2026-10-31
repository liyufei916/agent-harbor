package desktop

import (
	"context"
	"sync"
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
	var mu sync.Mutex
	var launched, quit bool
	relaunchFn = func() error {
		mu.Lock()
		launched = true
		mu.Unlock()
		return nil
	}
	quitFn = func() {
		mu.Lock()
		quit = true
		mu.Unlock()
	}
	// 须等异步 quitFn 跑完再还原钩子，否则与 RestartApp 内 goroutine 竞态。
	t.Cleanup(func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			done := quit
			mu.Unlock()
			if done {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		relaunchFn, quitFn = prevR, prevQ
	})

	app := NewAppWithDataDir(t.TempDir())
	app.SetSessionService(&busyListSession{})
	api := NewDesktopAPI(app)
	res := api.RestartApp(contracts.RestartAppInput{Force: true})
	mu.Lock()
	gotLaunch := launched
	mu.Unlock()
	if res.Error != nil || !res.Restarting || !gotLaunch {
		t.Fatalf("%+v launched=%v", res, gotLaunch)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := quit
		mu.Unlock()
		if done {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected quitFn")
}
