package runtime

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func (r *Docker) Exec(ctx context.Context, handle Handle, launch LaunchSpec, output func(contracts.OutputData)) (Process, error) {
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
	if launch.Terminal {
		// PTY-in-docker is a later slice; structured Interactive JSONL is supported.
		return nil, runtimeError("UNIMPLEMENTED")
	}
	guestCmd, err := e.guestCommand(launch.Command)
	if err != nil {
		return nil, err
	}
	guestArgs, err := rewriteDockerArgs(e, launch.Args)
	if err != nil {
		return nil, err
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
	env, err = rewriteDockerEnv(e, env)
	if err != nil {
		return nil, err
	}
	if err := validateConfigFiles(launch.ConfigFiles); err != nil {
		return nil, err
	}
	e.attempted = true
	if err := writeConfigFiles(handle.PrivateHome, launch.ConfigFiles); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, runtimeError("CANCELLED")
	}
	if !e.started {
		if _, err := r.runner.Run(ctx, "start", e.containerID); err != nil {
			return nil, runtimeError("ISOLATION_UNAVAILABLE")
		}
		e.started = true
	}
	dockerBin, err := r.runner.LookPath()
	if err != nil {
		return nil, runtimeError("ISOLATION_UNAVAILABLE")
	}
	execArgs := []string{"exec", "-i", "-w", dockerGuestWorkspace}
	for _, item := range env {
		execArgs = append(execArgs, "-e", item)
	}
	execArgs = append(execArgs, e.containerID, guestCmd)
	execArgs = append(execArgs, guestArgs...)
	cmd := exec.Command(dockerBin, execArgs...)
	var stdin *os.File
	if launch.Interactive {
		pipe, err := cmd.StdinPipe()
		if err != nil {
			return nil, runtimeError("ISOLATION_UNAVAILABLE")
		}
		stdin = pipe.(*os.File)
	} else {
		cmd.Stdin = &bytesReader{b: launch.Stdin}
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
	p := &dockerProcess{
		cmd: cmd, containerID: e.containerID, runner: r.runner,
		done: make(chan struct{}), rootDone: make(chan struct{}),
	}
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
		if err != nil && p.result.Code == -1 {
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

// bytesReader avoids string(launch.Stdin) on large payloads.
type bytesReader struct {
	b []byte
	i int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

type dockerProcess struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	containerID    string
	runner         dockerRunner
	done, rootDone chan struct{}
	result         Exit
	cancelled      bool
	confirmed      bool
	input          *processInput
}

var _ InteractiveProcess = (*dockerProcess)(nil)

func (p *dockerProcess) Write(ctx context.Context, data []byte) error {
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

func (p *dockerProcess) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 1000 || rows > 1000 {
		return runtimeError("INVALID_ARGUMENT")
	}
	return runtimeError("UNIMPLEMENTED")
}

func (p *dockerProcess) kill(cancelled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.rootDone:
		return
	default:
	}
	p.cancelled = cancelled
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	if p.containerID != "" && p.runner != nil {
		_, _ = p.runner.Run(context.Background(), "kill", p.containerID)
	}
	if p.input != nil {
		_ = p.input.pipe.Close()
	}
}

func (p *dockerProcess) Wait() Exit {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result
}
