package claude

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/kits"
)

func TestResolveLaunchDockerGuest(t *testing.T) {
	t.Setenv("HARBOR_RUNTIME", "docker")
	k, err := New("claude", "claude-sonnet-4-5")
	if err != nil {
		t.Fatal(err)
	}
	launch, err := k.resolveLaunch()
	if err != nil {
		t.Fatal(err)
	}
	if launch.Command != kits.GuestClaude {
		t.Fatalf("command=%q", launch.Command)
	}
	if len(launch.ReadOnly) != 1 || launch.ReadOnly[0] != kits.GuestClaude {
		t.Fatalf("readonly=%v", launch.ReadOnly)
	}
}
