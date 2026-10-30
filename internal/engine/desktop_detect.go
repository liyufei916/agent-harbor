package engine

import (
	"os"
	"path/filepath"
)

// DockerDesktopInstalled reports whether Docker Desktop.app is present (macOS).
// Used for UI copy: missing Desktop → recommend one-click Harbor Colima install.
func DockerDesktopInstalled() bool {
	candidates := []string{"/Applications/Docker.app"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "Applications", "Docker.app"))
	}
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

// ComponentsPresent reports whether pinned CLI binaries are already on disk / PATH.
func (c *Colima) ComponentsPresent() bool {
	for _, name := range []string{"colima", "docker", "limactl"} {
		if _, err := c.resolveBin(name); err != nil {
			return false
		}
	}
	return true
}
