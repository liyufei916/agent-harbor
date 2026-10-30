package runtime

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestProbeDockerGuestRejectsNonGuestPath(t *testing.T) {
	_, err := ProbeDockerGuest(context.Background(), "/opt/homebrew/bin/claude", nil)
	if err == nil {
		t.Fatal("expected AGENT_UNSUPPORTED")
	}
	if app, ok := err.(*contracts.AppError); !ok || app.Code != "AGENT_UNSUPPORTED" {
		t.Fatalf("got %#v", err)
	}
}

func TestProbeDockerGuestMissingImageMessage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("本机无 docker CLI")
	}
	info := exec.Command("docker", "info")
	if err := info.Run(); err != nil {
		t.Skip("docker daemon 不可用")
	}
	t.Setenv("HARBOR_DOCKER_IMAGE", "agent-harbor/missing-image-fixture:never")
	_, err := ProbeDockerGuest(context.Background(), "/bin/true", nil)
	if err == nil {
		t.Fatal("expected ISOLATION_UNAVAILABLE")
	}
	app, ok := err.(*contracts.AppError)
	if !ok || app.Code != "ISOLATION_UNAVAILABLE" {
		t.Fatalf("got %#v", err)
	}
	if !strings.Contains(app.Message, "镜像") {
		t.Fatalf("want image hint: %q", app.Message)
	}
}
