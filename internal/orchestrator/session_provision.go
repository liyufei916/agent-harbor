package orchestrator

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

func (o *Service) run(s *session) {
	provisionTimer := time.AfterFunc(time.Until(s.started.Add(o.timeouts.Provision)), func() { o.timeout(s) })
	sessionTimer := time.AfterFunc(time.Until(s.started.Add(o.timeouts.Session)), func() { o.sessionTimeout(s) })
	defer sessionTimer.Stop()
	go func() {
		select {
		case <-s.ctx.Done():
			o.mu.Lock()
			if s.snapshot.State == contracts.StateProvisioning || s.snapshot.State == contracts.StateRunning {
				o.stateLocked(s, contracts.StateStopping)
			}
			o.mu.Unlock()
		case <-s.finished:
		}
	}()
	err := o.provision(s)
	provisionTimer.Stop()
	if err != nil {
		o.fail(s, err, "INTERNAL_ERROR")
	}
	if err == nil && s.ctx.Err() == nil {
		select {
		case exit := <-s.wait:
			if exit.Code != 0 || exit.Err != nil || exit.Cancelled {
				o.fail(s, exit.Err, "AGENT_EXIT_FAILED")
			}
			// 保存已完成结果，清理阶段仍可重复等待。
			done := make(chan runtime.Exit, 1)
			done <- exit
			s.wait = done
		case <-s.ctx.Done():
		}
	}
	for {
		if o.cleanup(s) {
			return
		}
		<-s.retry
	}
}

func (o *Service) provision(s *session) error {
	ctx := s.ctx
	kit := o.deps.Kits[s.spec.AgentID]
	info, err := kit.Detect(ctx)
	if err != nil || !info.Agent.Available || info.Agent.ID != s.spec.AgentID {
		return publicError("AGENT_UNSUPPORTED")
	}
	if health, ok := o.deps.Runtime.(runtime.HealthChecker); ok {
		if err := health.HealthFor(ctx, info.NeedsChildProcesses); err != nil {
			return safeError(err, "ISOLATION_UNAVAILABLE")
		}
	} else if err := o.deps.Runtime.Health(ctx); err != nil {
		return safeError(err, "ISOLATION_UNAVAILABLE")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	model, err := o.deps.Catalog.Lookup(ctx, s.spec.ModelAccountID)
	if err != nil {
		return safeError(err, "ACCOUNT_NOT_FOUND")
	}
	if model.ID != s.spec.ModelAccountID || model.Kind != "model" {
		return publicError("INVALID_ARGUMENT")
	}
	mcps := make([]contracts.AccountInfo, 0, len(s.spec.MCPIDs))
	for _, id := range s.spec.MCPIDs {
		account, err := o.deps.Catalog.Lookup(ctx, id)
		if err != nil {
			return safeError(err, "ACCOUNT_NOT_FOUND")
		}
		if account.ID != id || account.Kind != "mcp" {
			return publicError("INVALID_ARGUMENT")
		}
		mcps = append(mcps, account)
	}
	p, err := o.deps.Policy.Build(ctx, policy.PolicyRequest{Workspace: s.spec.Workspace, AgentID: s.spec.AgentID, MountMode: s.spec.MountMode, Model: model, MCPs: mcps})
	if err != nil {
		return safeError(err, "WORKSPACE_DENIED")
	}
	if !filepath.IsAbs(p.Workspace) || (p.MountMode != "direct" && p.MountMode != "clone") || len(p.MCPMounts) != len(mcps) {
		return publicError("WORKSPACE_DENIED")
	}
	for i, mount := range p.MCPMounts {
		if mount.AccountID != mcps[i].ID {
			return publicError("WORKSPACE_DENIED")
		}
	}
	o.mu.Lock()
	s.snapshot.Workspace = p.Workspace
	s.boundary = sessionBoundary(p, mcps)
	o.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	extraHosts := map[string]string{}
	for _, host := range s.spec.RememberedHosts {
		name := strings.ToLower(strings.TrimSpace(host.Host))
		if name == "" {
			continue
		}
		scope := host.Scope
		if scope != "once" && scope != "project" {
			scope = "project"
		}
		extraHosts[name] = scope
	}
	binding, err := o.deps.Gate.Start(ctx, gate.GateSpec{SessionID: s.snapshot.SessionID, Policy: p, ModelAccountID: model.ID, ExtraHosts: extraHosts, EnableDemoAsk: s.spec.DemoExpandAsk, RequestPermission: func(call context.Context, request contracts.PermissionNeeded) (string, error) {
		return o.RequestPermission(call, s.snapshot.SessionID, request)
	}},
		func(data contracts.AuditData) { o.audit(s, data) }, func(err error) { o.fail(s, err, "INTERNAL_ERROR") })
	if err != nil {
		return err
	}
	s.gateStarted = true
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validBinding(binding) || len(bindingMCPURLs(binding)) != len(mcps) {
		return publicError("INVALID_SENTINEL")
	}
	o.mu.Lock()
	s.filters = map[string]*streamFilter{"stdout": {token: binding.Sentinel}, "stderr": {token: binding.Sentinel}}
	o.mu.Unlock()
	handle, err := o.deps.Runtime.Create(ctx, runtime.RuntimeSpec{SessionID: s.snapshot.SessionID, Workspace: p.Workspace, MountMode: p.MountMode, AllowedTCP: append([]string(nil), binding.AllowedTCP...), ReadOnlyPaths: append([]string(nil), info.ReadOnlyPaths...), AllowChildProcesses: info.NeedsChildProcesses, AllowDescendants: info.NeedsDescendants})
	// 接管成功返回的句柄，即使 Stop 已经取消，仍由统一清理器销毁。
	if handle.ID != "" {
		s.handle = handle
	}
	if err != nil {
		return err
	}
	if handle.ID == "" || !filepath.IsAbs(handle.PrivateHome) {
		return publicError("ISOLATION_UNAVAILABLE")
	}
	if p.MountMode == "clone" {
		if !filepath.IsAbs(handle.Workspace) || handle.Workspace == p.Workspace {
			return publicError("ISOLATION_UNAVAILABLE")
		}
		o.mu.Lock()
		s.snapshot.Workspace = handle.Workspace
		o.mu.Unlock()
		if err := o.restoreRetainedClone(ctx, s, handle.Workspace); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	mcpIDs := make([]string, 0, len(mcps))
	for _, acc := range mcps {
		mcpIDs = append(mcpIDs, acc.ID)
	}
	launch, err := kit.Build(ctx, kits.KitInput{
		Task: s.spec.Task, PrivateHome: handle.PrivateHome, ModelBaseURL: binding.ModelBaseURL,
		MCPURLs: bindingMCPURLs(binding), MCPIDs: mcpIDs, Sentinel: binding.Sentinel, Model: strings.TrimSpace(model.Model),
		Workspace: handle.Workspace,
	})
	if err != nil {
		return safeError(err, "AGENT_UNSUPPORTED")
	}
	// 创建任务仍交给 Kit 校验；再次开启不再写入 stdin，交互进程停在下一轮输入。
	if s.skipInitialTask {
		launch.Stdin = nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	process, err := o.deps.Runtime.Exec(ctx, handle, launch, func(data contracts.OutputData) { o.output(s, data) })
	if process != nil {
		done := make(chan runtime.Exit, 1)
		s.wait = done
		go func() { done <- process.Wait() }()
	}
	if err != nil {
		return err
	}
	if process == nil {
		return publicError("ISOLATION_UNAVAILABLE")
	}
	o.mu.Lock()
	s.process = process
	s.interactive = launch.Interactive || launch.Terminal
	defer o.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	o.stateLocked(s, contracts.StateRunning)
	s.outputReady = true
	o.drainEarlyLocked(s)
	return nil
}

func validBinding(b gate.GateBinding) bool {
	if len(b.Sentinel) < 32 || len(b.AllowedTCP) == 0 {
		return false
	}
	allowed := make(map[string]bool)
	for _, address := range b.AllowedTCP {
		host, port, err := net.SplitHostPort(address)
		number, portErr := strconv.Atoi(port)
		if err != nil || host != "127.0.0.1" || portErr != nil || number < 1 || number > 65535 || strconv.Itoa(number) != port {
			return false
		}
		allowed[address] = true
	}
	urls := append([]string{b.ModelBaseURL}, bindingMCPURLs(b)...)
	if len(b.MCPURLs) > 0 && b.MCPURL != "" && (len(b.MCPURLs) != 1 || b.MCPURLs[0] != b.MCPURL) {
		return false
	}
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "http" || !allowed[u.Host] || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return false
		}
	}
	return true
}

func (o *Service) cleanup(s *session) bool {
	o.mu.Lock()
	if s.snapshot.State != contracts.StateStopping {
		o.stateLocked(s, contracts.StateStopping)
	}
	s.cancel()
	o.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), o.timeouts.Cleanup)
	defer cancel()
	var cleanupErr error
	if s.gateStarted {
		// 为 Runtime 保留预算；即使撤销失败也继续销毁执行面。
		revokeCtx, revokeCancel := context.WithTimeout(ctx, o.timeouts.Cleanup/2)
		cleanupErr = o.deps.Gate.Revoke(revokeCtx, s.snapshot.SessionID)
		revokeCancel()
	}
	if s.handle.ID != "" {
		if s.spec.MountMode == "clone" && s.handle.Workspace != "" {
			if retained, err := retainCloneAside(s.snapshot.SessionID, s.handle.Workspace); err == nil {
				o.mu.Lock()
				s.retainedClone = retained
				s.snapshot.Workspace = retained
				o.mu.Unlock()
			}
		}
		cleanupErr = errors.Join(cleanupErr, o.deps.Runtime.Destroy(ctx, s.handle))
	}
	if s.wait != nil {
		select {
		case exit := <-s.wait:
			done := make(chan runtime.Exit, 1)
			done <- exit
			s.wait = done
		case <-ctx.Done():
			cleanupErr = errors.Join(cleanupErr, ctx.Err())
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	// 启动失败时也排空已产生的输出，终态之后不再接收回调。
	o.drainEarlyLocked(s)
	for stream, filter := range s.filters {
		if filter.pending != "" {
			o.outputLocked(s, contracts.OutputData{Stream: stream, Text: redacted})
			filter.pending = ""
		}
	}
	if cleanupErr != nil {
		s.snapshot.Error = publicError("CLEANUP_FAILED")
	} else {
		s.snapshot.Error = s.failure
		s.snapshot.CleanupComplete = true
		o.active = nil
	}
	if s.snapshot.Error != nil {
		errorCopy := *s.snapshot.Error
		o.emitLocked(s, contracts.Event{Type: "error", Error: &errorCopy})
		o.stateLocked(s, contracts.StateFailed)
	} else {
		o.stateLocked(s, contracts.StateStopped)
	}
	close(s.cleanupDone)
	if s.snapshot.CleanupComplete {
		close(s.finished)
		return true
	}
	return false
}

// restoreRetainedClone 把上次停止时旁路保留的副本盖回新的隔离目录。
func (o *Service) restoreRetainedClone(ctx context.Context, s *session, dest string) error {
	o.mu.Lock()
	src := s.resumeFrom
	o.mu.Unlock()
	if src == "" || sameWorkspace(src, dest) {
		return nil
	}
	if err := runtime.SyncWorkspace(ctx, src, dest); err != nil {
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		return publicError("INTERNAL_ERROR")
	}
	o.mu.Lock()
	if s.resumeFrom == src {
		s.resumeFrom = ""
	}
	o.mu.Unlock()
	return nil
}

func bindingMCPURLs(b gate.GateBinding) []string {
	if len(b.MCPURLs) > 0 {
		return append([]string{}, b.MCPURLs...)
	}
	if b.MCPURL != "" {
		return []string{b.MCPURL}
	}
	return []string{}
}
