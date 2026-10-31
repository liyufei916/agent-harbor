package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeDockerGuestWithFixtureCLI(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	script := "#!/bin/sh\ncase \"$1\" in\n image) exit 0;;\n run) echo probe-output;;\n esac\n"
	if err := os.WriteFile(docker, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := ProbeDockerGuest(context.Background(), "/bin/true", []string{"--version"})
	if err != nil || out != "probe-output\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
func TestProbeDockerGuestRejectsNUL(t *testing.T) {
	if _, err := ProbeDockerGuest(context.Background(), "/bin/true", []string{"bad\x00"}); err == nil {
		t.Fatal("NUL accepted")
	}
}
