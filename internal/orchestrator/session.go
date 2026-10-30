package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

type Service struct {
	mu               sync.Mutex
	app              context.Context
	deps             Dependencies
	timeouts         Timeouts
	sessions         map[string]*session
	active           *session
	closing          bool
	connectionDone   chan struct{}
	connectionCancel context.CancelFunc
	events           *eventQueue
}

type session struct {
	snapshot       contracts.SessionSnapshot
	spec           contracts.SessionSpec
	ctx            context.Context
	cancel         context.CancelFunc
	failure        *contracts.AppError
	handle         runtime.Handle
	retainedClone  string // 停止后保留的副本路径；Destroy 时删除
	gateStarted    bool
	wait           <-chan runtime.Exit
	process        runtime.Process
	interactive    bool
	boundary       contracts.PermissionOverview
	permissions    map[string]*permissionRequest
	cleanupDone    chan struct{}
	finished       chan struct{}
	retry          chan struct{}
	early          []contracts.OutputData
	earlyBytes     int
	earlyTruncated bool
	started        time.Time
	outputReady    bool
	filters        map[string]*streamFilter
}

var _ SessionService = (*Service)(nil)

func New(app context.Context, deps Dependencies, sink func(contracts.Event), timeouts Timeouts) (*Service, error) {
	if app == nil || deps.Runtime == nil || deps.Gate == nil || deps.Policy == nil || deps.Catalog == nil || len(deps.Kits) == 0 {
		return nil, publicError("INVALID_ARGUMENT")
	}
	registered := make(map[string]kits.Kit, len(deps.Kits))
	for id, kit := range deps.Kits {
		if id == "" || kit == nil {
			return nil, publicError("INVALID_ARGUMENT")
		}
		registered[id] = kit
	}
	deps.Kits = registered
	if timeouts.Provision <= 0 {
		timeouts.Provision = 30 * time.Second
	}
	if timeouts.Session <= 0 {
		timeouts.Session = 10 * time.Minute
	}
	if timeouts.Cleanup <= 0 {
		timeouts.Cleanup = 5 * time.Second
	}
	return &Service{app: app, deps: deps, timeouts: timeouts, sessions: make(map[string]*session), events: newEventQueue(sink)}, nil
}

func (o *Service) Create(ctx context.Context, spec contracts.SessionSpec) (contracts.SessionSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	if !utf8.ValidString(spec.Task) || len(spec.Task) < 1 || len(spec.Task) > 32768 || strings.TrimSpace(spec.Task) == "" ||
		strings.TrimSpace(spec.Workspace) == "" || spec.ModelAccountID == "" || len(spec.MCPIDs) > 8 || strings.ContainsRune(spec.Workspace, 0) {
		return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
	}
	seenMCP := map[string]bool{}
	for _, id := range spec.MCPIDs {
		if strings.TrimSpace(id) == "" || seenMCP[id] {
			return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
		}
		seenMCP[id] = true
	}
	if _, ok := o.deps.Kits[spec.AgentID]; !ok {
		return contracts.SessionSnapshot{}, publicError("AGENT_UNSUPPORTED")
	}
	if spec.MountMode == "" {
		spec.MountMode = "direct"
	}
	if spec.MountMode != "direct" && spec.MountMode != "clone" {
		return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return contracts.SessionSnapshot{}, publicError("INTERNAL_ERROR")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing || o.app.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	if o.connectionDone != nil {
		return contracts.SessionSnapshot{}, publicError("SESSION_BUSY")
	}
	workspace, err := canonicalWorkspace(spec.Workspace)
	if err != nil {
		return contracts.SessionSnapshot{}, publicError("WORKSPACE_DENIED")
	}
	activeCount := 0
	for _, existing := range o.sessions {
		if existing.snapshot.CleanupComplete {
			continue
		}
		activeCount++
		if existing.snapshot.Workspace != "" && sameWorkspace(existing.spec.Workspace, workspace) {
			return contracts.SessionSnapshot{}, publicError("WORKSPACE_BUSY")
		}
	}
	if activeCount >= 2 {
		return contracts.SessionSnapshot{}, publicError("SESSION_LIMIT")
	}
	spec.Workspace = workspace
	spec.MCPIDs = append([]string(nil), spec.MCPIDs...)
	sessionCtx, cancel := context.WithCancel(o.app)
	s := &session{spec: spec, ctx: sessionCtx, cancel: cancel, retry: make(chan struct{}, 1), cleanupDone: make(chan struct{}), finished: make(chan struct{})}
	s.started = time.Now()
	s.snapshot = contracts.SessionSnapshot{
		SessionID:       hex.EncodeToString(id[:]),
		Workspace:       workspace,
		SourceWorkspace: workspace,
		MountMode:       spec.MountMode,
		ModelAccountID:  spec.ModelAccountID,
		MCPIDs:          append([]string(nil), spec.MCPIDs...),
		AgentID:         spec.AgentID,
	}
	o.sessions[s.snapshot.SessionID] = s
	o.stateLocked(s, contracts.StateProvisioning)
	initial := snapshot(s)
	go o.run(s)
	return initial, nil
}

func (o *Service) Get(ctx context.Context, id string) (contracts.SessionSnapshot, error) {
	if ctx.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.sessions[id]
	if !ok {
		return contracts.SessionSnapshot{}, publicError("SESSION_NOT_FOUND")
	}
	return snapshot(s), nil
}

func (o *Service) Stop(ctx context.Context, id string) (contracts.SessionSnapshot, error) {
	if ctx.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.sessions[id]
	if !ok {
		return contracts.SessionSnapshot{}, publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.CleanupComplete || s.snapshot.State == contracts.StateStopping {
		return snapshot(s), nil
	}
	if s.snapshot.State == contracts.StateFailed {
		s.cleanupDone = make(chan struct{})
		o.stateLocked(s, contracts.StateStopping)
		s.retry <- struct{}{}
	} else {
		o.stateLocked(s, contracts.StateStopping)
		s.cancel()
	}
	return snapshot(s), nil
}

// WithConnection 供 ConnectModel/ConnectMcp：与并发 Connect、以及进行中的 Create 互斥。
// 已有 running 会话不拦截——新增账号只进本机列表，不改写当前会话已挂载的 MCP/模型。
func (o *Service) WithConnection(ctx context.Context, action func(context.Context) error) error {
	if ctx.Err() != nil {
		return publicError("CANCELLED")
	}
	o.mu.Lock()
	if o.closing || o.app.Err() != nil {
		o.mu.Unlock()
		return publicError("CANCELLED")
	}
	if o.connectionDone != nil {
		o.mu.Unlock()
		return publicError("SESSION_BUSY")
	}
	// Create 刚占槽、尚在 provisioning 时也互斥，避免探测与创建竞态写账号表。
	for _, s := range o.sessions {
		if s.snapshot.State == contracts.StateProvisioning {
			o.mu.Unlock()
			return publicError("SESSION_BUSY")
		}
	}
	call, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	o.connectionDone = done
	o.connectionCancel = cancel
	o.mu.Unlock()
	stop := context.AfterFunc(o.app, cancel)
	defer func() {
		stop()
		cancel()
		o.mu.Lock()
		o.connectionDone = nil
		o.connectionCancel = nil
		close(done)
		o.mu.Unlock()
	}()
	return action(call)
}

func (o *Service) Shutdown(ctx context.Context) error {
	o.mu.Lock()
	o.closing = true
	var active []*session
	for _, item := range o.sessions {
		if !item.snapshot.CleanupComplete {
			active = append(active, item)
		}
	}
	connectionDone := o.connectionDone
	if o.connectionCancel != nil {
		o.connectionCancel()
	}
	o.mu.Unlock()
	// 先通知全部会话，再等待连接及各会话清理，避免先遇到慢会话而漏停其他进程。
	for _, s := range active {
		// 即使调用方期限已到，取消也必须送达；Stop 本身不等待资源清理。
		if _, err := o.Stop(context.Background(), s.snapshot.SessionID); err != nil {
			return err
		}
	}
	if connectionDone != nil {
		select {
		case <-connectionDone:
		case <-ctx.Done():
			return publicError("CLEANUP_FAILED")
		}
	}
	for _, s := range active {
		o.mu.Lock()
		done := s.cleanupDone
		o.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return publicError("CLEANUP_FAILED")
		}
		o.mu.Lock()
		complete := s.snapshot.CleanupComplete
		o.mu.Unlock()
		if !complete {
			return publicError("CLEANUP_FAILED")
		}
	}
	o.events.close()
	select {
	case <-o.events.done:
		return nil
	case <-ctx.Done():
		return publicError("CLEANUP_FAILED")
	}
}

func snapshot(s *session) contracts.SessionSnapshot {
	result := s.snapshot
	if result.Error != nil {
		e := *result.Error
		result.Error = &e
	}
	return result
}

func canonicalWorkspace(path string) (string, error) {
	if strings.TrimSpace(path) == "" || strings.ContainsRune(path, 0) {
		return "", errors.New("invalid workspace")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Policy/Runtime 负责最终确认工作区存在；编排层仍需能为替身和
		// 正在创建的目录建立稳定的冲突键。
		if errors.Is(err, os.ErrNotExist) {
			return filepath.Clean(abs), nil
		}
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func sameWorkspace(a, b string) bool {
	ca, errA := canonicalWorkspace(a)
	cb, errB := canonicalWorkspace(b)
	return errA == nil && errB == nil && ca == cb
}

func hasActiveSessionLocked(o *Service) bool {
	for _, s := range o.sessions {
		if !s.snapshot.CleanupComplete {
			return true
		}
	}
	return false
}

// List 返回当前应用生命周期内的会话摘要，终态记录会保留到 Destroy。
func (o *Service) List(ctx context.Context) ([]contracts.SessionSummary, error) {
	if ctx.Err() != nil {
		return nil, publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	result := make([]contracts.SessionSummary, 0, len(o.sessions))
	for _, s := range o.sessions {
		result = append(result, contracts.SessionSummary{
			SessionID:       s.snapshot.SessionID,
			State:           s.snapshot.State,
			Workspace:       s.snapshot.Workspace,
			SourceWorkspace: s.snapshot.SourceWorkspace,
			MountMode:       s.snapshot.MountMode,
			MCPIDs:          append([]string(nil), s.snapshot.MCPIDs...),
			AgentID:         s.snapshot.AgentID,
			CleanupComplete: s.snapshot.CleanupComplete,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SessionID < result[j].SessionID })
	return result, nil
}

// Destroy 仅移除已完成清理的终态记录；运行中的会话必须先 Stop。
func (o *Service) Destroy(ctx context.Context, id string) (contracts.SessionSnapshot, error) {
	if ctx.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.sessions[id]
	if !ok {
		return contracts.SessionSnapshot{}, publicError("SESSION_NOT_FOUND")
	}
	if !s.snapshot.CleanupComplete || (s.snapshot.State != contracts.StateStopped && s.snapshot.State != contracts.StateFailed) {
		return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
	}
	result := snapshot(s)
	if s.retainedClone != "" {
		_ = os.RemoveAll(s.retainedClone)
	}
	delete(o.sessions, id)
	return result, nil
}

// SyncBack 将安全副本合并写回用户原工作区（覆盖同名文件，不删原目录多余文件）。
func (o *Service) SyncBack(ctx context.Context, id string) (contracts.SessionSnapshot, error) {
	if ctx.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	o.mu.Lock()
	s, ok := o.sessions[id]
	if !ok {
		o.mu.Unlock()
		return contracts.SessionSnapshot{}, publicError("SESSION_NOT_FOUND")
	}
	if s.spec.MountMode != "clone" {
		o.mu.Unlock()
		return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
	}
	source := s.snapshot.SourceWorkspace
	if source == "" {
		source = s.spec.Workspace
	}
	clonePath := s.retainedClone
	if clonePath == "" {
		clonePath = s.handle.Workspace
	}
	if clonePath == "" {
		clonePath = s.snapshot.Workspace
	}
	if source == "" || clonePath == "" || sameWorkspace(source, clonePath) {
		o.mu.Unlock()
		return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
	}
	o.mu.Unlock()
	if err := runtime.SyncWorkspace(ctx, clonePath, source); err != nil {
		return contracts.SessionSnapshot{}, safeError(err, "WORKSPACE_DENIED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok = o.sessions[id]
	if !ok {
		return contracts.SessionSnapshot{}, publicError("SESSION_NOT_FOUND")
	}
	return snapshot(s), nil
}

// WritePty 将后续轮次写入同一会话的交互 stdin，并由 Runtime 输出事件回传。
func (o *Service) WritePty(ctx context.Context, input contracts.PtyInput) error {
	if ctx.Err() != nil {
		return publicError("CANCELLED")
	}
	if input.SessionID == "" || !utf8.ValidString(input.Data) || len(input.Data) == 0 || len(input.Data) > 32768 || strings.ContainsRune(input.Data, '\x00') {
		return publicError("INVALID_ARGUMENT")
	}
	o.mu.Lock()
	s, ok := o.sessions[input.SessionID]
	if !ok {
		o.mu.Unlock()
		return publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.State != contracts.StateRunning || s.process == nil {
		o.mu.Unlock()
		return publicError("INVALID_ARGUMENT")
	}
	if !s.interactive {
		o.mu.Unlock()
		return publicError("UNIMPLEMENTED")
	}
	process := s.process
	o.mu.Unlock()
	interactive, ok := process.(runtime.InteractiveProcess)
	if !ok {
		return publicError("UNIMPLEMENTED")
	}
	if err := interactive.Write(ctx, []byte(input.Data)); err != nil {
		// 写入中断可能留下半条协议消息，必须清理会话，不能继续接收下一轮。
		var app *contracts.AppError
		if !errors.As(err, &app) || app.Code != "INVALID_ARGUMENT" {
			o.fail(s, err, "INTERNAL_ERROR")
		}
		return safeError(err, "INTERNAL_ERROR")
	}
	return nil
}

func (o *Service) ResizePty(ctx context.Context, input contracts.PtyInput) error {
	if ctx.Err() != nil {
		return publicError("CANCELLED")
	}
	if input.SessionID == "" {
		return publicError("INVALID_ARGUMENT")
	}
	o.mu.Lock()
	s, ok := o.sessions[input.SessionID]
	if !ok {
		o.mu.Unlock()
		return publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.State != contracts.StateRunning || s.process == nil || !s.interactive {
		o.mu.Unlock()
		return publicError("INVALID_ARGUMENT")
	}
	process := s.process
	o.mu.Unlock()
	interactive, ok := process.(runtime.InteractiveProcess)
	if !ok {
		return publicError("UNIMPLEMENTED")
	}
	if err := interactive.Resize(input.Cols, input.Rows); err != nil {
		return safeError(err, "UNIMPLEMENTED")
	}
	return nil
}

func (o *Service) emitLocked(s *session, e contracts.Event) {
	s.snapshot.Seq++
	e.SessionID, e.Seq = s.snapshot.SessionID, s.snapshot.Seq
	if e.State != nil {
		v := snapshot(s)
		e.State = &v
	}
	o.events.push(e)
}

func (o *Service) stateLocked(s *session, state string) {
	s.snapshot.State = state
	o.emitLocked(s, contracts.Event{Type: "state_changed", State: &contracts.SessionSnapshot{}})
}

func (o *Service) fail(s *session, err error, fallback string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	// 正常 Stop 引发的取消不变成任务故障。
	if s.ctx.Err() == nil && s.failure == nil {
		s.failure = safeError(err, fallback)
	}
	s.cancel()
}

func (o *Service) timeout(s *session) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.snapshot.State != contracts.StateProvisioning {
		return
	}
	o.timeoutLocked(s)
}

func (o *Service) sessionTimeout(s *session) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.timeoutLocked(s)
}

func (o *Service) timeoutLocked(s *session) {
	if s.ctx.Err() == nil {
		s.failure = publicError("UPSTREAM_TIMEOUT")
		s.cancel()
	}
}

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
	binding, err := o.deps.Gate.Start(ctx, gate.GateSpec{SessionID: s.snapshot.SessionID, Policy: p, ModelAccountID: model.ID, ExtraHosts: extraHosts, RequestPermission: func(call context.Context, request contracts.PermissionNeeded) (string, error) {
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
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	launch, err := kit.Build(ctx, kits.KitInput{
		Task: s.spec.Task, PrivateHome: handle.PrivateHome, ModelBaseURL: binding.ModelBaseURL,
		MCPURLs: bindingMCPURLs(binding), Sentinel: binding.Sentinel, Model: strings.TrimSpace(model.Model),
		Workspace: handle.Workspace,
	})
	if err != nil {
		return safeError(err, "AGENT_UNSUPPORTED")
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

func (o *Service) audit(s *session, data contracts.AuditData) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.snapshot.State != contracts.StateProvisioning && s.snapshot.State != contracts.StateRunning {
		return
	}
	if s.filters == nil {
		return
	}
	// Gate 必须提供脱敏主机名；拒绝整条 URL 或异常结果。
	if strings.ContainsAny(data.Target, "/?@#\r\n") || (data.Result != "allowed" && data.Result != "denied" && data.Result != "failed") {
		return
	}
	for _, filter := range s.filters {
		if strings.Contains(data.Target, filter.token) || strings.Contains(data.Tool, filter.token) {
			return
		}
		break
	}
	o.emitLocked(s, contracts.Event{Type: "audit", Audit: &data})
}

func (o *Service) output(s *session, data contracts.OutputData) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.snapshot.State == contracts.StateStopped || s.snapshot.State == contracts.StateFailed {
		return
	}
	filter := s.filters[data.Stream]
	if filter == nil {
		return
	}
	text := filter.write(data.Text)
	chunks(text, func(text string) {
		data := contracts.OutputData{Stream: data.Stream, Text: text}
		if s.outputReady {
			o.outputLocked(s, data)
			return
		}
		s.early = append(s.early, data)
		s.earlyBytes += len(text)
		for s.earlyBytes > outputLimit {
			s.earlyBytes -= len(s.early[0].Text)
			s.early[0] = contracts.OutputData{}
			s.early = s.early[1:]
			s.earlyTruncated = true
		}
	})
}

func (o *Service) outputLocked(s *session, data contracts.OutputData) {
	if s.interactive {
		o.emitLocked(s, contracts.Event{Type: contracts.EventTypePty, Pty: &data})
		return
	}
	o.emitLocked(s, contracts.Event{Type: contracts.EventTypeOutput, Output: &data})
}

func (o *Service) drainEarlyLocked(s *session) {
	if s.earlyTruncated {
		o.outputLocked(s, contracts.OutputData{Stream: "stderr", Text: "[输出已截断]\n"})
	}
	for _, data := range s.early {
		o.outputLocked(s, data)
	}
	s.early, s.earlyBytes, s.earlyTruncated = nil, 0, false
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

func bindingMCPURLs(b gate.GateBinding) []string {
	if len(b.MCPURLs) > 0 {
		return append([]string{}, b.MCPURLs...)
	}
	if b.MCPURL != "" {
		return []string{b.MCPURL}
	}
	return []string{}
}
