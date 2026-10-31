package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEngineProgressAndSelection(t *testing.T) {
	for input, want := range map[int]int{-1: 0, 0: 0, 42: 42, 100: 100, 101: 100} {
		if got := clampPercent(input); got != want {
			t.Fatalf("clampPercent(%d)=%d, want %d", input, got, want)
		}
	}
	if _, ok := NewSelected("system").(*SystemDocker); !ok {
		t.Fatal("system source must select system Docker")
	}
	for _, source := range []string{"", "harbor", "colima", " managed "} {
		if _, ok := NewSelected(source).(*Colima); !ok {
			t.Fatalf("%q must select Colima", source)
		}
	}
}

func TestColimaPathsAndResolveBin(t *testing.T) {
	root := t.TempDir()
	c := NewColima()
	c.Root = root
	if got := c.root(); got != root || c.binDir() != filepath.Join(root, "bin") {
		t.Fatalf("unexpected paths: root=%q bin=%q", got, c.binDir())
	}
	if got := c.DockerHost(); !strings.HasPrefix(got, "unix://") {
		t.Fatalf("DockerHost=%q", got)
	}
	if _, err := c.resolveBin("missing"); err == nil {
		t.Fatal("missing binary resolved")
	}
	local := filepath.Join(root, "bin", "docker")
	if err := os.MkdirAll(filepath.Dir(local), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("x"), 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := c.resolveBin("docker"); err != nil || got != local {
		t.Fatalf("resolve local: %q %v", got, err)
	}
}

func TestSystemDockerLifecycleAndHealth(t *testing.T) {
	s := &SystemDocker{
		lookPath: func(string) (string, error) { return "/usr/bin/docker", nil },
		run:      func(context.Context, string, ...string) (string, error) { return "27.5", nil },
	}
	if err := s.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := s.Health(context.Background())
	if err != nil || !st.Ready || st.Source != SourceSystemDocker {
		t.Fatalf("health=%+v err=%v", st, err)
	}
	missing := &SystemDocker{lookPath: func(string) (string, error) { return "", errors.New("missing") }}
	if st, err := missing.Health(context.Background()); err != nil || st.Ready || st.Source != SourceSystemDocker {
		t.Fatalf("missing health=%+v err=%v", st, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := missing.Ensure(cancelled); err == nil {
		t.Fatal("cancelled Ensure succeeded")
	}
}

func TestRuntimeImageBranches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	run := func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		calls = append(calls, args)
		if args[0] == "image" {
			return "", errors.New("missing")
		}
		return "", nil
	}
	var progress []Progress
	if err := ensureRuntimeImage(context.Background(), run, "docker", nil, dir, func(p Progress) { progress = append(progress, p) }); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[1][0] != "build" || len(progress) != 3 {
		t.Fatalf("calls=%v progress=%v", calls, progress)
	}
	if err := ensureRuntimeImage(context.Background(), func(context.Context, string, []string, []string) (string, error) { return "", nil }, "docker", nil, dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := ensureRuntimeImage(context.Background(), run, "docker", nil, "", nil); err == nil {
		t.Fatal("empty Dockerfile dir succeeded")
	}
	if err := ensureRuntimeImage(context.Background(), run, "docker", nil, filepath.Join(dir, "missing"), nil); err == nil {
		t.Fatal("missing Dockerfile succeeded")
	}
}

func TestDefaultRunAndErrorFormatting(t *testing.T) {
	if _, err := defaultRun(context.Background(), "/definitely/missing", nil, nil); err == nil {
		t.Fatal("missing command succeeded")
	}
	if got := errEngine("x"); got.Code == "" || got.Message != "x" {
		t.Fatalf("error=%+v", got)
	}
}
