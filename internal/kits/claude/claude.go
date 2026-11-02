// Package claude 为固定 Claude Code 版本生成无真实凭据的启动规格。
package claude

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

// SupportedVersion 是文档/联调基线；Detect 默认按能力放行，仅当设置
// HARBOR_CLAUDE_REQUIRE_VERSION 时才要求精确匹配。
const SupportedVersion = "2.1.292"

// wireModel 是 Claude Code CLI 校验用的 Anthropic 形模型 ID。
// 账号真实模型名（如 deepseek-chat）由 Gate rewriteJSONModelField 改写出站 JSON。
const wireModel = "claude-sonnet-4-5"

type Kit struct {
	executable, model string
	inspect           func(context.Context, string, []string, []string) (string, error)
}

// launchSpec 是受限探测/启动用的 Command+Args+只读路径。
// Homebrew Claude 是 node 包装的 cli.js，必须用 node 执行，不能直接 sandbox-exec cli.js。
type launchSpec struct {
	Command  string
	Args     []string // 已含脚本路径（若有）；Detect/Build 再追加 CLI 参数
	ReadOnly []string
	Label    string // 用于 reason 展示
}

var _ kits.Kit = (*Kit)(nil)

// New 的 model 来自宿主机固定接入配置，不从任务正文推导或自动切换。
func New(executable, model string) (*Kit, error) {
	if strings.TrimSpace(model) == "" || len(model) > 256 || strings.ContainsAny(model, "\x00\r\n") {
		return nil, kitError("INVALID_ARGUMENT")
	}
	if executable == "" {
		executable = "claude"
	}
	return &Kit{executable: executable, model: model, inspect: inspectExecutable}, nil
}

func kitError(code string) *contracts.AppError {
	message := "当前 Claude Code 未安装或不满足受限适配要求。"
	if code == "INVALID_ARGUMENT" {
		message = "Agent 配置参数不合法。"
	}
	if code == "CANCELLED" {
		message = "操作已取消。"
	}
	return &contracts.AppError{Code: code, Message: message}
}

func (k *Kit) resolve() (string, error) {
	candidates := []string{k.executable}
	base := filepath.Base(k.executable)
	if base == "claude" || k.executable == "claude" {
		candidates = append(candidates,
			"/opt/homebrew/bin/claude",
			"/usr/local/bin/claude",
		)
	}
	seen := map[string]bool{}
	for _, cand := range candidates {
		cand = strings.TrimSpace(cand)
		if cand == "" || seen[cand] {
			continue
		}
		seen[cand] = true
		path := cand
		if !filepath.IsAbs(path) {
			p, err := exec.LookPath(path)
			if err != nil {
				continue
			}
			path = p
		}
		path, err := filepath.EvalSymlinks(path)
		if err != nil || !filepath.IsAbs(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
			continue
		}
		// 原生二进制须可执行；node 脚本允许仅可读（由 node 执行）。
		if !isNodeScript(path) && info.Mode().Perm()&0111 == 0 {
			continue
		}
		return path, nil
	}
	return "", kitError("AGENT_UNSUPPORTED")
}

func isNodeScript(path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".js") {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 128)
	n, _ := f.Read(buf)
	line := string(buf[:n])
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return strings.HasPrefix(line, "#!") && strings.Contains(line, "node")
}

func resolveNode() (string, error) {
	for _, cand := range []string{"node", "/opt/homebrew/bin/node", "/usr/local/bin/node"} {
		path := cand
		if !filepath.IsAbs(path) {
			p, err := exec.LookPath(path)
			if err != nil {
				continue
			}
			path = p
		}
		path, err := filepath.EvalSymlinks(path)
		if err != nil || !filepath.IsAbs(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
			continue
		}
		return path, nil
	}
	return "", kitError("AGENT_UNSUPPORTED")
}

func (k *Kit) resolveLaunch() (launchSpec, error) {
	if kits.DockerGuest() {
		return dockerLaunch(), nil
	}
	script, err := k.resolve()
	if err != nil {
		return launchSpec{}, err
	}
	// macOS 上 Claude 启动会 execFile(/usr/bin/plutil) 读 MDM plist；须进白名单否则 spawn EPERM。
	extras := macOSAgentHelpers()
	if !isNodeScript(script) {
		readOnly := append([]string{script}, extras...)
		return launchSpec{
			Command:  script,
			ReadOnly: readOnly,
			Label:    script,
		}, nil
	}
	node, err := resolveNode()
	if err != nil {
		return launchSpec{}, err
	}
	root := filepath.Dir(script)
	// Homebrew Node：二进制在 Cellar/.../bin/node，并依赖 /opt/homebrew/opt/*/lib 下的 dylib。
	readOnly := []string{node, root}
	readOnly = append(readOnly, brewRuntimeRoots(node)...)
	readOnly = append(readOnly, extras...)
	return launchSpec{
		Command:  node,
		Args:     []string{script},
		ReadOnly: readOnly,
		Label:    script,
	}, nil
}

func macOSAgentHelpers() []string {
	var out []string
	for _, path := range []string{"/usr/bin/plutil"} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
			continue
		}
		out = append(out, path)
	}
	return out
}

// brewRuntimeRoots 返回运行 brew 版 Node 所需的只读前缀。
// opt 下的 dylib 是指向 Cellar 的符号链接，Seatbelt 按真实路径鉴权，因此必须授权对应 Cellar 公式目录。
func brewRuntimeRoots(node string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || seen[p] {
			return
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	if nodeHome := filepath.Dir(filepath.Dir(node)); strings.Contains(nodeHome, string(filepath.Separator)+"Cellar"+string(filepath.Separator)+"node"+string(filepath.Separator)) {
		add(nodeHome)
	}
	// dyld 先打开 opt/... 再落到 Cellar；OpenSSL 还读 etc/openssl@3。
	if strings.HasPrefix(node, "/opt/homebrew/") {
		add("/opt/homebrew/opt")
		add("/opt/homebrew/etc/openssl@3")
		add("/opt/homebrew/etc/ca-certificates")
	} else if strings.HasPrefix(node, "/usr/local/") {
		add("/usr/local/opt")
		add("/usr/local/etc/openssl@3")
		add("/usr/local/etc/ca-certificates")
	}
	cmd := exec.Command("otool", "-L", node)
	raw, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if i := strings.Index(line, " (compatibility"); i >= 0 {
				line = strings.TrimSpace(line[:i])
			}
			if !strings.HasPrefix(line, "/opt/homebrew/") && !strings.HasPrefix(line, "/usr/local/") {
				continue
			}
			resolved, err := filepath.EvalSymlinks(line)
			if err != nil {
				resolved = line
			}
			parts := strings.Split(resolved, string(filepath.Separator))
			for i := 0; i+2 < len(parts); i++ {
				if parts[i] == "Cellar" {
					add(string(filepath.Separator) + filepath.Join(parts[1:i+3]...))
					break
				}
			}
		}
	}
	return out
}

func detectProbeReason(label, step string, err error) string {
	if ae, ok := err.(*contracts.AppError); ok && ae != nil {
		if ae.Code == "ISOLATION_UNAVAILABLE" {
			return "本机隔离不可用，无法在受限环境中检测 Agent（先看侧栏隔离状态 / 重新检测隔离）。"
		}
		if ae.Code == "CANCELLED" {
			return "检测已取消。"
		}
		return "可执行文件 " + label + " 在受限环境执行 " + step + " 未通过（" + ae.Code + "）。"
	}
	return "可执行文件 " + label + " 在受限环境执行 " + step + " 未通过。"
}

func parseClaudeVersion(raw string) (string, bool) {
	line := strings.TrimSpace(strings.Split(raw, "\n")[0])
	const suffix = " (Claude Code)"
	if !strings.HasSuffix(line, suffix) {
		return "", false
	}
	ver := strings.TrimSpace(strings.TrimSuffix(line, suffix))
	if ver == "" || strings.ContainsAny(ver, " \t") {
		return "", false
	}
	return ver, true
}

// requiredHelpFlags 是 Gate 出站启动规格依赖、且须出现在 --help 中的参数。
// 交互 Ink TUI 会硬编码探测 api.anthropic.com，不能用于自定义 ANTHROPIC_BASE_URL/Gate。
var requiredHelpFlags = []string{
	"--print", "--input-format", "stream-json",
	"--strict-mcp-config", "--mcp-config", "--setting-sources",
	"--no-session-persistence", "--permission-mode",
	"--disable-slash-commands", "--tools",
}

func (k *Kit) Detect(ctx context.Context) (kits.KitInfo, error) {
	info := kits.KitInfo{
		Agent: contracts.AgentInfo{
			ID: "claude", Name: "Claude Code", ModelProfileID: "demo-model",
			Reason: "未安装 Claude Code，或 PATH/HARBOR_CLAUDE_BIN 未指向可执行文件。",
		},
		ReadOnlyPaths: []string{},
	}
	if ctx.Err() != nil {
		return info, kitError("CANCELLED")
	}
	if kits.DockerGuest() {
		return k.detectDockerGuest(ctx, info)
	}
	launch, err := k.resolveLaunch()
	if err != nil {
		return info, nil
	}
	versionOut, err := k.inspect(ctx, launch.Command, append(append([]string{}, launch.Args...), "--version"), launch.ReadOnly)
	if err != nil {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		info.Agent.Reason = detectProbeReason(launch.Label, "--version", err)
		return info, nil
	}
	version, ok := parseClaudeVersion(versionOut)
	if !ok {
		info.Agent.Reason = "版本输出无法识别（期望形如「x.y.z (Claude Code)」）。"
		return info, nil
	}
	info.Agent.Version = version
	if want := strings.TrimSpace(os.Getenv("HARBOR_CLAUDE_REQUIRE_VERSION")); want != "" && version != want {
		info.Agent.Reason = "已安装 " + version + "，当前要求 " + want + "（HARBOR_CLAUDE_REQUIRE_VERSION）。"
		return info, nil
	}
	help, err := k.inspect(ctx, launch.Command, append(append([]string{}, launch.Args...), "--help"), launch.ReadOnly)
	if err != nil {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		info.Agent.Reason = "已安装 " + version + "。" + detectProbeReason(launch.Label, "--help", err)
		return info, nil
	}
	var missing []string
	for _, flag := range requiredHelpFlags {
		if !strings.Contains(help, flag) {
			missing = append(missing, flag)
		}
	}
	if len(missing) > 0 {
		info.Agent.Reason = "已安装 " + version + "，缺少受限启动所需参数：" + strings.Join(missing, ", ") + "。"
		return info, nil
	}
	info.Agent.Available, info.Agent.Reason = true, ""
	info.NeedsDescendants = true // PTY 多轮 + Stop 杀树；工具面仍限 --tools Read,Write,Edit
	info.ReadOnlyPaths = append([]string{}, launch.ReadOnly...)
	return info, nil
}

func gateURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host, rawPort, err := net.SplitHostPort(u.Host)
	port, portErr := strconv.Atoi(rawPort)
	return err == nil && portErr == nil && host == "127.0.0.1" && port > 0 && port < 65536 && strconv.Itoa(port) == rawPort
}

func (k *Kit) Build(ctx context.Context, input kits.KitInput) (runtime.LaunchSpec, error) {
	if ctx.Err() != nil {
		return runtime.LaunchSpec{}, kitError("CANCELLED")
	}
	if !utf8.ValidString(input.Task) || strings.TrimSpace(input.Task) == "" || len(input.Task) > 32768 || !filepath.IsAbs(input.PrivateHome) || filepath.Clean(input.PrivateHome) != input.PrivateHome || !gateURL(input.ModelBaseURL) || len(input.Sentinel) < 32 || len(input.Sentinel) > 512 {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	urls, urlErr := kits.MCPGateURLs(input)
	if urlErr != nil {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	for _, r := range input.Sentinel {
		if r < 33 || r > 126 {
			return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
		}
	}
	launch, err := k.resolveLaunch()
	if err != nil {
		return runtime.LaunchSpec{}, err
	}
	mcpPath := filepath.Join(input.PrivateHome, "harbor-mcp.json")
	settingsPath := filepath.Join(input.PrivateHome, "harbor-settings.json")
	servers := map[string]any{}
	permissions := []string{"Read", "Write", "Edit"}
	for i, endpoint := range urls {
		name := kits.MCPServerName(input, i)
		servers[name] = map[string]any{"type": "http", "url": endpoint, "headers": map[string]string{"Authorization": "Bearer " + input.Sentinel}}
		permissions = append(permissions, "mcp__"+name+"__*")
	}
	mcp, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	settings, err := json.Marshal(map[string]any{"disableAllHooks": true, "permissions": map[string]any{"allow": permissions}})
	if err != nil {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	// 勿把账号模型（deepseek-chat 等）直接传给 --model：CLI 会 unrecognized_model。
	model := wireModel
	if m := strings.TrimSpace(k.model); m != "" && strings.HasPrefix(strings.ToLower(m), "claude") {
		model = m
	}
	workspace := strings.TrimSpace(input.Workspace)
	if workspace == "" || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace || strings.ContainsRune(workspace, 0) {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	// Claude Code 常把进程 cwd 切到 $HOME；须显式锁定工作区。
	// 非 Claude 上游（如 GPT via Anthropic 兼容网关）易漏传 tool 参数，须强调 file_path。
	workHint := "Project workspace (absolute): " + workspace +
		". All Read/Write/Edit must use absolute paths under this directory only. " +
		"Before any Read call, derive the absolute path from the user's request; if the path is unknown, ask the user instead of calling a tool. " +
		"When calling Read you MUST send a non-empty JSON object such as {\"file_path\":\"" + workspace + "/README.md\"}; never send {} or omit file_path. " +
		"Write and Edit likewise require a non-empty path/file_path and their other required fields. " +
		"Do not use $HOME or AgentHarbor session private dirs as the project root."
	// --print + stream-json：经 ANTHROPIC_BASE_URL（Gate）出站；工作台 xterm 用 WritePty 多轮。
	// 不用 Ink TUI：其预检固定请求 api.anthropic.com，沙箱内 ENOTFOUND 且绕过 Gate。
	args := append(append([]string{}, launch.Args...),
		"--print", "--output-format", "stream-json", "--verbose", "--input-format", "stream-json",
		"--no-session-persistence", "--strict-mcp-config", "--mcp-config", mcpPath, "--settings", settingsPath,
		"--setting-sources", "", "--permission-mode", "dontAsk", "--disable-slash-commands", "--no-chrome",
		"--tools", "Read,Write,Edit", "--model", model,
		"--add-dir", workspace,
		"--append-system-prompt", workHint,
	)
	return runtime.LaunchSpec{
		Command:     launch.Command,
		Args:        args,
		Env:         map[string]string{"ANTHROPIC_BASE_URL": input.ModelBaseURL, "ANTHROPIC_API_KEY": input.Sentinel, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "DISABLE_AUTOUPDATER": "1", "DISABLE_TELEMETRY": "1", "DISABLE_ERROR_REPORTING": "1"},
		Stdin:       []byte(input.Task),
		Interactive: true,
		EncodeInput: encodeMessage,
		ConfigFiles: []runtime.ConfigFile{{RelativePath: "harbor-mcp.json", Content: mcp}, {RelativePath: "harbor-settings.json", Content: settings}},
		// HOME=工作区，避免 Claude chdir 到私有 home 后 Read 全部 InputValidationError。
		ProjectHome: workspace,
	}, nil
}

// 每个用户轮次是一条 JSONL，正文中的换行和引号不能注入协议记录。
func encodeMessage(data []byte) ([]byte, error) {
	if !utf8.Valid(data) || len(data) == 0 || len(data) > 32768 || strings.ContainsRune(string(data), 0) {
		return nil, kitError("INVALID_ARGUMENT")
	}
	// WritePty 常带尾随 \n；正文去掉行尾空白，避免污染 stream-json content。
	text := strings.TrimRight(string(data), "\r\n")
	if strings.TrimSpace(text) == "" {
		return nil, kitError("INVALID_ARGUMENT")
	}
	message := map[string]any{"type": "user", "message": map[string]string{"role": "user", "content": text}}
	encoded, err := json.Marshal(message)
	if err != nil {
		return nil, kitError("INVALID_ARGUMENT")
	}
	return append(encoded, '\n'), nil
}
