package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type persistedState struct {
	Prefs contracts.UIPrefs          `json:"prefs"`
	Hosts []contracts.RememberedHost `json:"hosts"`
}

func (a *App) statePath() string {
	if a.dataDir == "" {
		return ""
	}
	return filepath.Join(a.dataDir, "ui-state.json")
}

func (a *App) loadPersistedState() {
	path := a.statePath()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var st persistedState
	if json.Unmarshal(raw, &st) != nil {
		return
	}
	// 旧落盘无 askClearOnStop 时保持默认 true（bool 零值 false 会误关）。
	var probe struct {
		Prefs map[string]json.RawMessage `json:"prefs"`
	}
	askClear := true
	if json.Unmarshal(raw, &probe) == nil {
		if v, ok := probe.Prefs["askClearOnStop"]; ok {
			_ = json.Unmarshal(v, &askClear)
		}
	}
	st.Prefs.AskClearOnStop = askClear
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prefs = normalizeUIPrefs(st.Prefs)
	if a.hosts == nil {
		a.hosts = make(map[string]contracts.RememberedHost)
	}
	for _, h := range st.Hosts {
		if h.Host == "" {
			continue
		}
		a.hosts[hostMapKey(h.Workspace, h.Host)] = h
	}
}

// normalizeUIPrefs 规范化 Theme / ClearOnStopDefault / SecurityTier。
func normalizeUIPrefs(p contracts.UIPrefs) contracts.UIPrefs {
	switch p.Theme {
	case "light", "dark", "system":
		// ok
	default:
		p.Theme = ""
	}
	if p.ClearOnStopDefault != "clear" {
		p.ClearOnStopDefault = "keep"
	}
	switch strings.ToLower(strings.TrimSpace(p.SecurityTier)) {
	case "strict", "standard", "relaxed":
		p.SecurityTier = strings.ToLower(strings.TrimSpace(p.SecurityTier))
	default:
		p.SecurityTier = "standard"
	}
	switch strings.ToLower(strings.TrimSpace(p.IsolationBackend)) {
	case "docker", "p2", "container":
		p.IsolationBackend = "docker"
	case "seatbelt", "p1", "":
		p.IsolationBackend = "seatbelt"
	default:
		p.IsolationBackend = "seatbelt"
	}
	return p
}

func (a *App) savePersistedState() {
	path := a.statePath()
	if path == "" {
		return
	}
	a.mu.Lock()
	st := persistedState{Prefs: a.prefs, Hosts: make([]contracts.RememberedHost, 0, len(a.hosts))}
	for _, h := range a.hosts {
		st.Hosts = append(st.Hosts, h)
	}
	a.mu.Unlock()
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	_ = os.WriteFile(path, raw, 0600)
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	// macOS: ~/Library/Application Support/AgentHarbor
	return filepath.Join(home, "Library", "Application Support", "AgentHarbor")
}
