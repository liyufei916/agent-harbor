package secrets

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestPutLookupResolveNoLeak(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	id, err := s.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "Demo", ProfileID: "demo-model", Host: "api.example.test",
	}, []byte("sk-secret-should-not-leak"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := s.Lookup(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID != id || meta.Host != "api.example.test" {
		t.Fatalf("meta=%+v", meta)
	}
	key, err := s.Resolve(ctx, id)
	if err != nil || string(key) != "sk-secret-should-not-leak" {
		t.Fatalf("resolve=%q err=%v", key, err)
	}
}

func TestPutRejectsNonEmptyID(t *testing.T) {
	s := NewStore()
	_, err := s.Put(context.Background(), contracts.AccountInfo{
		ID: "preset", Kind: "model", Name: "a", ProfileID: "demo-model", Host: "h",
	}, []byte("k"))
	if err != ErrInvalidInput {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteThenResolveFails(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	id, err := s.Put(ctx, contracts.AccountInfo{
		Kind: "mcp", Name: "Tool", ProfileID: "demo-mcp", Host: "mcp.example.test",
	}, []byte("token"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, id); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestListFiltersKind(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	if _, err := s.Put(ctx, contracts.AccountInfo{Kind: "model", Name: "M", ProfileID: "demo-model", Host: "m.example"}, []byte("k")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(ctx, contracts.AccountInfo{Kind: "mcp", Name: "T", ProfileID: "demo-mcp", Host: "t.example"}, []byte("k")); err != nil {
		t.Fatal(err)
	}
	all, err := s.List("")
	if err != nil || len(all) != 2 {
		t.Fatalf("all=%v err=%v", all, err)
	}
	models, err := s.List("model")
	if err != nil || len(models) != 1 || models[0].Kind != "model" {
		t.Fatalf("%+v", models)
	}
	if _, err := s.List("other"); err != ErrInvalidInput {
		t.Fatalf("got %v", err)
	}
}

func TestClear(t *testing.T) {
	s := NewStore()
	ctx := context.Background()
	_, _ = s.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "a", ProfileID: "demo-model", Host: "h",
	}, []byte("k"))
	s.Clear()
	if s.Count() != 0 {
		t.Fatal("expected empty")
	}
}
