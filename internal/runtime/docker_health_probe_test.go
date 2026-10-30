package runtime

import (
	"context"
	"strings"
	"testing"
)

type emptyInfoRunner struct{}

func (emptyInfoRunner) LookPath() (string, error) { return "/bin/docker", nil }

func (emptyInfoRunner) Run(_ context.Context, args ...string) (string, error) {
	if len(args) >= 1 && args[0] == "info" {
		return "", nil // pinned CLI quirk: exit 0, empty stdout when daemon down
	}
	return "ok", nil
}

func TestDockerHealthRejectsEmptyInfoVersion(t *testing.T) {
	r := NewDocker()
	r.runner = emptyInfoRunner{}
	err := r.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Docker daemon") {
		t.Fatalf("want daemon message, got %q", err.Error())
	}
}
