package secrets

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestStoreRejectsInvalidInputsAndContext(t *testing.T) {
	s := NewStore()
	base := contracts.AccountInfo{Kind: "model", Name: "name", ProfileID: "profile", Host: "host"}
	for name, info := range map[string]contracts.AccountInfo{
		"id":      {ID: "existing", Kind: "model", Name: "n", ProfileID: "p", Host: "h"},
		"kind":    {Kind: "other", Name: "n", ProfileID: "p", Host: "h"},
		"name":    {Kind: "model", ProfileID: "p", Host: "h"},
		"profile": {Kind: "model", Name: "n", Host: "h"},
		"host":    {Kind: "model", Name: "n", ProfileID: "p"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Put(context.Background(), info, []byte("key")); err == nil {
				t.Fatal("invalid account accepted")
			}
		})
	}
	if _, err := s.Put(context.Background(), base, nil); err != ErrEmptyKey {
		t.Fatalf("empty key error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Put(ctx, base, []byte("key")); err == nil {
		t.Fatal("cancelled Put succeeded")
	}
}
