package runtime

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerGuestPathAndGateRewrite(t *testing.T) {
	home := "/private/tmp/harbor-home"
	ws := "/private/tmp/harbor-ws"
	e := &dockerEntry{
		handle:     Handle{PrivateHome: home, Workspace: ws},
		workspace:  ws,
		mounts:     map[string]string{"/opt/harbor/node": "/harbor/ro/0/node"},
		allowedTCP: []string{"127.0.0.1:18080"},
	}
	if g, ok := e.guestPath(filepath.Join(ws, "a.txt")); !ok || g != "/workspace/a.txt" {
		t.Fatalf("workspace file: %q %v", g, ok)
	}
	if g, ok := e.guestPath(filepath.Join(home, "cfg")); !ok || g != "/home/harbor/cfg" {
		t.Fatalf("home: %q %v", g, ok)
	}
	if g, ok := e.guestPath("/opt/harbor/node"); !ok || g != "/harbor/ro/0/node" {
		t.Fatalf("mount: %q %v", g, ok)
	}
	guest, err := e.guestCommand("/bin/echo")
	if err != nil || guest != "/bin/echo" {
		t.Fatalf("image bin: %q %v", guest, err)
	}
	env, err := rewriteDockerEnv(e, []string{
		"HOME=" + home,
		"ANTHROPIC_BASE_URL=http://127.0.0.1:18080",
		"OTHER=http://127.0.0.1:9",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "HOME=/home/harbor") {
		t.Fatalf("HOME: %s", joined)
	}
	if !strings.Contains(joined, "host.docker.internal:18080") {
		t.Fatalf("gate rewrite: %s", joined)
	}
	if !strings.Contains(joined, "http://127.0.0.1:9") {
		t.Fatalf("non-allowed port must stay: %s", joined)
	}
}

func TestDockerCreateArgsHostGateway(t *testing.T) {
	r := NewDocker()
	mounts := map[string]string{}
	args := r.createArgs("/private/tmp/h", "/private/tmp/w", []string{"/usr/bin/true"}, mounts)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "host.docker.internal:host-gateway") {
		t.Fatal(joined)
	}
	if strings.Contains(joined, "docker.sock") || strings.Contains(joined, "--network none") {
		t.Fatal(joined)
	}
	if mounts["/usr/bin/true"] != "/harbor/ro/0/true" {
		t.Fatalf("mounts=%v", mounts)
	}
}
