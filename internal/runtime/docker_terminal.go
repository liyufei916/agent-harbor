package runtime

import (
	"context"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// execDockerTerminal attaches a host PTY to `docker exec -it` so guest programs
// that need a TTY (raw mode / curses) work. Structured Interactive JSONL stays
// on the non-Terminal path (pipe stdin).
func (r *Docker) execDockerTerminal(
	ctx context.Context,
	e *dockerEntry,
	dockerBin, guestCmd string,
	guestArgs, env []string,
	launch LaunchSpec,
	output func(contracts.OutputData),
) (Process, error) {
	execArgs := []string{"exec", "-i", "-t", "-w", dockerGuestWorkspace}
	for _, item := range env {
		execArgs = append(execArgs, "-e", item)
	}
	execArgs = append(execArgs, e.containerID, guestCmd)
	execArgs = append(execArgs, guestArgs...)
	cmd := exec.Command(dockerBin, execArgs...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	master, err := pty.Start(cmd)
	if err != nil {
		return nil, runtimeError("ISOLATION_UNAVAILABLE")
	}
	if err := pty.Setsize(master, &pty.Winsize{Cols: 80, Rows: 24}); err != nil {
		_ = master.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil, runtimeError("ISOLATION_UNAVAILABLE")
	}

	p := &dockerProcess{
		cmd:         cmd,
		containerID: e.containerID,
		runner:      r.runner,
		done:        make(chan struct{}),
		rootDone:    make(chan struct{}),
		terminal:    master,
		input:       newProcessInput(master, nil),
	}
	e.process = p

	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		readOutput(master, "stdout", output)
	}()
	go func() {
		err := cmd.Wait()
		close(p.rootDone)
		readers.Wait()
		p.mu.Lock()
		_ = master.Close()
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
	if len(launch.Stdin) > 0 {
		if err := p.Write(ctx, launch.Stdin); err != nil {
			p.kill(true)
			return nil, runtimeError("ISOLATION_UNAVAILABLE")
		}
	}
	return p, nil
}
