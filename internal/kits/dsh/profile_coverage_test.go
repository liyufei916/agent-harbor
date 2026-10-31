package dsh

import (
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/kits"
)

func TestBuildProfileConversationAndMCP(t *testing.T) {
	input := kits.KitInput{PrivateHome: "/tmp/home", ModelBaseURL: "https://api.example.test", MCPURLs: []string{"http://127.0.0.1:1234/mcp", "http://127.0.0.1:2345/mcp"}, Sentinel: "sentinel"}
	files, err := buildProfile(input, "deepseek-chat", true)
	if err != nil || len(files) != 3 {
		t.Fatalf("files=%d err=%v", len(files), err)
	}
	patch := string(files[2].Content)
	for _, want := range []string{"harbor-conversation.mjs", "harbor-2", "deepseek-chat", "127.0.0.1:1234"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("profile missing %q: %s", want, patch)
		}
	}
	if _, err := buildProfile(kits.KitInput{MCPURL: "bad"}, "model", false); err == nil {
		t.Fatal("invalid MCP accepted")
	}
	files, err = buildProfile(kits.KitInput{}, "model", false)
	if err != nil || !strings.Contains(string(files[2].Content), "headless-startup") {
		t.Fatalf("headless profile err=%v", err)
	}
}

func TestProfileWrapperAndDockerPaths(t *testing.T) {
	files, err := profile(kits.KitInput{}, "model")
	if err != nil || len(files) != 3 {
		t.Fatalf("profile=%d err=%v", len(files), err)
	}
	node, root, script, err := dockerPaths()
	if err != nil || node == "" || root == "" || script == "" {
		t.Fatalf("docker paths=%q %q %q %v", node, root, script, err)
	}
}

func TestDefaultInstallAndNodePaths(t *testing.T) {
	if DefaultInstallation() == "" {
		t.Fatal("default installation unexpectedly empty")
	}
	if DefaultNode("/tmp/install") == "" || DefaultNode("") != "" {
		t.Fatal("default node path mismatch")
	}
}
