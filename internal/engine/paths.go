package engine

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// ProfileName isolates Harbor's VM from the user's default Colima profile.
	ProfileName = "agent-harbor"
	binDirName  = "bin"
	colimaHome  = "colima"
)

func defaultEngineRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "AgentHarbor", "engine")
	}
	return filepath.Join(home, "Library", "Application Support", "AgentHarbor", "engine")
}

func (c *Colima) root() string {
	if r := strings.TrimSpace(c.Root); r != "" {
		return r
	}
	return defaultEngineRoot()
}

func (c *Colima) binDir() string {
	return filepath.Join(c.root(), binDirName)
}

// colimaHomeDir 放短路径：Lima 在 COLIMA_HOME/_lima/<profile>/ssh.sock.<suffix>
// 总长须 < UNIX_PATH_MAX(104)。Application Support/.../engine/colima 会超限（实测 117）。
func (c *Colima) colimaHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, "Library", "Caches", "AgentHarbor", colimaHome)
	}
	return filepath.Join(c.root(), colimaHome)
}

// DockerSocketPath is the unix socket Colima creates for ProfileName under COLIMA_HOME.
func (c *Colima) DockerSocketPath() string {
	return filepath.Join(c.colimaHomeDir(), ProfileName, "docker.sock")
}

// DockerHost returns DOCKER_HOST for the managed profile (unix://…).
func (c *Colima) DockerHost() string {
	return "unix://" + c.DockerSocketPath()
}

func (c *Colima) resolveBin(name string) (string, error) {
	local := filepath.Join(c.binDir(), name)
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return local, nil
	}
	if c.lookPath != nil {
		return c.lookPath(name)
	}
	return "", errEngine("未找到 " + name + "。请将二进制放入引擎 bin 目录，或安装 colima 与 docker CLI 后重试。")
}
