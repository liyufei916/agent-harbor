package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

const sentinel = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// 替身只存在于测试包，不启动宿主机任务，也不进入生产装配。
type fixture struct {
	mu             sync.Mutex
	calls          []string
	failAt         string
	destroyFails   bool
	revokeFails    bool
	blockAt        string
	entered        chan struct{}
	release        chan struct{}
	exit           chan runtime.Exit
	onOutput       func(contracts.OutputData)
	onFatal        func(error)
	onAudit        func(contracts.AuditData)
	beforeExec     func(func(contracts.OutputData))
	interactive    bool
	switchedModel  string
	events         []contracts.Event
	cloneWorkspace string // 非空时 clone 会话挂载此真实目录
}

func newFixture() *fixture {
	return &fixture{entered: make(chan struct{}), release: make(chan struct{}), exit: make(chan runtime.Exit, 1)}
}

func (f *fixture) step(name string) error {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	fail, block := f.failAt == name, f.blockAt == name
	f.mu.Unlock()
	if block {
		close(f.entered)
		<-f.release
	}
	if fail {
		return errors.New("原始内部错误：不能公开的测试凭据")
	}
	return nil
}

func (f *fixture) Health(context.Context) error { return f.step("health") }
func (f *fixture) Create(_ context.Context, input runtime.RuntimeSpec) (runtime.Handle, error) {
	if err := f.step("create"); err != nil {
		return runtime.Handle{}, err
	}
	workspace := input.Workspace
	if input.MountMode == "clone" {
		if f.cloneWorkspace != "" {
			workspace = f.cloneWorkspace
		} else {
			workspace = "/private/test-copy"
		}
	}
	return runtime.Handle{ID: "runtime", PrivateHome: "/private/test-home", Workspace: workspace}, nil
}
func (f *fixture) Exec(_ context.Context, _ runtime.Handle, _ runtime.LaunchSpec, output func(contracts.OutputData)) (runtime.Process, error) {
	if err := f.step("exec"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.onOutput = output
	before := f.beforeExec
	f.mu.Unlock()
	if before != nil {
		before(output)
	}
	return &fakeProcess{fixture: f, done: make(chan struct{})}, nil
}
func (f *fixture) Destroy(context.Context, runtime.Handle) error {
	_ = f.step("destroy")
	f.mu.Lock()
	fail := f.destroyFails
	f.mu.Unlock()
	if fail {
		return errors.New("测试清理失败")
	}
	select {
	case f.exit <- runtime.Exit{Cancelled: true}:
	default:
	}
	return nil
}

type fakeProcess struct {
	fixture *fixture
	once    sync.Once
	done    chan struct{}
	result  runtime.Exit
}

func (p *fakeProcess) Wait() runtime.Exit {
	p.once.Do(func() { p.result = <-p.fixture.exit; close(p.done) })
	<-p.done
	return p.result
}

func (p *fakeProcess) Write(_ context.Context, data []byte) error {
	p.fixture.mu.Lock()
	output := p.fixture.onOutput
	p.fixture.mu.Unlock()
	if output != nil {
		output(contracts.OutputData{Stream: "stdout", Text: string(data)})
	}
	return nil
}

func (p *fakeProcess) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return errors.New("invalid size")
	}
	return nil
}

func (f *fixture) Detect(context.Context) (kits.KitInfo, error) {
	if err := f.step("detect"); err != nil {
		return kits.KitInfo{}, err
	}
	return kits.KitInfo{Agent: contracts.AgentInfo{ID: "claude", Available: true}, ReadOnlyPaths: []string{"/test/claude"}, NeedsDescendants: true}, nil
}
func (f *fixture) Build(context.Context, kits.KitInput) (runtime.LaunchSpec, error) {
	return runtime.LaunchSpec{Command: "/test/claude", Interactive: f.interactive}, f.step("build")
}

type fakePolicy struct{ f *fixture }

func (p fakePolicy) Build(_ context.Context, req policy.PolicyRequest) (policy.PolicySnapshot, error) {
	if err := p.f.step("policy"); err != nil {
		return policy.PolicySnapshot{}, err
	}
	mode := req.MountMode
	if mode == "" {
		mode = "direct"
	}
	mounts := []policy.MCPMount{}
	for _, mcp := range req.MCPs {
		mounts = append(mounts, policy.MCPMount{AccountID: mcp.ID})
	}
	return policy.PolicySnapshot{Workspace: req.Workspace, MountMode: mode, MCPMounts: mounts}, nil
}
func (f *fixture) Lookup(_ context.Context, id string) (contracts.AccountInfo, error) {
	if err := f.step("lookup-" + id); err != nil {
		return contracts.AccountInfo{}, err
	}
	switch id {
	case "model", "model-next":
		return contracts.AccountInfo{ID: id, Kind: "model"}, nil
	case "mcp", "mcp-next":
		return contracts.AccountInfo{ID: id, Kind: "mcp"}, nil
	default:
		return contracts.AccountInfo{}, publicError("ACCOUNT_NOT_FOUND")
	}
}

func (f *fixture) SwitchModel(_ context.Context, _ string, account string) error {
	if err := f.step("switch-model"); err != nil {
		return err
	}
	f.mu.Lock()
	f.switchedModel = account
	f.mu.Unlock()
	return nil
}
func (f *fixture) Start(_ context.Context, spec gate.GateSpec, audit func(contracts.AuditData), fatal func(error)) (gate.GateBinding, error) {
	if err := f.step("start"); err != nil {
		return gate.GateBinding{}, err
	}
	f.mu.Lock()
	f.onFatal = fatal
	f.onAudit = audit
	f.mu.Unlock()
	binding := gate.GateBinding{Sentinel: sentinel, ModelBaseURL: "http://127.0.0.1:41001/model", AllowedTCP: []string{"127.0.0.1:41001"}}
	if len(spec.Policy.MCPMounts) > 0 {
		binding.AllowedTCP = append(binding.AllowedTCP, "127.0.0.1:41002")
	}
	for i := range spec.Policy.MCPMounts {
		binding.MCPURLs = append(binding.MCPURLs, "http://127.0.0.1:41002/mcp/"+strconv.Itoa(i))
	}
	return binding, nil
}
func (f *fixture) Revoke(context.Context, string) error {
	_ = f.step("revoke")
	f.mu.Lock()
	fail := f.revokeFails
	f.mu.Unlock()
	if fail {
		return errors.New("测试撤销失败")
	}
	return nil
}
func (f *fixture) record(e contracts.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func setup(t *testing.T, f *fixture, timeouts Timeouts, sink func(contracts.Event)) *Service {
	t.Helper()
	if sink == nil {
		sink = f.record
	}
	o, err := New(context.Background(), Dependencies{Runtime: f, Kits: map[string]kits.Kit{"claude": f}, Catalog: f, Policy: fakePolicy{f}, Gate: f}, sink, timeouts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.mu.Lock()
		f.destroyFails, f.revokeFails = false, false
		f.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := o.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return o
}

func spec() contracts.SessionSpec {
	return contracts.SessionSpec{Workspace: "/test/workspace", AgentID: "claude", ModelAccountID: "model", MCPIDs: []string{"mcp"}, Task: "测试任务"}
}
func create(t *testing.T, o *Service) contracts.SessionSnapshot {
	t.Helper()
	s, err := o.Create(context.Background(), spec())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func await(t *testing.T, o *Service, id string, predicate func(contracts.SessionSnapshot) bool) contracts.SessionSnapshot {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		s, err := o.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if predicate(s) {
			return s
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("等待状态超时：%+v", s)
		}
	}
}
func awaitState(t *testing.T, o *Service, id, state string) contracts.SessionSnapshot {
	return await(t, o, id, func(s contracts.SessionSnapshot) bool { return s.State == state })
}
func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var app *contracts.AppError
	if !errors.As(err, &app) || app.Code != code {
		t.Fatalf("错误=%v，预期=%s", err, code)
	}
}

func TestLifecycleAndEvents(t *testing.T) {
	f := newFixture()
	f.beforeExec = func(output func(contracts.OutputData)) {
		output(contracts.OutputData{Stream: "stdout", Text: "早到输出\n"})
	}
	o := setup(t, f, Timeouts{}, nil)
	request, cancel := context.WithCancel(context.Background())
	s, err := o.Create(request, spec())
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if s.State != contracts.StateProvisioning || s.Seq != 1 {
		t.Fatalf("初始快照：%+v", s)
	}
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	f.exit <- runtime.Exit{Code: 0}
	end := awaitState(t, o, s.SessionID, contracts.StateStopped)
	if !end.CleanupComplete || end.Error != nil {
		t.Fatalf("终态：%+v", end)
	}
	repeat, err := o.Stop(context.Background(), s.SessionID)
	if err != nil || repeat.Seq != end.Seq {
		t.Fatalf("终态停止不幂等：%+v %v", repeat, err)
	}
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := o.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) < 5 {
		t.Fatalf("事件缺失：%+v", f.events)
	}
	running, output, terminal := uint64(0), uint64(0), uint64(0)
	for i, e := range f.events {
		if e.Seq != uint64(i+1) {
			t.Fatalf("水位不连续：%+v", e)
		}
		payloads := 0
		if e.State != nil {
			payloads++
			if e.State.Seq != e.Seq {
				t.Fatal("状态水位不匹配")
			}
			if e.State.State == contracts.StateRunning {
				running = e.Seq
			}
			if e.State.CleanupComplete {
				terminal = e.Seq
			}
		}
		if e.Output != nil {
			payloads++
			output = e.Seq
		}
		if e.Audit != nil {
			payloads++
		}
		if e.Error != nil {
			payloads++
		}
		if payloads != 1 {
			t.Fatalf("事件载荷数量错误：%+v", e)
		}
	}
	if !(running < output && output < terminal) {
		t.Fatalf("事件顺序错误：%d %d %d", running, output, terminal)
	}
	joined := strings.Join(f.calls, ",")
	if !strings.Contains(joined, "revoke,destroy") {
		t.Fatalf("清理顺序错误：%s", joined)
	}
}

func TestListAndDestroyRetainOnlyExplicitlyDestroyedRecords(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	s := create(t, o)
	if _, err := o.Destroy(context.Background(), s.SessionID); err == nil {
		t.Fatal("运行中的会话不应允许 Destroy")
	} else {
		assertCode(t, err, "INVALID_ARGUMENT")
	}
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	f.exit <- runtime.Exit{Code: 0}
	awaitState(t, o, s.SessionID, contracts.StateStopped)
	items, err := o.List(context.Background())
	if err != nil || len(items) != 1 || items[0].SessionID != s.SessionID || !items[0].CleanupComplete {
		t.Fatalf("终态记录未保留：%+v %v", items, err)
	}
	if _, err := o.Destroy(context.Background(), s.SessionID); err != nil {
		t.Fatal(err)
	}
	items, err = o.List(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("Destroy 后仍有会话：%+v %v", items, err)
	}
}

func TestInteractiveSessionAcceptsAdditionalRounds(t *testing.T) {
	f := newFixture()
	f.interactive = true
	o := setup(t, f, Timeouts{}, nil)
	s := create(t, o)
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	if err := o.WritePty(context.Background(), contracts.PtyInput{SessionID: s.SessionID, Data: "第二轮\n"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(time.Second)
	for {
		f.mu.Lock()
		found := false
		for _, event := range f.events {
			if event.Type == contracts.EventTypePty && event.Pty != nil && strings.Contains(event.Pty.Text, "第二轮") {
				found = true
				break
			}
		}
		f.mu.Unlock()
		if found {
			break
		}
		select {
		case <-deadline:
			t.Fatal("未收到第二轮 pty 事件")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	_, _ = o.Stop(context.Background(), s.SessionID)
	await(t, o, s.SessionID, func(s contracts.SessionSnapshot) bool { return s.CleanupComplete })
}

func TestNonInteractiveSessionRejectsAdditionalRound(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	s := create(t, o)
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	err := o.WritePty(context.Background(), contracts.PtyInput{SessionID: s.SessionID, Data: "下一轮"})
	assertCode(t, err, "UNIMPLEMENTED")
	current, err := o.Get(context.Background(), s.SessionID)
	if err != nil || current.State != contracts.StateRunning {
		t.Fatalf("能力拒绝不应停止任务：%+v %v", current, err)
	}
}

func TestAllowsTwoDifferentWorkspacesAndCapsThird(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	second := spec()
	second.Workspace = "/test/other-workspace"
	third := spec()
	third.Workspace = "/test/third-workspace"
	one := create(t, o)
	two, err := o.Create(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Create(context.Background(), third); err == nil {
		t.Fatal("第三个并发会话不应获准")
	} else {
		assertCode(t, err, "SESSION_LIMIT")
	}
	_, _ = o.Stop(context.Background(), one.SessionID)
	_, _ = o.Stop(context.Background(), two.SessionID)
	await(t, o, one.SessionID, func(s contracts.SessionSnapshot) bool { return s.CleanupComplete })
	await(t, o, two.SessionID, func(s contracts.SessionSnapshot) bool { return s.CleanupComplete })
}

func TestCloneSnapshotKeepsOriginalWorkspaceLocked(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	input := spec()
	input.MountMode = "clone"
	s, err := o.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	running := awaitState(t, o, s.SessionID, contracts.StateRunning)
	if running.Workspace != "/private/test-copy" {
		t.Fatalf("未使用副本路径：%+v", running)
	}
	if running.SourceWorkspace == "" || running.SourceWorkspace == running.Workspace {
		t.Fatalf("应保留原工作区：%+v", running)
	}
	_, err = o.Create(context.Background(), input)
	assertCode(t, err, "WORKSPACE_BUSY")
	_, _ = o.Stop(context.Background(), s.SessionID)
	await(t, o, s.SessionID, func(s contracts.SessionSnapshot) bool { return s.CleanupComplete })
}

func TestSyncBackMergesCloneIntoOriginal(t *testing.T) {
	orig := t.TempDir()
	clone := t.TempDir()
	if err := os.WriteFile(filepath.Join(orig, "keep.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "from-clone.txt"), []byte("synced"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "keep.txt"), []byte("overwritten"), 0600); err != nil {
		t.Fatal(err)
	}
	f := newFixture()
	f.cloneWorkspace = clone
	o := setup(t, f, Timeouts{}, nil)
	input := spec()
	input.Workspace = orig
	input.MountMode = "clone"
	s, err := o.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	got, err := o.SyncBack(context.Background(), s.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !sameWorkspace(got.SourceWorkspace, orig) {
		t.Fatalf("SourceWorkspace=%q want~%q", got.SourceWorkspace, orig)
	}
	if data, err := os.ReadFile(filepath.Join(orig, "from-clone.txt")); err != nil || string(data) != "synced" {
		t.Fatalf("未导回新文件：%q %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(orig, "keep.txt")); err != nil || string(data) != "overwritten" {
		t.Fatalf("未覆盖同名文件：%q %v", data, err)
	}
	_, _ = o.Stop(context.Background(), s.SessionID)
	await(t, o, s.SessionID, func(s contracts.SessionSnapshot) bool { return s.CleanupComplete })
	// 停止后仍可导回（副本已 retain）
	if _, err := o.SyncBack(context.Background(), s.SessionID); err != nil {
		t.Fatalf("停止后导回失败：%v", err)
	}
}

func TestProvisionFailuresRollback(t *testing.T) {
	for _, tc := range []struct {
		step, code      string
		revoke, destroy bool
	}{
		{"health", "ISOLATION_UNAVAILABLE", false, false},
		{"detect", "AGENT_UNSUPPORTED", false, false},
		{"lookup-model", "ACCOUNT_NOT_FOUND", false, false},
		{"lookup-mcp", "ACCOUNT_NOT_FOUND", false, false},
		{"policy", "WORKSPACE_DENIED", false, false},
		{"start", "INTERNAL_ERROR", false, false},
		{"create", "INTERNAL_ERROR", true, false},
		{"build", "AGENT_UNSUPPORTED", true, true},
		{"exec", "INTERNAL_ERROR", true, true},
	} {
		t.Run(tc.step, func(t *testing.T) {
			f := newFixture()
			f.failAt = tc.step
			o := setup(t, f, Timeouts{}, nil)
			s := create(t, o)
			end := awaitState(t, o, s.SessionID, contracts.StateFailed)
			if !end.CleanupComplete {
				t.Fatal("回滚未释放锁")
			}
			assertCode(t, end.Error, tc.code)
			f.mu.Lock()
			calls := strings.Join(f.calls, ",")
			f.failAt = "health"
			f.mu.Unlock()
			if strings.Contains(calls, "revoke") != tc.revoke || strings.Contains(calls, "destroy") != tc.destroy {
				t.Fatalf("回滚资源错误：%s", calls)
			}
			create(t, o)
		})
	}
}

func TestStopTakesOwnershipOfLateResources(t *testing.T) {
	for _, step := range []string{"start", "create", "exec"} {
		t.Run(step, func(t *testing.T) {
			f := newFixture()
			f.blockAt = step
			o := setup(t, f, Timeouts{}, nil)
			s := create(t, o)
			select {
			case <-f.entered:
			case <-time.After(time.Second):
				t.Fatal("未进入阻塞点")
			}
			stopping, err := o.Stop(context.Background(), s.SessionID)
			if err != nil || stopping.State != contracts.StateStopping {
				t.Fatalf("Stop 未快速返回：%+v %v", stopping, err)
			}
			_, err = o.Create(context.Background(), spec())
			assertCode(t, err, "WORKSPACE_BUSY")
			close(f.release)
			end := awaitState(t, o, s.SessionID, contracts.StateStopped)
			if !end.CleanupComplete {
				t.Fatal("晚到资源未清理")
			}
			f.mu.Lock()
			calls := strings.Join(f.calls, ",")
			f.mu.Unlock()
			if !strings.Contains(calls, "revoke") || (step != "start" && !strings.Contains(calls, "destroy")) {
				t.Fatalf("资源泄漏：%s", calls)
			}
			if step == "start" && strings.Contains(calls, "create") {
				t.Fatal("停止后仍创建 Runtime")
			}
			if step == "create" && strings.Contains(calls, "exec") {
				t.Fatal("停止后仍启动 Agent")
			}
		})
	}
}

func TestCleanupFailureRetainsLockAndCanRetry(t *testing.T) {
	for _, resource := range []string{"gate", "runtime"} {
		t.Run(resource, func(t *testing.T) {
			f := newFixture()
			f.destroyFails = resource == "runtime"
			f.revokeFails = resource == "gate"
			o := setup(t, f, Timeouts{Cleanup: 30 * time.Millisecond}, nil)
			s := create(t, o)
			awaitState(t, o, s.SessionID, contracts.StateRunning)
			_, _ = o.Stop(context.Background(), s.SessionID)
			end := awaitState(t, o, s.SessionID, contracts.StateFailed)
			if end.CleanupComplete {
				t.Fatal("清理失败却释放锁")
			}
			assertCode(t, end.Error, "CLEANUP_FAILED")
			_, err := o.Create(context.Background(), spec())
			assertCode(t, err, "WORKSPACE_BUSY")
			f.mu.Lock()
			f.destroyFails, f.revokeFails = false, false
			f.mu.Unlock()
			_, err = o.Stop(context.Background(), s.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			awaitState(t, o, s.SessionID, contracts.StateStopped)
			create(t, o)
		})
	}
}

func TestNonzeroExitAndFatalGate(t *testing.T) {
	for _, fatal := range []bool{false, true} {
		t.Run(map[bool]string{false: "exit", true: "gate"}[fatal], func(t *testing.T) {
			f := newFixture()
			o := setup(t, f, Timeouts{}, nil)
			s := create(t, o)
			awaitState(t, o, s.SessionID, contracts.StateRunning)
			code := "AGENT_EXIT_FAILED"
			if fatal {
				f.mu.Lock()
				cb := f.onFatal
				f.mu.Unlock()
				cb(errors.New("不可公开"))
				code = "INTERNAL_ERROR"
			} else {
				f.exit <- runtime.Exit{Code: 7, Err: errors.New("不可公开")}
			}
			end := awaitState(t, o, s.SessionID, contracts.StateFailed)
			if !end.CleanupComplete {
				t.Fatal("故障资源未清理")
			}
			assertCode(t, end.Error, code)
		})
	}
}

func TestValidationAndSingleSessionLock(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	for _, change := range []func(*contracts.SessionSpec){
		func(s *contracts.SessionSpec) { s.Task = "" }, func(s *contracts.SessionSpec) { s.Task = strings.Repeat("x", 32769) },
		func(s *contracts.SessionSpec) { s.Task = string([]byte{0xff}) }, func(s *contracts.SessionSpec) { s.Workspace = "" },
		func(s *contracts.SessionSpec) { s.MCPIDs = []string{""} }, func(s *contracts.SessionSpec) { s.MCPIDs = []string{"a", "a"} },
	} {
		input := spec()
		change(&input)
		_, err := o.Create(context.Background(), input)
		assertCode(t, err, "INVALID_ARGUMENT")
	}
	input := spec()
	input.AgentID = "codex"
	_, err := o.Create(context.Background(), input)
	assertCode(t, err, "AGENT_UNSUPPORTED")
	_, err = o.Get(context.Background(), "unknown")
	assertCode(t, err, "SESSION_NOT_FOUND")
	_, err = o.Stop(context.Background(), "unknown")
	assertCode(t, err, "SESSION_NOT_FOUND")
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := o.Create(context.Background(), spec()); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			assertCode(t, err, "WORKSPACE_BUSY")
		}
	}
	if success != 1 {
		t.Fatalf("并发创建成功数=%d", success)
	}
}

func TestTimeouts(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{Session: 15 * time.Millisecond}, nil)
	s := create(t, o)
	end := awaitState(t, o, s.SessionID, contracts.StateFailed)
	assertCode(t, end.Error, "UPSTREAM_TIMEOUT")
	if !end.CleanupComplete {
		t.Fatal("超时未清理")
	}
	f2 := newFixture()
	f2.blockAt = "start"
	o2 := setup(t, f2, Timeouts{Provision: 15 * time.Millisecond}, nil)
	s2 := create(t, o2)
	<-f2.entered
	awaitState(t, o2, s2.SessionID, contracts.StateStopping)
	close(f2.release)
	end = awaitState(t, o2, s2.SessionID, contracts.StateFailed)
	assertCode(t, end.Error, "UPSTREAM_TIMEOUT")
}

func TestAccountKindsAreNotInterchangeable(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	input := spec()
	input.ModelAccountID, input.MCPIDs = "mcp", []string{"model"}
	s, err := o.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	end := awaitState(t, o, s.SessionID, contracts.StateFailed)
	assertCode(t, end.Error, "INVALID_ARGUMENT")
	if !end.CleanupComplete {
		t.Fatal("错误账号引用未释放锁")
	}
}

func TestApplicationCancellationCleansSession(t *testing.T) {
	f := newFixture()
	app, cancel := context.WithCancel(context.Background())
	defer cancel()
	o, err := New(app, Dependencies{Runtime: f, Kits: map[string]kits.Kit{"claude": f}, Catalog: f, Policy: fakePolicy{f}, Gate: f}, f.record, Timeouts{})
	if err != nil {
		t.Fatal(err)
	}
	s := create(t, o)
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	cancel()
	end := awaitState(t, o, s.SessionID, contracts.StateStopped)
	if !end.CleanupComplete {
		t.Fatal("应用取消未清理")
	}
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := o.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = o.Create(context.Background(), spec())
	assertCode(t, err, "CANCELLED")
}

func TestAuditsAreSanitizedAndDeniedDoesNotStop(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	s := create(t, o)
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	f.mu.Lock()
	callback := f.onAudit
	f.mu.Unlock()
	callback(contracts.AuditData{Target: "example.com", Tool: "read_item", Result: "denied"})
	callback(contracts.AuditData{Target: "https://example.com/?secret=value", Result: "allowed"})
	callback(contracts.AuditData{Target: "example.com", Tool: sentinel, Result: "allowed"})
	callback(contracts.AuditData{Target: "example.com", Result: "unknown"})
	current, _ := o.Get(context.Background(), s.SessionID)
	if current.State != contracts.StateRunning {
		t.Fatal("普通拒绝不应终止会话")
	}
	_, _ = o.Stop(context.Background(), s.SessionID)
	awaitState(t, o, s.SessionID, contracts.StateStopped)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := o.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, e := range f.events {
		if e.Audit != nil {
			count++
			if e.Audit.Result != "denied" {
				t.Fatal("不安全审计通过边界")
			}
		}
	}
	if count != 1 {
		t.Fatalf("审计数=%d", count)
	}
}

func TestGetSnapshotCannotChangeStoredError(t *testing.T) {
	f := newFixture()
	f.failAt = "health"
	o := setup(t, f, Timeouts{}, nil)
	s := create(t, o)
	end := awaitState(t, o, s.SessionID, contracts.StateFailed)
	end.Error.Code = "changed"
	again, _ := o.Get(context.Background(), s.SessionID)
	assertCode(t, again.Error, "ISOLATION_UNAVAILABLE")
}

func TestExpiredProvisionTimerCannotCancelRunningSession(t *testing.T) {
	f := newFixture()
	o := setup(t, f, Timeouts{}, nil)
	initial := create(t, o)
	awaitState(t, o, initial.SessionID, contracts.StateRunning)
	o.mu.Lock()
	s := o.sessions[initial.SessionID]
	o.mu.Unlock()
	// 模拟计时器已触发、等待锁的回调晚于 running 转移才得到执行。
	o.timeout(s)
	if s.ctx.Err() != nil {
		t.Fatal("晚到的准备计时器取消了已运行任务")
	}
}

func TestSessionDeadlineIncludesProvisioning(t *testing.T) {
	f := newFixture()
	f.blockAt = "start"
	o := setup(t, f, Timeouts{Provision: time.Second, Session: 20 * time.Millisecond}, nil)
	initial := create(t, o)
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		t.Fatal("未进入准备阻塞点")
	}
	defer close(f.release)
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		s, _ := o.Get(context.Background(), initial.SessionID)
		if s.State == contracts.StateStopping {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatal("会话总时限未覆盖准备阶段")
		}
	}
}

func TestCreateOptionalAndMultipleMCP(t *testing.T) {
	for _, ids := range [][]string{nil, {"mcp", "mcp-next"}} {
		f := newFixture()
		o := setup(t, f, Timeouts{}, nil)
		input := spec()
		input.MCPIDs = ids
		snap, err := o.Create(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		awaitState(t, o, snap.SessionID, contracts.StateRunning)
		_, err = o.Stop(context.Background(), snap.SessionID)
		if err != nil {
			t.Fatal(err)
		}
		awaitState(t, o, snap.SessionID, contracts.StateStopped)
	}
}
func TestCreateRejectsDuplicateAndTooManyMCP(t *testing.T) {
	o := setup(t, newFixture(), Timeouts{}, nil)
	for _, ids := range [][]string{{"mcp", "mcp"}, {""}, {"mcp", "mcp-next", "mcp", "mcp", "mcp", "mcp", "mcp", "mcp", "mcp"}} {
		input := spec()
		input.MCPIDs = ids
		_, err := o.Create(context.Background(), input)
		assertCode(t, err, "INVALID_ARGUMENT")
	}
}
