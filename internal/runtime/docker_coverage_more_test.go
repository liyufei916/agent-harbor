package runtime

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func coverageDockerEntry() *dockerEntry {
	return &dockerEntry{
		workspace:  "/host/work",
		handle:     Handle{PrivateHome: "/host/home"},
		mounts:     map[string]string{"/host/ro": "/harbor/ro/0"},
		allowedTCP: []string{"127.0.0.1:1234"},
	}
}

func TestDockerPathAndEnvironmentRewrites(t *testing.T) {
	e := coverageDockerEntry()
	for host, want := range map[string]string{"/host/work": "/workspace", "/host/work/a": "/workspace/a", "/host/home": "/home/harbor", "/host/home/.cfg": "/home/harbor/.cfg", "/host/ro": "/harbor/ro/0", "/unknown": ""} {
		if got, _ := e.guestPath(host); got != want {
			t.Fatalf("guestPath(%q)=%q want %q", host, got, want)
		}
	}
	for command, want := range map[string]string{"/bin/sh": "/bin/sh", "/host/work/run.sh": "/workspace/run.sh", "relative": ""} {
		got, err := e.guestCommand(command)
		if want == "" {
			if err == nil {
				t.Fatalf("unsupported command accepted: %q", command)
			}
		} else if err != nil || got != want {
			t.Fatalf("guestCommand=%q %v", got, err)
		}
	}
	args, err := rewriteDockerArgs(e, []string{"/host/work/a", "plain"})
	if err != nil || args[0] != "/workspace/a" || args[1] != "plain" {
		t.Fatalf("args=%v err=%v", args, err)
	}
	if _, err := rewriteDockerArgs(e, []string{"bad\x00"}); err == nil {
		t.Fatal("NUL argument accepted")
	}
	env, err := rewriteDockerEnv(e, []string{"HOME=/host/home", "TMPDIR=/host/work/tmp", "CODEX_HOME=/host/home/codex", "HARBOR_GATE_URL=http://127.0.0.1:1234/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "|")
	for _, want := range []string{"HOME=/home/harbor", "TMPDIR=/home/harbor/tmp", "CODEX_HOME=/home/harbor/codex", "harbor.gate:1234"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env=%s missing %s", joined, want)
		}
	}
	if _, err := rewriteDockerEnv(e, []string{"invalid"}); err == nil {
		t.Fatal("invalid env accepted")
	}
}

func TestDockerConfigAndGateRewrites(t *testing.T) {
	e := coverageDockerEntry()
	files, err := rewriteDockerConfigFiles(e, []ConfigFile{{RelativePath: "x", Content: []byte("http://127.0.0.1:1234/host/home /host/work")}})
	if err != nil || !strings.Contains(string(files[0].Content), "harbor.gate:1234") || strings.Contains(string(files[0].Content), "/host/home") {
		t.Fatalf("files=%v err=%v", files, err)
	}
	if got := rewriteGateHost("x 127.0.0.1:1234 y", map[string]bool{"1234": true}); got != "x harbor.gate:1234 y" {
		t.Fatal(got)
	}
	if got := rewriteGateHost("127.0.0.1:9999", map[string]bool{"1234": true}); got != "127.0.0.1:9999" {
		t.Fatal(got)
	}
	if _, err := rewriteDockerGateValue(&dockerEntry{allowedTCP: []string{"bad"}}, "x"); err == nil {
		t.Fatal("invalid allowed port accepted")
	}
}

func TestDockerBytesReaderAndErrors(t *testing.T) {
	r := &bytesReader{b: []byte("abc")}
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	if n != 2 || err != nil || string(buf) != "ab" {
		t.Fatalf("read=%d %v %q", n, err, buf)
	}
	n, err = r.Read(buf)
	if n != 1 || err != nil || string(buf[:1]) != "c" {
		t.Fatalf("read=%d %v %q", n, err, buf)
	}
	_, err = r.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("end=%v", err)
	}
	runErr := &dockerRunError{err: context.Canceled, detail: "daemon down"}
	if runErr.Error() != "daemon down" || !errors.Is(runErr, context.Canceled) {
		t.Fatal(runErr)
	}
	if (&dockerRunError{err: context.DeadlineExceeded}).Error() == "" {
		t.Fatal("empty error")
	}
}
