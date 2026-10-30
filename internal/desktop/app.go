// Package desktop 是桌面控制面装配与 Wails 绑定（研发 B）。
// 会话状态机在 orchestrator；本包只做校验、错误转换和组件注入。
package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/engine"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/mcpstdio"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/runtime"
	"github.com/liyufei916/agent-harbor/internal/secrets"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SessionService 与研发 A 的 orchestrator.SessionService 方法一致。
type SessionService interface {
	WithConnection(context.Context, func(context.Context) error) error
	Create(context.Context, contracts.SessionSpec) (contracts.SessionSnapshot, error)
	Get(context.Context, string) (contracts.SessionSnapshot, error)
	Stop(context.Context, string) (contracts.SessionSnapshot, error)
	List(context.Context) ([]contracts.SessionSummary, error)
	Destroy(context.Context, string) (contracts.SessionSnapshot, error)
	SyncBack(context.Context, string) (contracts.SessionSnapshot, error)
	Shutdown(context.Context) error
}

// AgentDetector 由 Kit 注册表实现。
type AgentDetector interface {
	ListAgents(context.Context) ([]contracts.AgentInfo, error)
}

// App 持有控制面组件。前端只看到 DesktopAPI。
type App struct {
	secrets *secrets.Store
	policy  *policy.Builder
	gate    *gate.Harbor
	conn    *gate.Connections
	dataDir string

	mu           sync.Mutex
	sessions     SessionService
	agents       AgentDetector
	rt           runtime.Runtime
	eng          engine.Engine
	prefs        contracts.UIPrefs
	hosts        map[string]contracts.RememberedHost // key: workspace\x00host
	oauth        map[string]*oauthPending
	stdioBridges map[string]*mcpstdio.Bridge
	stdioFailed  map[string]string // accountID → 脱敏失败原因（桥未就绪）
	ready        bool
}

// NewApp 创建纯内存控制面（单测默认）。桌面进程请用 NewDesktopApp。
func NewApp() *App {
	return NewAppWithDataDir("")
}

// NewDesktopApp 使用本机 Application Support 落盘偏好/记住主机，并持久化凭据。
func NewDesktopApp() *App {
	return NewAppWithDataDir(defaultDataDir())
}

// NewAppWithDataDir 供测试注入临时目录；dataDir 空则纯内存凭据与状态。
func NewAppWithDataDir(dataDir string) *App {
	var store *secrets.Store
	if dataDir != "" {
		_ = os.MkdirAll(dataDir, 0700)
		if s, err := secrets.NewPersistentStore(filepath.Join(dataDir, "accounts")); err == nil {
			store = s
		}
	}
	if store == nil {
		store = secrets.NewStore()
	}
	pol := policy.NewBuilder(nil)
	app := &App{
		secrets:  store,
		policy:   pol,
		dataDir:  dataDir,
		sessions: unavailableSession{},
		prefs: contracts.UIPrefs{
			SidebarCollapsed:      false,
			AutoCollapseWorkbench: true,
			AskClearOnStop:        true,
			ClearOnStopDefault:    "keep",
		},
		hosts: make(map[string]contracts.RememberedHost),
	}
	app.loadPersistedState()
	app.gate = gate.New(store, pol)
	app.conn = gate.NewConnections(store, pol).WithStdioKeeper(app)
	app.stdioBridges = make(map[string]*mcpstdio.Bridge)
	app.stdioFailed = make(map[string]string)
	// 持久化 stdio 账号：应用重启后按 Keychain/文件库中的 command/args/env 重拉回环桥并更新 Host。
	restoreCtx, cancelRestore := context.WithTimeout(context.Background(), 45*time.Second)
	_ = app.RestoreStdioBridges(restoreCtx)
	cancelRestore()
	return app
}

// AdoptStdioBridge 由 Connections 在 stdio MCP 探测成功后调用。
func (a *App) AdoptStdioBridge(accountID string, bridge *mcpstdio.Bridge) {
	if a == nil || accountID == "" || bridge == nil {
		return
	}
	a.mu.Lock()
	if a.stdioBridges == nil {
		a.stdioBridges = make(map[string]*mcpstdio.Bridge)
	}
	if old := a.stdioBridges[accountID]; old != nil {
		_ = old.Close()
	}
	a.stdioBridges[accountID] = bridge
	if a.stdioFailed != nil {
		delete(a.stdioFailed, accountID)
	}
	a.mu.Unlock()
}

func (a *App) markStdioFailed(accountID, reason string) {
	if a == nil || accountID == "" {
		return
	}
	a.mu.Lock()
	if a.stdioFailed == nil {
		a.stdioFailed = make(map[string]string)
	}
	if reason == "" {
		reason = "宿主 stdio 桥未就绪，请在工具页重新探测。"
	}
	a.stdioFailed[accountID] = reason
	a.mu.Unlock()
}

func (a *App) clearStdioFailed(accountID string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	delete(a.stdioFailed, accountID)
	a.mu.Unlock()
}

func (a *App) hasStdioBridge(accountID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stdioBridges[accountID] != nil
}

func (a *App) stdioOfflineHint(accountID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stdioBridges[accountID] != nil {
		return ""
	}
	if msg := a.stdioFailed[accountID]; msg != "" {
		return msg
	}
	return "宿主 stdio 桥未在本进程拉起，请重新探测或重启应用。"
}

// RestoreStdioBridges 从 Secrets 重拉所有 transport=stdio 的宿主桥；失败账号保留，用户可在工具页重新探测。
// 返回成功重拉的账号数（供测试）；单账号失败不阻断其余。
func (a *App) RestoreStdioBridges(ctx context.Context) int {
	if a == nil || a.secrets == nil {
		return 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ok := 0
	for _, acc := range a.secrets.List(ctx, "mcp") {
		if !strings.EqualFold(strings.TrimSpace(acc.Transport), "stdio") {
			continue
		}
		if err := ctx.Err(); err != nil {
			break
		}
		raw, err := a.secrets.Resolve(ctx, acc.ID)
		if err != nil || len(raw) == 0 {
			a.markStdioFailed(acc.ID, "无法读取已存 stdio 命令，请重新添加。")
			continue
		}
		spec, err := mcpstdio.DecodeSecret(raw)
		if err != nil {
			a.markStdioFailed(acc.ID, "stdio 凭据格式无效，请删除后重连。")
			continue
		}
		if strings.TrimSpace(spec.Command) == "" {
			spec.Command = strings.TrimSpace(acc.Command)
		}
		oneCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		bridge, err := mcpstdio.Start(oneCtx, spec)
		cancel()
		if err != nil {
			a.markStdioFailed(acc.ID, "无法启动宿主 stdio 进程，请确认命令仍可用后重新探测。")
			continue
		}
		hostPort := strings.TrimPrefix(bridge.Origin, "http://")
		if err := a.secrets.UpdateEndpoint(ctx, acc.ID, hostPort, bridge.Tools); err != nil {
			_ = bridge.Close()
			a.markStdioFailed(acc.ID, "桥已启动但未能更新账号端口，请重新探测。")
			continue
		}
		a.AdoptStdioBridge(acc.ID, bridge)
		ok++
	}
	return ok
}

func (a *App) closeStdioBridge(accountID string) {
	a.mu.Lock()
	bridge := a.stdioBridges[accountID]
	delete(a.stdioBridges, accountID)
	delete(a.stdioFailed, accountID)
	a.mu.Unlock()
	if bridge != nil {
		_ = bridge.Close()
	}
}

func (a *App) closeAllStdioBridges() {
	a.mu.Lock()
	bridges := a.stdioBridges
	a.stdioBridges = make(map[string]*mcpstdio.Bridge)
	a.mu.Unlock()
	for _, bridge := range bridges {
		if bridge != nil {
			_ = bridge.Close()
		}
	}
}

// SetSessionService 替换会话服务。nil 表示尚未接入编排。
func (a *App) SetSessionService(s SessionService) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s == nil {
		a.sessions = unavailableSession{}
		a.ready = false
		return
	}
	a.sessions = s
	a.ready = true
}

// SetAgentDetector 替换 Agent 检测。
func (a *App) SetAgentDetector(d AgentDetector) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.agents = d
}

func (a *App) sessionRef() SessionService {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions
}

func (a *App) agentRef() AgentDetector {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agents
}

// SetRuntime 注入隔离 Runtime，供 GetRuntimeHealth 探测。
func (a *App) SetRuntime(rt runtime.Runtime) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rt = rt
}

func (a *App) runtimeRef() runtime.Runtime {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rt
}

func (a *App) orchestratorReady() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ready
}

func hostMapKey(workspace, host string) string {
	return strings.TrimSpace(workspace) + "\x00" + strings.ToLower(strings.TrimSpace(host))
}

func (a *App) listRemembered(workspace string) []contracts.RememberedHost {
	a.mu.Lock()
	defer a.mu.Unlock()
	want := strings.TrimSpace(workspace)
	out := make([]contracts.RememberedHost, 0, len(a.hosts))
	for _, h := range a.hosts {
		if want != "" && h.Workspace != want {
			continue
		}
		out = append(out, h)
	}
	return out
}

func (a *App) rememberHost(host contracts.RememberedHost) {
	a.mu.Lock()
	if a.hosts == nil {
		a.hosts = make(map[string]contracts.RememberedHost)
	}
	a.hosts[hostMapKey(host.Workspace, host.Host)] = host
	a.mu.Unlock()
	a.savePersistedState()
}

func (a *App) revokeHost(workspace, host string) bool {
	a.mu.Lock()
	key := hostMapKey(workspace, host)
	if _, ok := a.hosts[key]; !ok {
		a.mu.Unlock()
		return false
	}
	delete(a.hosts, key)
	a.mu.Unlock()
	a.savePersistedState()
	return true
}

func (a *App) getPrefs() contracts.UIPrefs {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.prefs
}

func (a *App) setPrefs(p contracts.UIPrefs) contracts.UIPrefs {
	a.mu.Lock()
	a.prefs = normalizeUIPrefs(p)
	out := a.prefs
	a.mu.Unlock()
	a.savePersistedState()
	return out
}

// forwardEvent 不缓存或推导状态；准入由编排统一管理。
func (a *App) forwardEvent(ev contracts.Event) { a.EmitHarborEvent(ev) }

// EmitHarborEvent 把契约事件发到前端固定通道。
func (a *App) EmitHarborEvent(ev contracts.Event) {
	if app := application.Get(); app != nil {
		app.Event.Emit(contracts.HarborEventName, ev)
	}
}

// Shutdown 先停止编排并关闭宿主 stdio 桥。纯内存凭据在退出时清空；持久化 Store（Keychain/文件）保留跨重启。
func (a *App) Shutdown(ctx context.Context) error {
	err := a.sessionRef().Shutdown(ctx)
	a.closeAllStdioBridges()
	if err == nil && a.dataDir == "" {
		a.secrets.Clear()
	}
	return err
}
