package gate

import (
	"bytes"
	"testing"
)

func TestSecretRedactorErrorBodyScrubsFragments(t *testing.T) {
	secret := "sk-abcdefghijklmnop"
	r := newSecretRedactorOpts([]string{secret}, true)
	in := []byte("auth failed prefix sk-abcde and suffix mnop trailing")
	out := r.scrub(in)
	if bytes.Contains(out, []byte("sk-abcde")) || bytes.Contains(out, []byte("mnop")) {
		t.Fatalf("fragments leaked: %s", out)
	}
	if !bytes.Contains(out, []byte(redactedText)) {
		t.Fatalf("want redacted mark: %s", out)
	}
}

func TestSecretRedactorOKBodyKeepsShortFragments(t *testing.T) {
	secret := "sk-abcdefghijklmnop"
	r := newSecretRedactorOpts([]string{secret}, false)
	in := []byte("note sk-abcde is not full key")
	out := r.scrub(in)
	// 非 error 模式不抹 8 位前缀（仅完整密钥）
	if !bytes.Contains(out, []byte("sk-abcde")) {
		t.Fatalf("ok body should keep short fragment: %s", out)
	}
}
