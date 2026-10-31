package codex

import (
	"path/filepath"
	"testing"
)

func TestBrewRuntimeRootsHandlesUnknownPaths(t *testing.T) {
	_ = brewRuntimeRoots(filepath.Join(t.TempDir(), "node"))
	_ = brewRuntimeRoots("/unknown/node")
}

func TestEncodePromptValidation(t *testing.T) {
	for _, input := range [][]byte{nil, []byte("\n"), []byte{0}, []byte{0xff}} {
		if _, err := encodePrompt(input); err == nil {
			t.Fatalf("invalid prompt accepted: %q", input)
		}
	}
	out, err := encodePrompt([]byte("hello\n"))
	if err != nil || string(out) != "\"hello\"\n" {
		t.Fatalf("prompt=%q err=%v", out, err)
	}
}
