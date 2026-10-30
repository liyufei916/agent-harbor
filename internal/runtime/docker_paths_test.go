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
	e.gateToProxy = map[string]string{"18080": "49152"}
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
	if !strings.Contains(joined, dockerGateHost+":49152") {
		t.Fatalf("proxy rewrite: %s", joined)
	}
	if !strings.Contains(joined, "http://127.0.0.1:9") {
		t.Fatalf("non-mapped port must stay: %s", joined)
	}
}

func TestRewriteDockerConfigFilesMapsGateURLs(t *testing.T) {
	e := &dockerEntry{
		allowedTCP:  []string{"127.0.0.1:56780", "127.0.0.1:56781"},
		gateToProxy: map[string]string{"56780": "41001", "56781": "41002"},
	}
	raw := "base_url = \"http://127.0.0.1:56780/v1\"\nurl = \"http://127.0.0.1:56781/mcp/0\"\n"
	files, err := rewriteDockerConfigFiles(e, []ConfigFile{{RelativePath: "codex/config.toml", Content: []byte(raw)}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(files[0].Content)
	if !strings.Contains(got, "http://"+dockerGateHost+":41001/v1") {
		t.Fatalf("model: %s", got)
	}
	if !strings.Contains(got, "http://"+dockerGateHost+":41002/mcp/0") {
		t.Fatalf("mcp: %s", got)
	}
	if strings.Contains(got, "127.0.0.1") {
		t.Fatalf("loopback left in guest config: %s", got)
	}
	if files[0].RelativePath != "codex/config.toml" {
		t.Fatalf("path: %s", files[0].RelativePath)
	}
}

func TestRewriteDockerConfigFilesMapsPrivateHome(t *testing.T) {
	home := "/Users/he.s/Library/Application Support/AgentHarbor/sessions/x/home"
	ws := "/Users/he.s/proj"
	e := &dockerEntry{
		handle:    Handle{PrivateHome: home, Workspace: ws},
		workspace: ws,
	}
	raw := `[{"insert":[{"id":"harbor-conversation","name":"` + home + `/harbor-conversation.mjs"}]}]`
	files, err := rewriteDockerConfigFiles(e, []ConfigFile{{RelativePath: "dsh/profiles/harbor/cordis.patch.yml", Content: []byte(raw)}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(files[0].Content)
	if !strings.Contains(got, dockerGuestHome+"/harbor-conversation.mjs") {
		t.Fatalf("guest path missing: %s", got)
	}
	if strings.Contains(got, "Application Support") {
		t.Fatalf("host path left: %s", got)
	}
}

func TestDockerCreateArgsHostGateway(t *testing.T) {
	r := NewDocker()
	mounts := map[string]string{}
	args := r.createArgs("sess-abc", "/private/tmp/h", "/private/tmp/w", []string{"/usr/bin/true"}, mounts)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, dockerGateHost+":host-gateway") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "host.docker.internal:127.0.0.1") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "--network "+HarborDockerNetwork) {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "agent-harbor.session=sess-abc") {
		t.Fatal(joined)
	}
	if strings.Contains(joined, "docker.sock") {
		t.Fatal(joined)
	}
	if mounts["/usr/bin/true"] != "/harbor/ro/0/true" {
		t.Fatalf("mounts=%v", mounts)
	}
}
