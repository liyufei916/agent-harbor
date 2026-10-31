package secrets

import (
	"context"
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"testing"
)

func TestEnsureKeyHintAndClear(t *testing.T) {
	s := NewStore()
	id, err := s.Put(context.Background(), contracts.AccountInfo{Kind: "model", Name: "x", ProfileID: "custom", Host: "example.com"}, []byte("sk-abcdefghijklmnopqrstuvwxyz"))
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.byID[id].meta.KeyHint = ""
	s.mu.Unlock()
	info, err := s.Lookup(context.Background(), id)
	if err != nil || info.KeyHint == "" {
		t.Fatalf("lookup=%+v err=%v", info, err)
	}
	s.Clear()
	if s.Count() != 0 {
		t.Fatal("clear failed")
	}
}
func TestStoreListFiltersKinds(t *testing.T) {
	s := NewStore()
	for _, kind := range []string{"model", "mcp"} {
		if _, err := s.Put(context.Background(), contracts.AccountInfo{Kind: kind, Name: kind, ProfileID: "custom", Host: "example.com"}, []byte("key")); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.List(context.Background(), "model")) != 1 || len(s.List(context.Background(), "")) != 2 {
		t.Fatal("list filter")
	}
}

func TestKeyHintResolveDeleteBranches(t *testing.T) {
	for _, in := range []string{"", "abc", "abcd", "  key-1234  "} {
		_ = keyHint([]byte(in))
	}
	s := NewStore()
	if _, err := s.Resolve(context.Background(), "missing"); err == nil {
		t.Fatal("missing resolve")
	}
	if err := s.Delete(context.Background(), "missing"); err == nil {
		t.Fatal("missing delete")
	}
	id, err := s.Put(context.Background(), contracts.AccountInfo{Kind: "mcp", Name: "x", ProfileID: "custom", Host: "h"}, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Resolve(context.Background(), id); err != nil || string(got) != "secret" {
		t.Fatalf("resolve=%q %v", got, err)
	}
	if err := s.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}
