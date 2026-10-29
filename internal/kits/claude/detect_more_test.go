package claude

import (
	"context"
	"testing"
)

func TestDetectCapabilityBranches(t *testing.T) {
	k, err := New("/bin/sh", "model")
	if err != nil {
		t.Fatal(err)
	}
	k.inspect = func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		if len(args) > 0 && args[len(args)-1] == "--version" {
			return "2.1.292 (Claude Code)\n", nil
		}
		return "--print --input-format stream-json --strict-mcp-config --mcp-config --setting-sources --no-session-persistence --permission-mode --disable-slash-commands --tools", nil
	}
	info, err := k.Detect(context.Background())
	if err != nil || !info.Agent.Available || info.Agent.Version != "2.1.292" {
		t.Fatalf("Detect success: %#v %v", info, err)
	}
	k.inspect = func(_ context.Context, _ string, args []string, _ []string) (string, error) {
		if len(args) > 0 && args[len(args)-1] == "--version" {
			return "2.1.292 (Claude Code)", nil
		}
		return "--print", nil
	}
	info, err = k.Detect(context.Background())
	if err != nil || info.Agent.Available || info.Agent.Reason == "" {
		t.Fatalf("Detect missing flags: %#v %v", info, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := k.Detect(ctx); err == nil {
		t.Fatal("cancelled Detect succeeded")
	}
}
