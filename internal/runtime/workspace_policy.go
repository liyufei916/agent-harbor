package runtime

import (
	"os"
	"path/filepath"
)

func forbiddenWorkspace(path string) bool {
	for _, base := range []string{"/System", "/usr", "/bin", "/sbin", "/etc", "/private/etc", "/var", "/private/var", "/Library", "/opt"} {
		if path == base || within(path, base) || within(base, path) {
			return true
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return true
	}
	home, err = canonicalPath(home)
	if err != nil || path == home || within(path, home) {
		return true
	}
	for _, relative := range []string{".ssh", ".aws", ".config", ".kube", ".claude", ".codex", ".gnupg", ".azure", ".docker", "Library"} {
		if within(filepath.Join(home, relative), path) {
			return true
		}
	}
	return path == "/" || path == "/Users" || path == "/private" || path == "/private/tmp"
}

// 安装目录只读授权不能覆盖宿主配置、系统根或用户 HOME。
func safeInstallationRoot(path string) bool {
	for _, root := range []string{"/", "/Users", "/private", "/private/tmp", "/tmp", "/usr", "/usr/local", "/opt", "/opt/homebrew", "/Library", "/System", "/etc", "/private/etc", "/var", "/private/var"} {
		if path == root {
			return false
		}
	}
	for _, root := range []string{"/System", "/Library", "/private/etc", "/private/var"} {
		if within(root, path) {
			return false
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	home, err = canonicalPath(home)
	if err != nil || within(path, home) {
		return false
	}
	for _, name := range []string{".ssh", ".aws", ".config", ".kube", ".claude", ".codex", ".gnupg", ".azure", ".docker", "Library"} {
		if within(filepath.Join(home, name), path) {
			return false
		}
	}
	return true
}
