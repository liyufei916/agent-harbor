package gate

import (
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestOAuthCallbackDoesNotStoreWithoutTokenEndpoint(t *testing.T) {
	t.Setenv("HARBOR_MCP_OAUTH_AUTHORIZE_URL", "https://auth.example.test/authorize")
	t.Setenv("HARBOR_MCP_OAUTH_TOKEN_URL", "")
	t.Setenv("HARBOR_MCP_OAUTH_CLIENT_ID", "")
	store := secrets.NewStore()
	flow := NewOAuthFlow(store, policy.NewBuilder(nil))
	defer flow.Close()
	authURL, pendingID, err := flow.Start(contracts.StartMcpOAuthInput{Name: "Tool", Origin: "https://mcp.example.test"})
	if err != nil || authURL == "" || pendingID == "" {
		t.Fatalf("url=%s id=%s err=%v", authURL, pendingID, err)
	}
	port := flow.ln.Addr().(*net.TCPAddr).Port
	resp, err := http.Get(flow.redirectURI(port) + "?code=abc&state=" + pendingID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusOK {
		t.Fatal("missing token endpoint must not succeed")
	}
	list, err := store.List("mcp")
	if err != nil || len(list) != 0 {
		t.Fatalf("stored=%v err=%v", list, err)
	}
}
