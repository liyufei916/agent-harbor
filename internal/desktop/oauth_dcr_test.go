package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTryDynamicClientRegistration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"registration_endpoint": "http://" + r.Host + "/register",
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"client_id":     "dcr-client",
			"client_secret": "dcr-secret",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creds, err := tryDynamicClientRegistration(context.Background(), srv.URL, "http://127.0.0.1:9/callback")
	if err != nil {
		t.Fatal(err)
	}
	if creds.ID != "dcr-client" || creds.Secret != "dcr-secret" || creds.Source != "dcr" {
		t.Fatalf("%+v", creds)
	}
}

func TestResolveOAuthClientPrefersEnv(t *testing.T) {
	t.Setenv("HARBOR_MCP_OAUTH_CLIENT_ID", "from-env")
	t.Setenv("HARBOR_MCP_OAUTH_CLIENT_SECRET", "sec")
	creds := resolveOAuthClient(context.Background(), "https://example.invalid", "http://127.0.0.1:9/callback")
	if creds.Source != "env" || creds.ID != "from-env" || creds.Secret != "sec" {
		t.Fatalf("%+v", creds)
	}
}
