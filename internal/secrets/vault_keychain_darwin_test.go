//go:build darwin

package secrets

import (
	"strings"
	"testing"
)

func TestShellSingleQuoteEscapesApostrophe(t *testing.T) {
	got := shellSingleQuote(`a'b`)
	if got != `'a'\''b'` {
		t.Fatalf("shellSingleQuote = %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatal("引号结果不应含换行")
	}
}

func TestValidateVaultIDRejectsControl(t *testing.T) {
	for _, id := range []string{"", "a\nb", "a\rb", "a\x00b"} {
		if err := validateVaultID(id); err == nil {
			t.Fatalf("应拒绝 id=%q", id)
		}
	}
	if err := validateVaultID("mcp:https://example.com"); err != nil {
		t.Fatal(err)
	}
}
