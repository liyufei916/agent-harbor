package codex

import (
	"context"
	"errors"
	"testing"
)

func TestDetectFailureBranches(t *testing.T) {
	k, err := New("/bin/sh", "model")
	if err != nil {
		t.Fatal(err)
	}
	k.inspect = func(_ context.Context, _ string, args []string) (string, error) {
		if len(args) == 1 && args[0] == "--version" {
			return "not codex", nil
		}
		return "", nil
	}
	info, err := k.Detect(context.Background())
	if err != nil || info.Agent.Available {
		t.Fatalf("invalid version accepted: %#v %v", info, err)
	}
	k.inspect = func(_ context.Context, _ string, _ []string) (string, error) { return "", errors.New("probe failed") }
	if _, err := k.Detect(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := k.Detect(ctx); err == nil {
		t.Fatal("cancelled detection succeeded")
	}
}
