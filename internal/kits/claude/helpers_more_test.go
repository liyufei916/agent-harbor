package claude

import "testing"

func TestClaudeValidationHelpers(t *testing.T) {
	for _, raw := range []string{"1.2.3 (Claude Code)", " 2.0.0 (Claude Code)\n"} {
		if version, ok := parseClaudeVersion(raw); !ok || version == "" {
			t.Fatalf("parse version %q = %q, %v", raw, version, ok)
		}
	}
	for _, raw := range []string{"", "1.2.3", "1 2 (Claude Code)"} {
		if _, ok := parseClaudeVersion(raw); ok {
			t.Fatalf("invalid version accepted: %q", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.1:1", "http://localhost:1", "https://127.0.0.1:1", "http://127.0.0.1:01"} {
		if gateURL(raw) != (raw == "http://127.0.0.1:1") {
			t.Errorf("gateURL(%q)", raw)
		}
	}
	if _, err := encodeMessage([]byte(" \n")); err == nil {
		t.Fatal("blank Claude message accepted")
	}
	if got, err := encodeMessage([]byte("hello\n")); err != nil || len(got) == 0 {
		t.Fatalf("encode message: %s, %v", got, err)
	}
	if got := detectProbeReason("claude", "--version", kitError("CANCELLED")); got != "检测已取消。" {
		t.Fatalf("cancel reason: %s", got)
	}
	if _, err := New("", ""); err == nil {
		t.Fatal("empty model accepted")
	}
	if _, err := New("", "model"); err != nil {
		t.Fatal(err)
	}
}
