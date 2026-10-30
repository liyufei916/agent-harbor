package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/kits"
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
	snapshot        contracts.SessionSnapshot
	spec            contracts.SessionSpec
	ctx             context.Context
	cancel          context.CancelFunc
	failure         *contracts.AppError
	handle          runtime.Handle
	retainedClone   string // 停止后保留的副本路径；Destroy 时删除
	resumeFrom      string // 再次开启前挪走的副本，成功盖回新隔离目录后删除
	skipInitialTask bool   // 再次开启：进程起来后等待用户输入，不再把创建任务写入 stdin
	gateStarted     bool
	wait            <-chan runtime.Exit
	process         runtime.Process
	interactive     bool
	boundary        contracts.PermissionOverview
	permissions     map[string]*permissionRequest
	cleanupDone     chan struct{}
	finished        chan struct{}
	retry           chan struct{}
	early           []contracts.OutputData
	earlyBytes      int
	earlyTruncated  bool
	started         time.Time
	outputReady     bool
	filters         map[string]*streamFilter
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

// Start 在已清理的原会话上重新拉起 Agent，不分配新的 session_id。
func (o *Service) Start(ctx context.Context, id string) (contracts.SessionSnapshot, error) {
	if ctx.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing || o.app.Err() != nil {
		return contracts.SessionSnapshot{}, publicError("CANCELLED")
	}
	if o.connectionDone != nil {
		return contracts.SessionSnapshot{}, publicError("SESSION_BUSY")
	}
	s, ok := o.sessions[id]
	if !ok {
		return contracts.SessionSnapshot{}, publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.State != contracts.StateStopped && s.snapshot.State != contracts.StateFailed {
		return contracts.SessionSnapshot{}, publicError("INVALID_ARGUMENT")
	}
	if !s.snapshot.CleanupComplete {
		return contracts.SessionSnapshot{}, publicError("CLEANUP_FAILED")
	}
	activeCount := 0
	for _, existing := range o.sessions {
		if existing == s || existing.snapshot.CleanupComplete {
			continue
		}
		activeCount++
		if existing.snapshot.Workspace != "" && sameWorkspace(existing.spec.Workspace, s.spec.Workspace) {
			return contracts.SessionSnapshot{}, publicError("WORKSPACE_BUSY")
		}
	}
	if activeCount >= 2 {
		return contracts.SessionSnapshot{}, publicError("SESSION_LIMIT")
	}
	if s.retainedClone != "" {
		bak := s.retainedClone + ".resume"
		_ = os.RemoveAll(bak)
		if err := os.Rename(s.retainedClone, bak); err == nil {
			s.resumeFrom = bak
		}
		s.retainedClone = ""
	}
	sessionCtx, cancel := context.WithCancel(o.app)
	s.ctx, s.cancel = sessionCtx, cancel
	s.failure = nil
	s.snapshot.Error = nil
	s.snapshot.CleanupComplete = false
	s.handle = runtime.Handle{}
	s.gateStarted = false
	s.wait = nil
	s.process = nil
	s.interactive = false
	s.permissions = nil
	s.cleanupDone = make(chan struct{})
	s.finished = make(chan struct{})
	s.retry = make(chan struct{}, 1)
	s.early, s.earlyBytes, s.earlyTruncated = nil, 0, false
	s.started = time.Now()
	s.outputReady = false
	s.filters = nil
	s.skipInitialTask = true
	if s.snapshot.SourceWorkspace != "" {
		s.snapshot.Workspace = s.snapshot.SourceWorkspace
	}
	o.stateLocked(s, contracts.StateProvisioning)
	initial := snapshot(s)
	go o.run(s)
	return initial, nil
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

