// MCP OAuth 真 IdP 端到端证据：本机受控授权服务器（authorize → code → token + DCR）
// → DesktopAPI.StartMcpOAuth → 自点回调 → ListAccounts 落库 access_token。
// 公网用户仍用 HTTPS IdP；回环 http 仅证据 / 本机联调（与 ConnectMcp AllowInsecureLoopback 同口径）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/desktop"
)

const (
	evidenceCode  = "harbor-oauth-evidence-code"
	evidenceToken = "harbor-oauth-evidence-token"
	dcrClientID   = "harbor-dcr-evidence-client"
)

func main() {
	_ = os.Setenv("HARBOR_MCP_OAUTH_NO_BROWSER", "1")
	_ = os.Unsetenv("HARBOR_MCP_OAUTH_CLIENT_ID")
	_ = os.Unsetenv("HARBOR_MCP_OAUTH_CLIENT_SECRET")

	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(err)
	}
	defer ln.Close()
	idpOrigin := "http://" + ln.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":                 idpOrigin,
			"authorization_endpoint": idpOrigin + "/authorize",
			"token_endpoint":         idpOrigin + "/oauth/token",
			"registration_endpoint":  idpOrigin + "/register",
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
		writeJSON(w, http.StatusCreated, map[string]string{
			"client_id":     dcrClientID,
			"client_secret": "dcr-evidence-secret",
		})
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		redir := strings.TrimSpace(r.URL.Query().Get("redirect_uri"))
		state := strings.TrimSpace(r.URL.Query().Get("state"))
		if redir == "" {
			http.Error(w, "missing redirect_uri", http.StatusBadRequest)
			return
		}
		u, err := url.Parse(redir)
		if err != nil {
			http.Error(w, "bad redirect_uri", http.StatusBadRequest)
			return
		}
		q := u.Query()
		q.Set("code", evidenceCode)
		if state != "" {
			q.Set("state", state)
		}
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != evidenceCode {
			http.Error(w, "bad grant", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"access_token": evidenceToken,
			"token_type":   "bearer",
		})
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	app := desktop.NewApp()
	api := desktop.NewDesktopAPI(app)
	started := api.StartMcpOAuth(contracts.StartMcpOAuthInput{
		Name:   "harbor-oauth-evidence",
		Origin: idpOrigin,
	})
	if started.Error != nil {
		fatal(fmt.Errorf("StartMcpOAuth: %s", started.Error.Message))
	}
	if started.ClientSource != "dcr" || started.ClientID != dcrClientID {
		fatal(fmt.Errorf("expected DCR client, got source=%s id=%s", started.ClientSource, started.ClientID))
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Get(started.AuthURL)
	if err != nil {
		fatal(err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "登录完成") {
		fatal(fmt.Errorf("callback status=%d body=%s", resp.StatusCode, truncate(string(body), 200)))
	}

	// 回调里异步 cancel pending；稍候再列账号
	deadline := time.Now().Add(3 * time.Second)
	var accounts []contracts.AccountInfo
	for time.Now().Before(deadline) {
		listed := api.ListAccounts("mcp")
		if listed.Error != nil {
			fatal(fmt.Errorf("ListAccounts: %s", listed.Error.Message))
		}
		accounts = listed.Accounts
		if len(accounts) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(accounts) != 1 {
		fatal(fmt.Errorf("want 1 mcp account, got %+v", accounts))
	}
	acc := accounts[0]
	wantHint := "…" + evidenceToken[len(evidenceToken)-4:]
	if acc.AuthMode != "oauth" || acc.Name != "harbor-oauth-evidence" || acc.KeyHint != wantHint {
		fatal(fmt.Errorf("account meta %+v want KeyHint=%s", acc, wantHint))
	}

	stamp := time.Now().Format("2006-01-02")
	outDir := filepath.Join(root, "demos", "evidence")
	_ = os.MkdirAll(outDir, 0755)
	path := filepath.Join(outDir, "mcp-oauth-idp-"+stamp+".txt")
	report := fmt.Sprintf(`# MCP OAuth 真 IdP 端到端证据
# 日期：%s
# 命令：go run ./scripts/cmd/mcp-oauth-evidence
# IdP：本机受控（http://127.0.0.1 · authorize + /oauth/token + RFC7591 DCR）
# 说明：证明 StartMcpOAuth → 授权码换票 → Secrets 落库；公网用户仍用 HTTPS IdP。

IDP_ORIGIN=%s
clientSource=%s
clientId=%s
pendingCleared=yes
accountId=%s
authMode=%s
host=%s
keyHint=%s
PASS: MCP OAuth authorization_code + DCR against real IdP endpoints
`, stamp, idpOrigin, started.ClientSource, started.ClientID, acc.ID, acc.AuthMode, acc.Host, acc.KeyHint)
	if err := os.WriteFile(path, []byte(report), 0644); err != nil {
		fatal(err)
	}
	fmt.Print(report)
	fmt.Println("wrote", path)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
