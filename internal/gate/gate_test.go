package gate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestSentinelLength(t *testing.T) {
	s, err := newSentinel()
	if err != nil {
		t.Fatal(err)
	}
	tok, ok := parseBearerSentinel("Bearer " + s)
	if !ok || tok != s {
		t.Fatalf("%v %q", ok, tok)
	}
	if len(s) < len(sentinelPrefix)+64 {
		t.Fatalf("too short: %s", s)
	}
}

func TestModelProxyRewriteAndDeny(t *testing.T) {
	var gotAuth string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, "echo "+r.Header.Get("Authorization"))
	}))
	defer origin.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, err := store.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "demo", ProfileID: policy.ProfileDemoModel, Host: "127.0.0.1",
	}, []byte("sk-real-key"))
	if err != nil {
		t.Fatal(err)
	}
	mcpID, err := store.Put(ctx, contracts.AccountInfo{
		Kind: "mcp", Name: "demo-mcp", ProfileID: policy.ProfileDemoMCP, Host: "127.0.0.1",
	}, []byte("mcp-key"))
	if err != nil {
		t.Fatal(err)
	}

	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {
			ID: policy.ProfileDemoModel, Kind: "model", Origin: origin.URL, Paths: []string{"/v1/"}, Host: "127.0.0.1",
		},
		policy.ProfileDemoMCP: {
			ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: origin.URL, Paths: []string{"/mcp"}, Host: "127.0.0.1",
		},
	})
	h := New(store, pol)

	var audits []contracts.AuditData
	bind, err := h.Start(ctx, GateSpec{
		SessionID:      "sess1",
		Policy:         policy.PolicySnapshot{Model: policy.EndpointRule{Origin: origin.URL, Paths: []string{"/v1/"}}, MCP: policy.EndpointRule{Origin: origin.URL, Paths: []string{"/mcp"}}, MCPID: mcpID},
		ModelAccountID: modelID,
		MCPAccountID:   mcpID,
	}, func(a contracts.AuditData) { audits = append(audits, a) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "sess1")

	req, _ := http.NewRequest(http.MethodGet, bind.ModelBaseURL+"/v1/chat", nil)
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if gotAuth != "Bearer sk-real-key" {
		t.Fatalf("upstream auth=%q", gotAuth)
	}
	if strings.Contains(string(body), "sk-real-key") || strings.Contains(string(body), bind.Sentinel) {
		t.Fatalf("response leaked credentials: %q", body)
	}
	if len(audits) == 0 || audits[0].Tool != auditToolModel || audits[0].Result != "allowed" {
		t.Fatalf("audit=%+v", audits)
	}

	req2, _ := http.NewRequest(http.MethodGet, bind.ModelBaseURL+"/v1/chat", nil)
	req2.Header.Set("Authorization", "Bearer wrong")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", resp2.StatusCode)
	}
}

func TestRevokeInvalidatesSentinel(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, _ := store.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "m", ProfileID: policy.ProfileDemoModel, Host: "h",
	}, []byte("k"))
	mcpID, _ := store.Put(ctx, contracts.AccountInfo{
		Kind: "mcp", Name: "t", ProfileID: policy.ProfileDemoMCP, Host: "h",
	}, []byte("k"))
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {ID: policy.ProfileDemoModel, Kind: "model", Origin: origin.URL, Paths: []string{"/"}, Host: "h"},
		policy.ProfileDemoMCP:   {ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: origin.URL, Paths: []string{"/mcp"}, Host: "h"},
	})
	h := New(store, pol)
	bind, err := h.Start(ctx, GateSpec{
		SessionID: "s2",
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: origin.URL, Paths: []string{"/"}},
			MCP:   policy.EndpointRule{Origin: origin.URL, Paths: []string{"/mcp"}},
			MCPID: mcpID,
		},
		ModelAccountID: modelID,
		MCPAccountID:   mcpID,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Revoke(ctx, "s2"); err != nil {
		t.Fatal(err)
	}
	// 监听器关闭后请求应失败
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = client.Get(bind.ModelBaseURL + "/")
	if err == nil {
		t.Fatal("expected connection error after revoke")
	}
}

func TestConnectModelWithLocalUpstream(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyOK := r.Header.Get("X-Api-Key") == "good-key" || r.Header.Get("Authorization") == "Bearer good-key"
		if !keyOK {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"OK"}]}`))
	}))
	defer origin.Close()

	store := secrets.NewStore()
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {
			ID: policy.ProfileDemoModel, Kind: "model", Protocol: "anthropic", Model: "fixture", AllowInsecureLoopback: true, Origin: origin.URL,
			Paths: []string{"/"}, Host: "127.0.0.1", TestPath: "/",
		},
		policy.ProfileDemoMCP: {
			ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: origin.URL,
			Paths: []string{"/mcp"}, Host: "127.0.0.1", TestPath: "/",
		},
	})
	conn := NewConnections(store, pol)
	meta, err := conn.ConnectModel(context.Background(), contracts.ConnectInput{
		ProfileID: policy.ProfileDemoModel, Name: "Demo", APIKey: "good-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" || meta.Kind != "model" {
		t.Fatalf("%+v", meta)
	}
	_, err = conn.ConnectModel(context.Background(), contracts.ConnectInput{
		ProfileID: policy.ProfileDemoModel, Name: "Bad", APIKey: "bad-key",
	})
	if !errors.Is(err, ErrUpstreamAuth) {
		if ae, ok := err.(*contracts.AppError); !ok || ae.Code != contracts.ErrUpstreamAuthFailed {
			t.Fatalf("got %v", err)
		}
	}
}

func TestConnectCustomDeepSeekAnthropicPrefix(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-ds" && r.Header.Get("X-Api-Key") != "sk-ds" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/anthropic/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"}]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/anthropic/v1/messages":
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Model != "deepseek-chat" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"OK"}]}`))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer origin.Close()

	conn := NewConnections(secrets.NewStore(), policy.NewBuilder(map[string]policy.FixedProfile{}))
	prof := policy.FixedProfile{
		ID: policy.ProfileCustom, Kind: "model", Protocol: "anthropic", Model: "deepseek-chat",
		AllowInsecureLoopback: true, Origin: origin.URL, Host: "127.0.0.1",
		BasePath: "/anthropic", Paths: []string{"/v1/messages"}, TestPath: "/v1/messages",
	}
	if err := conn.probe(context.Background(), prof, "sk-ds"); err != nil {
		t.Fatal(err)
	}
	bad := prof
	bad.BasePath = ""
	if err := conn.probe(context.Background(), bad, "sk-ds"); err == nil {
		t.Fatal("expected missing prefix to fail")
	}
}

func TestAuthProbeErrorLiteLLMSKHint(t *testing.T) {
	err := authProbeError("llmapi.qiniu.io", 401, []byte(`{"error":{"message":"LiteLLM Virtual Key expected. Received=****, expected to start with 'sk-'.","type":"auth_error"}}`))
	ae, ok := err.(*contracts.AppError)
	if !ok || ae.Code != contracts.ErrUpstreamAuthFailed || !strings.Contains(ae.Message, "sk-") {
		t.Fatalf("%v", err)
	}
	if strings.Contains(ae.Message, "Received=") {
		t.Fatalf("leaked upstream detail: %s", ae.Message)
	}
	err = authProbeError("llmapi.qiniu.io", 401, []byte(`{"error":{"message":"Authentication Error, Invalid proxy server token passed. Unable to find token in cache or LiteLLM_VerificationTokenTable","type":"token_not_found_in_db"}}`))
	ae, ok = err.(*contracts.AppError)
	if !ok || !strings.Contains(ae.Message, "LiteLLM") {
		t.Fatalf("%v", err)
	}
	err = authProbeError("api.qnaigc.com", 401, []byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`))
	ae, ok = err.(*contracts.AppError)
	if !ok || !strings.Contains(ae.Message, "不是同一套") {
		t.Fatalf("%v", err)
	}
	wrapped := annotateAuthKeyMeta(err, "sk-abc")
	ae, ok = wrapped.(*contracts.AppError)
	if !ok || !strings.Contains(ae.Message, "已提交 6 字节") || !strings.Contains(ae.Message, "前缀 sk-") {
		t.Fatalf("%v", wrapped)
	}
}

func TestConnectModelAcceptsBearerOnlyUpstream(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bearer-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"OK"}]}`))
	}))
	defer origin.Close()
	store := secrets.NewStore()
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {
			ID: policy.ProfileDemoModel, Kind: "model", Protocol: "anthropic", Model: "fixture",
			AllowInsecureLoopback: true, Origin: origin.URL, Paths: []string{"/"}, Host: "127.0.0.1", TestPath: "/",
		},
	})
	meta, err := NewConnections(store, pol).ConnectModel(context.Background(), contracts.ConnectInput{
		ProfileID: policy.ProfileDemoModel, Name: "Bearer", APIKey: "bearer-key",
	})
	if err != nil || meta.ID == "" {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
}

func TestRedactorSplitsSecret(t *testing.T) {
	red := newSecretRedactor([]string{"sk-real-key"})
	var out []byte
	out = append(out, red.Push([]byte("echo sk-rea"))...)
	out = append(out, red.Push([]byte("l-key tail"))...)
	out = append(out, red.Flush()...)
	if strings.Contains(string(out), "sk-real-key") || !strings.Contains(string(out), redactedText) {
		t.Fatalf("out=%q", out)
	}
}

func TestMCPFiltersToolsListAndCall(t *testing.T) {
	var upstreamCalls int
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.Header.Get("Authorization") != "Bearer mcp-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Mcp-Session-Id") != "sess-up" {
			t.Errorf("missing session header")
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "tools/list") {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"list_resources"},{"name":"delete_all"}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`)
	}))
	defer origin.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, err := store.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "m", ProfileID: policy.ProfileDemoModel, Host: "127.0.0.1",
	}, []byte("sk-real-key"))
	if err != nil {
		t.Fatal(err)
	}
	mcpID, err := store.Put(ctx, contracts.AccountInfo{
		Kind: "mcp", Name: "t", ProfileID: policy.ProfileDemoMCP, Host: "127.0.0.1",
	}, []byte("mcp-secret"))
	if err != nil {
		t.Fatal(err)
	}
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {ID: policy.ProfileDemoModel, Kind: "model", Origin: origin.URL, Paths: []string{"/v1/"}, Host: "127.0.0.1"},
		policy.ProfileDemoMCP:   {ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: origin.URL, Paths: []string{"/mcp"}, Host: "127.0.0.1", Tools: []string{"list_resources"}},
	})
	h := New(store, pol)
	bind, err := h.Start(ctx, GateSpec{
		SessionID: "mcp1",
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: origin.URL, Paths: []string{"/v1/"}},
			MCP:   policy.EndpointRule{Origin: origin.URL, Paths: []string{"/mcp"}},
			MCPID: mcpID,
			Tools: []string{"list_resources"},
		},
		ModelAccountID: modelID,
		MCPAccountID:   mcpID,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "mcp1")

	listReq, _ := http.NewRequest(http.MethodPost, bind.MCPURL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	listReq.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	listReq.Header.Set("Content-Type", "application/json")
	listReq.Header.Set("Mcp-Session-Id", "sess-up")
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	listBody, _ := io.ReadAll(listResp.Body)
	_ = listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK || !strings.Contains(string(listBody), "list_resources") || strings.Contains(string(listBody), "delete_all") {
		t.Fatalf("status=%d body=%s", listResp.StatusCode, listBody)
	}

	callReq, _ := http.NewRequest(http.MethodPost, bind.MCPURL, strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"delete_all"}}`))
	callReq.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	callReq.Header.Set("Content-Type", "application/json")
	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = callResp.Body.Close()
	if callResp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 got %d", callResp.StatusCode)
	}
	if upstreamCalls != 1 {
		t.Fatalf("denied tool reached upstream: %d", upstreamCalls)
	}
}

func TestModelProxyIdleTimeout(t *testing.T) {
	// 上游只冲刷响应头后挂起，不写正文：逼出 Gate 空闲读超时 → 504。
	// （旧写法 sleep 后再写 "late" 在 -race/CI 负载下偶发 Client 等头超时。）
	flushed := make(chan struct{})
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(flushed)
		<-r.Context().Done()
	}))
	defer origin.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, _ := store.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "m", ProfileID: policy.ProfileDemoModel, Host: "127.0.0.1",
	}, []byte("sk-real-key"))
	mcpID, _ := store.Put(ctx, contracts.AccountInfo{
		Kind: "mcp", Name: "t", ProfileID: policy.ProfileDemoMCP, Host: "127.0.0.1",
	}, []byte("mcp-secret"))
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {ID: policy.ProfileDemoModel, Kind: "model", Origin: origin.URL, Paths: []string{"/v1/"}, Host: "127.0.0.1"},
		policy.ProfileDemoMCP:   {ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: origin.URL, Paths: []string{"/mcp"}, Host: "127.0.0.1"},
	})
	h := New(store, pol)
	h.idleTimeout = 40 * time.Millisecond
	h.totalTimeout = 5 * time.Second
	bind, err := h.Start(ctx, GateSpec{
		SessionID: "idle1",
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: origin.URL, Paths: []string{"/v1/"}},
			MCP:   policy.EndpointRule{Origin: origin.URL, Paths: []string{"/mcp"}},
			MCPID: mcpID,
		},
		ModelAccountID: modelID,
		MCPAccountID:   mcpID,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "idle1")

	req, _ := http.NewRequest(http.MethodGet, bind.ModelBaseURL+"/v1/chat", nil)
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	select {
	case <-flushed:
	default:
		t.Fatal("upstream never flushed headers")
	}
	if resp.StatusCode != http.StatusGatewayTimeout {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
}
