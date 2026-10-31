package desktop

import (
	"context"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestCreateSessionBeforeWire(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	resp := api.CreateSession(contracts.SessionSpec{
		Workspace: "/tmp/ws", AgentID: "claude", ModelAccountID: "m", MCPIDs: []string{"t"}, Task: "hi",
	})
	if resp.Session != nil || resp.Error == nil || resp.Error.Code != contracts.ErrOrchestratorUnavailable {
		t.Fatalf("%+v", resp)
	}
}

func TestCreateSessionForwardsInjectedService(t *testing.T) {
	app := NewApp()
	rec := &recordingSession{}
	app.SetSessionService(rec)
	api := NewDesktopAPI(app)
	resp := api.CreateSession(contracts.SessionSpec{
		Workspace: "/demo/project", AgentID: "claude", ModelAccountID: "m1", MCPIDs: []string{"t1"}, Task: "hello",
	})
	if resp.Error != nil || resp.Session == nil || resp.Session.State != "provisioning" {
		t.Fatalf("%+v", resp)
	}
	if rec.last.Task != "hello" {
		t.Fatalf("forward=%+v", rec.last)
	}
}

func TestConnectRejectedWhileSessionBusy(t *testing.T) {
	app := NewApp()
	app.SetSessionService(&recordingSession{connectionErr: contracts.NewAppError(contracts.ErrSessionBusy, "已有会话进行中", false)})
	api := NewDesktopAPI(app)
	model := api.ConnectModel(contracts.ConnectInput{ProfileID: "demo-model", Name: "n", APIKey: "secret"})
	mcp := api.ConnectMcp(contracts.ConnectInput{ProfileID: "demo-mcp", Name: "n", APIKey: "secret"})
	if model.Error == nil || model.Error.Code != contracts.ErrSessionBusy || model.Account != nil {
		t.Fatalf("model=%+v", model)
	}
	if mcp.Error == nil || mcp.Error.Code != contracts.ErrSessionBusy {
		t.Fatalf("mcp=%+v", mcp)
	}

}

func TestMapErrorPassesPublicAppError(t *testing.T) {
	src := contracts.NewAppError(contracts.ErrSessionBusy, "已有会话正在准备、运行或等待清理。", false)
	got := mapError(src)
	if got.Code != contracts.ErrSessionBusy || got.Message != src.Message {
		t.Fatalf("%+v", got)
	}
}

func TestAttachOrchestratorWiresRealService(t *testing.T) {
	app := NewApp()
	if err := app.AttachOrchestrator(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !app.orchestratorReady() {
		t.Fatal("expected orchestrator")
	}
	api := NewDesktopAPI(app)
	agents := api.ListAgents()
	if agents.Error != nil || len(agents.Agents) != 3 || agents.Agents[0].ID != "claude" || agents.Agents[1].ID != "codex" || agents.Agents[2].ID != "dsh" || agents.Agents[1].ModelProfileID != "codex-model" {
		t.Fatalf("%+v", agents)
	}
	health := api.GetRuntimeHealth()
	if health.Error != nil && health.Error.Code == contracts.ErrUnimplemented {
		t.Fatalf("Health 仍未接线: %+v", health)
	}
	// 非 Darwin25/arm64 时 Available=false 且带隔离错误，属预期；不得再报「尚未接线」
	if health.Reason == "健康检查尚未接线" {
		t.Fatalf("仍返回占位文案: %+v", health)
	}
}

func TestStartCancelMcpOAuth(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	started := api.StartMcpOAuth(contracts.StartMcpOAuthInput{
		Name: "Notion", Origin: "https://auth.example.test",
	})
	if started.Error != nil || started.PendingID == "" || !strings.Contains(started.AuthURL, "127.0.0.1") {
		t.Fatalf("%+v", started)
	}
	cancelled := api.CancelMcpOAuth(started.PendingID)
	if cancelled.Error != nil {
		t.Fatalf("%+v", cancelled)
	}
	again := api.CancelMcpOAuth(started.PendingID)
	if again.Error == nil {
		t.Fatal("重复取消应失败")
	}
}

func TestListAccountsAndPrefs(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	_, err := app.secrets.Put(context.Background(), contracts.AccountInfo{
		Kind: "model", Name: "demo", ProfileID: "demo-model", Host: "api.example.test",
	}, []byte("secret-key"))
	if err != nil {
		t.Fatal(err)
	}
	listed := api.ListAccounts("model")
	if listed.Error != nil || len(listed.Accounts) != 1 || listed.Accounts[0].Host != "api.example.test" {
		t.Fatalf("%+v", listed)
	}
	prefs := api.SetUIPrefs(contracts.UIPrefs{SidebarCollapsed: true, AutoCollapseWorkbench: false})
	if prefs.Error != nil || prefs.Prefs == nil || !prefs.Prefs.SidebarCollapsed {
		t.Fatalf("%+v", prefs)
	}
	again := api.GetUIPrefs()
	if again.Error != nil || again.Prefs == nil || again.Prefs.AutoCollapseWorkbench {
		t.Fatalf("%+v", again)
	}
	sessions := api.ListSessions()
	if sessions.Error == nil || sessions.Error.Code != contracts.ErrOrchestratorUnavailable {
		t.Fatalf("未装配编排时应失败: %+v", sessions)
	}
	app.SetSessionService(&recordingSession{})
	sessions = api.ListSessions()
	if sessions.Error != nil || len(sessions.Sessions) != 1 {
		t.Fatalf("%+v", sessions)
	}
}

type recordingSession struct {
	last          contracts.SessionSpec
	connectionErr error
	shutdownErr   error
}

type permissionSession struct {
	recordingSession
	input contracts.ResolvePermissionInput
}

func (s *permissionSession) ResolvePermission(_ context.Context, input contracts.ResolvePermissionInput) error {
	s.input = input
	return contracts.NewAppError(contracts.ErrInvalidArgument, "无挂起的扩权请求。", false)
}

func TestResolvePermissionForwardsToSessionService(t *testing.T) {
	app := NewApp()
	svc := &permissionSession{}
	app.SetSessionService(svc)
	input := contracts.ResolvePermissionInput{SessionID: "session", Target: "docs.example.test", Decision: "once"}
	response := NewDesktopAPI(app).ResolvePermission(input)
	if svc.input != input || response.Error == nil || response.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("决策未转交：%+v %+v", svc.input, response)
	}
}

type modelSession struct {
	recordingSession
	input contracts.SwitchModelInput
}

func (s *modelSession) SwitchModel(_ context.Context, input contracts.SwitchModelInput) (contracts.SessionSnapshot, error) {
	s.input = input
	return contracts.SessionSnapshot{SessionID: input.SessionID, State: contracts.StateRunning}, nil
}

func TestSwitchModelForwardsToSessionService(t *testing.T) {
	app := NewApp()
	svc := &modelSession{}
	app.SetSessionService(svc)
	input := contracts.SwitchModelInput{SessionID: "session", ModelAccountID: "new-model"}
	response := NewDesktopAPI(app).SwitchModel(input)
	if svc.input != input || response.Error != nil || response.Session == nil || response.Session.SessionID != input.SessionID {
		t.Fatalf("切换未转交：%+v", response)
	}
}

type extendedSession struct {
	recordingSession
	snapshot contracts.SessionSnapshot
	err      error
	input    contracts.PtyInput
	boundary contracts.PermissionOverview
	resolved contracts.ResolvePermissionInput
	grants   []contracts.RememberedHost
	demoAsk  *bool
}

func (s *extendedSession) SetOutboundGrants(_ string, hosts []contracts.RememberedHost, demoAsk bool) error {
	s.grants = append([]contracts.RememberedHost(nil), hosts...)
	v := demoAsk
	s.demoAsk = &v
	return nil
}

func (s *extendedSession) Get(context.Context, string) (contracts.SessionSnapshot, error) {
	return s.snapshot, s.err
}
func (s *extendedSession) Stop(context.Context, string) (contracts.SessionSnapshot, error) {
	return s.snapshot, s.err
}
func (s *extendedSession) Start(_ context.Context, id string) (contracts.SessionSnapshot, error) {
	s.snapshot.SessionID = id
	s.snapshot.State = contracts.StateProvisioning
	return s.snapshot, s.err
}
func (s *extendedSession) Destroy(context.Context, string) (contracts.SessionSnapshot, error) {
	return s.snapshot, s.err
}
func (s *extendedSession) SyncBack(context.Context, string) (contracts.SessionSnapshot, error) {
	return s.snapshot, s.err
}
func (s *extendedSession) Boundary(context.Context, string) (contracts.PermissionOverview, error) {
	return s.boundary, s.err
}
func (s *extendedSession) ResolvePermission(_ context.Context, in contracts.ResolvePermissionInput) error {
	s.resolved = in
	return s.err
}
func (s *extendedSession) ProjectWorkspace(context.Context, string) (string, error) {
	return "/tmp/project", s.err
}
func (s *extendedSession) SwitchModel(_ context.Context, in contracts.SwitchModelInput) (contracts.SessionSnapshot, error) {
	s.snapshot.ModelAccountID = in.ModelAccountID
	return s.snapshot, s.err
}
func (s *extendedSession) WritePty(_ context.Context, in contracts.PtyInput) error {
	s.input = in
	return s.err
}
func (s *extendedSession) ResizePty(_ context.Context, in contracts.PtyInput) error {
	s.input = in
	return s.err
}

func TestDesktopSessionLifecycleAndPtyBindings(t *testing.T) {
	app := NewApp()
	svc := &extendedSession{snapshot: contracts.SessionSnapshot{
		SessionID: "s1", State: contracts.StateRunning, Workspace: "/tmp/project", SourceWorkspace: "/tmp/project",
	}}
	app.SetSessionService(svc)
	api := NewDesktopAPI(app)
	app.rememberHost(contracts.RememberedHost{Workspace: "/tmp/project", Host: "cdn.example.test", Scope: "project"})

	if got := api.GetSession("s1"); got.Error != nil || got.Session == nil {
		t.Fatalf("get=%+v", got)
	}
	if got := api.StopSession("s1"); got.Error != nil || got.Session == nil {
		t.Fatalf("stop=%+v", got)
	}
	if got := api.StartSession("s1"); got.Error != nil || got.Session == nil || got.Session.SessionID != "s1" || got.Session.State != contracts.StateProvisioning {
		t.Fatalf("start=%+v", got)
	}
	if len(svc.grants) == 0 {
		t.Fatal("StartSession 应从落盘记住表回灌 SetOutboundGrants")
	}
	found := false
	for _, h := range svc.grants {
		if h.Host == "cdn.example.test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("grants=%+v", svc.grants)
	}
	if got := api.DestroySession("s1"); got.Error != nil || got.Session == nil {
		t.Fatalf("destroy=%+v", got)
	}
	if got := api.SyncBackWorkspace("s1"); got.Error != nil || got.Session == nil {
		t.Fatalf("sync=%+v", got)
	}
	input := contracts.PtyInput{SessionID: "s1", Data: "next"}
	if got := api.WritePty(input); got.Error != nil || svc.input != input {
		t.Fatalf("write=%+v input=%+v", got, svc.input)
	}
	if got := api.ResizePty(input); got.Error != nil || svc.input != input {
		t.Fatalf("resize=%+v input=%+v", got, svc.input)
	}
}

func TestDesktopPermissionOverviewAndRememberedHost(t *testing.T) {
	app := NewApp()
	svc := &extendedSession{boundary: contracts.PermissionOverview{SessionID: "s1", Workspace: "/tmp/project", AllowedHosts: []string{"api.example.test"}}}
	app.SetSessionService(svc)
	api := NewDesktopAPI(app)

	got := api.GetPermissionOverview("s1")
	if got.Error != nil || got.Overview == nil || got.Overview.Workspace != "/tmp/project" {
		t.Fatalf("boundary=%+v", got)
	}
	input := contracts.ResolvePermissionInput{SessionID: "s1", Target: "docs.example.test", Decision: "project"}
	if response := api.ResolvePermission(input); response.Error != nil || svc.resolved != input {
		t.Fatalf("resolve=%+v input=%+v", response, svc.resolved)
	}
	hosts := api.ListRememberedHosts("/tmp/project")
	if len(hosts.Hosts) != 1 || hosts.Hosts[0].Host != input.Target {
		t.Fatalf("hosts=%+v", hosts)
	}
	if response := api.RevokeRememberedHost(contracts.RevokeHostInput{Workspace: "/tmp/project", Host: input.Target}); response.Error != nil {
		t.Fatalf("revoke=%+v", response)
	}
	if response := api.RevokeRememberedHost(contracts.RevokeHostInput{}); response.Error == nil {
		t.Fatal("空主机应被拒绝")
	}
}

func TestDesktopOptionalBindingsReturnUnimplemented(t *testing.T) {
	api := NewDesktopAPI(NewApp())
	if got := api.GetPermissionOverview("missing"); got.Error == nil || got.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("boundary=%+v", got)
	}
	if got := api.SwitchModel(contracts.SwitchModelInput{SessionID: "s", ModelAccountID: "m"}); got.Error == nil || got.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("switch=%+v", got)
	}
	if got := api.WritePty(contracts.PtyInput{SessionID: "s", Data: "x"}); got.Error == nil || got.Error.Code != contracts.ErrUnimplemented {
		t.Fatalf("write=%+v", got)
	}
}

func TestRemoveAccountDeletesSecretAndMapsMissing(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	accountID, err := app.secrets.Put(context.Background(), contracts.AccountInfo{Kind: "model", Name: "fixture", ProfileID: "m", Host: "example.test"}, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if response := api.RemoveAccount(accountID); response.Error != nil {
		t.Fatalf("删除账号失败: %+v", response)
	}
	if response := api.RemoveAccount(accountID); response.Error == nil || response.Error.Code != contracts.ErrAccountNotFound {
		t.Fatalf("重复删除应返回 ACCOUNT_NOT_FOUND: %+v", response)
	}
}

func TestMapErrorCoversCommonFailures(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{"cancelled", context.Canceled, contracts.ErrCancelled},
		{"deadline", context.DeadlineExceeded, contracts.ErrUpstreamTimeout},
		{"invalid", contracts.NewAppError(contracts.ErrInvalidArgument, "bad", false), contracts.ErrInvalidArgument},
		{"workspace", policy.ErrInvalidWorkspace, contracts.ErrWorkspaceDenied},
		{"not-found", secrets.ErrNotFound, contracts.ErrAccountNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapError(tc.err); got.Code != tc.code {
				t.Fatalf("got=%+v want=%s", got, tc.code)
			}
		})
	}
}

func (r *recordingSession) Create(_ context.Context, spec contracts.SessionSpec) (contracts.SessionSnapshot, error) {
	r.last = spec
	return contracts.SessionSnapshot{
		SessionID: "fake-session", State: "provisioning", Seq: 1,
		Workspace: spec.Workspace, AgentID: spec.AgentID,
	}, nil
}

func (r *recordingSession) Get(context.Context, string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{}, nil
}

func (r *recordingSession) Stop(context.Context, string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{State: "stopping"}, nil
}

func (r *recordingSession) List(context.Context) ([]contracts.SessionSummary, error) {
	return []contracts.SessionSummary{{SessionID: "fake-session", State: "running", Workspace: "/demo", AgentID: "claude"}}, nil
}

func (r *recordingSession) Destroy(_ context.Context, id string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{SessionID: id, State: "stopped", CleanupComplete: true}, nil
}

func (r *recordingSession) SyncBack(_ context.Context, id string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{SessionID: id, MountMode: "clone", CleanupComplete: false}, nil
}

func (r *recordingSession) Shutdown(context.Context) error { return r.shutdownErr }

func (r *recordingSession) WithConnection(ctx context.Context, action func(context.Context) error) error {
	if r.connectionErr != nil {
		return r.connectionErr
	}
	return action(ctx)
}

func TestShutdownRetainsCredentialsUntilCleanupConfirmed(t *testing.T) {
	app := NewApp()
	svc := &recordingSession{shutdownErr: contracts.NewAppError(contracts.ErrCleanupFailed, "清理未确认完成", false)}
	app.SetSessionService(svc)
	_, _ = app.secrets.Put(context.Background(), contracts.AccountInfo{Kind: "model", Name: "fixture", ProfileID: "m", Host: "example.test"}, []byte("fixture"))
	if err := app.Shutdown(context.Background()); err == nil || app.secrets.Count() != 1 {
		t.Fatal("失败后无法保留资源重试")
	}
	svc.shutdownErr = nil
	if err := app.Shutdown(context.Background()); err != nil || app.secrets.Count() != 0 {
		t.Fatal("确认清理后没有清空凭据")
	}
}
