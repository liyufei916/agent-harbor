package gate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/mcpstdio"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

// 连接流程错误，由 app.go 映射为公开 AppError。
var (
	ErrInvalidConnectInput = errors.New(contracts.ErrInvalidArgument)
	ErrUpstreamAuth        = errors.New(contracts.ErrUpstreamAuthFailed)
	ErrUpstreamTimeout     = errors.New(contracts.ErrUpstreamTimeout)
	ErrUpstreamDown        = errors.New(contracts.ErrUpstreamUnavailable)
	ErrCancelled           = errors.New(contracts.ErrCancelled)
)

// Connections 实现模型/MCP 连接测试：成功后才写入 Secrets。
type Connections struct {
	secrets     *secrets.Store
	policy      *policy.Builder
	client      *http.Client
	stdioKeeper StdioBridgeKeeper
}

// NewConnections 创建连接服务。
func NewConnections(store *secrets.Store, pol *policy.Builder) *Connections {
	return &Connections{
		secrets: store,
		policy:  pol,
		// 单次请求上限略高于 Connect 的 Codex 探测预算，避免 Client.Timeout 先于 ctx 掐断。
		client: &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone(), Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

// ConnectModel 在 10 秒内测试固定模型上游，成功才保存账号。
func (c *Connections) ConnectModel(ctx context.Context, in contracts.ConnectInput) (contracts.AccountInfo, error) {
	return c.connect(ctx, in, "model")
}

// ConnectMcp 在 10 秒内测试固定 MCP 上游，成功才保存账号。
func (c *Connections) ConnectMcp(ctx context.Context, in contracts.ConnectInput) (contracts.AccountInfo, error) {
	if strings.EqualFold(strings.TrimSpace(in.Transport), "stdio") {
		return c.connectStdioMCP(ctx, in)
	}
	return c.connect(ctx, in, "mcp")
}

// StdioBridgeKeeper 由桌面层实现：探测成功后接管回环桥生命周期。
type StdioBridgeKeeper interface {
	AdoptStdioBridge(accountID string, bridge *mcpstdio.Bridge)
}

// WithStdioKeeper 可选注入；未注入时 Connect stdio 仍探测但立即关闭桥（仅测连通）。
func (c *Connections) WithStdioKeeper(k StdioBridgeKeeper) *Connections {
	c.stdioKeeper = k
	return c
}

// connectStdioMCP：宿主拉起本地 MCP 进程 → 回环 HTTP → 落盘账号（Host=127.0.0.1:port）。
func (c *Connections) connectStdioMCP(ctx context.Context, in contracts.ConnectInput) (contracts.AccountInfo, error) {
	name := strings.TrimSpace(in.Name)
	command := strings.TrimSpace(in.Command)
	if name == "" || command == "" {
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	if utf8.RuneCountInString(name) > 100 {
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	spec := mcpstdio.Spec{Command: command, Args: append([]string(nil), in.Args...), Env: in.Env}
	bridge, err := mcpstdio.Start(ctx, spec)
	if err != nil {
		return contracts.AccountInfo{}, fmt.Errorf("%w: %v", ErrUpstreamDown, err)
	}
	secret, err := mcpstdio.EncodeSecret(spec)
	if err != nil || len(secret) == 0 || len(secret) > 16384 {
		_ = bridge.Close()
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	hostPort := strings.TrimPrefix(bridge.Origin, "http://")
	meta := contracts.AccountInfo{
		Kind:      "mcp",
		Name:      name,
		ProfileID: policy.ProfileCustom,
		Host:      hostPort,
		Tools:     append([]string(nil), bridge.Tools...),
		Transport: "stdio",
		Command:   command,
		AuthMode:  "stdio",
	}
	id, err := c.secrets.Put(ctx, meta, secret)
	if err != nil {
		_ = bridge.Close()
		return contracts.AccountInfo{}, err
	}
	meta.ID = id
	meta.KeyHint = "stdio"
	if c.stdioKeeper != nil {
		c.stdioKeeper.AdoptStdioBridge(id, bridge)
	} else {
		_ = bridge.Close()
	}
	return meta, nil
}

func (c *Connections) connect(ctx context.Context, in contracts.ConnectInput, kind string) (contracts.AccountInfo, error) {
	name := strings.TrimSpace(in.Name)
	profileID := strings.TrimSpace(in.ProfileID)
	origin := strings.TrimSpace(in.Origin)
	key := sanitizeAPIKey(in.APIKey)
	if name == "" || len(key) == 0 {
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	if utf8.RuneCountInString(name) > 100 || len(key) > 16384 {
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	var prof policy.FixedProfile
	var customCandidates []policy.FixedProfile
	modelHint := strings.TrimSpace(in.Model)
	if kind == "model" && (profileID == "" || profileID == policy.ProfileCustom) && origin != "" {
		agentsPreview := policy.NormalizeModelAgents(policy.ProfileCustom, in.Agents)
		onlyCodex := len(agentsPreview) == 1 && agentsPreview[0] == "codex"
		if onlyCodex {
			// 仅 Codex：直接探测 Responses，避免先走 Anthropic 成功后「看起来像保存成功」。
			p, err := policy.CustomResponsesModelProfile(origin, modelHint)
			if err != nil {
				return contracts.AccountInfo{}, ErrInvalidConnectInput
			}
			customCandidates = []policy.FixedProfile{p}
			prof = p
		} else {
			cands, err := policy.CustomModelProbeCandidates(origin, modelHint)
			if err != nil || len(cands) == 0 {
				return contracts.AccountInfo{}, ErrInvalidConnectInput
			}
			customCandidates = cands
			prof = cands[0]
		}
		profileID = policy.ProfileCustom
	} else if kind == "mcp" && origin != "" && (profileID == "" || profileID == policy.ProfileCustom || profileID == policy.ProfileDemoMCP) {
		// 填了服务地址：按自定义 MCP 处理（demo-mcp + 异主机不再误报「输入不合法」）。
		cands, err := policy.CustomMCPProbeCandidates(origin)
		if err != nil || len(cands) == 0 {
			return contracts.AccountInfo{}, contracts.NewAppError(contracts.ErrInvalidArgument, "服务地址须为 https://主机[/路径]，无查询串。", false)
		}
		custom := cands[0]
		if profileID == policy.ProfileDemoMCP {
			if fixed, ok := c.policy.Profile(policy.ProfileDemoMCP); ok && strings.EqualFold(fixed.Host, custom.Host) && fixed.ValidConnection() {
				prof = fixed
			} else {
				customCandidates = cands
				prof = custom
				profileID = policy.ProfileCustom
			}
		} else {
			customCandidates = cands
			prof = custom
			profileID = policy.ProfileCustom
		}
	} else {
		if profileID == "" {
			return contracts.AccountInfo{}, ErrInvalidConnectInput
		}
		var ok bool
		prof, ok = c.policy.Profile(profileID)
		if !ok || prof.Kind != kind {
			return contracts.AccountInfo{}, ErrInvalidConnectInput
		}
		if origin != "" && !strings.EqualFold(hostOfOrigin(origin), prof.Host) {
			return contracts.AccountInfo{}, contracts.NewAppError(contracts.ErrInvalidArgument, "服务地址与 profile 主机不一致；自定义 MCP 请改用服务地址并可将 profileId 留空或填 custom。", false)
		}
		if !prof.ValidConnection() {
			return contracts.AccountInfo{}, contracts.NewAppError(contracts.ErrInvalidArgument, "后端接入配置尚未完整登记或不受支持。", false)
		}
	}
	modelAgents := policy.NormalizeModelAgents(profileID, in.Agents)
	if kind == "model" {
		// 已知不兼容组合先拦，避免 DNS/探测拖延或沙箱无解析时误报「主机校验失败」。
		if err := rejectCodexOnAnthropicOnlyHost(prof.Host, prof.BasePath, origin, modelAgents); err != nil {
			return contracts.AccountInfo{}, err
		}
	}
	if !prof.AllowInsecureLoopback {
		if err := ValidatePublicHostname(prof.Host); err != nil {
			return contracts.AccountInfo{}, contracts.NewAppError(contracts.ErrInvalidArgument, "目标主机未通过出站校验（须为可解析公网地址）。", false)
		}
	}
	skipProbe := in.SkipProbe && kind == "model" && profileID == policy.ProfileCustom
	if skipProbe {
		// 跳过探测时不得勾选需另一协议的 Agent（例如 custom Anthropic + Codex）。
		for _, a := range modelAgents {
			if a == "codex" {
				return contracts.AccountInfo{}, contracts.NewAppError(contracts.ErrInvalidArgument, "跳过探测时不能勾选 Codex（须探测 /v1/responses）。请取消跳过，或只用 Claude。", false)
			}
		}
		meta := contracts.AccountInfo{
			Kind: kind, Name: name, ProfileID: profileID,
			Host: prof.Host, BasePath: prof.BasePath, Model: prof.Model,
			Agents: modelAgents,
		}
		id, err := c.secrets.Put(ctx, meta, []byte(key))
		if err != nil {
			return contracts.AccountInfo{}, err
		}
		meta.ID = id
		return meta, nil
	}
	// Codex Responses 首包偶发较慢（冷启动/代理排队）；Anthropic/MCP 仍 10s。
	probeBudget := 10 * time.Second
	if kind == "model" {
		for _, a := range modelAgents {
			if a == "codex" {
				probeBudget = 25 * time.Second
				break
			}
		}
		if prof.Protocol == "openai-responses" {
			probeBudget = 25 * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(ctx, probeBudget)
	defer cancel()
	defer c.client.CloseIdleConnections()
	if kind == "mcp" {
		// MCP 必须走 probeMCP，才能把 tools/list 写入账号；勿复用模型的 customCandidates 探测环。
		var tools []string
		var err error
		if len(customCandidates) > 0 {
			var last error
			ok := false
			for _, cand := range customCandidates {
				tools, err = c.probeMCP(ctx, cand, key)
				if err == nil {
					prof = cand
					ok = true
					break
				}
				last = err
				if ctx.Err() != nil {
					return contracts.AccountInfo{}, mapProbeErr(ctx, ctx.Err())
				}
			}
			if !ok {
				if last != nil {
					return contracts.AccountInfo{}, last
				}
				return contracts.AccountInfo{}, ErrUpstreamDown
			}
		} else {
			tools, err = c.probeMCP(ctx, prof, key)
			if err != nil {
				if ctx.Err() != nil {
					return contracts.AccountInfo{}, mapProbeErr(ctx, ctx.Err())
				}
				return contracts.AccountInfo{}, err
			}
		}
		if profileID == policy.ProfileCustom {
			// 工具发现为空：仍可保存账号，会话内 call 受空白名单限制（可继续但工具受限）。
			prof.Tools = append([]string(nil), tools...)
		}
	} else if len(customCandidates) > 0 {
		var probeErr error
		sawAuth, sawOther := false, false
		ok := false
		for _, cand := range customCandidates {
			if err := c.probe(ctx, cand, key); err != nil {
				probeErr = err
				if isUpstreamAuthErr(err) {
					sawAuth = true
				} else {
					sawOther = true
				}
				if ctx.Err() != nil {
					return contracts.AccountInfo{}, preferProbeErr(probeErr, ctx.Err())
				}
				continue
			}
			prof = cand
			ok = true
			break
		}
		if !ok {
			if sawAuth && !sawOther {
				if probeErr != nil {
					return contracts.AccountInfo{}, probeErr
				}
				return contracts.AccountInfo{}, ErrUpstreamAuth
			}
			if probeErr != nil {
				return contracts.AccountInfo{}, probeErr
			}
			return contracts.AccountInfo{}, ErrUpstreamDown
		}
	} else if err := c.probe(ctx, prof, key); err != nil {
		if ctx.Err() != nil {
			return contracts.AccountInfo{}, preferProbeErr(err, ctx.Err())
		}
		return contracts.AccountInfo{}, err
	}

	var protocols []string
	if kind == "model" {
		if prof.Protocol == "openai-responses" {
			protocols = append(protocols, "openai-responses")
		}
		if prof.Protocol == "anthropic" {
			protocols = append(protocols, "anthropic")
		}
		extra, err := c.probeExtraAgentProtocols(ctx, prof, key, modelAgents)
		if err != nil {
			if ctx.Err() != nil {
				return contracts.AccountInfo{}, mapProbeErr(ctx, ctx.Err())
			}
			return contracts.AccountInfo{}, err
		}
		protocols = append(protocols, extra...)
	}
	meta := contracts.AccountInfo{
		Kind:      kind,
		Name:      name,
		ProfileID: profileID,
		Host:      prof.Host,
		BasePath:  prof.BasePath,
		Model:     prof.Model,
	}
	if kind == "model" {
		meta.Agents = append([]string(nil), modelAgents...)
		meta.Protocols = uniqStrings(protocols)
	}
	if kind == "mcp" && profileID == policy.ProfileCustom {
		meta.Tools = append([]string(nil), prof.Tools...)
		if len(prof.Paths) == 1 && prof.Paths[0] != "/mcp" {
			meta.BasePath = prof.Paths[0]
		}
		if len(meta.Tools) == 0 {
			meta.OfflineHint = "可继续但工具受限：未发现可用工具，会话内 tools/call 将全部拒绝"
		}
	}
	id, err := c.secrets.Put(ctx, meta, []byte(key))
	if err != nil {
		return contracts.AccountInfo{}, err
	}
	meta.ID = id
	return meta, nil
}

// probeExtraAgentProtocols：多 Agent 勾选时补测协议，返回本次额外成功的 protocol 名。
func (c *Connections) probeExtraAgentProtocols(ctx context.Context, primary policy.FixedProfile, key string, agents []string) ([]string, error) {
	needAnthropic, needResponses := false, false
	for _, a := range agents {
		if a == "claude" || a == "dsh" {
			needAnthropic = true
		}
		if a == "codex" {
			needResponses = true
		}
	}
	var extra []string
	if needResponses && primary.Protocol != "openai-responses" {
		p := policy.FixedProfile{
			ID: policy.ProfileCodexModel, Kind: "model", Protocol: "openai-responses",
			Model: primary.Model, Origin: primary.Origin, Host: primary.Host,
			Paths: []string{"/v1/responses"}, TestPath: "/v1/responses",
			AllowInsecureLoopback: primary.AllowInsecureLoopback,
		}
		if strings.TrimSpace(p.Model) == "" {
			p.Model = "gpt-5-codex"
		}
		if err := c.probe(ctx, p, key); err != nil {
			// probe 已 wrapResponsesProbeErr；若仍是哨兵错误再补一句。
			if ae, ok := err.(*contracts.AppError); ok && ae != nil {
				return nil, ae
			}
			return nil, contracts.NewAppError(contracts.ErrUpstreamUnavailable, "已勾选 Codex，但该地址未通过 Responses（/v1/responses）探测。api.deepseek.com 等仅 Anthropic 的地址不能配 Codex；请改用「OpenAI Codex」或双协议网关。", true)
		}
		extra = append(extra, "openai-responses")
	}
	if needAnthropic && primary.Protocol != "anthropic" {
		origin := primary.Origin
		if bp := strings.TrimRight(strings.TrimSpace(primary.BasePath), "/"); bp != "" {
			origin = strings.TrimRight(primary.Origin, "/") + bp
		}
		p, err := policy.CustomModelProfile(origin, primary.Model)
		if err != nil {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "已勾选 Claude/dsh，但无法从该地址构造 Anthropic 探测配置。", false)
		}
		p.AllowInsecureLoopback = primary.AllowInsecureLoopback
		if err := c.probe(ctx, p, key); err != nil {
			return nil, contracts.NewAppError(contracts.ErrUpstreamUnavailable, "已勾选 Claude/dsh，但该地址未通过 Anthropic Messages 探测。多 Agent 共用须网关同时提供两种协议。", true)
		}
		extra = append(extra, "anthropic")
	}
	return extra, nil
}

func uniqStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// 客户端优先提出的版本；接受列表覆盖评审要求的邻近修订。
const mcpProtocolVersion = "2025-03-26"

// mcpAcceptedProtocolVersions 钉扎邻近修订；未知但形如 YYYY-MM-DD 的版本走宽松协商。
var mcpAcceptedProtocolVersions = []string{
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

func mcpProtocolAccepted(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		// 上游未回版本时按客户端提出版本协商成功。
		return true
	}
	for _, a := range mcpAcceptedProtocolVersions {
		if a == v {
			return true
		}
	}
	// 宽松：符合 MCP 日期版式的修订（避免私有方言硬拒；工具面仍受白名单约束）。
	if len(v) == 10 && v[4] == '-' && v[7] == '-' {
		for i, c := range v {
			if i == 4 || i == 7 {
				continue
			}
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func (c *Connections) probe(ctx context.Context, prof policy.FixedProfile, apiKey string) error {
	if prof.Kind == "mcp" {
		_, err := c.probeMCP(ctx, prof, apiKey)
		return err
	}
	if prof.Protocol == "openai-responses" {
		err := c.probeOpenAIResponses(ctx, prof, apiKey)
		return wrapResponsesProbeErr(err)
	}
	// 自定义：先 GET /v1/models 验 Key（与模型名无关），再 POST /v1/messages。
	if prof.ID == policy.ProfileCustom {
		if err := c.probeModelsList(ctx, prof, apiKey); err != nil {
			return annotateAuthKeyMeta(err, apiKey)
		}
	}
	body, _ := json.Marshal(map[string]any{"model": prof.Model, "max_tokens": 1, "messages": []any{map[string]string{"role": "user", "content": "Reply OK"}}})
	modes := []modelAuthMode{modelAuthBoth}
	if prof.ID == policy.ProfileCustom {
		modes = []modelAuthMode{modelAuthBearer, modelAuthXAPIKey, modelAuthBoth}
	}
	var last error
	for _, mode := range modes {
		resp, err := c.probeRequest(ctx, prof, apiKey, "", http.MethodPost, body, mode)
		if err != nil {
			last = annotateAuthKeyMeta(err, apiKey)
			if isUpstreamAuthErr(err) {
				continue
			}
			return last
		}
		err = readAnthropicProbeOK(ctx, resp)
		resp.Body.Close()
		if err == nil {
			return nil
		}
		last = annotateAuthKeyMeta(err, apiKey)
		if !isUpstreamAuthErr(err) {
			return last
		}
	}
	if last != nil {
		return last
	}
	return annotateAuthKeyMeta(ErrUpstreamAuth, apiKey)
}

func (c *Connections) probeOpenAIResponses(ctx context.Context, prof policy.FixedProfile, apiKey string) error {
	// 强制非流式，避免代理以 SSE 挂起直到 Client/ctx 超时 → 笼统 UPSTREAM_TIMEOUT。
	body, _ := json.Marshal(map[string]any{
		"model":             prof.Model,
		"input":             "Reply OK",
		"max_output_tokens": 8,
		"store":             false,
		"stream":            false,
	})
	resp, err := c.probeRequestJSON(ctx, prof, apiKey, http.MethodPost, body, modelAuthBearer)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	raw, err := readProbeBody(resp)
	if err != nil || json.Unmarshal(raw, &result) != nil || result.ID == "" || result.Object != "response" {
		return ErrUpstreamDown
	}
	return ctx.Err()
}

// preferProbeErr：超时时常有更具体的 AppError（如 Responses 文案），优先返回它。
func preferProbeErr(probeErr, ctxErr error) error {
	if probeErr != nil {
		var ae *contracts.AppError
		if errors.As(probeErr, &ae) && ae != nil && ae.Code != "" {
			return ae
		}
		if errors.Is(probeErr, ErrUpstreamTimeout) || errors.Is(probeErr, context.DeadlineExceeded) {
			return mapProbeErr(context.Background(), probeErr)
		}
	}
	return mapProbeErr(context.Background(), ctxErr)
}

// rejectCodexOnAnthropicOnlyHost：已知仅 Anthropic 的地址禁止勾选 Codex（勿等探测误判）。
// 例：api.deepseek.com 对 /v1/responses 也会回 401（路由存在），不能当作 Codex 可用。
func rejectCodexOnAnthropicOnlyHost(host, basePath, origin string, agents []string) error {
	wantsCodex := false
	for _, a := range agents {
		if strings.TrimSpace(a) == "codex" {
			wantsCodex = true
			break
		}
	}
	if !wantsCodex {
		return nil
	}
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		h = strings.ToLower(hostOfOrigin(origin))
	}
	bp := strings.ToLower(strings.TrimRight(strings.TrimSpace(basePath), "/"))
	o := strings.ToLower(strings.TrimSpace(origin))
	if bp == "/anthropic" || strings.Contains(o, "/anthropic") || strings.Contains(h, "deepseek") {
		return contracts.NewAppError(contracts.ErrInvalidArgument, "Codex 不能使用 DeepSeek 或 …/anthropic 地址（仅 Anthropic Messages）。请改用「OpenAI Codex」快捷项，或去掉 Codex 勾选后保存给 Claude。", false)
	}
	return nil
}

// wrapResponsesProbeErr 把 Responses 探测失败写成可展示文案（勿落成笼统「上游不可用」）。
func wrapResponsesProbeErr(err error) error {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*contracts.AppError); ok && ae != nil {
		return ae
	}
	if isUpstreamAuthErr(err) {
		return contracts.NewAppError(contracts.ErrUpstreamAuthFailed, "OpenAI Responses（/v1/responses）认证失败：请检查 API Key 是否属于该 Origin。", false)
	}
	if errors.Is(err, ErrUpstreamTimeout) || errors.Is(err, context.DeadlineExceeded) {
		return contracts.NewAppError(contracts.ErrUpstreamTimeout, "OpenAI Responses（/v1/responses）探测超时。请确认模型名（如 gpt-5.6-sol）与网关支持 Responses；七牛冷启动时可稍后重试。", true)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrCancelled) {
		return err
	}
	return contracts.NewAppError(contracts.ErrUpstreamUnavailable, "OpenAI Responses（/v1/responses）探测失败。Codex 不能使用 DeepSeek/仅 Anthropic 地址；请用「OpenAI Codex」并 export HARBOR_CODEX_MODEL_ORIGIN，或改用双协议网关。", true)
}

// probeModelsList 用 GET /v1/models 验证 Key；多认证头轮试（LiteLLM 部署差异大）。
func (c *Connections) probeModelsList(ctx context.Context, prof policy.FixedProfile, key string) error {
	u := strings.TrimRight(prof.Origin, "/") + prof.BasePath + "/v1/models"
	type hdr struct{ k, v string }
	attempts := [][]hdr{
		{{"Authorization", "Bearer " + key}},
		{{"Authorization", key}},
		{{"api-key", key}},
		{{"X-Api-Key", key}},
		{{"Authorization", "Bearer " + key}, {"X-Api-Key", key}},
	}
	var last error
	for _, hs := range attempts {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return ErrUpstreamDown
		}
		req.Header.Set("Accept", "application/json")
		for _, h := range hs {
			req.Header.Set(h.k, h.v)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return mapProbeErr(ctx, err)
		}
		raw, _ := readProbeBody(resp)
		code := resp.StatusCode
		resp.Body.Close()
		if code >= 200 && code < 300 {
			return nil
		}
		if code == 401 || code == 403 {
			last = authProbeError(prof.Host, code, raw)
			continue
		}
		// 404/5xx：该路径可能未开 models 列表，不据此判 Key 无效，交给后续 /v1/messages。
		return nil
	}
	if last != nil {
		return last
	}
	return ErrUpstreamAuth
}

func readAnthropicProbeOK(ctx context.Context, resp *http.Response) error {
	var result struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	raw, err := readProbeBody(resp)
	if err != nil {
		return mapProbeErr(ctx, err)
	}
	if json.Unmarshal(raw, &result) != nil || result.Type != "message" || len(result.Content) == 0 || result.Content[0].Type != "text" || result.Content[0].Text == "" {
		return ErrUpstreamDown
	}
	return ctx.Err()
}

type modelAuthMode int

const (
	modelAuthBoth modelAuthMode = iota
	modelAuthBearer
	modelAuthXAPIKey
)

func (c *Connections) probeRequest(ctx context.Context, prof policy.FixedProfile, key, session, method string, body []byte, authMode modelAuthMode) (*http.Response, error) {
	return c.probeRequestAccept(ctx, prof, key, session, method, body, authMode, "application/json, text/event-stream")
}

func (c *Connections) probeRequestJSON(ctx context.Context, prof policy.FixedProfile, key, method string, body []byte, authMode modelAuthMode) (*http.Response, error) {
	return c.probeRequestAccept(ctx, prof, key, "", method, body, authMode, "application/json")
}

func (c *Connections) probeRequestAccept(ctx context.Context, prof policy.FixedProfile, key, session, method string, body []byte, authMode modelAuthMode, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(prof.Origin, "/")+prof.BasePath+prof.TestPath, bytes.NewReader(body))
	if err != nil {
		return nil, ErrUpstreamDown
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	if prof.Kind == "model" {
		switch authMode {
		case modelAuthBearer:
			req.Header.Set("Authorization", "Bearer "+key)
		case modelAuthXAPIKey:
			req.Header.Set("X-Api-Key", key)
		default:
			req.Header.Set("X-Api-Key", key)
			req.Header.Set("Authorization", "Bearer "+key)
		}
		req.Header.Set("Anthropic-Version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, mapProbeErr(ctx, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := readProbeBody(resp)
		resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, authProbeError(prof.Host, resp.StatusCode, raw)
		}
		return nil, ErrUpstreamDown
	}
	return resp, nil
}

func sanitizeAPIKey(raw string) string {
	k := strings.TrimSpace(raw)
	k = strings.Trim(k, "\"'")
	k = strings.Map(func(r rune) rune {
		switch r {
		case '\uFEFF', '\u200B', '\u200C', '\u200D', '\u00A0':
			return -1
		}
		if r < 32 {
			return -1
		}
		return r
	}, k)
	return strings.TrimSpace(k)
}

func isUpstreamAuthErr(err error) bool {
	if errors.Is(err, ErrUpstreamAuth) {
		return true
	}
	var ae *contracts.AppError
	return errors.As(err, &ae) && ae != nil && ae.Code == contracts.ErrUpstreamAuthFailed
}

// authProbeError 把上游 401/403 正文收成固定中文提示，不回传 Key/哈希等敏感片段。
func authProbeError(host string, status int, raw []byte) error {
	low := strings.ToLower(string(raw))
	host = strings.ToLower(host)
	msg := "上游认证失败"
	switch {
	case strings.Contains(low, "expected to start with 'sk-'") || strings.Contains(low, `expected to start with "sk-"`):
		msg = "上游要求 API Key 以 sk- 开头（LiteLLM 虚拟 Key）"
	case strings.Contains(low, "invalid proxy server token") || strings.Contains(low, "token_not_found") || strings.Contains(low, "litellm_verification"):
		msg = "上游拒绝了该 Key（LiteLLM 未登记或已失效）。请在签发方确认 Key 仍有效"
	case strings.Contains(host, "qnaigc") && (strings.Contains(low, "invalid api key") || strings.Contains(low, "authentication_error")):
		msg = "上游认证失败：API Key 无效。api.qnaigc.com 与 llmapi.qiniu.io 不是同一套 Key"
	case strings.Contains(low, "invalid api key") || strings.Contains(low, "invalid x-api-key") || strings.Contains(low, "authentication_error"):
		msg = "上游认证失败：API Key 无效"
	case len(bytes.TrimSpace(raw)) == 0:
		msg = "上游认证失败（无错误正文）"
	}
	if status > 0 {
		msg = msg + " HTTP " + strconv.Itoa(status)
	}
	return contracts.NewAppError(contracts.ErrUpstreamAuthFailed, msg, false)
}

// annotateAuthKeyMeta 在认证错误上附加 Key 长度/前缀提示，便于确认桌面是否传丢，不包含密钥本身。
func annotateAuthKeyMeta(err error, key string) error {
	if err == nil || !isUpstreamAuthErr(err) {
		return err
	}
	meta := "已提交 " + strconv.Itoa(len(key)) + " 字节"
	if strings.HasPrefix(key, "sk-") {
		meta += "，前缀 sk-"
	} else {
		meta += "，前缀非 sk-"
	}
	var ae *contracts.AppError
	if errors.As(err, &ae) && ae != nil {
		msg := ae.Message
		if !strings.Contains(msg, "已提交") {
			msg = msg + "（" + meta + "）"
		}
		return contracts.NewAppError(ae.Code, msg, ae.Retryable)
	}
	return contracts.NewAppError(contracts.ErrUpstreamAuthFailed, "上游认证失败（"+meta+"）", false)
}

func readProbeBody(resp *http.Response) ([]byte, error) {
	const limit = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > limit {
		return nil, ErrUpstreamDown
	}
	return raw, nil
}

func decodeRPC(resp *http.Response, id int, result any) error {
	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	decode := func(raw []byte) bool {
		return json.Unmarshal(raw, &msg) == nil && msg.JSONRPC == "2.0" && msg.ID == id && len(msg.Result) > 0 && string(msg.Result) != "null" && (len(msg.Error) == 0 || string(msg.Error) == "null")
	}
	// SSE 在匹配到该请求的完整事件后即可返回，不等待持久流关闭。
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, (2<<20)+1))
		scanner.Buffer(make([]byte, 4096), 2<<20)
		var data []byte
		matched := false
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if len(data) > 0 && decode(bytes.TrimSpace(data)) {
					matched = true
					break
				}
				data = nil
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(line, "data:")...)
				data = append(data, '\n')
			}
		}
		if !matched {
			return ErrUpstreamDown
		}
	} else {
		raw, err := readProbeBody(resp)
		if err != nil || !decode(raw) {
			return ErrUpstreamDown
		}
	}
	if json.Unmarshal(msg.Result, result) != nil {
		return ErrUpstreamDown
	}
	return nil
}

func (c *Connections) probeMCP(ctx context.Context, prof policy.FixedProfile, key string) ([]string, error) {
	initBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"agent-harbor","version":"0.1.0"}}}`)
	resp, err := c.probeRequest(ctx, prof, key, "", http.MethodPost, initBody, modelAuthBearer)
	if err != nil {
		return nil, err
	}
	session := resp.Header.Get("Mcp-Session-Id")
	if session != "" {
		defer func() {
			// 测试创建的上游会话也必须关闭，失败不触发任何工具调用。
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			if r, e := c.probeRequest(cleanup, prof, key, session, http.MethodDelete, nil, modelAuthBearer); e == nil {
				r.Body.Close()
			}
		}()
	}
	var initialized struct {
		Protocol     string `json:"protocolVersion"`
		Capabilities struct {
			Tools json.RawMessage `json:"tools"`
		} `json:"capabilities"`
	}
	err = decodeRPC(resp, 1, &initialized)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if !mcpProtocolAccepted(initialized.Protocol) {
		return nil, ErrUpstreamDown
	}
	toolsCapOK := len(initialized.Capabilities.Tools) > 0 && string(initialized.Capabilities.Tools) != "null"
	resp, err = c.probeRequest(ctx, prof, key, session, http.MethodPost, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`), modelAuthBearer)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return nil, ErrUpstreamDown
	}
	// 无 tools 能力：可继续连接但工具受限（空发现集）；固定 profile 仍须登记工具齐全。
	if !toolsCapOK {
		if len(prof.Tools) > 0 {
			return nil, ErrUpstreamDown
		}
		return nil, nil
	}
	found := map[string]bool{}
	var discovered []string
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < 20; page++ {
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]string{"cursor": cursor}})
		resp, err = c.probeRequest(ctx, prof, key, session, http.MethodPost, payload, modelAuthBearer)
		if err != nil {
			// 工具发现失败：自定义可降级为空列表；固定 profile 仍硬失败。
			if len(prof.Tools) > 0 {
				return nil, err
			}
			return nil, nil
		}
		var list struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		err = decodeRPC(resp, 2, &list)
		resp.Body.Close()
		if err != nil {
			if len(prof.Tools) > 0 {
				return nil, err
			}
			return discovered, nil
		}
		for _, tool := range list.Tools {
			if tool.Name == "" || found[tool.Name] {
				continue
			}
			found[tool.Name] = true
			discovered = append(discovered, tool.Name)
		}
		if list.NextCursor == "" {
			for _, tool := range prof.Tools {
				if !found[tool] {
					return nil, ErrUpstreamDown
				}
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return discovered, nil
		}
		if seen[list.NextCursor] {
			if len(prof.Tools) > 0 {
				return nil, ErrUpstreamDown
			}
			return discovered, nil
		}
		seen[list.NextCursor] = true
		cursor = list.NextCursor
	}
	if len(prof.Tools) > 0 {
		return nil, ErrUpstreamDown
	}
	return discovered, nil
}

func mapProbeErr(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) {
		return ErrCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return ErrUpstreamTimeout
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "timeout") {
		return ErrUpstreamTimeout
	}
	return fmt.Errorf("%w: %v", ErrUpstreamDown, err)
}
