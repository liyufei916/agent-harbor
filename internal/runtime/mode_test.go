package runtime

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDockerEnabledAndGuestPaths(t *testing.T) {
	t.Setenv("HARBOR_RUNTIME", "")
	if DockerEnabled() {
		t.Fatal("default off")
	}
	t.Setenv("HARBOR_RUNTIME", "docker")
	if !DockerEnabled() {
		t.Fatal("docker on")
	}
	t.Setenv("HARBOR_DOCKER_IMAGE", "example:test")
	if DockerImage() != "example:test" {
		t.Fatal(DockerImage())
	}
	for _, p := range []string{"/usr/local/bin/claude", "/opt/harbor/dsh-0.2.1-rc.2/x", "/bin/echo"} {
		if !IsGuestImagePath(p) {
			t.Fatalf("want guest %s", p)
		}
	}
	if IsGuestImagePath("/Users/me/claude") || IsGuestImagePath("") {
		t.Fatal("host path must not count as guest")
	}
}

func TestDockerCreateGuestOnlyReadOnly(t *testing.T) {
	ws, err := os.MkdirTemp("/private/tmp", "harbor-docker-guest-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws) })
	fake := &fakeDockerRunner{binOK: true}
	r := NewDocker()
	r.runner = fake
	h, err := r.Create(context.Background(), RuntimeSpec{
		SessionID: "g1", MountMode: "direct", Workspace: ws,
		AllowedTCP: []string{"127.0.0.1:7"}, ReadOnlyPaths: []string{"/usr/local/bin/claude"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var create []string
	for _, c := range fake.calls {
		if len(c) > 0 && c[0] == "create" {
			create = c
		}
	}
	joined := strings.Join(create, " ")
	if strings.Contains(joined, "src=/usr/local/bin/claude") {
		t.Fatal("guest image path must not be bind-mounted from host:", joined)
	}
	if err := r.Destroy(context.Background(), h); err != nil {
		t.Fatal(err)
	}
}
