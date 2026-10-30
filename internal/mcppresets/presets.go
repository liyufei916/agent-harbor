package mcppresets

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed scripts/*.py
var scriptFS embed.FS

// Info is a UI-safe preset descriptor (no secrets).
type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"` // stdio | http
	NeedsAPIKey bool   `json:"needsApiKey"`
	Origin      string `json:"origin,omitempty"`
}

// Launch is host-side connect material for a preset.
type Launch struct {
	Info
	Transport string            `json:"transport,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ProfileID string            `json:"profileId,omitempty"`
}

// Catalog returns the first-batch Harbor presets.
func Catalog() []Info {
	return []Info{
		{
			ID:          "harbor-time",
			Name:        "本机时间",
			Description: "只读：返回当前本地时间（宿主 stdio，无密钥）。",
			Kind:        "stdio",
			NeedsAPIKey: false,
		},
		{
			ID:          "harbor-filesystem",
			Name:        "演示工作区文件（只读）",
			Description: "只读列举/读取 demos/workspace（路径锁死，宿主 stdio）。",
			Kind:        "stdio",
			NeedsAPIKey: false,
		},
		{
			ID:          "context7",
			Name:        "Context7 文档",
			Description: "远程只读文档检索；须自备 Token（HTTPS）。",
			Kind:        "http",
			NeedsAPIKey: true,
			Origin:      "https://mcp.context7.com",
		},
	}
}

// Prepare materializes stdio scripts under destDir and resolves python3 / workspace root.
func Prepare(id, destDir, workspaceRoot string) (Launch, error) {
	id = strings.TrimSpace(id)
	var info Info
	found := false
	for _, p := range Catalog() {
		if p.ID == id {
			info = p
			found = true
			break
		}
	}
	if !found {
		return Launch{}, fmt.Errorf("unknown preset %q", id)
	}
	out := Launch{Info: info, ProfileID: "custom"}
	switch id {
	case "context7":
		out.Transport = ""
		out.Origin = info.Origin
		out.ProfileID = "custom"
		return out, nil
	case "harbor-time", "harbor-filesystem":
		py, err := resolvePython3()
		if err != nil {
			return Launch{}, err
		}
		if err := os.MkdirAll(destDir, 0700); err != nil {
			return Launch{}, err
		}
		scriptName := "time_server.py"
		if id == "harbor-filesystem" {
			scriptName = "filesystem_server.py"
		}
		scriptPath, err := installScript(destDir, scriptName)
		if err != nil {
			return Launch{}, err
		}
		out.Transport = "stdio"
		out.Command = py
		out.Args = []string{scriptPath}
		if id == "harbor-filesystem" {
			root := strings.TrimSpace(workspaceRoot)
			if root == "" {
				root = defaultWorkspaceRoot()
			}
			if root == "" {
				sample := filepath.Join(destDir, "workspace")
				if err := ensureSampleWorkspace(sample); err != nil {
					return Launch{}, err
				}
				root = sample
			}
			out.Args = append(out.Args, root)
		}
		return out, nil
	default:
		return Launch{}, fmt.Errorf("unhandled preset")
	}
}

func installScript(destDir, name string) (string, error) {
	data, err := scriptFS.ReadFile("scripts/" + name)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, name)
	if err := os.WriteFile(dest, data, 0755); err != nil {
		return "", err
	}
	return dest, nil
}

func resolvePython3() (string, error) {
	candidates := []string{"/usr/bin/python3", "/opt/homebrew/bin/python3", "/usr/local/bin/python3"}
	if p, err := exec.LookPath("python3"); err == nil && filepath.IsAbs(p) {
		candidates = append([]string{p}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("未找到 python3（预置 MCP 需要本机 Python 3）")
}

func ensureSampleWorkspace(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	readme := filepath.Join(dir, "README.txt")
	if _, err := os.Stat(readme); err == nil {
		return nil
	}
	return os.WriteFile(readme, []byte("Harbor preset MCP sample workspace (read-only).\n"), 0600)
}

func defaultWorkspaceRoot() string {
	candidates := []string{
		"demos/workspace",
		filepath.Join("docs", "agent-harbor", "demos", "workspace"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "demos", "workspace"),
			filepath.Join(wd, "..", "demos", "workspace"),
		)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		dir := filepath.Dir(file) // internal/mcppresets
		candidates = append(candidates, filepath.Clean(filepath.Join(dir, "..", "..", "demos", "workspace")))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}
