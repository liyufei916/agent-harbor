package runtime

import (
	"context"
	"strings"
	"testing"
)

type coverageDockerRunner struct{ calls []string }

func (r *coverageDockerRunner) LookPath() (string, error) { return "/usr/bin/true", nil }
func (r *coverageDockerRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, strings.Join(args, " "))
	return "ok", nil
}

func TestDockerExecClosedLoopWithFakeRunner(t *testing.T) {
	runner := &coverageDockerRunner{}
	r := &Docker{runner: runner, image: DefaultDockerImage, entries: map[string]*dockerEntry{}}
	h := Handle{ID: "s", PrivateHome: t.TempDir(), Workspace: t.TempDir()}
	r.entries[h.ID] = &dockerEntry{handle: h, workspace: h.Workspace, root: h.Workspace, containerID: "cid", mounts: map[string]string{}, readOnly: map[string]bool{}, allowedTCP: []string{"127.0.0.1:1234"}}
	p, err := r.Exec(context.Background(), h, LaunchSpec{Command: "/bin/echo", Args: []string{"hello"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Wait(); got.Code != 0 {
		t.Fatalf("exit=%+v", got)
	}
	if len(runner.calls) == 0 || !strings.HasPrefix(runner.calls[0], "start cid") {
		t.Fatalf("calls=%v", runner.calls)
	}
	if _, err := r.Exec(context.Background(), h, LaunchSpec{Command: "relative"}, nil); err == nil {
		t.Fatal("relative command accepted")
	}
}
