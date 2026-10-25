package gate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

// OAuthFlow 在 127.0.0.1 收回环回调。未登记令牌端点时不保存账号。
type OAuthFlow struct {
	secrets *secrets.Store
	policy  *policy.Builder
	client  *http.Client

	mu      sync.Mutex
	ln      net.Listener
	srv     *http.Server
	pending map[string]oauthPending
}

type oauthPending struct {
	name      string
	profileID string
	host      string
}

// NewOAuthFlow 创建 OAuth 回环。监听端口在第一次 Start 时绑定。
func NewOAuthFlow(store *secrets.Store, pol *policy.Builder) *OAuthFlow {
	return &OAuthFlow{
		secrets: store,
		policy:  pol,
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		pending: map[string]oauthPending{},
	}
}

// Start 登记 pending 并返回给系统浏览器打开的地址。
func (f *OAuthFlow) Start(in contracts.StartMcpOAuthInput) (string, string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", "", ErrInvalidConnectInput
	}
	profileID := strings.TrimSpace(in.ProfileID)
	originIn := strings.TrimSpace(in.Origin)
	host, profileID, err := f.oauthHost(profileID, originIn)
	if err != nil {
		return "", "", err
	}
	authBase := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_AUTHORIZE_URL"))
	authURL, err := url.Parse(authBase)
	if err != nil || authURL.Scheme != "https" || authURL.Host == "" || authURL.User != nil {
		return "", "", contracts.NewAppError(contracts.ErrInvalidArgument, "OAuth 授权地址尚未登记", false)
	}
	if err := f.ensureListener(); err != nil {
		return "", "", err
	}
	id, err := randomPendingID()
	if err != nil {
		return "", "", err
	}
	f.mu.Lock()
	port := f.ln.Addr().(*net.TCPAddr).Port
	f.pending[id] = oauthPending{name: name, profileID: profileID, host: host}
	f.mu.Unlock()

	q := authURL.Query()
	q.Set("redirect_uri", f.redirectURI(port))
	q.Set("state", id)
	q.Set("response_type", "code")
	if clientID := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_CLIENT_ID")); clientID != "" {
		q.Set("client_id", clientID)
	}
	authURL.RawQuery = q.Encode()
	return authURL.String(), id, nil
}

// Cancel 清除 pending。未知 ID 也视为成功。
func (f *OAuthFlow) Cancel(pendingID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.pending, strings.TrimSpace(pendingID))
	return nil
}

// Close 关闭回环监听。
func (f *OAuthFlow) Close() error {
	f.mu.Lock()
	srv := f.srv
	f.pending = map[string]oauthPending{}
	f.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func (f *OAuthFlow) oauthHost(profileID, originIn string) (string, string, error) {
	if originIn != "" {
		if profileID != "" && profileID != policy.ProfileCustom {
			return "", "", ErrInvalidConnectInput
		}
		_, host, err := policy.ParseCustomOrigin(originIn)
		if err != nil {
			return "", "", ErrInvalidConnectInput
		}
		return host, policy.ProfileCustom, nil
	}
	if profileID == "" {
		profileID = policy.ProfileDemoMCP
	}
	prof, ok := f.policy.Profile(profileID)
	if !ok || prof.Kind != "mcp" || !prof.ValidConnection() {
		return "", "", contracts.NewAppError(contracts.ErrInvalidArgument, "后端接入配置尚未完整登记或不受支持。", false)
	}
	return prof.Host, profileID, nil
}

func (f *OAuthFlow) ensureListener() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ln != nil {
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", f.callback)
	srv := &http.Server{Handler: mux}
	f.ln = ln
	f.srv = srv
	go func() { _ = srv.Serve(ln) }()
	return nil
}

func (f *OAuthFlow) redirectURI(port int) string {
	return "http://127.0.0.1:" + itoa(port) + "/callback"
}

func (f *OAuthFlow) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	f.mu.Lock()
	item, ok := f.pending[state]
	if ok {
		delete(f.pending, state)
	}
	port := 0
	if f.ln != nil {
		port = f.ln.Addr().(*net.TCPAddr).Port
	}
	f.mu.Unlock()
	if !ok || code == "" {
		http.Error(w, "授权未完成，未保存账号", http.StatusBadRequest)
		return
	}
	tokenURL := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_TOKEN_URL"))
	clientID := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_CLIENT_ID"))
	if tokenURL == "" || clientID == "" {
		http.Error(w, "令牌端点尚未登记，未保存账号", http.StatusBadGateway)
		return
	}
	token, err := f.exchange(r.Context(), tokenURL, code, f.redirectURI(port), clientID)
	if err != nil {
		http.Error(w, "令牌交换失败，未保存账号", http.StatusBadGateway)
		return
	}
	_, err = f.secrets.Put(r.Context(), contracts.AccountInfo{
		Kind: "mcp", Name: item.name, ProfileID: item.profileID, Host: item.host,
	}, []byte(token))
	if err != nil {
		http.Error(w, "账号保存失败", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "授权完成，可以回到 Agent Harbor。账号列表刷新后可见。")
}

func (f *OAuthFlow) exchange(ctx context.Context, tokenURL, code, redirectURI, clientID string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	if secret := os.Getenv("HARBOR_MCP_OAUTH_CLIENT_SECRET"); secret != "" {
		form.Set("client_secret", secret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ErrUpstreamAuth
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(raw, &body) != nil || body.AccessToken == "" {
		return "", ErrUpstreamAuth
	}
	return body.AccessToken, nil
}

func randomPendingID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
