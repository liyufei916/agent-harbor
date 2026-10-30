package desktop

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestSelectWorkspaceRequiresApp(t *testing.T) {
	api := NewDesktopAPI(NewApp())
	resp := api.SelectWorkspace()
	if resp.Error == nil || resp.Error.Code != contracts.ErrInternal || resp.Path != "" {
		t.Fatalf("无 Wails 应用时应 INTERNAL：%+v", resp)
	}
}

func TestSaveTextFileRejectsBadInputAndMissingApp(t *testing.T) {
	api := NewDesktopAPI(NewApp())
	badName := api.SaveTextFile(contracts.SaveTextFileInput{SuggestedName: "../x.json", Content: "{}"})
	if badName.Error == nil || badName.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("路径分隔符应拒：%+v", badName)
	}
	empty := api.SaveTextFile(contracts.SaveTextFileInput{SuggestedName: "ok.json", Content: ""})
	if empty.Error == nil || empty.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("空内容应拒：%+v", empty)
	}
	noApp := api.SaveTextFile(contracts.SaveTextFileInput{SuggestedName: "ok.json", Content: "{}"})
	if noApp.Error == nil || noApp.Error.Code != contracts.ErrInternal {
		t.Fatalf("无 Wails 应用时应 INTERNAL：%+v", noApp)
	}
}

type promptSession struct {
	extendedSession
	mu       sync.Mutex
	calls    int
	lastID   string
	lastNeed contracts.PermissionNeeded
}

func (s *promptSession) RequestPermission(_ context.Context, sessionID string, need contracts.PermissionNeeded) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastID = sessionID
	s.lastNeed = need
	return "pending-1", nil
}

func TestPromptPermissionValidatesAndAsks(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)

	missing := api.PromptPermission(contracts.PromptPermissionInput{SessionID: "", Target: "cdn.example.test"})
	if missing.Error == nil || missing.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("缺会话应拒：%+v", missing)
	}
	badHost := api.PromptPermission(contracts.PromptPermissionInput{SessionID: "s1", Target: "not a host"})
	if badHost.Error == nil || badHost.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("非法主机应拒：%+v", badHost)
	}

	// 未实现 RequestPermission → UNIMPLEMENTED
	app.SetSessionService(&extendedSession{
		snapshot: contracts.SessionSnapshot{SessionID: "s1", State: contracts.StateRunning},
	})
	unimpl := api.PromptPermission(contracts.PromptPermissionInput{SessionID: "s1", Target: "cdn.example.test"})
	if unimpl.Error == nil || unimpl.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("缺能力应 UNIMPLEMENTED：%+v", unimpl)
	}

	svc := &promptSession{
		extendedSession: extendedSession{
			snapshot: contracts.SessionSnapshot{SessionID: "s1", State: contracts.StateStopped},
		},
	}
	app.SetSessionService(svc)
	notRunning := api.PromptPermission(contracts.PromptPermissionInput{SessionID: "s1", Target: "cdn.example.test"})
	if notRunning.Error == nil || notRunning.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("非 running 应拒：%+v", notRunning)
	}

	svc.snapshot.State = contracts.StateRunning
	ok := api.PromptPermission(contracts.PromptPermissionInput{SessionID: "s1", Target: "CDN.Example.TEST"})
	if ok.Error != nil {
		t.Fatalf("running 应接受 ask：%+v", ok)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		svc.mu.Lock()
		calls := svc.calls
		id := svc.lastID
		target := svc.lastNeed.Target
		svc.mu.Unlock()
		if calls >= 1 {
			if id != "s1" || target != "cdn.example.test" {
				t.Fatalf("RequestPermission 参数不对：id=%s target=%s", id, target)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("未异步调用 RequestPermission")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
