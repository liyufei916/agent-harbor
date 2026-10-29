package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
)

// OAuth 回环：默认授权码流（response_type=code），回调后向 Origin 换票。
// client_id / client_secret 来自环境变量（不进 UI、不进事件）。

type oauthPending struct {
	id          string
	name        string
	origin      string // 用户填写的 MCP/IdP URL（可含路径）
	authority   string // scheme://host，用于 /authorize 与换票
	host        string
	basePath    string
	profileID   string
	redirectURI string
	ln          net.Listener
	server      *http.Server
	done        chan struct{}
}

func oauthAuthority(origin string) (authority, host, basePath string, err error) {
	u, perr := url.Parse(origin)
	if perr != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", "", "", contracts.NewAppError(contracts.ErrInvalidArgument, "OAuth Origin 必须是 https://host[/path]", false)
	}
	authority = u.Scheme + "://" + u.Host
	host = strings.ToLower(u.Hostname())
	basePath = strings.TrimRight(u.Path, "/")
	if basePath == "/" {
		basePath = ""
	}
	return authority, host, basePath, nil
}

func (a *App) startMcpOAuth(in contracts.StartMcpOAuthInput) (authURL, pendingID string, err error) {
	name := strings.TrimSpace(in.Name)
	origin := strings.TrimRight(strings.TrimSpace(in.Origin), "/")
	profileID := strings.TrimSpace(in.ProfileID)
	if name == "" {
		return "", "", contracts.NewAppError(contracts.ErrInvalidArgument, "显示名不能为空", false)
	}
	if origin == "" && profileID != "" {
		if prof, ok := a.policy.Profile(profileID); ok {
			origin = strings.TrimRight(prof.Origin, "/")
		}
	}
	if origin == "" {
		return "", "", contracts.NewAppError(contracts.ErrInvalidArgument, "OAuth 需要 HTTPS 服务地址", false)
	}
	authority, host, basePath, aerr := oauthAuthority(origin)
	if aerr != nil {
		return "", "", aerr
	}
	ln, lerr := net.Listen("tcp", "127.0.0.1:0")
	if lerr != nil {
		return "", "", contracts.NewAppError(contracts.ErrInternal, "无法监听本机回调", false)
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	id := "oauth_" + hex.EncodeToString(b[:])
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	if profileID == "" {
		profileID = policy.ProfileCustom
	}
	p := &oauthPending{
		id: id, name: name, origin: origin, authority: authority,
		host: host, basePath: basePath, profileID: profileID,
		redirectURI: redirectURI, ln: ln, done: make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		a.completeMcpOAuth(p, w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>Harbor OAuth</title>
<p>Agent Harbor OAuth 回调已就绪。请将 IdP 的 redirect_uri 设为：</p>
<p><code>%s</code></p>`, redirectURI)
	})
	p.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	a.mu.Lock()
	if a.oauth == nil {
		a.oauth = make(map[string]*oauthPending)
	}
	a.oauth[id] = p
	a.mu.Unlock()
	go func() {
		_ = p.server.Serve(ln)
		close(p.done)
	}()
	clientID := oauthClientID()
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", id)
	q.Set("client_id", clientID)
	authURL = authority + "/authorize?" + q.Encode()
	openSystemBrowser(authURL)
	return authURL, id, nil
}

func oauthClientID() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_CLIENT_ID")); v != "" {
		return v
	}
	return "agent-harbor"
}

func oauthClientSecret() string {
	return strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_CLIENT_SECRET"))
}

func oauthTokenPath() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_TOKEN_PATH")); v != "" {
		if !strings.HasPrefix(v, "/") {
			v = "/" + v
		}
		return v
	}
	return "/oauth/token"
}

func (a *App) completeMcpOAuth(p *oauthPending, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	token := strings.TrimSpace(q.Get("access_token"))
	code := strings.TrimSpace(q.Get("code"))
	if errMsg := strings.TrimSpace(q.Get("error")); errMsg != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>Harbor OAuth</title>
<p>授权失败：<code>%s</code> %s</p>
<p>请回到 Agent Harbor 取消后重试，或改用 API Key / Token。</p>`,
			htmlEscape(errMsg), htmlEscape(strings.TrimSpace(q.Get("error_description"))))
		return
	}
	if token == "" && code == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><meta charset=utf-8><title>Harbor OAuth</title>
<p>等待授权码回调。若地址栏 hash 含 access_token（隐式流），将尝试转成查询参数。</p>
<script>
if (location.hash) {
  const p = new URLSearchParams(location.hash.slice(1));
  const t = p.get("access_token");
  if (t) location.replace(location.pathname + "?access_token=" + encodeURIComponent(t) + "&state=" + encodeURIComponent(p.get("state")||""));
}
</script>`))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	authority := p.authority
	if authority == "" {
		authority = p.origin
	}
	if token == "" && code != "" {
		exchanged, err := exchangeAuthCode(ctx, authority, code, p.redirectURI, oauthClientID(), oauthClientSecret())
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>Harbor OAuth</title>
<p>换票失败，未能写入 MCP 账号。</p>
<p class="meta"><code>%s</code></p>
<p>请回到 Agent Harbor 点「我已完成登录」会提示仍未写入；可取消后检查 IdP token 端点或改用 Token。</p>`, htmlEscape(err.Error()))
			return
		}
		token = exchanged
	}
	host := p.host
	if host == "" {
		if u, err := url.Parse(p.origin); err == nil {
			host = strings.ToLower(u.Hostname())
		}
	}
	profileID := p.profileID
	if profileID == "" {
		profileID = policy.ProfileCustom
	}
	meta := contracts.AccountInfo{
		Kind: "mcp", Name: p.name, ProfileID: profileID, Host: host, BasePath: p.basePath,
	}
	id, err := a.secrets.Put(ctx, meta, []byte(token))
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>Harbor OAuth</title>
<p>登录已拿到令牌，但保存 MCP 账号失败：<code>%s</code></p>`, htmlEscape(err.Error()))
		return
	}
	_ = id
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><meta charset=utf-8><title>Harbor OAuth</title>
<p><strong>登录完成。</strong>请回到 Agent Harbor 工具页，点「我已完成登录」或等待列表自动刷新。</p>`))
	go a.cancelMcpOAuth(p.id)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// exchangeAuthCode 用授权码换 access_token；可选 client_secret（机密客户端）。
func exchangeAuthCode(ctx context.Context, origin, code, redirectURI, clientID, clientSecret string) (string, error) {
	tokenURL := strings.TrimRight(origin, "/") + oauthTokenPath()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint status %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if json.Unmarshal(raw, &body) != nil || body.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}
	return body.AccessToken, nil
}

func (a *App) cancelMcpOAuth(pendingID string) error {
	a.mu.Lock()
	p, ok := a.oauth[pendingID]
	if ok {
		delete(a.oauth, pendingID)
	}
	a.mu.Unlock()
	if !ok {
		return contracts.NewAppError(contracts.ErrInvalidArgument, "无此 OAuth 请求", false)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.server.Shutdown(ctx)
	_ = p.ln.Close()
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
	}
	return nil
}
