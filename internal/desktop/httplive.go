package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

const (
	httpLiveDefaultAddr = "127.0.0.1:18765"
	httpLiveEnvEnable   = "HARBOR_HTTP_LIVE"
	httpLiveEnvAddr     = "HARBOR_HTTP_LIVE_ADDR"
	httpLiveEnvToken    = "HARBOR_HTTP_LIVE_TOKEN"
	httpLiveTokenFile   = "http-live.token"
)

// loadOrCreateHTTPLiveToken 持久化 token，保证 RestartApp 后浏览器 URL 仍可用。
func loadOrCreateHTTPLiveToken(app *App) string {
	dir := ""
	if app != nil {
		dir = app.dataDir
	}
	if dir == "" {
		dir = defaultDataDir()
	}
	_ = os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, httpLiveTokenFile)
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t
		}
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return hex.EncodeToString([]byte("harbor-http-live-fallback"))
	}
	tok := hex.EncodeToString(raw)
	_ = os.WriteFile(path, []byte(tok+"\n"), 0600)
	return tok
}

// HTTPLiveServer 本机 loopback 复验桥：浏览器打同一 DesktopAPI（非托管 SaaS）。
type HTTPLiveServer struct {
	api    *DesktopAPI
	token  string
	addr   string
	server *http.Server

	mu        sync.Mutex
	eventSubs map[chan contracts.Event]struct{}
	engSubs   map[chan contracts.EngineProgressPayload]struct{}
}

// HTTPLiveConfig 桥监听配置（仅 loopback）。
type HTTPLiveConfig struct {
	Addr  string
	Token string
	// UIDir 若指向 frontend/dist，则同端口提供 SPA（免另开 Vite）。
	UIDir string
}

// MaybeStartHTTPLive 仅当 HARBOR_HTTP_LIVE=1/true 时启动；否则返回 nil。
func MaybeStartHTTPLive(api *DesktopAPI) (*HTTPLiveServer, error) {
	if !httpLiveEnvOn() {
		return nil, nil
	}
	return StartHTTPLive(api, HTTPLiveConfigFromEnv())
}

func httpLiveEnvOn() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(httpLiveEnvEnable)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// HTTPLiveConfigFromEnv 读取 HARBOR_HTTP_LIVE_ADDR / HARBOR_HTTP_LIVE_TOKEN。
func HTTPLiveConfigFromEnv() HTTPLiveConfig {
	addr := strings.TrimSpace(os.Getenv(httpLiveEnvAddr))
	if addr == "" {
		addr = httpLiveDefaultAddr
	}
	tok := strings.TrimSpace(os.Getenv(httpLiveEnvToken))
	return HTTPLiveConfig{Addr: addr, Token: tok}
}

// StartHTTPLive 启动仅绑定 loopback 的 RPC + SSE 桥。
func StartHTTPLive(api *DesktopAPI, cfg HTTPLiveConfig) (*HTTPLiveServer, error) {
	if api == nil || api.app == nil {
		return nil, fmt.Errorf("httplive: DesktopAPI 未就绪")
	}
	addr := cfg.Addr
	if addr == "" {
		addr = httpLiveDefaultAddr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("httplive: 地址无效: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" {
		return nil, fmt.Errorf("httplive: 仅允许 127.0.0.1/localhost，拒绝 %q", host)
	}
	tok := strings.TrimSpace(cfg.Token)
	if tok == "" {
		tok = loadOrCreateHTTPLiveToken(api.app)
	}
	s := &HTTPLiveServer{
		api:       api,
		token:     tok,
		addr:      addr,
		eventSubs: make(map[chan contracts.Event]struct{}),
		engSubs:   make(map[chan contracts.EngineProgressPayload]struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", s.withCORS(s.handleHealth))
	mux.HandleFunc("/v1/rpc", s.withCORS(s.requireAuth(s.handleRPC)))
	mux.HandleFunc("/v1/events", s.withCORS(s.requireAuth(s.handleEvents)))
	mux.HandleFunc("/v1/engine-progress", s.withCORS(s.requireAuth(s.handleEngineProgress)))
	if ui := strings.TrimSpace(cfg.UIDir); ui != "" {
		if st, err := os.Stat(ui); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("httplive: UIDir 无效: %s", ui)
		}
		s.mountUI(mux, ui)
	} else {
		mux.HandleFunc("/", s.handleRootHint)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("httplive: listen: %w", err)
	}
	s.addr = ln.Addr().String()
	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	api.app.setHTTPLive(s)
	go func() {
		_ = s.server.Serve(ln)
	}()
	return s, nil
}

func (s *HTTPLiveServer) handleRootHint(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	q := url.Values{}
	q.Set("transport", "http-live")
	q.Set("token", s.token)
	q.Set("bridge", "http://"+s.addr)
	vite := "http://127.0.0.1:5173/?" + q.Encode()
	_, _ = fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>http-live</title>
<p>这是 API 桥（无页面）。请打开前端：</p>
<p><a href="%s">%s</a></p>
<p>或：<code>cd frontend && npm run dev</code> 后再点链接。<br/>
也可让 <code>http-live-serve</code> 带上 <code>frontend/dist</code> 同端口出 UI。</p>
<p>健康检查：<a href="/v1/health">/v1/health</a></p>`, vite, vite)
}

func (s *HTTPLiveServer) mountUI(mux *http.ServeMux, uiDir string) {
	fileServer := http.FileServer(http.Dir(uiDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// 无 transport 时补齐查询，避免误开成 mock
			if r.URL.Query().Get("transport") == "" {
				q := r.URL.Query()
				q.Set("transport", "http-live")
				q.Set("token", s.token)
				if q.Get("bridge") == "" {
					q.Set("bridge", "http://"+s.addr)
				}
				http.Redirect(w, r, "/?"+q.Encode(), http.StatusFound)
				return
			}
		}
		// SPA：缺文件回落 index.html
		path := filepath.Join(uiDir, filepath.Clean("/"+r.URL.Path))
		if !strings.HasPrefix(path, filepath.Clean(uiDir)) {
			http.NotFound(w, r)
			return
		}
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			http.ServeFile(w, r, filepath.Join(uiDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (s *HTTPLiveServer) Addr() string  { return s.addr }
func (s *HTTPLiveServer) Token() string { return s.token }
func (s *HTTPLiveServer) URL() string   { return "http://" + s.addr }

func (s *HTTPLiveServer) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	if s.api != nil && s.api.app != nil {
		s.api.app.setHTTPLive(nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// BroadcastEvent 扇出会话事件到 SSE 订阅者。
func (s *HTTPLiveServer) BroadcastEvent(ev contracts.Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.eventSubs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// BroadcastEngineProgress 扇出引擎安装进度。
func (s *HTTPLiveServer) BroadcastEngineProgress(p contracts.EngineProgressPayload) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.engSubs {
		select {
		case ch <- p:
		default:
		}
	}
}

func (s *HTTPLiveServer) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if originOK(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func originOK(origin string) bool {
	if origin == "" {
		return true
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:")
}

func (s *HTTPLiveServer) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		if got == "" {
			got = strings.TrimSpace(r.URL.Query().Get("token"))
			if got != "" {
				got = "Bearer " + got
			}
		}
		want := "Bearer " + s.token
		if got != want {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid token"}}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *HTTPLiveServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"surface":       "http-live",
		"addr":          s.addr,
		"tokenRequired": true,
	})
}

type rpcRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (s *HTTPLiveServer) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"INVALID_ARGUMENT","message":"bad json"}}`, http.StatusBadRequest)
		return
	}
	method := normalizeRPCMethod(req.Method)
	result, err := s.dispatch(method, req.Params)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"error": contracts.NewAppError(contracts.ErrInvalidArgument, err.Error(), false),
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func normalizeRPCMethod(m string) string {
	m = strings.TrimSpace(m)
	if m == "" {
		return ""
	}
	// camelCase → PascalCase
	if m[0] >= 'a' && m[0] <= 'z' {
		return strings.ToUpper(m[:1]) + m[1:]
	}
	return m
}

func (s *HTTPLiveServer) dispatch(method string, params json.RawMessage) (any, error) {
	d := s.api
	switch method {
	case "ListAgents":
		return d.ListAgents(), nil
	case "SelectWorkspace":
		var in struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(params, &in)
		if len(params) > 0 && params[0] == '[' {
			var arr []struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(params, &arr); err == nil && len(arr) > 0 {
				in.Path = arr[0].Path
			}
		}
		return ValidateWorkspacePath(in.Path), nil
	case "ListWorkspaceShortcuts":
		return map[string]any{"shortcuts": listWorkspaceShortcuts()}, nil
	case "ConnectModel":
		var in contracts.ConnectInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.ConnectModel(in), nil
	case "ConnectMcp":
		var in contracts.ConnectInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.ConnectMcp(in), nil
	case "ListMcpPresets":
		return d.ListMcpPresets(), nil
	case "AddMcpPreset":
		var in contracts.AddMcpPresetInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.AddMcpPreset(in), nil
	case "ListAccounts":
		var kind string
		_ = unmarshalOne(params, &kind)
		if kind == "" && len(params) > 0 && params[0] == '[' {
			var arr []string
			if json.Unmarshal(params, &arr) == nil && len(arr) > 0 {
				kind = arr[0]
			}
		}
		return d.ListAccounts(kind), nil
	case "RemoveAccount":
		var id string
		if err := unmarshalOne(params, &id); err != nil {
			return nil, err
		}
		return d.RemoveAccount(id), nil
	case "CreateSession":
		var in contracts.SessionSpec
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.CreateSession(in), nil
	case "GetSession", "StopSession", "StartSession", "DestroySession", "SyncBackWorkspace":
		var id string
		if err := unmarshalOne(params, &id); err != nil {
			return nil, err
		}
		switch method {
		case "GetSession":
			return d.GetSession(id), nil
		case "StopSession":
			return d.StopSession(id), nil
		case "StartSession":
			return d.StartSession(id), nil
		case "DestroySession":
			return d.DestroySession(id), nil
		default:
			return d.SyncBackWorkspace(id), nil
		}
	case "ListSessions":
		return d.ListSessions(), nil
	case "GetRuntimeHealth":
		return d.GetRuntimeHealth(), nil
	case "GetContainerEngineHealth":
		return d.GetContainerEngineHealth(), nil
	case "EnsureContainerEngine":
		return d.EnsureContainerEngine(), nil
	case "StopContainerEngine":
		return d.StopContainerEngine(), nil
	case "GetPermissionOverview":
		var id string
		_ = unmarshalOne(params, &id)
		return d.GetPermissionOverview(id), nil
	case "ListRememberedHosts":
		var ws string
		_ = unmarshalOne(params, &ws)
		return d.ListRememberedHosts(ws), nil
	case "RevokeRememberedHost":
		var in contracts.RevokeHostInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.RevokeRememberedHost(in), nil
	case "ResolvePermission":
		var in contracts.ResolvePermissionInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.ResolvePermission(in), nil
	case "PromptPermission":
		var in contracts.PromptPermissionInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.PromptPermission(in), nil
	case "GetUIPrefs":
		return d.GetUIPrefs(), nil
	case "SetUIPrefs":
		var in contracts.UIPrefs
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.SetUIPrefs(in), nil
	case "SwitchModel":
		var in contracts.SwitchModelInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.SwitchModel(in), nil
	case "StartMcpOAuth":
		var in contracts.StartMcpOAuthInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.StartMcpOAuth(in), nil
	case "CancelMcpOAuth":
		var id string
		if err := unmarshalOne(params, &id); err != nil {
			return nil, err
		}
		return d.CancelMcpOAuth(id), nil
	case "WritePty":
		var in contracts.PtyInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.WritePty(in), nil
	case "ResizePty":
		var in contracts.PtyInput
		if err := unmarshalOne(params, &in); err != nil {
			return nil, err
		}
		return d.ResizePty(in), nil
	case "SaveTextFile":
		return contracts.SaveFileResponse{
			Error: contracts.NewAppError(contracts.ErrUnimplemented, "http-live：请用浏览器下载；另存为/访达仅桌面壳", false),
		}, nil
	case "RestartApp":
		var in contracts.RestartAppInput
		_ = unmarshalOne(params, &in)
		return d.RestartApp(in), nil
	default:
		return nil, fmt.Errorf("未知方法 %s", method)
	}
}

func unmarshalOne(params json.RawMessage, dest any) error {
	if len(params) == 0 || string(params) == "null" {
		return nil
	}
	if params[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(params, &arr); err != nil {
			return err
		}
		if len(arr) == 0 {
			return nil
		}
		return json.Unmarshal(arr[0], dest)
	}
	return json.Unmarshal(params, dest)
}

func (s *HTTPLiveServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan contracts.Event, 32)
	s.mu.Lock()
	s.eventSubs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.eventSubs, ch)
		s.mu.Unlock()
		close(ch)
	}()
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func (s *HTTPLiveServer) handleEngineProgress(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan contracts.EngineProgressPayload, 16)
	s.mu.Lock()
	s.engSubs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.engSubs, ch)
		s.mu.Unlock()
		close(ch)
	}()
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case p := <-ch:
			b, _ := json.Marshal(p)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// workspaceShortcut 浏览器 live 快捷工作区（绝对路径；非系统选目录）。
type workspaceShortcut struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

func listWorkspaceShortcuts() []workspaceShortcut {
	var out []workspaceShortcut
	seen := map[string]bool{}
	add := func(id, label, path string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || seen[path] {
			return
		}
		st, err := os.Stat(path)
		if err != nil || !st.IsDir() {
			return
		}
		seen[path] = true
		out = append(out, workspaceShortcut{ID: id, Label: label, Path: path})
	}
	cwd, _ := os.Getwd()
	roots := []string{cwd}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		add("demos-workspace", "演示工作区（demos/workspace）", filepath.Join(root, "demos", "workspace"))
		add("demos-workspace", "演示工作区（demos/workspace）", filepath.Join(root, "..", "demos", "workspace"))
	}
	return out
}

// ValidateWorkspacePath http-live 工作区降级：绝对路径目录校验（非系统面板）。
func ValidateWorkspacePath(path string) contracts.WorkspaceResponse {
	path = strings.TrimSpace(path)
	if path == "" {
		return contracts.WorkspaceResponse{Cancelled: true}
	}
	if !filepath.IsAbs(path) {
		return contracts.WorkspaceResponse{
			Error: contracts.NewAppError(contracts.ErrInvalidArgument, "http-live 须填绝对路径（非系统目录面板）", false),
		}
	}
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return contracts.WorkspaceResponse{
			Error: contracts.NewAppError(contracts.ErrInvalidArgument, "路径非法", false),
		}
	}
	info, err := os.Stat(clean)
	if err != nil {
		return contracts.WorkspaceResponse{Error: mapError(err)}
	}
	if !info.IsDir() {
		return contracts.WorkspaceResponse{
			Error: contracts.NewAppError(contracts.ErrInvalidArgument, "须为目录", false),
		}
	}
	return contracts.WorkspaceResponse{Path: clean}
}
