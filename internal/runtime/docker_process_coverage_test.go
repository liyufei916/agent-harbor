package runtime

import (
	"context"
	"os/exec"
	"testing"
)

func TestDockerProcessGuardsAndLifecycle(t *testing.T) {
	p := &dockerProcess{done: make(chan struct{}), rootDone: make(chan struct{})}
	if err := p.Write(context.Background(), []byte("x")); err == nil {
		t.Fatal("write without input succeeded")
	}
	for _, size := range [][2]int{{0, 1}, {1, 0}, {1001, 1}} {
		if err := p.Resize(size[0], size[1]); err == nil {
			t.Fatalf("resize accepted: %v", size)
		}
	}
	if err := p.Resize(80, 24); err == nil {
		t.Fatal("resize without terminal succeeded")
	}
	close(p.rootDone)
	if err := p.Resize(80, 24); err == nil {
		t.Fatal("resize after exit succeeded")
	}
	p.kill(true)
	cmd := exec.Command("/usr/bin/true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	q := &dockerProcess{cmd: cmd, done: make(chan struct{}), rootDone: make(chan struct{}), containerID: "", result: Exit{Code: 0}}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	close(q.rootDone)
	close(q.done)
	if got := q.Wait(); got.Code != 0 {
		t.Fatalf("wait=%+v", got)
	}
}
