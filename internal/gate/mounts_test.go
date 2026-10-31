package gate

import (
	"context"
	"fmt"
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPMountsKeepCredentialsAndToolsSeparate(t *testing.T) {
	ctx := context.Background()
	store := secrets.NewStore()
	model := putMountAccount(t, store, "model", "model-fixture")
	first := putMountAccount(t, store, "mcp", "first-fixture")
	second := putMountAccount(t, store, "mcp", "second-fixture")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			t.Error("转发了 Gateway 选择路径")
		}
		body, _ := io.ReadAll(r.Body)
		want := "Bearer first-fixture"
		if strings.Contains(string(body), `"second"`) {
			want = "Bearer second-fixture"
		}
		if r.Header.Get("Authorization") != want {
			t.Error("挂载凭据串用")
		}
		io.WriteString(w, r.Header.Get("Authorization"))
	}))
	defer upstream.Close()
	h := New(store, policy.NewBuilder(nil))
	spec := GateSpec{SessionID: "multi", ModelAccountID: model, Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{
		{AccountID: first, Endpoint: policy.EndpointRule{Origin: upstream.URL, Paths: []string{"/mcp"}}, Tools: []string{"first"}},
		{AccountID: second, Endpoint: policy.EndpointRule{Origin: upstream.URL, Paths: []string{"/mcp"}}, Tools: []string{"second"}},
	}}}
	binding, err := h.Start(ctx, spec, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, spec.SessionID)
	if len(binding.MCPURLs) != 2 {
		t.Fatalf("%+v", binding)
	}
	for i, endpoint := range binding.MCPURLs {
		for _, tool := range []string{"first", "second"} {
			req, _ := http.NewRequest("POST", endpoint, strings.NewReader(fmt.Sprintf(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":%q}}`, tool)))
			req.Header.Set("Authorization", "Bearer "+binding.Sentinel)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			want := http.StatusForbidden
			if (i == 0 && tool == "first") || (i == 1 && tool == "second") {
				want = http.StatusOK
			}
			if resp.StatusCode != want {
				t.Fatalf("mount=%d tool=%s status=%d", i, tool, resp.StatusCode)
			}
			if strings.Contains(string(body), "-fixture") {
				t.Fatal("凭据泄漏")
			}
		}
	}
}
func TestMCPGatewayForwardsCustomNestedPath(t *testing.T) {
	// 回归：mcpcn 等 Origin+长 path；旧实现写死 Origin+"/mcp" 导致 Agent 看不到工具。
	ctx := context.Background()
	store := secrets.NewStore()
	model := putMountAccount(t, store, "model", "model-fixture")
	mcpID := putMountAccount(t, store, "mcp", "ocr-token")
	nested := "/mcp/mcp-image-ocr-mcp/b252ea47-4d31-4eee-9fdd-2ac0a84e4a39"
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer ocr-token" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"ocr_extract_text"}]}}`)
	}))
	defer upstream.Close()

	h := New(store, policy.NewBuilder(nil))
	spec := GateSpec{SessionID: "nested-mcp", ModelAccountID: model, Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{
		{AccountID: mcpID, Endpoint: policy.EndpointRule{Origin: upstream.URL, Paths: []string{nested}}, Tools: []string{"ocr_extract_text"}},
	}}}
	binding, err := h.Start(ctx, spec, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, spec.SessionID)

	req, _ := http.NewRequest(http.MethodPost, binding.MCPURL, strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+binding.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if gotPath != nested {
		t.Fatalf("upstream path=%q want %q (must not collapse to /mcp)", gotPath, nested)
	}
	if !strings.Contains(string(body), "ocr_extract_text") {
		t.Fatalf("tools filtered incorrectly: %s", body)
	}
}

func TestZeroMCPStartsOnlyModelListener(t *testing.T) {
	ctx := context.Background()
	store := secrets.NewStore()
	model := putMountAccount(t, store, "model", "fixture")
	h := New(store, policy.NewBuilder(nil))
	binding, err := h.Start(ctx, GateSpec{SessionID: "zero", ModelAccountID: model}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "zero")
	if binding.MCPURL != "" || len(binding.MCPURLs) != 0 || len(binding.AllowedTCP) != 1 {
		t.Fatalf("%+v", binding)
	}
}

func putMountAccount(t *testing.T, store *secrets.Store, kind, key string) string {
	t.Helper()
	id, err := store.Put(context.Background(), contracts.AccountInfo{Kind: kind, Name: "fixture", ProfileID: "fixture", Host: "127.0.0.1"}, []byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return id
}
