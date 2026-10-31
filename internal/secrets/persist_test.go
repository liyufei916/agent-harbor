package secrets

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestPersistentStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s, err := NewPersistentStore(filepath.Join(dir, "accounts"))
	if err != nil {
		if strings.Contains(err.Error(), "Keychain") || strings.Contains(err.Error(), "钥匙串") {
			t.Skipf("当前环境不可用 Keychain: %v", err)
		}
		t.Fatal(err)
	}
	id, err := s.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "Persist", ProfileID: "demo-model", Host: "api.example.test",
	}, []byte("sk-persist-secret"))
	if err != nil {
		if strings.Contains(err.Error(), "Keychain") || strings.Contains(err.Error(), "钥匙串") {
			t.Skipf("当前环境不可用 Keychain: %v", err)
		}
		t.Fatal(err)
	}

	s2, err := NewPersistentStore(filepath.Join(dir, "accounts"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := s2.Lookup(ctx, id)
	if err != nil || meta.Name != "Persist" {
		t.Fatalf("lookup=%+v err=%v", meta, err)
	}
	key, err := s2.Resolve(ctx, id)
	if err != nil || string(key) != "sk-persist-secret" {
		t.Fatalf("resolve=%q err=%v", key, err)
	}
	if err := s2.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	s3, err := NewPersistentStore(filepath.Join(dir, "accounts"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s3.Lookup(ctx, id); err != ErrNotFound {
		t.Fatalf("expected gone, got %v", err)
	}
}
