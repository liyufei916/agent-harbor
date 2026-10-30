//go:build darwin

package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// MacOS：Health 保持严格 deny process-fork；Agent 会话可通过 RuntimeSpec.AllowDescendants
// 在白名单 process-exec 内允许后代，并由杀树在 Stop/Destroy 时尽力清理。
type MacOS struct {
	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	mu                       sync.Mutex
	handle                   Handle
	root, workspace, profile string
	readOnly                 map[string]bool
	process                  *macProcess
	attempted, destroyed     bool
}

var _ Runtime = (*MacOS)(nil)

func NewMacOS() *MacOS { return &MacOS{entries: make(map[string]*entry)} }

func (r *MacOS) Health(ctx context.Context) error {
	return r.HealthFor(ctx, false)
}

// HealthFor 验证指定的 Runtime 能力。允许子进程时仍只开放固定工具路径；不启用 AllowDescendants。
func (r *MacOS) HealthFor(ctx context.Context, allowChildren bool) error {
	if ctx.Err() != nil {
		return runtimeError("CANCELLED")
	}
	// 当前只在这条 OS/CPU 主线上实测，未知环境失败关闭。
	release, err := syscall.Sysctl("kern.osrelease")
	if err != nil || !strings.HasPrefix(release, "25.") || runtime.GOARCH != "arm64" || os.Geteuid() == 0 || os.Geteuid() != os.Getuid() || os.Getegid() != os.Getgid() {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	root, err := os.MkdirTemp("/private/tmp", "agent-harbor-health-")
	if err != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	defer os.RemoveAll(root)
	allowed := filepath.Join(root, "allowed")
	if err := os.Mkdir(allowed, 0700); err != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	outside := filepath.Join(root, "denied.txt")
	if err := os.WriteFile(outside, []byte("harbor-health-fixture"), 0600); err != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	inside := filepath.Join(allowed, "allowed.txt")
	if err := os.WriteFile(inside, []byte("harbor-health-fixture"), 0600); err != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	profile, err := makeProfile(allowed, allowed, []string{"/bin/cat"}, nil, allowChildren, false)
	if err != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	if strings.Contains(profile, "(allow default)") || !strings.Contains(profile, "(deny default)") || strings.Contains(profile, "(allow process-fork)") != allowChildren || strings.Contains(profile, "(deny process-fork)") == allowChildren {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	probeCommand, probeArgs := "/usr/bin/true", []string{}
	if allowChildren {
		probeCommand, probeArgs = "/bin/sh", []string{"-c", "/usr/bin/true"}
	}
	cmd := exec.CommandContext(probeCtx, "/usr/bin/sandbox-exec", append([]string{"-p", profile, probeCommand}, probeArgs...)...)
	cmd.Dir, cmd.Env = allowed, []string{"HOME=" + allowed, "PATH=/usr/bin:/bin"}
	if cmd.Run() != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	positive := exec.CommandContext(probeCtx, "/usr/bin/sandbox-exec", "-p", profile, "/bin/cat", inside)
	positive.Dir, positive.Env = cmd.Dir, cmd.Env
	output, err := positive.Output()
	if err != nil || string(output) != "harbor-health-fixture" {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	negative := exec.CommandContext(probeCtx, "/usr/bin/sandbox-exec", "-p", profile, "/bin/cat", outside)
	negative.Dir, negative.Env = cmd.Dir, cmd.Env
	output, err = negative.Output()
	exit, ok := err.(*exec.ExitError)
	// 规则编译失败、崩溃、超时都不是权限拒绝的证据。
	if !ok || exit.ExitCode() != 1 || len(output) != 0 || probeCtx.Err() != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	return nil
}

func (r *MacOS) Create(ctx context.Context, spec RuntimeSpec) (Handle, error) {
	return r.create(ctx, spec, false)
}

func (r *MacOS) create(ctx context.Context, spec RuntimeSpec, offline bool) (Handle, error) {
	if err := r.HealthFor(ctx, spec.AllowChildProcesses); err != nil {
		return Handle{}, err
	}
	if spec.SessionID == "" || (spec.MountMode != "direct" && spec.MountMode != "clone") || (!offline && len(spec.AllowedTCP) == 0) || len(spec.ReadOnlyPaths) == 0 {
		return Handle{}, runtimeError("INVALID_ARGUMENT")
	}
	workspace, err := canonicalPath(spec.Workspace)
	if err != nil {
		return Handle{}, err
	}
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() || forbiddenWorkspace(workspace) {
		return Handle{}, runtimeError("WORKSPACE_DENIED")
	}
	readOnly := make([]string, 0, len(spec.ReadOnlyPaths))
	for _, path := range spec.ReadOnlyPaths {
		resolved, err := canonicalPath(path)
		if err != nil || within(workspace, resolved) {
			return Handle{}, runtimeError("INVALID_ARGUMENT")
		}
		info, statErr := os.Stat(resolved)
		if statErr != nil || (info.IsDir() && (!safeInstallationRoot(resolved) || within(resolved, workspace))) {
			return Handle{}, runtimeError("INVALID_ARGUMENT")
		}
		readOnly = append(readOnly, resolved)
	}
	root, err := sessionRoot(spec.MountMode, workspace)
	if err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	owned := false
	defer func() {
		if !owned {
			_ = os.RemoveAll(root)
		}
	}()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	if err := os.Mkdir(filepath.Join(home, "tmp"), 0700); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	if spec.MountMode == "clone" {
		clonePath := filepath.Join(root, "workspace")
		if err := cloneWorkspace(ctx, workspace, clonePath); err != nil {
			return Handle{}, err
		}
		workspace = clonePath
	}
	profile, err := makeProfile(workspace, home, readOnly, spec.AllowedTCP, spec.AllowChildProcesses, spec.AllowDescendants)
	if err != nil {
		return Handle{}, err
	}
	allowFork := spec.AllowChildProcesses || spec.AllowDescendants
	if allowFork {
		if !strings.Contains(profile, "(allow process-fork)") || strings.Contains(profile, "(deny process-fork)") {
			return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
		}
	} else if !strings.Contains(profile, "(deny process-fork)") || strings.Contains(profile, "(allow process-fork)") {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	profilePath := filepath.Join(root, "sandbox.sb")
	if err := os.WriteFile(profilePath, []byte(profile), 0600); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	// 每次 Create 均实际编译并应用本次参数生成的规则。
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "/usr/bin/sandbox-exec", "-f", profilePath, "/usr/bin/true")
	cmd.Dir, cmd.Env = workspace, []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	if err := cmd.Run(); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	if ctx.Err() != nil {
		return Handle{}, runtimeError("CANCELLED")
	}
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	handle := Handle{ID: hex.EncodeToString(id[:]), PrivateHome: home, Workspace: workspace}
	e := &entry{handle: handle, root: root, workspace: workspace, profile: profilePath, readOnly: make(map[string]bool)}
	for _, path := range readOnly {
		e.readOnly[path] = true
	}
	r.mu.Lock()
	r.entries[handle.ID] = e
	r.mu.Unlock()
	owned = true
	return handle, nil
}

func (r *MacOS) lookup(handle Handle) (*entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[handle.ID]
	if e == nil || e.handle.PrivateHome != handle.PrivateHome {
		return nil, runtimeError("INVALID_ARGUMENT")
	}
	return e, nil
}

func (r *MacOS) Exec(ctx context.Context, handle Handle, launch LaunchSpec, output func(contracts.OutputData)) (Process, error) {
	e, err := r.lookup(handle)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil {
		return nil, runtimeError("CANCELLED")
	}
	if e.destroyed || e.attempted {
		return nil, runtimeError("INVALID_ARGUMENT")
	}
	if launch.Terminal && (launch.Interactive || launch.EncodeInput != nil) {
		return nil, runtimeError("INVALID_ARGUMENT")
	}
	command, err := canonicalPath(launch.Command)
	if err != nil || !e.readOnly[command] {
		return nil, runtimeError("AGENT_UNSUPPORTED")
	}
	for _, arg := range launch.Args {
		if strings.ContainsRune(arg, 0) {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
	}
	projectHome := handle.PrivateHome
	if ph := strings.TrimSpace(launch.ProjectHome); ph != "" {
		if ph != e.workspace {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
		projectHome = ph
	}
	env, err := launchEnvironment(projectHome, handle.PrivateHome, launch.Env)
	if err != nil {
		return nil, err
	}
	if err := validateConfigFiles(launch.ConfigFiles); err != nil {
		return nil, err
	}
	// 单次尝试：失败后只能 Destroy，不能复用半写入的 HOME。
	e.attempted = true
	if err := writeConfigFiles(handle.PrivateHome, launch.ConfigFiles); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, runtimeError("CANCELLED")
	}
	args := append([]string{"-f", e.profile, command}, launch.Args...)
	cmd := exec.Command("/usr/bin/sandbox-exec", args...)
	cmd.Dir, cmd.Env = e.workspace, env
	if launch.Terminal {
		return r.execTerminal(ctx, e, cmd, launch, output)
	}
	var stdin *os.File
	if launch.Interactive {
		var pipe io.WriteCloser
		pipe, err = cmd.StdinPipe()
		if err == nil {
			stdin = pipe.(*os.File)
		}
		if err != nil {
			return nil, runtimeError("ISOLATION_UNAVAILABLE")
		}
	} else {
		cmd.Stdin = bytes.NewReader(launch.Stdin)
	}
	started := false
	defer func() {
		if !started && stdin != nil {
			_ = stdin.Close()
		}
	}()
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, runtimeError("ISOLATION_UNAVAILABLE")
	}
	stderr, stderrWriter, err := os.Pipe()
	if err != nil {
		stdout.Close()
		stdoutWriter.Close()
		return nil, runtimeError("ISOLATION_UNAVAILABLE")
	}
	cmd.Stdout, cmd.Stderr = stdoutWriter, stderrWriter
	if err := cmd.Start(); err != nil {
		stdout.Close()
		stdoutWriter.Close()
		stderr.Close()
		stderrWriter.Close()
		return nil, runtimeError("ISOLATION_UNAVAILABLE")
	}
	started = true
	stdoutWriter.Close()
	stderrWriter.Close()
	p := &macProcess{cmd: cmd, done: make(chan struct{}), rootDone: make(chan struct{})}
	if stdin != nil {
		p.input = newProcessInput(stdin, launch.EncodeInput)
	}
	e.process = p
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); defer stdout.Close(); readOutput(stdout, "stdout", output) }()
	go func() { defer readers.Done(); defer stderr.Close(); readOutput(stderr, "stderr", output) }()
	go func() {
		err := cmd.Wait()
		close(p.rootDone)
		readers.Wait()
		p.mu.Lock()
		p.result = Exit{Code: -1, Cancelled: p.cancelled}
		if cmd.ProcessState != nil {
			p.confirmed = true
			p.result.Code = cmd.ProcessState.ExitCode()
		}
		if err != nil {
			p.result.Err = runtimeError("AGENT_EXIT_FAILED")
		}
		p.mu.Unlock()
		close(p.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			p.kill(true)
		case <-p.rootDone:
		}
	}()
	if launch.Interactive && len(launch.Stdin) > 0 {
		if err := p.Write(ctx, launch.Stdin); err != nil {
			p.kill(true)
			return nil, runtimeError("ISOLATION_UNAVAILABLE")
		}
	}
	return p, nil
}

func (r *MacOS) Destroy(ctx context.Context, handle Handle) error {
	e, err := r.lookup(handle)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.destroyed {
		return nil
	}
	if e.process != nil {
		e.process.kill(true)
		select {
		case <-e.process.done:
		case <-ctx.Done():
			return runtimeError("CLEANUP_FAILED")
		}
		e.process.mu.Lock()
		confirmed := e.process.confirmed
		e.process.mu.Unlock()
		if !confirmed {
			return runtimeError("CLEANUP_FAILED")
		}
	}
	if ctx.Err() != nil {
		return runtimeError("CLEANUP_FAILED")
	}
	if err := os.RemoveAll(e.root); err != nil {
		return runtimeError("CLEANUP_FAILED")
	}
	if _, err := os.Lstat(e.root); !os.IsNotExist(err) {
		return runtimeError("CLEANUP_FAILED")
	}
	e.destroyed = true
	// 保留句柄墓碑以维持 Destroy 幂等；释放 profile、路径和进程引用，避免会话元数据累积。
	e.root, e.workspace, e.profile = "", "", ""
	e.readOnly = nil
	e.process = nil
	return nil
}

type macProcess struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	done, rootDone chan struct{}
	result         Exit
	cancelled      bool
	confirmed      bool
	input          *processInput
	terminal       *os.File
}

var _ InteractiveProcess = (*macProcess)(nil)

func (p *macProcess) Write(ctx context.Context, data []byte) error {
	if p.input == nil {
		return runtimeError("UNIMPLEMENTED")
	}
	select {
	case <-p.rootDone:
		return runtimeError("INVALID_ARGUMENT")
	default:
	}
	return p.input.write(ctx, data)
}

func (p *macProcess) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 1000 || rows > 1000 {
		return runtimeError("INVALID_ARGUMENT")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.rootDone:
		return runtimeError("INVALID_ARGUMENT")
	default:
	}
	if p.terminal == nil {
		return runtimeError("UNIMPLEMENTED")
	}
	if err := pty.Setsize(p.terminal, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
		return runtimeError("AGENT_EXIT_FAILED")
	}
	return nil
}

func (p *macProcess) kill(cancelled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.rootDone:
		return
	default:
	}
	p.cancelled = cancelled
	if p.cmd != nil && p.cmd.Process != nil {
		killProcessTree(p.cmd.Process.Pid)
	}
	if p.input != nil {
		_ = p.input.pipe.Close()
	}
}

func (p *macProcess) Wait() Exit { <-p.done; p.mu.Lock(); defer p.mu.Unlock(); return p.result }
