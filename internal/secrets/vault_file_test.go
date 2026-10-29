package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileVaultRoundTripAndDelete(t *testing.T) {
	dir := t.TempDir()
	v := newFileVault(filepath.Join(dir, "keys"))
	if err := v.Set("account", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "keys", "account.key")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("key permissions: %v, %v", info, err)
	}
	got, err := v.Get("account")
	if err != nil || string(got) != "secret" {
		t.Fatalf("Get() = %q, %v", got, err)
	}
	if err := v.Delete("account"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get("account"); err != ErrNotFound {
		t.Fatalf("missing key error = %v", err)
	}
	if err := v.Delete("account"); err != nil {
		t.Fatal(err)
	}
}
