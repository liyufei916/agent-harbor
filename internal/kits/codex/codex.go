// Package codex 适配 OpenAI Codex CLI，执行面只接收 Harbor 的代理地址和会话 Sentinel。
package codex

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
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

//go:embed turn_bridge.mjs
var turnBridgeJS []byte

type Kit struct {
	executable string
	model      string
	inspect    func(context.Context, string, []string) (string, error)
}

type launchSpec struct {
	Command  string
	Args     []string
	ReadOnly []string
	Label    string
}

var _ kits.Kit = (*Kit)(nil)

func New(executable, model string) (*Kit, error) {
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 256 || strings.ContainsAny(model, "\x00\r\n") {
		return nil, kitError("INVALID_ARGUMENT")
	}
	if executable == "" {
		executable = "codex"
	}
	return &Kit{executable: executable, model: model, inspect: inspectExecutable}, nil
}

func kitError(code string) *contracts.AppError {
	message := "当前 Codex CLI 未安装或不满足受限适配要求。"
	if code == "INVALID_ARGUMENT" {
		message = "Agent 配置参数不合法。"
	}
	if code == "CANCELLED" {
		message = "操作已取消。"
	}
	return contracts.NewAppError(code, message, false)
}

func (k *Kit) resolveScript() (string, error) {
	path, err := exec.LookPath(k.executable)
	if err != nil {
		return "", kitError("AGENT_UNSUPPORTED")
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) {
		return "", kitError("AGENT_UNSUPPORTED")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return "", kitError("AGENT_UNSUPPORTED")
	}
	// node 脚本可无 +x；原生二进制须可执行。
	if !isNodeScript(path) && info.Mode().Perm()&0111 == 0 {
		return "", kitError("AGENT_UNSUPPORTED")
	}
	return path, nil
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

// brewRuntimeRoots：Seatbelt 按真实路径鉴权，须放行 Node Cellar/opt 依赖。
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

// packageRootForScript：npm 全局安装时脚本在 …/codex/bin/codex.js，
// 平台包在 …/codex/node_modules/@openai/codex-darwin-*；只放行 bin 会让 require 失败并误报 Missing optional dependency。
func packageRootForScript(script string) string {
	dir := filepath.Dir(script)
	if filepath.Base(dir) == "bin" {
		return filepath.Dir(dir)
	}
	return dir
}

// resolveLaunch：Homebrew Codex 是 #!/usr/bin/env node 脚本；沙箱内 env 找不到 node。
// 必须 node + codex.js，与 Claude Kit 同模式；ReadOnly 须覆盖整个 npm 包根（含 optional 原生二进制）。
func (k *Kit) resolveLaunch() (launchSpec, error) {
	script, err := k.resolveScript()
	if err != nil {
		return launchSpec{}, err
	}
	if !isNodeScript(script) {
		return launchSpec{Command: script, ReadOnly: []string{script}, Label: script}, nil
	}
	node, err := resolveNode()
	if err != nil {
		return launchSpec{}, err
	}
	root := packageRootForScript(script)
	readOnly := []string{node, root}
	readOnly = append(readOnly, brewRuntimeRoots(node)...)
	return launchSpec{
		Command:  node,
		Args:     []string{script},
		ReadOnly: readOnly,
		Label:    script,
	}, nil
}

func (k *Kit) Detect(ctx context.Context) (kits.KitInfo, error) {
	info := kits.KitInfo{Agent: contracts.AgentInfo{ID: "codex", Name: "OpenAI Codex", ModelProfileID: "codex-model", Reason: "未安装或受限检测未通过。"}, ReadOnlyPaths: []string{}, NeedsChildProcesses: true, NeedsDescendants: true}
	if ctx.Err() != nil {
		return info, kitError("CANCELLED")
	}
	launch, err := k.resolveLaunch()
	if err != nil {
		return info, nil
	}
	versionArgs := append(append([]string{}, launch.Args...), "--version")
	version, err := k.inspect(ctx, launch.Command, versionArgs)
	if err != nil {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		return info, nil
	}
	version = strings.TrimSpace(version)
	if version == "" || !strings.Contains(strings.ToLower(version), "codex") {
		return info, nil
	}
	helpArgs := append(append([]string{}, launch.Args...), "exec", "--help")
	help, err := k.inspect(ctx, launch.Command, helpArgs)
	if err != nil {
		return info, nil
	}
	for _, flag := range []string{"--json", "--ephemeral", "--skip-git-repo-check"} {
		if !strings.Contains(help, flag) {
			return info, nil
		}
	}
	info.Agent.Available, info.Agent.Version, info.Agent.Reason = true, version, ""
	info.ReadOnlyPaths = append([]string{}, launch.ReadOnly...)
	return info, nil
}

func gateURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host, port, err := net.SplitHostPort(u.Host)
	n, portErr := strconv.Atoi(port)
	return err == nil && portErr == nil && host == "127.0.0.1" && n > 0 && n < 65536 && strconv.Itoa(n) == port
}

func validSentinel(value string) bool {
	if len(value) < 32 || len(value) > 512 {
		return false
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

// encodePrompt：每轮一条 JSON 字符串行，供 turn_bridge 多轮读取（与 Claude EncodeInput 同形）。
func encodePrompt(data []byte) ([]byte, error) {
	if !utf8.Valid(data) || len(data) == 0 || len(data) > 32768 || strings.ContainsRune(string(data), 0) {
		return nil, kitError("INVALID_ARGUMENT")
	}
	text := strings.TrimRight(string(data), "\r\n")
	if strings.TrimSpace(text) == "" {
		return nil, kitError("INVALID_ARGUMENT")
	}
	enc, err := json.Marshal(text)
	if err != nil {
		return nil, kitError("INVALID_ARGUMENT")
	}
	return append(enc, '\n'), nil
}

// Build 使用 Codex 的 Responses provider；MCP 可选（编排只传 MCPURLs，允许 0 个）。
// npm 版 Codex 经 turn_bridge 常驻 node + app-server：WritePty 只发 turn/start，避免每轮冷启动。
func (k *Kit) Build(ctx context.Context, input kits.KitInput) (runtime.LaunchSpec, error) {
	if ctx.Err() != nil {
		return runtime.LaunchSpec{}, kitError("CANCELLED")
	}
	if !utf8.ValidString(input.Task) || strings.TrimSpace(input.Task) == "" || len(input.Task) > 32768 || !filepath.IsAbs(input.PrivateHome) || filepath.Clean(input.PrivateHome) != input.PrivateHome || !gateURL(input.ModelBaseURL) || !validSentinel(input.Sentinel) {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	mcpURLs, urlErr := kits.MCPGateURLs(input)
	if urlErr != nil {
		return runtime.LaunchSpec{}, kitError("INVALID_ARGUMENT")
	}
	launch, err := k.resolveLaunch()
	if err != nil {
		return runtime.LaunchSpec{}, err
	}
	// Codex CLI 把 base_url + "/responses" 拼成请求路径；Gate 白名单是 /v1/responses。
	// 因此写入 …/v1，使实际请求变为 /v1/responses（与 OpenAI Responses 探测路径一致）。
	providerBase := strings.TrimRight(input.ModelBaseURL, "/") + "/v1"
	providerURL := strconv.Quote(providerBase)
	modelName := strings.TrimSpace(input.Model)
	if modelName == "" {
		modelName = k.model
	}
	model := strconv.Quote(modelName)
	sentinel := strconv.Quote("Bearer " + input.Sentinel)
	var b strings.Builder
	// Harbor 外层 Seatbelt 已隔离；Codex 再套 workspace-write 会嵌套 sandbox-exec → EPERM，
	// apply_patch 也会报 read-only sandbox。danger-full-access + bypass 旗标专供「外部已沙箱」。
	fmt.Fprintf(&b, "model = %s\nmodel_provider = \"harbor\"\napproval_policy = \"never\"\nsandbox_mode = \"danger-full-access\"\n\n[model_providers.harbor]\nname = \"Agent Harbor\"\nbase_url = %s\nwire_api = \"responses\"\nenv_key = \"OPENAI_API_KEY\"\n", model, providerURL)
	for i, endpoint := range mcpURLs {
		name := kits.MCPName(i)
		fmt.Fprintf(&b, "\n[mcp_servers.%s]\nurl = %s\nhttp_headers = { Authorization = %s }\n", name, strconv.Quote(endpoint), sentinel)
	}
	configs := []runtime.ConfigFile{{RelativePath: "codex/config.toml", Content: []byte(b.String())}}
	env := map[string]string{"CODEX_HOME": filepath.Join(input.PrivateHome, "codex"), "OPENAI_API_KEY": input.Sentinel}

	// 测试替身 / 原生二进制：保持单轮 exec（无常驻桥）。
	if len(launch.Args) == 0 {
		args := append(append([]string{}, launch.Args...), "exec", "--json", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--model", modelName, "-")
		return runtime.LaunchSpec{
			Command: launch.Command, Args: args, Env: env, Stdin: []byte(input.Task), ConfigFiles: configs,
		}, nil
	}

	// node + turn_bridge：多轮。codex.js 路径在 Label / Args[0]。
	codexScript := launch.Label
	if codexScript == "" && len(launch.Args) > 0 {
		codexScript = launch.Args[0]
	}
	bridgeRel := "codex/harbor-turn.mjs"
	bridgePath := filepath.Join(input.PrivateHome, bridgeRel)
	configs = append(configs, runtime.ConfigFile{RelativePath: bridgeRel, Content: turnBridgeJS})
	args := []string{bridgePath, "--codex", codexScript, "--model", modelName}
	return runtime.LaunchSpec{
		Command:     launch.Command,
		Args:        args,
		Env:         env,
		Stdin:       []byte(input.Task),
		Interactive: true,
		EncodeInput: encodePrompt,
		ConfigFiles: configs,
		ProjectHome: strings.TrimSpace(input.Workspace),
	}, nil
}

func inspectExecutable(ctx context.Context, command string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
