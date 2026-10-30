package runtime

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	dockerGuestHome      = "/home/harbor"
	dockerGuestWorkspace = "/workspace"
	// dockerGateHost is the only host-gateway alias guests should use for Gate proxies.
	// host.docker.internal is intentionally blackholed to container loopback.
	dockerGateHost = "harbor.gate"
)

func dockerROMount(index int, hostPath string) string {
	return "/harbor/ro/" + strconv.Itoa(index) + "/" + filepath.Base(hostPath)
}

func (e *dockerEntry) guestPath(host string) (string, bool) {
	host = filepath.Clean(host)
	if host == e.workspace {
		return dockerGuestWorkspace, true
	}
	if rel, err := filepath.Rel(e.workspace, host); err == nil && filepath.IsLocal(rel) {
		return filepath.Join(dockerGuestWorkspace, rel), true
	}
	if host == e.handle.PrivateHome {
		return dockerGuestHome, true
	}
	if rel, err := filepath.Rel(e.handle.PrivateHome, host); err == nil && filepath.IsLocal(rel) {
		return filepath.Join(dockerGuestHome, rel), true
	}
	if guest, ok := e.mounts[host]; ok {
		return guest, true
	}
	for hostRoot, guestRoot := range e.mounts {
		if host == hostRoot {
			return guestRoot, true
		}
		if rel, err := filepath.Rel(hostRoot, host); err == nil && filepath.IsLocal(rel) {
			return filepath.Join(guestRoot, rel), true
		}
	}
	return "", false
}

// guestCommand maps a host launch command into the guest.
// Bind-mounted Kit tools win; otherwise allow image builtins under /bin and /usr/bin
// (and /usr/local/bin for the Harbor Node image).
func (e *dockerEntry) guestCommand(hostCommand string) (string, error) {
	if strings.TrimSpace(hostCommand) == "" || strings.ContainsRune(hostCommand, 0) || !filepath.IsAbs(hostCommand) {
		return "", runtimeError("AGENT_UNSUPPORTED")
	}
	if resolved, err := filepath.EvalSymlinks(hostCommand); err == nil {
		resolved = filepath.Clean(resolved)
		if guest, ok := e.guestPath(resolved); ok {
			return guest, nil
		}
	}
	clean := filepath.Clean(hostCommand)
	if guest, ok := e.guestPath(clean); ok {
		return guest, nil
	}
	if isDockerImageBin(clean) {
		return clean, nil
	}
	return "", runtimeError("AGENT_UNSUPPORTED")
}

func isDockerImageBin(path string) bool {
	prefixes := []string{"/bin/", "/usr/bin/", "/usr/local/bin/"}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func rewriteDockerArgs(e *dockerEntry, args []string) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
		if filepath.IsAbs(arg) {
			if guest, ok := e.guestPath(filepath.Clean(arg)); ok {
				out[i] = guest
				continue
			}
		}
		out[i] = arg
	}
	return out, nil
}

func rewriteDockerEnv(e *dockerEntry, env []string) ([]string, error) {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
		rewritten, err := rewriteDockerGateValue(e, value)
		if err != nil {
			return nil, err
		}
		value = rewritten
		if filepath.IsAbs(value) {
			if guest, ok := e.guestPath(filepath.Clean(value)); ok {
				value = guest
			}
		}
		switch key {
		case "HOME":
			value = dockerGuestHome
		case "TMPDIR", "CLAUDE_CODE_TMPDIR":
			value = dockerGuestHome + "/tmp"
		case "DSH_HOME":
			value = dockerGuestHome + "/dsh"
		case "CODEX_HOME":
			value = dockerGuestHome + "/codex"
		}
		out = append(out, key+"="+value)
	}
	return out, nil
}

// rewriteDockerConfigFiles maps Gate loopback URLs and host PrivateHome/workspace
// prefixes in Kit config payloads (Codex config.toml, Claude harbor-mcp.json, dsh cordis.patch).
// Env/Args rewrite alone left guests still hitting 127.0.0.1 or macOS Application Support paths.
func rewriteDockerConfigFiles(e *dockerEntry, files []ConfigFile) ([]ConfigFile, error) {
	if len(files) == 0 {
		return files, nil
	}
	out := make([]ConfigFile, len(files))
	for i, file := range files {
		content, err := rewriteDockerGateValue(e, string(file.Content))
		if err != nil {
			return nil, err
		}
		content = rewriteDockerHostPrefixes(e, content)
		out[i] = ConfigFile{RelativePath: file.RelativePath, Content: []byte(content)}
	}
	return out, nil
}

func rewriteDockerHostPrefixes(e *dockerEntry, content string) string {
	if e == nil {
		return content
	}
	home := filepath.Clean(e.handle.PrivateHome)
	ws := filepath.Clean(e.workspace)
	if home != "" && home != "." {
		content = strings.ReplaceAll(content, home, dockerGuestHome)
	}
	if ws != "" && ws != "." && ws != home {
		content = strings.ReplaceAll(content, ws, dockerGuestWorkspace)
	}
	return content
}

func rewriteDockerGateValue(e *dockerEntry, value string) (string, error) {
	if len(e.gateToProxy) > 0 {
		return rewriteGateHostMapped(value, e.gateToProxy), nil
	}
	allowedPorts := map[string]bool{}
	for _, addr := range e.allowedTCP {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return "", runtimeError("INVALID_ARGUMENT")
		}
		allowedPorts[port] = true
	}
	return rewriteGateHost(value, allowedPorts), nil
}

func rewriteGateHost(value string, allowedPorts map[string]bool) string {
	const loop = "127.0.0.1:"
	for {
		i := strings.Index(value, loop)
		if i < 0 {
			return value
		}
		rest := value[i+len(loop):]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		port := rest[:j]
		if port == "" || !allowedPorts[port] {
			return value
		}
		value = value[:i] + dockerGateHost + ":" + port + rest[j:]
	}
}
