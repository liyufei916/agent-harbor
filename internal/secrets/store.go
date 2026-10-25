// Package secrets 在宿主机进程内存中保存凭据。
// 负责人：研发 B。Resolve 只注入 Gate；编排层仅获得 AccountCatalog.Lookup。
package secrets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

var (
	ErrNotFound     = errors.New("secrets: not found")
	ErrInvalidInput = errors.New("secrets: invalid input")
	ErrEmptyKey     = errors.New("secrets: api key required")
)

type record struct {
	meta contracts.AccountInfo
	key  []byte
}

// Store 是进程内凭据库，同时实现 SecretStore 与 AccountCatalog。
type Store struct {
	mu   sync.RWMutex
	byID map[string]*record
}

// NewStore 创建空的内存凭据库。
func NewStore() *Store {
	return &Store{byID: make(map[string]*record)}
}

func newID(kind string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return kind + "_" + hex.EncodeToString(b[:])
}

// Put 要求 info.ID 为空；成功后分配 ID 并保存密钥。失败不留记录。
func (s *Store) Put(ctx context.Context, info contracts.AccountInfo, key []byte) (string, error) {
	if info.ID != "" {
		return "", ErrInvalidInput
	}
	kind := strings.TrimSpace(info.Kind)
	if kind != "model" && kind != "mcp" {
		return "", ErrInvalidInput
	}
	name := strings.TrimSpace(info.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", ErrInvalidInput
	}
	profile := strings.TrimSpace(info.ProfileID)
	if profile == "" {
		return "", ErrInvalidInput
	}
	host := strings.TrimSpace(info.Host)
	if host == "" {
		return "", ErrInvalidInput
	}
	if len(key) == 0 || len(key) > 16384 {
		return "", ErrEmptyKey
	}
	id := newID(kind)
	meta := contracts.AccountInfo{
		ID:        id,
		Kind:      kind,
		Name:      name,
		ProfileID: profile,
		Host:      host,
	}
	keyCopy := append([]byte(nil), key...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.byID[id] = &record{meta: meta, key: keyCopy}
	return id, nil
}

// Resolve 返回真实凭据副本；仅供 Gate 使用。
func (s *Store) Resolve(_ context.Context, id string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), rec.key...), nil
}

// Delete 删除账号及其凭据。
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return ErrNotFound
	}
	delete(s.byID, id)
	return nil
}

// List 返回脱敏元信息。kind 为空表示全部，否则只能是 model 或 mcp。
func (s *Store) List(kind string) ([]contracts.AccountInfo, error) {
	kind = strings.TrimSpace(kind)
	if kind != "" && kind != "model" && kind != "mcp" {
		return nil, ErrInvalidInput
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]contracts.AccountInfo, 0)
	for _, rec := range s.byID {
		if kind == "" || rec.meta.Kind == kind {
			out = append(out, rec.meta)
		}
	}
	sortAccounts(out)
	return out, nil
}

func sortAccounts(list []contracts.AccountInfo) {
	for i := 1; i < len(list); i++ {
		item := list[i]
		j := i
		for j > 0 && list[j-1].ID > item.ID {
			list[j] = list[j-1]
			j--
		}
		list[j] = item
	}
}

// Lookup 返回脱敏元信息，不返回密钥。
func (s *Store) Lookup(_ context.Context, id string) (contracts.AccountInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.byID[id]
	if !ok {
		return contracts.AccountInfo{}, ErrNotFound
	}
	return rec.meta, nil
}

// Clear 在应用退出时清空全部凭据。
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = make(map[string]*record)
}

// Count 供诊断使用，不向 UI 暴露。
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}
