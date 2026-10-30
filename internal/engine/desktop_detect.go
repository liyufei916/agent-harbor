package engine

import (
	"os"
	"path/filepath"
)

// DockerDesktopInstalled reports whether Docker Desktop appears available (macOS).
// Checks the .app bundle and the Desktop CLI socket (App 可能已装/在跑但路径非标准).
func DockerDesktopInstalled() bool {
	candidates := []string{"/Applications/Docker.app"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "Applications", "Docker.app"))
		sock := filepath.Join(home, ".docker", "run", "docker.sock")
		if st, err := os.Stat(sock); err == nil && !st.IsDir() {
			return true
		}
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
