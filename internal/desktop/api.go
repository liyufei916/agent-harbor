package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/orchestrator"
	"github.com/liyufei916/agent-harbor/internal/runtime"
	"github.com/liyufei916/agent-harbor/internal/secrets"
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

// CreateSession 转发给编排层；注入本项目记住的出站主机供 Gate 启动种子。
func (d *DesktopAPI) CreateSession(spec contracts.SessionSpec) contracts.SessionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if spec.Workspace != "" {
		spec.RememberedHosts = d.app.listRemembered(spec.Workspace)
	}
	spec.RememberedHosts = mergeRememberedWithTier(spec.RememberedHosts, spec.Workspace, d.app.prefs.SecurityTier)
	prefs := d.app.getPrefs()
	spec.DemoExpandAsk = prefs.DemoExpandAsk || envDemoExpandAsk()
	snap, err := d.app.sessionRef().Create(ctx, spec)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
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

// StartSession 在已停止的原会话上重新拉起，不创建新会话。
func (d *DesktopAPI) StartSession(id string) contracts.SessionResponse {
	starter, ok := d.app.sessionRef().(interface {
		Start(context.Context, string) (contracts.SessionSnapshot, error)
	})
	if !ok {
		return contracts.SessionResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := starter.Start(ctx, id)
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
	return contracts.NewAppError(contracts.ErrUnimplemented, "该方法尚未实现，形状已按契约占位。", false)
}

// ListAccounts 列出已保存账号元信息（无密钥）。stdio 账号附带 OfflineHint（桥是否在本进程就绪）。
func (d *DesktopAPI) ListAccounts(kind string) contracts.AccountsResponse {
	accounts := d.app.secrets.List(context.Background(), kind)
	if accounts == nil {
		accounts = []contracts.AccountInfo{}
	}
	out := make([]contracts.AccountInfo, len(accounts))
	for i, acc := range accounts {
		out[i] = acc
		if strings.EqualFold(strings.TrimSpace(acc.Transport), "stdio") {
			out[i].OfflineHint = d.app.stdioOfflineHint(acc.ID)
		}
	}
	return contracts.AccountsResponse{Accounts: out}
}

// RemoveAccount 删除账号及其内存凭据。
func (d *DesktopAPI) RemoveAccount(accountID string) contracts.EmptyResponse {
	d.app.closeStdioBridge(accountID)
	if err := d.app.secrets.Delete(context.Background(), accountID); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// ListSessions 转发编排层会话摘要。
func (d *DesktopAPI) ListSessions() contracts.SessionsResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items, err := d.app.sessionRef().List(ctx)
	if err != nil {
		return contracts.SessionsResponse{Sessions: []contracts.SessionSummary{}, Error: mapError(err)}
	}
	if items == nil {
		items = []contracts.SessionSummary{}
	}
	return contracts.SessionsResponse{Sessions: items}
}

// DestroySession 销毁已清理的终态会话记录。
func (d *DesktopAPI) DestroySession(sessionID string) contracts.SessionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := d.app.sessionRef().Destroy(ctx, sessionID)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

// SyncBackWorkspace 将 clone 副本合并写回原工作区（不自动打开访达，避免打断工作流）。
func (d *DesktopAPI) SyncBackWorkspace(sessionID string) contracts.SessionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	snap, err := d.app.sessionRef().SyncBack(ctx, sessionID)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

// GetRuntimeHealth 调用 Runtime.Health；未装配 Runtime 时返回编排未就绪。
func (d *DesktopAPI) GetRuntimeHealth() contracts.RuntimeHealthResponse {
	vaultNote := secrets.VaultBackendNotice()
	rt := d.app.runtimeRef()
	if rt == nil {
		return contracts.RuntimeHealthResponse{
			Available:   false,
			Reason:      "编排/Runtime 尚未装配",
			VaultNotice: vaultNote,
			Error:       contracts.NewAppError(contracts.ErrOrchestratorUnavailable, "编排服务尚未就绪", false),
		}
	}
	// Docker Health may inspect daemon + network + image (nested timeouts up to ~20s).
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	d.app.applyEngineDockerHost(ctx)
	if err := rt.Health(ctx); err != nil {
		ae := mapError(err)
		reason := ae.Message
		if ae.Code == contracts.ErrIsolationUnavailable {
			reason = ae.Message
		}
		return contracts.RuntimeHealthResponse{
			Available:   false,
			Reason:      reason,
			VaultNotice: vaultNote,
			Error:       ae,
		}
	}
	level := "seatbelt"
	reason := ""
	if runtime.DockerEnabled() {
		level = "docker"
		reason = "P2 Docker（harbor.gate 代理 · 镜像内 Agent）"
	}
	return contracts.RuntimeHealthResponse{
		Available:      true,
		Reason:         reason,
		IsolationLevel: level,
		VaultNotice:    vaultNote,
		Error:          nil,
	}
}

// GetPermissionOverview 返回只读边界摘要（无 Key/Sentinel）。
func (d *DesktopAPI) GetPermissionOverview(sessionID string) contracts.PermissionOverviewResponse {
	workspace := ""
	if sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		reader, ok := d.app.sessionRef().(interface {
			Boundary(context.Context, string) (contracts.PermissionOverview, error)
		})
		if !ok {
			return contracts.PermissionOverviewResponse{Error: unimplemented()}
		}
		overview, err := reader.Boundary(ctx, sessionID)
		if err != nil {
			return contracts.PermissionOverviewResponse{Error: mapError(err)}
		}
		return contracts.PermissionOverviewResponse{Overview: &overview}
	}
	hosts := map[string]struct{}{}
	var allowed []string
	for _, acc := range d.app.secrets.List(context.Background(), "") {
		if acc.Host == "" {
			continue
		}
		if _, ok := hosts[acc.Host]; ok {
			continue
		}
		hosts[acc.Host] = struct{}{}
		allowed = append(allowed, acc.Host)
	}
	for _, h := range d.app.listRemembered(workspace) {
		if _, ok := hosts[h.Host]; ok {
			continue
		}
		hosts[h.Host] = struct{}{}
		allowed = append(allowed, h.Host)
	}
	mcpNames := []string{}
	for _, acc := range d.app.secrets.List(context.Background(), "mcp") {
		mcpNames = append(mcpNames, contracts.FormatMCPSummary(acc))
	}
	overview := &contracts.PermissionOverview{
		SessionID:            sessionID,
		Workspace:            workspace,
		AllowedHosts:         allowed,
		MCPSummary:           mcpNames,
		DeniedPathCategories: []string{"系统目录", "SSH 密钥与云厂商凭证", "其它项目路径"},
	}
	return contracts.PermissionOverviewResponse{Overview: overview}
}

// ListRememberedHosts 返回本工作区记住的出站主机（落盘 ui-state.json；跨会话/跨重启回灌）。
func (d *DesktopAPI) ListRememberedHosts(workspace string) contracts.RememberedHostsResponse {
	hosts := d.app.listRemembered(workspace)
	if hosts == nil {
		hosts = []contracts.RememberedHost{}
	}
	return contracts.RememberedHostsResponse{Hosts: hosts}
}

// RevokeRememberedHost 撤销本项目记住的主机，并清掉活动会话 Gate 内同名扩权（否则同会话再 ask 仍直接放行、不弹窗）。
func (d *DesktopAPI) RevokeRememberedHost(in contracts.RevokeHostInput) contracts.EmptyResponse {
	if in.Host == "" {
		return contracts.EmptyResponse{Error: contracts.NewAppError(contracts.ErrInvalidArgument, "主机不能为空", false)}
	}
	_ = d.app.revokeHost(in.Workspace, in.Host) // 未知项幂等（CONTRACT）；仍须清 Gate
	if g := d.app.gateRef(); g != nil {
		g.RevokeExtraHostAll(in.Host)
	}
	return contracts.EmptyResponse{}
}

// PromptPermission 主动挂起一次出站扩权（竞品式 ask）；不依赖上游 302。事件经 harbor:event 送达。
func (d *DesktopAPI) PromptPermission(in contracts.PromptPermissionInput) contracts.EmptyResponse {
	target := strings.ToLower(strings.TrimSpace(in.Target))
	if in.SessionID == "" || !orchestrator.ValidPermissionTarget(target) {
		return contracts.EmptyResponse{Error: contracts.NewAppError(contracts.ErrInvalidArgument, "会话或目标主机不合法", false)}
	}
	prompter, ok := d.app.sessionRef().(orchestrator.PermissionAsker)
	if !ok {
		return contracts.EmptyResponse{Error: unimplemented()}
	}
	snap := d.GetSession(in.SessionID)
	if snap.Error != nil {
		return contracts.EmptyResponse{Error: snap.Error}
	}
	if snap.Session == nil || snap.Session.State != contracts.StateRunning {
		return contracts.EmptyResponse{Error: contracts.NewAppError(contracts.ErrInvalidArgument, "仅 running 会话可请求扩权", false)}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		_, _ = prompter.RequestPermission(ctx, in.SessionID, contracts.PermissionNeeded{
			Target: target,
			Action: "connect",
		})
	}()
	return contracts.EmptyResponse{}
}

// ResolvePermission 将扩权决策转交给唯一的会话状态机；project 成功时写入本机记住表。
func (d *DesktopAPI) ResolvePermission(in contracts.ResolvePermissionInput) contracts.EmptyResponse {
	resolver, ok := d.app.sessionRef().(interface {
		ResolvePermission(context.Context, contracts.ResolvePermissionInput) error
	})
	if !ok {
		return contracts.EmptyResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := resolver.ResolvePermission(ctx, in); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	if (in.Decision == "project" || in.Decision == "once") && in.Target != "" {
		ws := ""
		if project, ok := d.app.sessionRef().(orchestrator.ProjectWorkspaceReader); ok {
			ws, _ = project.ProjectWorkspace(ctx, in.SessionID)
		}
		if in.Decision == "project" && ws != "" {
			d.app.rememberHost(contracts.RememberedHost{
				Host:      in.Target,
				Workspace: ws,
				Scope:     "project",
			})
		}
	}
	return contracts.EmptyResponse{}
}

// GetUIPrefs 读取本机 UI 偏好（落盘 ui-state.json；跨重启保留）。
func (d *DesktopAPI) GetUIPrefs() contracts.UIPrefsResponse {
	prefs := d.app.getPrefs()
	return contracts.UIPrefsResponse{Prefs: &prefs}
}

// SetUIPrefs 写入本机 UI 偏好。
func (d *DesktopAPI) SetUIPrefs(prefs contracts.UIPrefs) contracts.UIPrefsResponse {
	prev := d.app.getPrefs()
	saved := d.app.setPrefs(prefs)
	if normalizeEngineSource(prev.EngineSource) != normalizeEngineSource(saved.EngineSource) {
		d.app.resetEngine()
	}
	return contracts.UIPrefsResponse{Prefs: &saved}
}

// SwitchModel 切换会话之后出站所用模型账号。
func (d *DesktopAPI) SwitchModel(in contracts.SwitchModelInput) contracts.SessionResponse {
	switcher, ok := d.app.sessionRef().(interface {
		SwitchModel(context.Context, contracts.SwitchModelInput) (contracts.SessionSnapshot, error)
	})
	if !ok {
		return contracts.SessionResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := switcher.SwitchModel(ctx, in)
	if err != nil {
		return contracts.SessionResponse{Error: mapError(err)}
	}
	return contracts.SessionResponse{Session: &snap}
}

// StartMcpOAuth 启动本机回环 OAuth 回调（authUrl 打开系统浏览器）。
func (d *DesktopAPI) StartMcpOAuth(in contracts.StartMcpOAuthInput) contracts.McpOAuthResponse {
	out, err := d.app.startMcpOAuth(in)
	if err != nil {
		return contracts.McpOAuthResponse{Error: mapError(err)}
	}
	return out
}

// CancelMcpOAuth 取消进行中的 OAuth pending。
func (d *DesktopAPI) CancelMcpOAuth(pendingID string) contracts.EmptyResponse {
	if err := d.app.cancelMcpOAuth(pendingID); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// WritePty 写入 PTY（占位）。
func (d *DesktopAPI) WritePty(in contracts.PtyInput) contracts.EmptyResponse {
	writer, ok := d.app.sessionRef().(interface {
		WritePty(context.Context, contracts.PtyInput) error
	})
	if !ok {
		return contracts.EmptyResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := writer.WritePty(ctx, in); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// ResizePty 调整 PTY（占位）。
func (d *DesktopAPI) ResizePty(in contracts.PtyInput) contracts.EmptyResponse {
	resizer, ok := d.app.sessionRef().(interface {
		ResizePty(context.Context, contracts.PtyInput) error
	})
	if !ok {
		return contracts.EmptyResponse{Error: unimplemented()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := resizer.ResizePty(ctx, in); err != nil {
		return contracts.EmptyResponse{Error: mapError(err)}
	}
	return contracts.EmptyResponse{}
}

// SaveTextFile 弹出系统另存为；可选在访达中显示已保存文件。
func (d *DesktopAPI) SaveTextFile(in contracts.SaveTextFileInput) contracts.SaveFileResponse {
	name := strings.TrimSpace(in.SuggestedName)
	if name == "" || strings.ContainsAny(name, `/\`) || len(name) > 200 {
		return contracts.SaveFileResponse{Error: contracts.NewAppError(contracts.ErrInvalidArgument, "文件名不合法", false)}
	}
	if len(in.Content) == 0 || len(in.Content) > 2<<20 {
		return contracts.SaveFileResponse{Error: contracts.NewAppError(contracts.ErrInvalidArgument, "内容为空或过大", false)}
	}
	wailsApp := application.Get()
	if wailsApp == nil {
		return contracts.SaveFileResponse{Error: contracts.NewAppError(contracts.ErrInternal, "应用未就绪", false)}
	}
	downloads := ""
	if home, err := os.UserHomeDir(); err == nil {
		downloads = filepath.Join(home, "Downloads")
	}
	dlg := wailsApp.Dialog.SaveFile().
		SetMessage("导出脱敏 JSON（不含 Key）").
		SetFilename(name).
		CanCreateDirectories(true)
	if downloads != "" {
		dlg = dlg.SetDirectory(downloads)
	}
	path, err := dlg.PromptForSingleSelection()
	return finishSaveTextFile(path, err, in.Content, in.Reveal, revealInFileManager)
}

// finishSaveTextFile 对话框选定后的写盘/取消/reveal（单测可注入路径）。
func finishSaveTextFile(path string, err error, content string, reveal bool, revealFn func(string)) contracts.SaveFileResponse {
	if err != nil {
		return contracts.SaveFileResponse{Error: mapError(err)}
	}
	if path == "" {
		return contracts.SaveFileResponse{Cancelled: true}
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return contracts.SaveFileResponse{Error: contracts.NewAppError(contracts.ErrInternal, "写入失败", true)}
	}
	if reveal && revealFn != nil {
		revealFn(path)
	}
	return contracts.SaveFileResponse{Path: path}
}

func envDemoExpandAsk() bool {
	v := strings.TrimSpace(os.Getenv("HARBOR_DEMO_ASK"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// revealInFileManager 在系统文件管理器中显示文件（失败静默）。
func revealInFileManager(path string) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	_ = cmd.Start()
}
