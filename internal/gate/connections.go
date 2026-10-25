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
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
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
	secrets *secrets.Store
	policy  *policy.Builder
	client  *http.Client
}

// NewConnections 创建连接服务。
func NewConnections(store *secrets.Store, pol *policy.Builder) *Connections {
	return &Connections{
		secrets: store,
		policy:  pol,
		client:  &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone(), Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

// ConnectModel 在 10 秒内测试固定模型上游，成功才保存账号。
func (c *Connections) ConnectModel(ctx context.Context, in contracts.ConnectInput) (contracts.AccountInfo, error) {
	return c.connect(ctx, in, "model")
}

// ConnectMcp 在 10 秒内测试固定 MCP 上游，成功才保存账号。
func (c *Connections) ConnectMcp(ctx context.Context, in contracts.ConnectInput) (contracts.AccountInfo, error) {
	return c.connect(ctx, in, "mcp")
}

func (c *Connections) connect(ctx context.Context, in contracts.ConnectInput, kind string) (contracts.AccountInfo, error) {
	name := strings.TrimSpace(in.Name)
	profileID := strings.TrimSpace(in.ProfileID)
	originIn := strings.TrimSpace(in.Origin)
	key := in.APIKey
	if name == "" || len(key) == 0 {
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	if utf8.RuneCountInString(name) > 100 || len(key) > 16384 {
		return contracts.AccountInfo{}, ErrInvalidConnectInput
	}
	var prof policy.FixedProfile
	if originIn != "" {
		if profileID != "" && profileID != policy.ProfileCustom {
			return contracts.AccountInfo{}, ErrInvalidConnectInput
		}
		parsed, host, err := policy.ParseCustomOrigin(originIn)
		if err != nil {
			return contracts.AccountInfo{}, ErrInvalidConnectInput
		}
		prof = customProfile(kind, parsed, host)
		profileID = policy.ProfileCustom
	} else {
		if profileID == "" {
			return contracts.AccountInfo{}, ErrInvalidConnectInput
		}
		var ok bool
		prof, ok = c.policy.Profile(profileID)
		if !ok || prof.Kind != kind {
			return contracts.AccountInfo{}, ErrInvalidConnectInput
		}
	}

	if !prof.ValidConnection() {
		return contracts.AccountInfo{}, contracts.NewAppError(contracts.ErrInvalidArgument, "后端接入配置尚未完整登记或不受支持。", false)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	defer c.client.CloseIdleConnections()
	if err := c.probe(ctx, prof, key); err != nil {
		if ctx.Err() != nil {
			return contracts.AccountInfo{}, mapProbeErr(ctx, ctx.Err())
		}
		return contracts.AccountInfo{}, err
	}

	meta := contracts.AccountInfo{
		Kind:      kind,
		Name:      name,
		ProfileID: profileID,
		Host:      prof.Host,
	}
	id, err := c.secrets.Put(ctx, meta, []byte(key))
	if err != nil {
		return contracts.AccountInfo{}, err
	}
	meta.ID = id
	return meta, nil
}

func customProfile(kind, origin, host string) policy.FixedProfile {
	prof := policy.FixedProfile{
		ID:       policy.ProfileCustom,
		Kind:     kind,
		Origin:   origin,
		Host:     host,
		Protocol: "anthropic",
		Model:    strings.TrimSpace(os.Getenv("HARBOR_CLAUDE_MODEL")),
		Paths:    []string{"/v1/messages"},
		TestPath: "/v1/messages",
	}
	if prof.Model == "" {
		prof.Model = "claude-sonnet-4-5"
	}
	if kind == "mcp" {
		prof.Protocol = "streamable-http"
		prof.Model = ""
		prof.Paths = []string{"/mcp"}
		prof.TestPath = "/mcp"
		for _, tool := range strings.Split(os.Getenv("HARBOR_DEMO_MCP_TOOLS"), ",") {
			if tool = strings.TrimSpace(tool); tool != "" {
				prof.Tools = append(prof.Tools, tool)
			}
		}
	}
	return prof
}

const mcpProtocolVersion = "2025-03-26"

func (c *Connections) probe(ctx context.Context, prof policy.FixedProfile, apiKey string) error {
	if prof.Kind == "mcp" {
		return c.probeMCP(ctx, prof, apiKey)
	}
	body, _ := json.Marshal(map[string]any{"model": prof.Model, "max_tokens": 1, "messages": []any{map[string]string{"role": "user", "content": "Reply OK"}}})
	resp, err := c.probeRequest(ctx, prof, apiKey, "", http.MethodPost, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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

func (c *Connections) probeRequest(ctx context.Context, prof policy.FixedProfile, key, session, method string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(prof.Origin, "/")+prof.BasePath+prof.TestPath, bytes.NewReader(body))
	if err != nil {
		return nil, ErrUpstreamDown
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if prof.Kind == "model" {
		req.Header.Set("X-Api-Key", key)
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
		resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, ErrUpstreamAuth
		}
		return nil, ErrUpstreamDown
	}
	return resp, nil
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

func (c *Connections) probeMCP(ctx context.Context, prof policy.FixedProfile, key string) error {
	initBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"agent-harbor","version":"0.1.0"}}}`)
	resp, err := c.probeRequest(ctx, prof, key, "", http.MethodPost, initBody)
	if err != nil {
		return err
	}
	session := resp.Header.Get("Mcp-Session-Id")
	if session != "" {
		defer func() {
			// 测试创建的上游会话也必须关闭，失败不触发任何工具调用。
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			if r, e := c.probeRequest(cleanup, prof, key, session, http.MethodDelete, nil); e == nil {
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
	if err != nil || initialized.Protocol != mcpProtocolVersion || len(initialized.Capabilities.Tools) == 0 || string(initialized.Capabilities.Tools) == "null" {
		return ErrUpstreamDown
	}
	resp, err = c.probeRequest(ctx, prof, key, session, http.MethodPost, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return ErrUpstreamDown
	}
	found := map[string]bool{}
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < 20; page++ {
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]string{"cursor": cursor}})
		resp, err = c.probeRequest(ctx, prof, key, session, http.MethodPost, payload)
		if err != nil {
			return err
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
			return err
		}
		for _, tool := range list.Tools {
			found[tool.Name] = true
		}
		if list.NextCursor == "" {
			for _, tool := range prof.Tools {
				if !found[tool] {
					return ErrUpstreamDown
				}
			}
			return ctx.Err()
		}
		if seen[list.NextCursor] {
			return ErrUpstreamDown
		}
		seen[list.NextCursor] = true
		cursor = list.NextCursor
	}
	return ErrUpstreamDown
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
