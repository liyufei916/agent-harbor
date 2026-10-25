package desktop

import (
	"context"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DesktopAPI 是唯一绑定到 Wails 的业务服务。
// 方法返回统一 Response，不把原始 Go error 交给前端。
type DesktopAPI struct {
	app *App
}

// NewDesktopAPI 创建前端绑定对象。
func NewDesktopAPI(app *App) *DesktopAPI {
	return &DesktopAPI{app: app}
}

// ListAgents 返回本机 Agent 检测结果。
func (d *DesktopAPI) ListAgents() contracts.AgentsResponse {
	det := d.app.agentRef()
	if det == nil {
		return contracts.AgentsResponse{Agents: []contracts.AgentInfo{}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	list, err := det.ListAgents(ctx)
	if err != nil {
		return contracts.AgentsResponse{Agents: []contracts.AgentInfo{}, Error: mapError(err)}
	}
	if list == nil {
		list = []contracts.AgentInfo{}
	}
	return contracts.AgentsResponse{Agents: list}
}

// SelectWorkspace 打开系统目录面板。
func (d *DesktopAPI) SelectWorkspace() contracts.WorkspaceResponse {
	wailsApp := application.Get()
	if wailsApp == nil {
		return contracts.WorkspaceResponse{Error: contracts.NewAppError(contracts.ErrInternal, "应用未就绪", false)}
	}
	path, err := wailsApp.Dialog.OpenFile().
		SetTitle("选择工作区").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	return workspaceResult(path, err)
}

func workspaceResult(path string, err error) contracts.WorkspaceResponse {
	if err != nil {
		return contracts.WorkspaceResponse{Error: mapError(err)}
	}
	if path == "" {
		return contracts.WorkspaceResponse{Cancelled: true}
	}
	return contracts.WorkspaceResponse{Path: path}
}

// ConnectModel 测试并保存模型账号。
func (d *DesktopAPI) ConnectModel(in contracts.ConnectInput) contracts.AccountResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var meta contracts.AccountInfo
	err := d.app.sessionRef().WithConnection(ctx, func(call context.Context) error {
		var err error
		meta, err = d.app.conn.ConnectModel(call, in)
		return err
	})
	if err != nil {
		return contracts.AccountResponse{Error: mapError(err)}
	}
	return contracts.AccountResponse{Account: &meta}
}

// ConnectMcp 测试并保存 MCP 账号。
func (d *DesktopAPI) ConnectMcp(in contracts.ConnectInput) contracts.AccountResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var meta contracts.AccountInfo
	err := d.app.sessionRef().WithConnection(ctx, func(call context.Context) error {
		var err error
		meta, err = d.app.conn.ConnectMcp(call, in)
		return err
	})
	if err != nil {
		return contracts.AccountResponse{Error: mapError(err)}
	}
	return contracts.AccountResponse{Account: &meta}
}

// CreateSession 转发给编排层。
func (d *DesktopAPI) CreateSession(spec contracts.SessionSpec) contracts.SessionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := d.app.sessionRef().Create(ctx, spec)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	d.app.noteCreate(spec, snap)
	return contracts.SessionResponse{Session: &snap}
}

// GetSession 返回会话快照。
func (d *DesktopAPI) GetSession(id string) contracts.SessionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := d.app.sessionRef().Get(ctx, id)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

// StopSession 请求停止会话。
func (d *DesktopAPI) StopSession(id string) contracts.SessionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := d.app.sessionRef().Stop(ctx, id)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

func unimplemented() *contracts.AppError {
	return contracts.NewAppError(contracts.ErrUnimplemented, "该方法尚未由编排层实现。", false)
}

// ListAccounts 返回已保存账号，不含密钥。
func (d *DesktopAPI) ListAccounts(kind string) contracts.AccountsResponse {
	list, err := d.app.secrets.List(kind)
	if err != nil {
		return contracts.AccountsResponse{Accounts: []contracts.AccountInfo{}, Error: mapError(err)}
	}
	return contracts.AccountsResponse{Accounts: list}
}

// RemoveAccount 删除账号。活动会话仍引用时拒绝。
func (d *DesktopAPI) RemoveAccount(accountID string) contracts.EmptyResponse {
	if accountID == "" {
		return contracts.EmptyResponse{Error: contracts.NewAppError(contracts.ErrInvalidArgument, "输入不合法", false)}
	}
	if d.app.accountInUse(accountID) {
		return contracts.EmptyResponse{Error: contracts.NewAppError(contracts.ErrSessionBusy, "账号仍被活动会话使用", false)}
	}
	if err := d.app.secrets.Delete(context.Background(), accountID); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// ListSessions 转发编排层。编排未提供列表时保持方法存在并返回 UNIMPLEMENTED。
func (d *DesktopAPI) ListSessions() contracts.SessionsResponse {
	type lister interface {
		List(context.Context) ([]contracts.SessionSummary, error)
	}
	svc, ok := d.app.sessionRef().(lister)
	if !ok {
		return contracts.SessionsResponse{Sessions: []contracts.SessionSummary{}, Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := svc.List(ctx)
	if err != nil {
		return contracts.SessionsResponse{Sessions: []contracts.SessionSummary{}, Error: mapError(err)}
	}
	if list == nil {
		list = []contracts.SessionSummary{}
	}
	return contracts.SessionsResponse{Sessions: list}
}

// DestroySession 转发编排层销毁终态会话。
func (d *DesktopAPI) DestroySession(sessionID string) contracts.SessionResponse {
	type destroyer interface {
		Destroy(context.Context, string) (contracts.SessionSnapshot, error)
	}
	svc, ok := d.app.sessionRef().(destroyer)
	if !ok {
		return contracts.SessionResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := svc.Destroy(ctx, sessionID)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

// GetRuntimeHealth 调用 Runtime.Health。隔离不可用时 available=false，不把探测结果当成接口故障。
func (d *DesktopAPI) GetRuntimeHealth() contracts.RuntimeHealthResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ok, reason := d.app.probeHealth(ctx)
	return contracts.RuntimeHealthResponse{Available: ok, Reason: reason, IsolationLevel: "seatbelt"}
}

// GetPermissionOverview 返回不含密钥的边界摘要。
func (d *DesktopAPI) GetPermissionOverview(sessionID string) contracts.PermissionOverviewResponse {
	workspace := ""
	if sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		snap, err := d.app.sessionRef().Get(ctx, sessionID)
		if err != nil {
			return contracts.PermissionOverviewResponse{Error: mapError(err)}
		}
		workspace = snap.Workspace
	}
	mcp := []string{}
	if prof, ok := d.app.policy.Profile("demo-mcp"); ok && prof.Host != "" {
		mcp = append(mcp, prof.ID)
	}
	hosts := d.app.policy.AllowedHostNames()
	return contracts.PermissionOverviewResponse{Overview: &contracts.PermissionOverview{
		SessionID:            sessionID,
		Workspace:            workspace,
		AllowedHosts:         hosts,
		MCPSummary:           mcp,
		DeniedPathCategories: []string{"ssh", "aws", "gnupg", "keychain", "home"},
	}}
}

// ListRememberedHosts 返回已记住主机。
func (d *DesktopAPI) ListRememberedHosts(workspace string) contracts.RememberedHostsResponse {
	return contracts.RememberedHostsResponse{Hosts: d.app.policy.ListRememberedHosts(workspace)}
}

// RevokeRememberedHost 撤销记住项。未知主机也成功。
func (d *DesktopAPI) RevokeRememberedHost(in contracts.RevokeHostInput) contracts.EmptyResponse {
	d.app.policy.RevokeRememberedHost(in.Host, in.Workspace)
	return contracts.EmptyResponse{}
}

// ResolvePermission 转发给编排层。尚无挂起处理时返回 UNIMPLEMENTED。
func (d *DesktopAPI) ResolvePermission(in contracts.ResolvePermissionInput) contracts.EmptyResponse {
	type resolver interface {
		ResolvePermission(context.Context, contracts.ResolvePermissionInput) error
	}
	svc, ok := d.app.sessionRef().(resolver)
	if !ok {
		return contracts.EmptyResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.ResolvePermission(ctx, in); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// GetUIPrefs 读取本机偏好。没有文件时返回默认值。
func (d *DesktopAPI) GetUIPrefs() contracts.UIPrefsResponse {
	prefs := d.app.loadPrefs()
	return contracts.UIPrefsResponse{Prefs: &prefs}
}

// SetUIPrefs 只写入侧栏偏好，不接受密钥字段。
func (d *DesktopAPI) SetUIPrefs(prefs contracts.UIPrefs) contracts.UIPrefsResponse {
	if err := d.app.savePrefs(prefs); err != nil {
		return contracts.UIPrefsResponse{Error: contracts.NewAppError(contracts.ErrInternal, "内部错误", false)}
	}
	stored := d.app.loadPrefs()
	return contracts.UIPrefsResponse{Prefs: &stored}
}

// SwitchModel 转发编排层。编排未实现时返回 UNIMPLEMENTED。
func (d *DesktopAPI) SwitchModel(in contracts.SwitchModelInput) contracts.SessionResponse {
	type switcher interface {
		SwitchModel(context.Context, contracts.SwitchModelInput) (contracts.SessionSnapshot, error)
	}
	svc, ok := d.app.sessionRef().(switcher)
	if !ok {
		return contracts.SessionResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := svc.SwitchModel(ctx, in)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

// StartMcpOAuth 登记回环并返回授权地址。浏览器由桌面打开。
func (d *DesktopAPI) StartMcpOAuth(in contracts.StartMcpOAuthInput) contracts.McpOAuthResponse {
	authURL, pendingID, err := d.app.oauth.Start(in)
	if err != nil {
		return contracts.McpOAuthResponse{Error: mapError(err)}
	}
	openBrowser(authURL)
	return contracts.McpOAuthResponse{AuthURL: authURL, PendingID: pendingID}
}

// CancelMcpOAuth 清除 pending。
func (d *DesktopAPI) CancelMcpOAuth(pendingID string) contracts.EmptyResponse {
	if err := d.app.oauth.Cancel(pendingID); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// WritePty 转发编排层。当前 Runtime 没有 PTY 时返回 UNIMPLEMENTED。
func (d *DesktopAPI) WritePty(in contracts.PtyInput) contracts.EmptyResponse {
	return d.forwardPty(in, true)
}

// ResizePty 转发编排层。当前 Runtime 没有 PTY 时返回 UNIMPLEMENTED。
func (d *DesktopAPI) ResizePty(in contracts.PtyInput) contracts.EmptyResponse {
	return d.forwardPty(in, false)
}

func (d *DesktopAPI) forwardPty(in contracts.PtyInput, write bool) contracts.EmptyResponse {
	type ptyBridge interface {
		WritePty(context.Context, contracts.PtyInput) error
		ResizePty(context.Context, contracts.PtyInput) error
	}
	svc, ok := d.app.sessionRef().(ptyBridge)
	if !ok {
		return contracts.EmptyResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	if write {
		err = svc.WritePty(ctx, in)
	} else {
		err = svc.ResizePty(ctx, in)
	}
	if err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}
