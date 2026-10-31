package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestMCPPayloadHelpers(t *testing.T) {
	if !toolAllowed([]string{"read"}, "read") || toolAllowed([]string{"read"}, "write") || toolAllowed(nil, "") {
		t.Fatal("tool allowlist mismatch")
	}
	method, tool, ok := parseMCPRequest([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read"}}`))
	if !ok || method != "tools/call" || tool != "read" {
		t.Fatalf("parsed request: %q %q %v", method, tool, ok)
	}
	for _, body := range [][]byte{nil, []byte(`bad`), []byte(`{"method":"missing-rpc"}`)} {
		if _, _, ok := parseMCPRequest(body); body != nil && ok {
			t.Fatalf("invalid request accepted: %q", body)
		}
	}
	body := []byte(`{"jsonrpc":"2.0","result":{"tools":[{"name":"read"},{"name":"write"}]}}`)
	filtered := filterToolsPayload(body, []string{"read"})
	if bytes.Contains(filtered, []byte(`"write"`)) || !bytes.Contains(filtered, []byte(`"read"`)) {
		t.Fatalf("tools not filtered: %s", filtered)
	}
	sse := []byte("event: message\ndata: " + string(body) + "\n\n")
	filteredSSE := filterToolsPayload(sse, []string{"read"})
	if bytes.Contains(filteredSSE, []byte(`"write"`)) {
		t.Fatalf("SSE tools not filtered: %s", filteredSSE)
	}
	if got := filterToolsPayload([]byte("invalid"), nil); string(got) != "invalid" {
		t.Fatalf("invalid payload changed: %s", got)
	}
	if got, ok := toolsSSEToJSON([]byte("data: {\"jsonrpc\":\"2.0\"}\n")); !ok || len(got) == 0 {
		t.Fatalf("SSE conversion failed: %s %v", got, ok)
	}
}

func TestMCPHeadersAndMountSelection(t *testing.T) {
	dst := make(http.Header)
	src := http.Header{"Content-Type": []string{"application/json"}, "Mcp-Session-Id": []string{"sid"}}
	copyMCPHeaders(dst, src)
	if dst.Get("Content-Type") != "application/json" || dst.Get("Accept") == "" || dst.Get("Mcp-Session-Id") != "sid" {
		t.Fatalf("headers: %#v", dst)
	}
	if got, err := readToolsList(bytes.NewBufferString(`{"result":{"tools":[]}}`), "application/json", 1024); err != nil || len(got) == 0 {
		t.Fatalf("read JSON tools: %s %v", got, err)
	}
	if _, err := readToolsList(errorReader{}, "application/json", 1024); err == nil {
		t.Fatal("reader error ignored")
	}
	rt := &sessionRuntime{spec: GateSpec{Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{{AccountID: "a"}, {AccountID: "b"}}}}}
	ln := httptest.NewRecorder()
	_ = ln
	g := &mcpGateway{}
	if _, ok := g.mountForPath(rt, "/mcp"); ok {
		t.Fatal("ambiguous single path accepted for multiple mounts")
	}
	if mount, ok := g.mountForPath(rt, "/mcp/1"); !ok || mount.AccountID != "b" {
		t.Fatalf("mount selection: %#v %v", mount, ok)
	}
	if _, ok := g.mountForPath(rt, "/mcp/9"); ok {
		t.Fatal("unknown mount accepted")
	}
}

func TestInjectHarborDemoAskTool(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"other"}]}}`)
	out := injectHarborDemoAskTool(in)
	if !bytes.Contains(out, []byte(`"harbor_demo_ask"`)) || !bytes.Contains(out, []byte(`"other"`)) {
		t.Fatalf("%s", out)
	}
	// idempotent
	out2 := injectHarborDemoAskTool(out)
	if bytes.Count(out2, []byte(`"harbor_demo_ask"`)) != 1 {
		t.Fatalf("duplicated: %s", out2)
	}
}

func TestInjectHarborDemoBlockedTool(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"other"}]}}`)
	out := injectHarborDemoBlockedTool(in)
	if !bytes.Contains(out, []byte(`"harbor_demo_blocked"`)) || !bytes.Contains(out, []byte(`"other"`)) {
		t.Fatalf("%s", out)
	}
	out2 := injectHarborDemoBlockedTool(out)
	if bytes.Count(out2, []byte(`"harbor_demo_blocked"`)) != 1 {
		t.Fatalf("duplicated: %s", out2)
	}
}

func TestDemoBlockedCallAuditsDeniedNoModal(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("demo blocked must not hit upstream")
	}))
	t.Cleanup(up.Close)
	ctx := context.Background()
	store := secrets.NewStore()
	modelID := putMountAccount(t, store, "model", "mk")
	mcpID := putMountAccount(t, store, "mcp", "tok")
	audits := make(chan contracts.AuditData, 2)
	h := New(store, policy.NewBuilder(nil))
	bind, err := h.Start(ctx, GateSpec{
		SessionID:      "demo-blocked",
		ModelAccountID: modelID,
		EnableDemoAsk:  false,
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: up.URL, Paths: []string{"/"}},
			MCPMounts: []policy.MCPMount{{
				AccountID: mcpID,
				Endpoint:  policy.EndpointRule{Origin: up.URL, Paths: []string{"/mcp"}},
				Tools:     []string{"other_only"},
			}},
		},
		RequestPermission: func(context.Context, contracts.PermissionNeeded) (string, error) {
			t.Fatal("RequestPermission must not run for harbor_demo_blocked")
			return "deny", nil
		},
	}, func(a contracts.AuditData) { audits <- a }, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Revoke(ctx, "demo-blocked") })
	mcpURL := bind.MCPURL
	if mcpURL == "" && len(bind.MCPURLs) > 0 {
		mcpURL = bind.MCPURLs[0]
	}
	body := []byte(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"harbor_demo_blocked","arguments":{}}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.Contains(raw, []byte("tool not allowed")) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	select {
	case a := <-audits:
		if a.Tool != "harbor_demo_blocked" || a.Result != "denied" {
			t.Fatalf("audit %+v", a)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no audit")
	}
}

func TestParseDemoAskAndValidHost(t *testing.T) {
	id, host, ok := parseDemoAskCall([]byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"harbor_demo_ask","arguments":{"host":"Demo-Ask.Example"}}}`))
	if !ok || host != "demo-ask.example" || string(id) != "7" {
		t.Fatalf("id=%s host=%q ok=%v", id, host, ok)
	}
	if !validDemoAskHost("demo-ask.example") || validDemoAskHost("localhost") || validDemoAskHost("http://x.com") || validDemoAskHost("a") {
		t.Fatal("validDemoAskHost mismatch")
	}
	rec := httptest.NewRecorder()
	writeJSONRPCToolText(rec, json.RawMessage("1"), true, "ok")
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"ok"`)) {
		t.Fatalf("%s", rec.Body.Bytes())
	}
}

func TestDemoAskDisabledForbidden(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("demo ask must not hit upstream")
	}))
	t.Cleanup(up.Close)
	ctx := context.Background()
	store := secrets.NewStore()
	modelID := putMountAccount(t, store, "model", "mk")
	mcpID := putMountAccount(t, store, "mcp", "tok")
	h := New(store, policy.NewBuilder(nil))
	bind, err := h.Start(ctx, GateSpec{
		SessionID:      "demo-ask-off",
		ModelAccountID: modelID,
		EnableDemoAsk:  false,
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: up.URL, Paths: []string{"/"}},
			MCPMounts: []policy.MCPMount{{
				AccountID: mcpID,
				Endpoint:  policy.EndpointRule{Origin: up.URL, Paths: []string{"/mcp"}},
				Tools:     []string{"other_only"},
			}},
		},
		RequestPermission: func(context.Context, contracts.PermissionNeeded) (string, error) {
			t.Fatal("RequestPermission must not run when demo ask disabled")
			return "deny", nil
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Revoke(ctx, "demo-ask-off") })
	mcpURL := bind.MCPURL
	if mcpURL == "" && len(bind.MCPURLs) > 0 {
		mcpURL = bind.MCPURLs[0]
	}
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"harbor_demo_ask","arguments":{"host":"demo-ask.example"}}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
}

func TestDemoAskBypassesToolAllowlist(t *testing.T) {
	// 白名单没有 harbor_demo_ask 时，仍须走扩权回调，而不是 403 tool not allowed。
	asked := make(chan string, 1)
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("demo ask must not hit upstream")
	}))
	t.Cleanup(up.Close)
	ctx := context.Background()
	store := secrets.NewStore()
	modelID := putMountAccount(t, store, "model", "mk")
	mcpID := putMountAccount(t, store, "mcp", "tok")
	h := New(store, policy.NewBuilder(nil))
	bind, err := h.Start(ctx, GateSpec{
		SessionID:      "demo-ask-allowlist",
		ModelAccountID: modelID,
		EnableDemoAsk:  true,
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: up.URL, Paths: []string{"/"}},
			MCPMounts: []policy.MCPMount{{
				AccountID: mcpID,
				Endpoint:  policy.EndpointRule{Origin: up.URL, Paths: []string{"/mcp"}},
				Tools:     []string{"other_only"},
			}},
		},
		RequestPermission: func(_ context.Context, need contracts.PermissionNeeded) (string, error) {
			asked <- need.Target
			return "deny", nil
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Revoke(ctx, "demo-ask-allowlist") })
	mcpURL := bind.MCPURL
	if mcpURL == "" && len(bind.MCPURLs) > 0 {
		mcpURL = bind.MCPURLs[0]
	}
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"harbor_demo_ask","arguments":{"host":"demo-ask.example"}}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.Contains(raw, []byte("outbound denied")) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	select {
	case got := <-asked:
		if got != "demo-ask.example" {
			t.Fatalf("target %q", got)
		}
	default:
		t.Fatal("RequestPermission not called")
	}
}

func TestWriteDiscoverLegacyFallback(t *testing.T) {
	rec := httptest.NewRecorder()
	writeDiscoverLegacyFallback(rec, []byte(`{"jsonrpc":"2.0","id":"server-discover-probe-1","method":"server/discover"}`))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var out struct {
		ID    string `json:"id"`
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID != "server-discover-probe-1" || out.Error == nil || out.Error.Code != -32601 {
		t.Fatalf("body %s err %v", rec.Body.Bytes(), err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
