package codex

import "testing"

func TestCodexValidationHelpers(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:1", "http://localhost:1", "https://127.0.0.1:1", "http://127.0.0.1:01"} {
		if gateURL(raw) != (raw == "http://127.0.0.1:1") {
			t.Errorf("gateURL(%q)", raw)
		}
	}
	valid := "abcdefghijklmnopqrstuvwxyz123456"
	if !validSentinel(valid) || validSentinel("short") || validSentinel(valid+"\n") {
		t.Fatal("sentinel validation mismatch")
	}
	if _, err := New("", ""); err == nil {
		t.Fatal("empty model accepted")
	}
	if _, err := New("", "model"); err != nil {
		t.Fatal(err)
	}
}
