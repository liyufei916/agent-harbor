package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestColimaEnsureRequiresBinaries(t *testing.T) {
	c := NewColima()
	c.Root = t.TempDir()
	c.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	err := c.Ensure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "colima") {
		t.Fatalf("got %v", err)
	}
}

func TestColimaEnsureFindsLocalBin(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"colima", "docker"} {
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	c := NewColima()
	c.Root = root
	c.lookPath = func(string) (string, error) { return "", errors.New("ignore path") }
	if err := c.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.DockerHost(), "agent-harbor") {
		t.Fatalf("host=%s", c.DockerHost())
	}
}

func TestColimaStartIdempotentWhenHealthy(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	_ = os.MkdirAll(bin, 0700)
	for _, name := range []string{"colima", "docker"} {
		_ = os.WriteFile(filepath.Join(bin, name), []byte("x"), 0755)
	}
	var calls []string
	c := NewColima()
	c.Root = root
	c.lookPath = func(string) (string, error) { return "", errors.New("x") }
	c.run = func(_ context.Context, name string, args []string, env []string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "info") {
			return "24.0.0", nil
		}
		t.Fatalf("unexpected start when healthy: %s %v env=%v", name, args, env)
		return "", nil
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("DOCKER_HOST")
		_ = os.Unsetenv("COLIMA_HOME")
		_ = os.Unsetenv("COLIMA_PROFILE")
	})
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("DOCKER_HOST") == "" {
		t.Fatal("expected DOCKER_HOST applied")
	}
	if len(calls) < 1 {
		t.Fatal("expected health probe")
	}
}

func TestColimaStartInvokesColima(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	_ = os.MkdirAll(bin, 0700)
	colimaBin := filepath.Join(bin, "colima")
	dockerBin := filepath.Join(bin, "docker")
	_ = os.WriteFile(colimaBin, []byte("x"), 0755)
	_ = os.WriteFile(dockerBin, []byte("x"), 0755)

	var sawStart bool
	infoCalls := 0
	c := NewColima()
	c.Root = root
	c.lookPath = func(string) (string, error) { return "", errors.New("x") }
	c.run = func(_ context.Context, name string, args []string, _ []string) (string, error) {
		joined := strings.Join(args, " ")
		if name == colimaBin && strings.HasPrefix(joined, "start "+ProfileName) {
			sawStart = true
			if !strings.Contains(joined, "--activate=false") {
				t.Fatal("must not steal global docker context")
			}
			return "", nil
		}
		if name == dockerBin && strings.Contains(joined, "info") {
			infoCalls++
			if infoCalls == 1 {
				return "", errors.New("not ready") // first Health before start
			}
			return "24.0.0", nil
		}
		return "", errors.New("unexpected " + name + " " + joined)
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !sawStart {
		t.Fatal("expected colima start")
	}
}
