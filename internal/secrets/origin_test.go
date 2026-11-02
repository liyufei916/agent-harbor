package secrets

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestOriginSurvivesStoreAndMetadataReload(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	s.metaPath = filepath.Join(t.TempDir(), "accounts.json")
	want := "http://127.0.0.1:4000"
	id, err := s.Put(ctx, contracts.AccountInfo{Kind: "model", Name: "本机模型", ProfileID: "custom", Host: "127.0.0.1", Origin: want}, []byte("fixture-only"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := s.Lookup(ctx, id)
	if err != nil || meta.Origin != want {
		t.Fatalf("Origin 未保留：%q，err=%v", meta.Origin, err)
	}
	reloaded := NewStore()
	reloaded.metaPath = s.metaPath
	if err := reloaded.loadMeta(); err != nil {
		t.Fatal(err)
	}
	meta, err = reloaded.Lookup(ctx, id)
	if err != nil || meta.Origin != want {
		t.Fatalf("重载后 Origin 丢失：%q，err=%v", meta.Origin, err)
	}
}
