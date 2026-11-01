package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeImageNameUsesConfiguredValue(t *testing.T) {
	t.Setenv("HARBOR_DOCKER_IMAGE", "  example/harbor:test  ")
	if got := RuntimeImageName(); got != "example/harbor:test" {
		t.Fatalf("RuntimeImageName=%q", got)
	}
	t.Setenv("HARBOR_DOCKER_IMAGE", "   ")
	if got := RuntimeImageName(); got != "agent-harbor/runtime:dev" {
		t.Fatalf("empty image should use default, got %q", got)
	}
}

func TestEnsureRuntimeImageReportsPresentAndBuildFailure(t *testing.T) {
	var progress []Progress
	runPresent := func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		if len(args) < 2 || args[0] != "image" {
			t.Fatalf("unexpected inspect args: %v", args)
		}
		return "sha256:present", nil
	}
	if err := ensureRuntimeImage(context.Background(), runPresent, "docker", nil, t.TempDir(), func(p Progress) { progress = append(progress, p) }); err != nil {
		t.Fatal(err)
	}
	if len(progress) != 2 || progress[1].Percent != 100 {
		t.Fatalf("present progress=%+v", progress)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runBuildFail := func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		if args[0] == "image" {
			return "", errors.New("missing")
		}
		return "", errors.New("network unavailable")
	}
	if err := ensureRuntimeImage(context.Background(), runBuildFail, "docker", []string{"PATH=/bin"}, dir, nil); err == nil || !strings.Contains(err.Error(), "构建 Harbor") {
		t.Fatalf("build failure=%v", err)
	}
}

func TestSystemDockerEnsureImageRejectsUnavailableDaemon(t *testing.T) {
	s := &SystemDocker{
		lookPath: func(string) (string, error) { return "/usr/bin/docker", nil },
		run:      func(context.Context, string, ...string) (string, error) { return "", errors.New("daemon down") },
	}
	if err := s.EnsureImage(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "daemon") {
		t.Fatalf("expected daemon error, got %v", err)
	}
}
