// Package gate 实现 Harbor Gate：模型反代、MCP Gateway 与连接测试。
// 负责人：研发 B。不导入 orchestrator / runtime / kits。
package gate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

const (
	auditToolModel = "model"
	auditToolMCP   = "mcp"
)

var (
	ErrSessionExists   = errors.New("gate: session already started")
	ErrSessionNotFound = errors.New("gate: session not found")
	ErrRevoked         = errors.New("gate: sentinel revoked")
)

// GateSpec 是启动闸口的编排输入。
type GateSpec struct {
	SessionID      string
	Policy         policy.PolicySnapshot
	ModelAccountID string
	MCPAccountID   string
	// ExtraHosts 会话启动时注入的记住主机（通常为 project）；key 小写主机名 → once|project。
	ExtraHosts map[string]string
	// EnableDemoAsk：为 true 时才向 tools/list 注入 harbor_demo_ask / harbor_demo_blocked，
	// 并本地拦截其 tools/call。默认关闭；由桌面 UIPrefs.demoExpandAsk 或 HARBOR_DEMO_ASK=1 打开。
	EnableDemoAsk bool
	// 可选的宿主机扩权决策回调；nil 时必须拒绝，不能自动放行。
	// 返回决策后 Gate/Policy 仍须重新求值授权，不得直接绕过固定规则。
	RequestPermission func(context.Context, contracts.PermissionNeeded) (string, error)
}

// GateBinding 返回给编排层的注入信息（不得进入前端 DTO）。
type GateBinding struct {
	ModelBaseURL string
	MCPURL       string
	MCPURLs      []string
	Sentinel     string
	AllowedTCP   []string
}

type sessionRuntime struct {
	spec       GateSpec
	sentinel   string
	model      *modelProxy
	mcp        *mcpGateway
	modelAddr  string
	mcpAddr    string
	cancel     context.CancelFunc
	ctx        context.Context
	revoked    bool
	active     sync.WaitGroup
	serving    sync.WaitGroup
	revokeOnce sync.Once
	done       chan struct{}
	audit      func(contracts.AuditData)
	fatal      func(error)
	// extraHosts：扩权追加的主机 → once|project（仅本会话热路径；once 消费后删除）。
	// project 跨会话持久化由桌面落盘记住表 + 新会话种子回灌负责，不在本 map。
	extraHosts map[string]string
}

// Harbor 是控制面出站闸口。
type Harbor struct {
	mu           sync.Mutex
	secrets      *secrets.Store
	policy       *policy.Builder
	client       *http.Client
	sessions     map[string]*sessionRuntime
	idleTimeout  time.Duration
	totalTimeout time.Duration
}

// New 创建 Gate；policy 用于查找固定 profile（连接测试）。
func New(store *secrets.Store, pol *policy.Builder) *Harbor {
	return &Harbor{
		secrets: store,
		policy:  pol,
		client: &http.Client{
			// 会话流量用单次请求的空闲与总超时，避免 Client.Timeout 在流式响应中途掐断。
			Timeout:   0,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		sessions: make(map[string]*sessionRuntime),
	}
}

// Start 创建随机 Sentinel，启动精确绑定 127.0.0.1 的模型与 MCP 端口。
// 仅在监听器 ready 后返回；部分失败会关闭已创建资源。
func (h *Harbor) Start(ctx context.Context, spec GateSpec, audit func(contracts.AuditData), fatal func(error)) (GateBinding, error) {
	if spec.SessionID == "" || spec.ModelAccountID == "" {
		return GateBinding{}, fmt.Errorf("gate: incomplete spec")
	}
	if _, err := h.secrets.Lookup(ctx, spec.ModelAccountID); err != nil {
		return GateBinding{}, err
	}
	mounts, err := h.mcpMounts(ctx, spec)
	if err != nil {
		return GateBinding{}, err
	}
	spec.Policy.MCPMounts = mounts

	sentinel, err := newSentinel()
	if err != nil {
		return GateBinding{}, err
	}

	h.mu.Lock()
	if _, exists := h.sessions[spec.SessionID]; exists {
		h.mu.Unlock()
		return GateBinding{}, ErrSessionExists
	}
	h.mu.Unlock()

	modelLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return GateBinding{}, err
	}
	var mcpLn net.Listener
	if len(mounts) > 0 {
		mcpLn, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			_ = modelLn.Close()
			return GateBinding{}, err
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	mp := newModelProxy(h, spec.SessionID, modelLn)
	var gw *mcpGateway
	if mcpLn != nil {
		gw = newMCPGateway(h, spec.SessionID, mcpLn)
	}

	extra := make(map[string]string)
	for host, scope := range spec.ExtraHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if scope != "once" && scope != "project" {
			scope = "project"
		}
		extra[host] = scope
	}
	rt := &sessionRuntime{
		spec:      spec,
		sentinel:  sentinel,
		model:     mp,
		mcp:       gw,
		modelAddr: modelLn.Addr().String(),
		cancel:    cancel,
		ctx:       runCtx, done: make(chan struct{}),
		audit:      audit,
		fatal:      fatal,
		extraHosts: extra,
	}

	mp.server.BaseContext = func(net.Listener) context.Context { return runCtx }
	rt.serving.Add(1)
	if gw != nil {
		rt.mcpAddr = mcpLn.Addr().String()
		gw.server.BaseContext = func(net.Listener) context.Context { return runCtx }
		rt.serving.Add(1)
	}

	h.mu.Lock()
	if _, exists := h.sessions[spec.SessionID]; exists {
		h.mu.Unlock()
		cancel()
		_ = modelLn.Close()
		if mcpLn != nil {
			_ = mcpLn.Close()
		}
		return GateBinding{}, ErrSessionExists
	}
	h.sessions[spec.SessionID] = rt
	h.mu.Unlock()

	go func() { defer rt.serving.Done(); mp.serve(runCtx) }()
	if gw != nil {
		go func() { defer rt.serving.Done(); gw.serve(runCtx) }()
	}
	go func() { <-runCtx.Done(); _ = h.Revoke(context.Background(), spec.SessionID) }()

	// 确认端口可接受连接
	addrs := []string{rt.modelAddr}
	if gw != nil {
		addrs = append(addrs, rt.mcpAddr)
	}
	if err := waitReady(ctx, addrs...); err != nil {
		_ = h.Revoke(context.Background(), spec.SessionID)
		return GateBinding{}, err
	}

	binding := GateBinding{ModelBaseURL: "http://" + rt.modelAddr, Sentinel: sentinel, AllowedTCP: addrs, MCPURLs: []string{}}
	for i := range mounts {
		endpoint := "http://" + rt.mcpAddr + "/mcp"
		if len(mounts) > 1 {
			endpoint += "/" + strconv.Itoa(i)
		}
		binding.MCPURLs = append(binding.MCPURLs, endpoint)
	}
	if len(mounts) == 1 {
		binding.MCPURL = binding.MCPURLs[0]
	}
	return binding, nil
}

func waitReady(ctx context.Context, addrs ...string) error {
	deadline := time.Now().Add(2 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	for _, addr := range addrs {
		for {
			d := net.Dialer{Timeout: 100 * time.Millisecond}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err == nil {
				_ = conn.Close()
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("gate: listener not ready: %s", addr)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

// Revoke 先本地失效令牌，再关闭监听器；幂等。
func (h *Harbor) Revoke(ctx context.Context, sessionID string) error {
	h.mu.Lock()
	rt, ok := h.sessions[sessionID]
	if ok {
		rt.revoked = true
	}
	h.mu.Unlock()
	if !ok {
		return nil
	}
	// Sentinel 内容不可并发改写；在锁内标记失效，取消所有入站及上游 context。
	rt.cancel()
	rt.revokeOnce.Do(func() {
		go func() {
			if rt.model != nil {
				rt.model.close()
			}
			if rt.mcp != nil {
				rt.mcp.close()
			}
			rt.serving.Wait()
			rt.active.Wait()
			h.client.CloseIdleConnections()
			close(rt.done)
		}()
	})
	select {
	case <-rt.done:
		h.mu.Lock()
		if h.sessions[sessionID] == rt {
			delete(h.sessions, sessionID)
		}
		h.mu.Unlock()
		return nil
	case <-ctx.Done():
		return errors.New(contracts.ErrCleanupFailed)
	}
}

func (h *Harbor) getSession(sessionID string) (*sessionRuntime, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.revoked || rt.ctx.Err() != nil {
		return nil, false
	}
	return rt, true
}

// acquireSession 和撤销共用锁，禁止 Wait 开始后再增加在途计数。
func (h *Harbor) acquireSession(sessionID string) (*sessionRuntime, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.revoked || rt.ctx.Err() != nil {
		return nil, false
	}
	rt.active.Add(1)
	return rt, true
}

func (h *Harbor) emitAudit(rt *sessionRuntime, data contracts.AuditData) {
	if rt != nil && rt.audit != nil {
		rt.audit(data)
	}
}

func (h *Harbor) emitFatal(rt *sessionRuntime, err error) {
	if rt != nil && rt.fatal != nil && err != nil {
		rt.fatal(err)
	}
}

// Health 返回闸口进程自身存活状态。
func (h *Harbor) Health() string {
	return "ok"
}

// SetModelAccount 更新运行中会话的模型账号（仅影响之后出站代填；不改 Origin）。
func (h *Harbor) SetModelAccount(sessionID, accountID string) error {
	return h.SwitchModel(context.Background(), sessionID, accountID)
}

// AllowExtraHost 将扩权通过的主机记入本会话 extraHosts（once 仅一次；project 本会话常驻到撤销）。
// 跨会话「本项目」持久化不在此方法：由桌面落盘 + CreateSession 种子回灌完成。
func (h *Harbor) AllowExtraHost(sessionID, host, scope string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if sessionID == "" || host == "" {
		return fmt.Errorf("gate: incomplete host allow")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.revoked || rt.ctx.Err() != nil {
		return ErrSessionNotFound
	}
	if rt.extraHosts == nil {
		rt.extraHosts = make(map[string]string)
	}
	if scope != "once" && scope != "project" {
		scope = "once"
	}
	rt.extraHosts[host] = scope
	return nil
}

// HasExtraHost 查询是否已扩权；once 命中后消费删除。
func (h *Harbor) HasExtraHost(sessionID, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.extraHosts == nil {
		return false
	}
	scope, ok := rt.extraHosts[host]
	if !ok {
		return false
	}
	if scope == "once" {
		delete(rt.extraHosts, host)
	}
	return true
}

// RevokeExtraHost 从指定会话去掉已扩权主机（幂等）。用于 UI「撤销记住」后同会话再 ask 能再弹窗。
func (h *Harbor) RevokeExtraHost(sessionID, host string) {
	host = strings.ToLower(strings.TrimSpace(host))
	if sessionID == "" || host == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.extraHosts == nil {
		return
	}
	delete(rt.extraHosts, host)
}

// RevokeExtraHostAll 从全部活动会话去掉该主机（幂等）。
func (h *Harbor) RevokeExtraHostAll(host string) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, rt := range h.sessions {
		if rt == nil || rt.extraHosts == nil {
			continue
		}
		delete(rt.extraHosts, host)
	}
}
