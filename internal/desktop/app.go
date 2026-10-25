// Package desktop 是桌面控制面装配与 Wails 绑定（研发 B）。
// 会话状态机在 orchestrator；本包只做校验、错误转换和组件注入。
package desktop

import (
	"context"
	"sync"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
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

	mu        sync.Mutex
	sessions  SessionService
	agents    AgentDetector
	ready     bool
	oauth     *gate.OAuthFlow
	prefsPath string
	uses      map[string]accountUse
}

type accountUse struct {
	model  string
	mcps   []string
	active bool
}

// NewApp 创建 Gate、Policy 与内存凭据。编排在 AttachOrchestrator 中注入。
func NewApp() *App {
	store := secrets.NewStore()
	pol := policy.NewBuilder(nil)
	app := &App{
		secrets:  store,
		policy:   pol,
		sessions: unavailableSession{},
	}
	app.gate = gate.New(store, pol)
	app.conn = gate.NewConnections(store, pol)
	app.oauth = gate.NewOAuthFlow(store, pol)
	app.uses = map[string]accountUse{}
	return app
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

func (a *App) orchestratorReady() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ready
}

// forwardEvent 转发契约事件，并记下账号是否仍被活动会话引用。
func (a *App) forwardEvent(ev contracts.Event) {
	if ev.State != nil {
		a.noteSnapshot(*ev.State)
	}
	a.EmitHarborEvent(ev)
}

// EmitHarborEvent 把契约事件发到前端固定通道。
func (a *App) EmitHarborEvent(ev contracts.Event) {
	if app := application.Get(); app != nil {
		app.Event.Emit(contracts.HarborEventName, ev)
	}
}

// Shutdown 先停止编排，再清空内存凭据。
func (a *App) Shutdown(ctx context.Context) error {
	err := a.sessionRef().Shutdown(ctx)
	if a.oauth != nil {
		_ = a.oauth.Close()
	}
	if err == nil {
		a.secrets.Clear()
	}
	return err
}

func (a *App) noteCreate(spec contracts.SessionSpec, snap contracts.SessionSnapshot) {
	if snap.SessionID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.uses[snap.SessionID] = accountUse{model: spec.ModelAccountID, mcps: append([]string(nil), spec.MCPIDs...), active: sessionActive(snap)}
}

func (a *App) noteSnapshot(snap contracts.SessionSnapshot) {
	if snap.SessionID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	use, ok := a.uses[snap.SessionID]
	if !ok {
		return
	}
	use.active = sessionActive(snap)
	a.uses[snap.SessionID] = use
}

func (a *App) accountInUse(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, use := range a.uses {
		if !use.active {
			continue
		}
		if use.model == id {
			return true
		}
		for _, mcp := range use.mcps {
			if mcp == id {
				return true
			}
		}
	}
	return false
}

func sessionActive(snap contracts.SessionSnapshot) bool {
	switch snap.State {
	case contracts.StateProvisioning, contracts.StateRunning, contracts.StateStopping:
		return true
	case contracts.StateFailed:
		return !snap.CleanupComplete
	default:
		return false
	}
}

func (a *App) probeHealth(ctx context.Context) (bool, string) {
	err := runtime.NewMacOS().Health(ctx)
	if err == nil {
		return true, ""
	}
	return false, "隔离能力不可用"
}
