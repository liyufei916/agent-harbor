package policy

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

// 固定 profileId：首小时确认后不得由前端随意改写 origin。
const (
	ProfileDemoModel  = "demo-model"
	ProfileDemoMCP    = "demo-mcp"
	ProfileDSHModel   = "dsh-model"
	ProfileCodexModel = "codex-model"
	ProfileCustom     = "custom" // 用户自填 Anthropic 兼容 origin（可带 /anthropic 等 BasePath）
)

// CustomModelProfile 由用户 Origin 构造临时探测/策略 profile（不进 DefaultProfiles 表）。
// Origin 允许带 path：https://api.deepseek.com/anthropic → Origin 主机 + BasePath=/anthropic。
func CustomModelProfile(origin, modelName string) (FixedProfile, error) {
	raw := strings.TrimSpace(origin)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return FixedProfile{}, errors.New("policy: invalid custom origin")
	}
	basePath := strings.TrimRight(u.Path, "/")
	if basePath == "/" {
		basePath = ""
	}
	if basePath != "" && !ValidBasePath(basePath) {
		return FixedProfile{}, errors.New("policy: invalid custom base path")
	}
	host := strings.ToLower(u.Hostname())
	cleanOrigin := u.Scheme + "://" + u.Host
	model := defaultModelForHost(host, modelName)
	p := FixedProfile{
		ID:       ProfileCustom,
		Kind:     "model",
		Protocol: "anthropic",
		Model:    model,
		Origin:   cleanOrigin,
		Host:     host,
		BasePath: basePath,
		Paths:    []string{"/v1/messages"},
		TestPath: "/v1/messages",
	}
	if !p.ValidConnection() {
		return FixedProfile{}, errors.New("policy: custom origin not allowed")
	}
	return p, nil
}

// CustomModelProbeCandidates 返回自定义 Origin 的探测候选（BasePath × 模型）。
// 先试用户显式 path/模型，再为 DeepSeek 等补 /anthropic 与厂商模型名。
func CustomModelProbeCandidates(origin, modelName string) ([]FixedProfile, error) {
	base, err := CustomModelProfile(origin, modelName)
	if err != nil {
		return nil, err
	}
	paths := []string{base.BasePath}
	if base.BasePath == "" && strings.Contains(base.Host, "deepseek") {
		paths = append(paths, "/anthropic")
	} else if base.BasePath == "/anthropic" {
		paths = append(paths, "")
	}
	models := []string{base.Model}
	for _, m := range defaultModelCandidates(base.Host, modelName) {
		if m != base.Model {
			models = append(models, m)
		}
	}
	seen := map[string]bool{}
	var out []FixedProfile
	for _, path := range paths {
		for _, model := range models {
			key := path + "\x00" + model
			if seen[key] {
				continue
			}
			seen[key] = true
			cand := base
			cand.BasePath = path
			cand.Model = model
			if cand.ValidConnection() {
				out = append(out, cand)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("policy: no custom probe candidates")
	}
	return out, nil
}

func defaultModelForHost(host, explicit string) string {
	if m := strings.TrimSpace(explicit); m != "" {
		return m
	}
	if m := strings.TrimSpace(os.Getenv("HARBOR_CLAUDE_MODEL")); m != "" {
		return m
	}
	cands := defaultModelCandidates(host, "")
	if len(cands) > 0 {
		return cands[0]
	}
	return "claude-haiku-4-5-20251001"
}

func defaultModelCandidates(host, explicit string) []string {
	var out []string
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m == "" {
			return
		}
		for _, x := range out {
			if x == m {
				return
			}
		}
		out = append(out, m)
	}
	add(explicit)
	add(os.Getenv("HARBOR_CLAUDE_MODEL"))
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "deepseek"):
		add("deepseek-chat")
		add("deepseek-reasoner")
	case strings.Contains(h, "anthropic"):
		add("claude-haiku-4-5-20251001")
	default:
		add("claude-haiku-4-5-20251001")
		add("deepseek-chat")
	}
	return out
}

// FixedProfile 描述受信后端配置中的固定上游。
type FixedProfile struct {
	ID                    string
	Kind                  string // model | mcp
	Origin                string // 例如 https://api.example.com
	BasePath              string // 受信模型路径前缀，不由执行面选择
	Paths                 []string
	Tools                 []string // MCP 工具白名单；模型侧可为空
	Host                  string   // origin 主机名
	Protocol              string   // anthropic 或 streamable-http
	Model                 string   // 固定模型名，与 Kit 一致
	AllowInsecureLoopback bool     // 仅受控测试可显式启用
	TestPath              string   // 连接测试使用的路径
}

// DefaultProfiles 返回两天版固定组合。
// 必须显式登记 origin、模型及工具白名单；没有占位供应商或隐式全工具授权。
func DefaultProfiles() map[string]FixedProfile {
	// demo-model 由 Claude Kit 使用。优先采用 Claude 变量，同时兼容旧 DEMO 变量。
	modelOrigin := envFirst("HARBOR_CLAUDE_MODEL_ORIGIN", "HARBOR_DEMO_MODEL_ORIGIN")
	modelBasePath := envFirst("HARBOR_CLAUDE_MODEL_BASE_PATH", "HARBOR_DEMO_MODEL_BASE_PATH")
	mcpOrigin := envOr("HARBOR_DEMO_MCP_ORIGIN", "")
	tools := []string{}
	for _, tool := range strings.Split(os.Getenv("HARBOR_DEMO_MCP_TOOLS"), ",") {
		if tool = strings.TrimSpace(tool); tool != "" {
			tools = append(tools, tool)
		}
	}
	codexOrigin := envOr("HARBOR_CODEX_MODEL_ORIGIN", "")
	codexBasePath := strings.TrimRight(strings.TrimSpace(os.Getenv("HARBOR_CODEX_MODEL_BASE_PATH")), "/")
	return map[string]FixedProfile{
		ProfileDSHModel:   {ID: ProfileDSHModel, Kind: "model", Protocol: "anthropic", Model: strings.TrimSpace(os.Getenv("HARBOR_DSH_MODEL")), Origin: envOr("HARBOR_DSH_MODEL_ORIGIN", ""), Host: hostOfOrigin(envOr("HARBOR_DSH_MODEL_ORIGIN", "")), BasePath: strings.TrimRight(strings.TrimSpace(os.Getenv("HARBOR_DSH_MODEL_BASE_PATH")), "/"), Paths: []string{"/v1/messages"}, TestPath: "/v1/messages"},
		ProfileCodexModel: {ID: ProfileCodexModel, Kind: "model", Protocol: "openai-responses", Model: strings.TrimSpace(os.Getenv("HARBOR_CODEX_MODEL")), Origin: codexOrigin, Host: hostOfOrigin(codexOrigin), BasePath: codexBasePath, Paths: []string{"/v1/responses"}, TestPath: "/v1/responses"},
		ProfileDemoModel:  {ID: ProfileDemoModel, Kind: "model", Protocol: "anthropic", Model: strings.TrimSpace(os.Getenv("HARBOR_CLAUDE_MODEL")), Origin: modelOrigin, Host: hostOfOrigin(modelOrigin), BasePath: modelBasePath, Paths: []string{"/v1/messages"}, TestPath: "/v1/messages"},
		ProfileDemoMCP:    {ID: ProfileDemoMCP, Kind: "mcp", Protocol: "streamable-http", Origin: mcpOrigin, Host: hostOfOrigin(mcpOrigin), Paths: []string{"/mcp"}, TestPath: "/mcp", Tools: tools},
	}
}

// ValidConnection 拒绝未登记、含用户信息或不安全的生产接入配置。
func (p FixedProfile) ValidConnection() bool {
	if !ValidBasePath(p.BasePath) {
		return false
	}
	u, err := url.Parse(p.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if !p.AllowInsecureLoopback || u.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	if !strings.EqualFold(p.Host, u.Hostname()) || !strings.HasPrefix(p.TestPath, "/") || strings.ContainsAny(p.TestPath, "?#") {
		return false
	}
	if p.Kind == "model" {
		return (p.Protocol == "anthropic" || p.Protocol == "openai-responses") && strings.TrimSpace(p.Model) != "" && len(p.Model) <= 256 && !strings.ContainsAny(p.Model, "\x00\r\n")
	}
	if p.Kind != "mcp" || p.Protocol != "streamable-http" {
		return false
	}
	// 自定义 MCP：工具名在 Connect 探测后写入账号；固定 profile 仍须预登记白名单。
	if p.ID == ProfileCustom {
		return true
	}
	return len(p.Tools) > 0
}

// CustomMCPProfile 由用户 Origin 构造 MCP 临时 profile（不进 DefaultProfiles）。
// 无 path：探测 https://host/mcp；有 path：探测该 path（如 https://host/v1/mcp）。
func CustomMCPProfile(origin string) (FixedProfile, error) {
	cands, err := CustomMCPProbeCandidates(origin)
	if err != nil || len(cands) == 0 {
		return FixedProfile{}, err
	}
	return cands[0], nil
}

// CustomMCPProbeCandidates 返回探测路径候选：用户 path → path+/mcp → /mcp。
func CustomMCPProbeCandidates(origin string) ([]FixedProfile, error) {
	raw := strings.TrimSpace(origin)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("policy: invalid custom mcp origin")
	}
	path := strings.TrimRight(u.Path, "/")
	if path == "/" {
		path = ""
	}
	if path != "" && !ValidBasePath(path) {
		return nil, errors.New("policy: invalid custom mcp path")
	}
	host := strings.ToLower(u.Hostname())
	cleanOrigin := u.Scheme + "://" + u.Host
	var tryPaths []string
	add := func(p string) {
		if p == "" {
			return
		}
		for _, x := range tryPaths {
			if x == p {
				return
			}
		}
		tryPaths = append(tryPaths, p)
	}
	if path != "" {
		add(path)
		if !strings.Contains(strings.ToLower(path), "mcp") {
			add(path + "/mcp")
		}
	}
	add("/mcp")
	loopback := false
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	var out []FixedProfile
	for _, tp := range tryPaths {
		p := FixedProfile{
			ID:                    ProfileCustom,
			Kind:                  "mcp",
			Protocol:              "streamable-http",
			Origin:                cleanOrigin,
			Host:                  host,
			Paths:                 []string{tp},
			TestPath:              tp,
			AllowInsecureLoopback: loopback && strings.EqualFold(u.Scheme, "http"),
		}
		if p.ValidConnection() {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("policy: no custom mcp probe candidates")
	}
	return out, nil
}

// 路径前缀只能来自宿主配置，拒绝编码、路径穿越或 URL 组成部分。
func ValidBasePath(path string) bool {
	if path == "" {
		return true
	}
	if !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.Contains(path, "//") || strings.ContainsAny(path, "%?#\\\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(path, "/")[1:] {
		if part == "." || part == ".." || strings.TrimSpace(part) != part {
			return false
		}
	}
	return true
}

// JoinBasePath 将受信路径前缀与请求路径拼接一次，避免重复添加 provider 前缀。
func JoinBasePath(base, path string) string {
	base = strings.TrimRight(base, "/")
	if base == "" || path == base || strings.HasPrefix(path, base+"/") {
		return path
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return strings.TrimRight(fallback, "/")
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	return ""
}

func hostOfOrigin(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
