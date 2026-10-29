// Package secrets 在宿主机保存凭据。
// 负责人：研发 B。Resolve 只注入 Gate；编排层仅获得 AccountCatalog.Lookup。
package secrets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
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
}

// Store 是凭据库，同时实现 SecretStore 与 AccountCatalog。
type Store struct {
	mu       sync.RWMutex
	byID     map[string]*record
	vault    vault
	metaPath string
}

// NewStore 创建纯内存凭据库（单测默认）。
func NewStore() *Store {
	return &Store{byID: make(map[string]*record), vault: newMemoryVault()}
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
		BasePath:  strings.TrimRight(strings.TrimSpace(info.BasePath), "/"),
		Model:     strings.TrimSpace(info.Model),
		KeyHint:   keyHint(key),
		Tools:     append([]string(nil), info.Tools...),
		Agents:    append([]string(nil), info.Agents...),
	}
	keyCopy := append([]byte(nil), key...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := s.vault.Set(id, keyCopy); err != nil {
		return "", err
	}
	s.byID[id] = &record{meta: meta}
	if err := s.saveMetaLocked(); err != nil {
		_ = s.vault.Delete(id)
		delete(s.byID, id)
		return "", err
	}
	return id, nil
}

// keyHint 只保留末四位供 UI「已保存」提示；短于 4 则统一遮罩，永不回传全文。
func keyHint(key []byte) string {
	runes := []rune(strings.TrimSpace(string(key)))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) < 4 {
		return "••••"
	}
	return "…" + string(runes[len(runes)-4:])
}

// Resolve 返回真实凭据副本；仅供 Gate 使用。
func (s *Store) Resolve(_ context.Context, id string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.byID[id]; !ok {
		return nil, ErrNotFound
	}
	return s.vault.Get(id)
}

// Delete 删除账号及其凭据。
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return ErrNotFound
	}
	_ = s.vault.Delete(id)
	delete(s.byID, id)
	return s.saveMetaLocked()
}

// Lookup 返回脱敏元信息，不返回密钥。
func (s *Store) Lookup(_ context.Context, id string) (contracts.AccountInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[id]
	if !ok {
		return contracts.AccountInfo{}, ErrNotFound
	}
	s.ensureKeyHintLocked(rec)
	return rec.meta, nil
}

// List 返回脱敏账号元信息；kind 为空时返回全部账号。
func (s *Store) List(_ context.Context, kind string) []contracts.AccountInfo {
	want := strings.TrimSpace(kind)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]contracts.AccountInfo, 0, len(s.byID))
	for _, rec := range s.byID {
		if want == "" || rec.meta.Kind == want {
			s.ensureKeyHintLocked(rec)
			out = append(out, rec.meta)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// 旧元数据无 KeyHint 时从 vault 回填末四位并落盘；仍不向调用方返回全文。
func (s *Store) ensureKeyHintLocked(rec *record) {
	if rec == nil || rec.meta.KeyHint != "" {
		return
	}
	key, err := s.vault.Get(rec.meta.ID)
	if err != nil || len(key) == 0 {
		return
	}
	rec.meta.KeyHint = keyHint(key)
	_ = s.saveMetaLocked()
}

// Clear 在应用退出时清空全部凭据。
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.byID {
		_ = s.vault.Delete(id)
	}
	s.byID = make(map[string]*record)
	_ = s.saveMetaLocked()
}

// Count 供诊断使用，不向 UI 暴露。
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}
