// Package dsh 适配固定版本的官方 DeepSeek Harness，执行面不持有真实凭据。
package dsh

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

const SupportedVersion = "0.2.1-rc.2"
const SupportedNodeVersion = "v26.12.1"

// installProbe 默认同 runtime.ProbeInstallation；单测可注入避免依赖 Seatbelt。
type installProbe func(ctx context.Context, command string, args, readOnly []string) (string, error)

type Kit struct {
	node, installation, model string
	conversation              bool
	probe                     installProbe
}

var _ kits.Kit = (*Kit)(nil)

// DefaultInstallation is the path written by scripts/install-dsh.sh with no custom target.
// Empty when $HOME cannot be resolved.
func DefaultInstallation() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "agent-harbor", "dsh-"+SupportedVersion)
}

// DefaultNode is the bundled Node binary under an install root (Darwin arm64 layout).
func DefaultNode(installation string) string {
	if installation == "" {
		return ""
	}
	return filepath.Join(installation, "node-"+SupportedNodeVersion+"-darwin-arm64", "bin", "node")
}

// ResolvePaths fills empty node/installation from the default install layout so a
// packaged desktop app can Detect without HARBOR_DSH_* in the process environment.
// Explicit non-empty values are kept (custom installs still use env/CLI).
func ResolvePaths(node, installation string) (string, string) {
	installation = strings.TrimSpace(installation)
	node = strings.TrimSpace(node)
	if installation == "" {
		installation = DefaultInstallation()
	}
	if node == "" || node == "node" {
		if bundled := DefaultNode(installation); bundled != "" {
			if _, err := os.Stat(bundled); err == nil {
				return bundled, installation
			}
		}
		if node == "" {
			node = "node"
		}
	}
	return node, installation
}

func New(node, installation, model string) (*Kit, error) {
	if strings.TrimSpace(model) == "" || len(model) > 256 || strings.ContainsAny(model, "\x00\r\n") {
		return nil, kitError("INVALID_ARGUMENT")
	}
	node, installation = ResolvePaths(node, installation)
	return &Kit{node: node, installation: installation, model: model}, nil
}

func kitError(code string) *contracts.AppError {
	return contracts.NewAppError(code, "dsh 未安装、版本不支持或受限配置不合法。", false)
}

// 独立安装根只能包含此运行时，不能使用用户 HOME 或全局 node_modules 作为安装根。
func (k *Kit) paths() (string, string, string, error) {
	if kits.DockerGuest() {
		return dockerPaths()
	}
	node, err := exec.LookPath(k.node)
	if err != nil {
		return "", "", "", kitError("AGENT_UNSUPPORTED")
	}
	node, err = filepath.EvalSymlinks(node)
	if err != nil || !filepath.IsAbs(node) {
		return "", "", "", kitError("AGENT_UNSUPPORTED")
	}
	root, err := filepath.EvalSymlinks(k.installation)
	if err != nil || !filepath.IsAbs(root) {
		return "", "", "", kitError("AGENT_UNSUPPORTED")
	}
	script := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh", "lib", "bin.js")
	manifest := filepath.Join(root, "node_modules", "@deepseek-ai", "dsh", "package.json")
	for _, path := range []string{script, manifest} {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", "", "", kitError("AGENT_UNSUPPORTED")
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			return "", "", "", kitError("AGENT_UNSUPPORTED")
		}
	}
	for _, path := range []string{node, script} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
			return "", "", "", kitError("AGENT_UNSUPPORTED")
		}
	}
	raw, err := os.ReadFile(manifest)
	var meta struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err != nil || json.Unmarshal(raw, &meta) != nil || meta.Name != "@deepseek-ai/dsh" || meta.Version != SupportedVersion {
		return "", "", "", kitError("AGENT_UNSUPPORTED")
	}
	return node, root, script, nil
}

func (k *Kit) probeInstall(ctx context.Context, command string, args, readOnly []string) (string, error) {
	if k != nil && k.probe != nil {
		return k.probe(ctx, command, args, readOnly)
	}
	return runtime.ProbeInstallation(ctx, command, args, readOnly)
}

func (k *Kit) Detect(ctx context.Context) (kits.KitInfo, error) {
	info := kits.KitInfo{Agent: contracts.AgentInfo{ID: "dsh", Name: "DeepSeek dsh", ModelProfileID: "dsh-model", Reason: "未安装固定版本或受限检测未通过。"}}
	if ctx.Err() != nil {
		return info, kitError("CANCELLED")
	}
	if kits.DockerGuest() {
		return k.detectDockerGuest(ctx, info)
	}
	node, root, script, err := k.paths()
	if err != nil {
		return info, nil
	}
	version, err := k.probeInstall(ctx, node, []string{"--version"}, nil)
	if err != nil || strings.TrimSpace(version) != SupportedNodeVersion {
		return info, nil
	}
	text, err := k.probeInstall(ctx, node, []string{script, "--version"}, []string{root})
	if err != nil || strings.TrimSpace(text) != SupportedVersion {
		return info, nil
	}
	text, err = k.probeInstall(ctx, node, []string{script, "--help"}, []string{root})
	if err != nil || !strings.Contains(text, "--profile") || !strings.Contains(text, "headless") {
		return info, nil
	}
	info.Agent.Available = true
	info.Agent.Version = SupportedVersion
	info.Agent.Reason = ""
	info.NeedsDescendants = true // 会话生命周期与杀树；非开放沙箱 Bash
	info.ReadOnlyPaths = []string{node, root}
	return info, nil
}

func gateURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host, port, err := net.SplitHostPort(u.Host)
	n, e := strconv.Atoi(port)
	return err == nil && e == nil && host == "127.0.0.1" && n > 0 && n < 65536 && strconv.Itoa(n) == port
}

func (k *Kit) Build(ctx context.Context, input kits.KitInput) (runtime.LaunchSpec, error) {
	if ctx.Err() != nil {
		return runtime.LaunchSpec{}, kitError("CANCELLED")
	}
	if !utf8.ValidString(input.Task) || strings.TrimSpace(input.Task) == "" || len(input.Task) > 32768 || !filepath.IsAbs(input.PrivateHome) || filepath.Clean(input.PrivateHome) != input.PrivateHome || !gateURL(input.ModelBaseURL) || len(input.Sentinel) < 32 || len(input.Sentinel) > 512 {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	_, urlErr := kits.MCPGateURLs(input)
	if urlErr != nil {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	for _, r := range input.Sentinel {
		if r < 33 || r > 126 {
			return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
		}
	}
	node, root, script, err := k.paths()
	if err != nil {
		return runtime.LaunchSpec{}, err
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = k.model
	}
	files, err := buildProfile(input, model, k.conversation)
	if err != nil {
		return runtime.LaunchSpec{}, err
	}
	launch := runtime.LaunchSpec{Command: node, Args: []string{script, "--profile", "harbor", "--json", "-"}, Env: map[string]string{"DSH_HOME": filepath.Join(input.PrivateHome, "dsh"), "DSH_TELEMETRY_DISABLED": "1", "DEEPSEEK_API_KEY": input.Sentinel}, Stdin: []byte(input.Task), ConfigFiles: files}
	if k.conversation {
		launch.Args = []string{script, "--profile", "harbor"}
		launch.Interactive = true
		launch.EncodeInput = encodeConversationMessage
		launch.ConfigFiles = append(launch.ConfigFiles, conversationFile(root))
	}
	return launch, nil
}
