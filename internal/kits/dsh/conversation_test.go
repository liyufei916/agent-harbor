package dsh

import (
	"strings"
	"testing"
)

func TestConversationMessageValidationAndEncoding(t *testing.T) {
	for _, input := range [][]byte{nil, []byte("   "), []byte{0}, []byte{0xff}} {
		if _, err := encodeConversationMessage(input); err == nil {
			t.Fatalf("invalid message accepted: %q", input)
		}
	}
	encoded, err := encodeConversationMessage([]byte("hello"))
	if err != nil || !strings.Contains(string(encoded), `"text":"hello"`) || !strings.HasSuffix(string(encoded), "\n") {
		t.Fatalf("encoded=%q err=%v", encoded, err)
	}
	if _, err := NewConversation("/missing/node", "/missing/install", ""); err == nil {
		t.Fatal("invalid conversation kit accepted")
	}
}

func TestConversationFileReplacesModules(t *testing.T) {
	file := conversationFile("/tmp/harbor")
	content := string(file.Content)
	if file.RelativePath != "harbor-conversation.mjs" || strings.Contains(content, "__AGENT_MODULE__") || !strings.Contains(content, "dsh-agent") || !strings.Contains(content, "/tmp/harbor") {
		t.Fatalf("conversation file not rendered: %s", content)
	}
}
