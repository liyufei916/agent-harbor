package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func defaultPrefs() contracts.UIPrefs {
	return contracts.UIPrefs{SidebarCollapsed: false, AutoCollapseWorkbench: true}
}

func (a *App) prefsFile() string {
	if a.prefsPath != "" {
		return a.prefsPath
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "agent-harbor", "ui-prefs.json")
}

func (a *App) loadPrefs() contracts.UIPrefs {
	prefs := defaultPrefs()
	path := a.prefsFile()
	if path == "" {
		return prefs
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return prefs
	}
	var stored struct {
		SidebarCollapsed      bool  `json:"sidebarCollapsed"`
		AutoCollapseWorkbench *bool `json:"autoCollapseWorkbench"`
	}
	if json.Unmarshal(raw, &stored) != nil {
		return prefs
	}
	prefs.SidebarCollapsed = stored.SidebarCollapsed
	if stored.AutoCollapseWorkbench != nil {
		prefs.AutoCollapseWorkbench = *stored.AutoCollapseWorkbench
	}
	return prefs
}

func (a *App) savePrefs(prefs contracts.UIPrefs) error {
	path := a.prefsFile()
	if path == "" {
		return errors.New("prefs path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
