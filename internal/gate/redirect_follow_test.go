package gate

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestFollowRedirectAfterPermissionWithoutCredentials(t *testing.T) {
	var secondAuth string
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, "redirect-ok")
	}))
	defer second.Close()
	secondAddr := second.Listener.Addr().String()

	oldLookup := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		if host == "cdn.example.test" {
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		}
		return oldLookup(host)
	}
	defer func() { lookupIP = oldLookup }()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://cdn.example.test/payload", http.StatusFound)
	}))
	defer first.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, err := store.Put(ctx, contracts.AccountInfo{
		Kind: "model", Name: "demo", ProfileID: policy.ProfileDemoModel, Host: "127.0.0.1",
	}, []byte("sk-must-not-travel"))
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
			ID: policy.ProfileDemoModel, Kind: "model", Origin: first.URL, Paths: []string{"/"}, Host: "127.0.0.1",
		},
		policy.ProfileDemoMCP: {
			ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: first.URL, Paths: []string{"/mcp"}, Host: "127.0.0.1",
		},
	})
	h := New(store, pol)
	// Dial cdn.example.test → 第二台 httptest；并允许 http 跟随（仅测）。
	insecureRedirectOK = func() bool { return true }
	defer func() { insecureRedirectOK = func() bool { return false } }()
	base := h.client.Transport.(*http.Transport).Clone()
	base.Proxy = nil
	base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.HasPrefix(addr, "cdn.example.test:") || strings.HasPrefix(addr, "203.0.113.10:") {
			return (&net.Dialer{}).DialContext(ctx, network, secondAddr)
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	h.client.Transport = base

	bind, err := h.Start(ctx, GateSpec{
		SessionID: "redir1",
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: first.URL, Paths: []string{"/"}},
			MCP:   policy.EndpointRule{Origin: first.URL, Paths: []string{"/mcp"}},
			MCPID: mcpID,
		},
		ModelAccountID: modelID,
		MCPAccountID:   mcpID,
		RequestPermission: func(_ context.Context, need contracts.PermissionNeeded) (string, error) {
			if need.Target != "cdn.example.test" {
				t.Fatalf("target=%q", need.Target)
			}
			return "once", nil
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "redir1")

	req, _ := http.NewRequest(http.MethodGet, bind.ModelBaseURL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "redirect-ok" {
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}
	if secondAuth != "" {
		t.Fatalf("credentials leaked to redirect host: %q", secondAuth)
	}
}

func TestRedirectBlockedWithoutPermission(t *testing.T) {
	oldLookup := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		if host == "cdn.example.test" {
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		}
		return oldLookup(host)
	}
	defer func() { lookupIP = oldLookup }()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://cdn.example.test/x", http.StatusFound)
	}))
	defer first.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, _ := store.Put(ctx, contracts.AccountInfo{Kind: "model", Name: "m", ProfileID: policy.ProfileDemoModel, Host: "127.0.0.1"}, []byte("k"))
	mcpID, _ := store.Put(ctx, contracts.AccountInfo{Kind: "mcp", Name: "t", ProfileID: policy.ProfileDemoMCP, Host: "127.0.0.1"}, []byte("k"))
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {ID: policy.ProfileDemoModel, Kind: "model", Origin: first.URL, Paths: []string{"/"}, Host: "127.0.0.1"},
		policy.ProfileDemoMCP:   {ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: first.URL, Paths: []string{"/mcp"}, Host: "127.0.0.1"},
	})
	h := New(store, pol)
	bind, err := h.Start(ctx, GateSpec{
		SessionID: "redir2",
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: first.URL, Paths: []string{"/"}},
			MCP:   policy.EndpointRule{Origin: first.URL, Paths: []string{"/mcp"}}, MCPID: mcpID,
		},
		ModelAccountID: modelID, MCPAccountID: mcpID,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "redir2")

	req, _ := http.NewRequest(http.MethodGet, bind.ModelBaseURL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("want 502 got %d", resp.StatusCode)
	}
}

func TestRedirectDeniedWhenDNSIsPrivate(t *testing.T) {
	oldLookup := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.8")}, nil
	}
	defer func() { lookupIP = oldLookup }()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.internal/x", http.StatusFound)
	}))
	defer first.Close()

	store := secrets.NewStore()
	ctx := context.Background()
	modelID, _ := store.Put(ctx, contracts.AccountInfo{Kind: "model", Name: "m", ProfileID: policy.ProfileDemoModel, Host: "127.0.0.1"}, []byte("k"))
	mcpID, _ := store.Put(ctx, contracts.AccountInfo{Kind: "mcp", Name: "t", ProfileID: policy.ProfileDemoMCP, Host: "127.0.0.1"}, []byte("k"))
	pol := policy.NewBuilder(map[string]policy.FixedProfile{
		policy.ProfileDemoModel: {ID: policy.ProfileDemoModel, Kind: "model", Origin: first.URL, Paths: []string{"/"}, Host: "127.0.0.1"},
		policy.ProfileDemoMCP:   {ID: policy.ProfileDemoMCP, Kind: "mcp", Origin: first.URL, Paths: []string{"/mcp"}, Host: "127.0.0.1"},
	})
	h := New(store, pol)
	insecureRedirectOK = func() bool { return true }
	defer func() { insecureRedirectOK = func() bool { return false } }()
	bind, err := h.Start(ctx, GateSpec{
		SessionID: "redir3",
		Policy: policy.PolicySnapshot{
			Model: policy.EndpointRule{Origin: first.URL, Paths: []string{"/"}},
			MCP:   policy.EndpointRule{Origin: first.URL, Paths: []string{"/mcp"}}, MCPID: mcpID,
		},
		ModelAccountID: modelID, MCPAccountID: mcpID,
		RequestPermission: func(context.Context, contracts.PermissionNeeded) (string, error) {
			return "project", nil
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "redir3")

	req, _ := http.NewRequest(http.MethodGet, bind.ModelBaseURL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+bind.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("want 502 got %d", resp.StatusCode)
	}
}
