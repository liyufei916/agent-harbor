package desktop

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestOAuthInputAndEnvironmentValidation(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	for _, in := range []contracts.StartMcpOAuthInput{
		{}, {Name: "tool"}, {Name: "tool", Origin: "http://example.com"}, {Name: "tool", Origin: "https://user:pass@example.com"},
	} {
		if got := api.StartMcpOAuth(in); got.Error == nil {
			t.Fatalf("invalid OAuth input accepted: %#v", in)
		}
	}
	// 本机回环 http 允许启动（公网 http 仍拒）
	loop := api.StartMcpOAuth(contracts.StartMcpOAuthInput{Name: "loop", Origin: "http://127.0.0.1:9"})
	if loop.Error != nil || loop.PendingID == "" {
		t.Fatalf("loopback http OAuth: %+v", loop)
	}
	_ = api.CancelMcpOAuth(loop.PendingID)
	t.Setenv("HARBOR_MCP_OAUTH_CLIENT_ID", " client-id ")
	t.Setenv("HARBOR_MCP_OAUTH_CLIENT_SECRET", " secret ")
	t.Setenv("HARBOR_MCP_OAUTH_TOKEN_PATH", "oauth/token")
	if oauthClientID() != "client-id" || oauthClientSecret() != "secret" || oauthTokenPath() != "/oauth/token" {
		t.Fatalf("OAuth env helpers incorrect")
	}
	t.Setenv("HARBOR_MCP_OAUTH_TOKEN_PATH", "")
	if oauthTokenPath() != "/oauth/token" {
		t.Fatal("default token path incorrect")
	}
}
