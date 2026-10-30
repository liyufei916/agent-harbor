package runtime

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type fakeDockerRunner struct {
	binOK   bool
	infoErr error
	create  func(args []string) (string, error)
	rmErr   error
	calls   [][]string
}

func (f *fakeDockerRunner) LookPath() (string, error) {
	if !f.binOK {
		return "", errors.New("docker not found")
	}
	return "/usr/local/bin/docker", nil
}

func (f *fakeDockerRunner) Run(_ context.Context, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if len(args) == 0 {
		return "", errors.New("empty")
	}
	switch args[0] {
	case "info":
		return "24.0.0", f.infoErr
	case "create":
		if f.create != nil {
			return f.create(args)
		}
		return "cid-test", nil
	case "start", "kill":
		return "", nil
	case "rm":
		return "", f.rmErr
	default:
		return "", errors.New("unexpected " + args[0])
	}
}

func TestDockerHealthFailClosedWithoutBinary(t *testing.T) {
	r := NewDocker()
	r.runner = &fakeDockerRunner{binOK: false}
	if err := r.Health(context.Background()); err == nil {
		t.Fatal("expected ISOLATION_UNAVAILABLE")
	} else if app, ok := err.(*contracts.AppError); !ok || app.Code != "ISOLATION_UNAVAILABLE" {
		t.Fatalf("got %#v", err)
	}
}

func TestDockerHealthFailClosedWhenDaemonDown(t *testing.T) {
	r := NewDocker()
	r.runner = &fakeDockerRunner{binOK: true, infoErr: errors.New("Cannot connect to the Docker daemon")}
	if err := r.Health(context.Background()); err == nil {
		t.Fatal("expected ISOLATION_UNAVAILABLE")
	}
}

func TestDockerCreateRejectsForbiddenWorkspace(t *testing.T) {
	r := NewDocker()
	r.runner = &fakeDockerRunner{binOK: true}
	_, err := r.Create(context.Background(), RuntimeSpec{
		SessionID: "s1", MountMode: "direct", Workspace: "/etc",
		AllowedTCP: []string{"127.0.0.1:1"}, ReadOnlyPaths: []string{"/usr/bin/true"},
	})
	if err == nil {
		t.Fatal("expected WORKSPACE_DENIED")
	}
	if app, ok := err.(*contracts.AppError); !ok || app.Code != "WORKSPACE_DENIED" {
		t.Fatalf("got %#v", err)
	}
}

func TestDockerCreateDestroyAndExecUnsupported(t *testing.T) {
	ws, err := os.MkdirTemp("/private/tmp", "harbor-docker-ws-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws) })
	fake := &fakeDockerRunner{
		binOK: true,
		create: func(args []string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "--network none") {
				t.Fatal("Gate needs bridge + host-gateway, not network none")
			}
			if !strings.Contains(joined, "host.docker.internal:host-gateway") {
				t.Fatalf("want host-gateway: %s", joined)
			}
			if strings.Contains(joined, "docker.sock") {
				t.Fatal("must not mount docker.sock")
			}
			if !strings.Contains(joined, "--cap-drop ALL") {
				t.Fatal("want cap-drop ALL")
			}
			return "cid-abc", nil
		},
	}
	r := NewDocker()
	r.runner = fake
	r.image = "harbor-test:fixture"
	h, err := r.Create(context.Background(), RuntimeSpec{
		SessionID: "s1", MountMode: "direct", Workspace: ws,
		AllowedTCP: []string{"127.0.0.1:9"}, ReadOnlyPaths: []string{"/usr/bin/true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.PrivateHome == "" || h.Workspace != ws {
		t.Fatalf("handle=%+v", h)
	}
	if err := r.Destroy(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	if err := r.Destroy(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	var sawRm bool
	for _, c := range fake.calls {
		if len(c) >= 3 && c[0] == "rm" && c[1] == "-f" && c[2] == "cid-abc" {
			sawRm = true
		}
	}
	if !sawRm {
		t.Fatalf("expected docker rm -f: %#v", fake.calls)
	}
}

func TestNewSelectedDockerEnv(t *testing.T) {
	t.Setenv("HARBOR_RUNTIME", "docker")
	rt := NewSelected()
	if _, ok := rt.(*Docker); !ok {
		t.Fatalf("want *Docker got %T", rt)
	}
	t.Setenv("HARBOR_RUNTIME", "")
	rt = NewSelected()
	if _, ok := rt.(*Docker); ok {
		t.Fatal("default must not be Docker")
	}
}
