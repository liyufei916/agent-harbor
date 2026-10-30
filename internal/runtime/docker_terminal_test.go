package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestDockerTerminalRejectsInteractiveCombo(t *testing.T) {
	ws, err := os.MkdirTemp("/private/tmp", "harbor-docker-pty-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws) })

	r := NewDocker()
	r.runner = &fakeDockerRunner{binOK: true}
	h, err := r.Create(context.Background(), RuntimeSpec{
		SessionID: "pty-combo", MountMode: "direct", Workspace: ws,
		AllowedTCP: []string{"127.0.0.1:9"}, ReadOnlyPaths: []string{"/usr/bin/true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Destroy(context.Background(), h) })

	_, err = r.Exec(context.Background(), h, LaunchSpec{
		Command: "/bin/echo", Terminal: true, Interactive: true,
	}, nil)
	if err == nil {
		t.Fatal("expected INVALID_ARGUMENT")
	}
	if app, ok := err.(*contracts.AppError); !ok || app.Code != "INVALID_ARGUMENT" {
		t.Fatalf("got %#v", err)
	}
}

func TestDockerResizeWithoutTerminalUnimplemented(t *testing.T) {
	p := &dockerProcess{rootDone: make(chan struct{})}
	if err := p.Resize(80, 24); err == nil {
		t.Fatal("expected UNIMPLEMENTED")
	} else if app, ok := err.(*contracts.AppError); !ok || app.Code != "UNIMPLEMENTED" {
		t.Fatalf("got %#v", err)
	}
	if err := p.Resize(0, 24); err == nil {
		t.Fatal("expected INVALID_ARGUMENT")
	} else if app, ok := err.(*contracts.AppError); !ok || app.Code != "INVALID_ARGUMENT" {
		t.Fatalf("got %#v", err)
	}
}
