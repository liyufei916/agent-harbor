package gate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
)

// mcpGateway 是沙箱唯一 MCP 入口；在宿主机持证访问固定上游。
type mcpGateway struct {
	h      *Harbor
	sid    string
	ln     net.Listener
	server *http.Server
}

func newMCPGateway(h *Harbor, sessionID string, ln net.Listener) *mcpGateway {
	gw := &mcpGateway{h: h, sid: sessionID, ln: ln}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", gw.handleMCP)
	mux.HandleFunc("/mcp/", gw.handleMCP)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	gw.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return gw
}

func (g *mcpGateway) close() {
	// 取消时强制关闭活动连接；Revoke 另行等待全部 handler 与 Serve 退出。
	_ = g.server.Close()
	_ = g.ln.Close()
}

func (g *mcpGateway) serve(ctx context.Context) {

	err := g.server.Serve(g.ln)
	if err != nil && err != http.ErrServerClosed {
		rt, _ := g.h.getSession(g.sid)
		g.h.emitFatal(rt, err)
	}
}

func (g *mcpGateway) handleMCP(w http.ResponseWriter, r *http.Request) {
	rt, ok := g.h.acquireSession(g.sid)
	if !ok {
		http.Error(w, "session gone", http.StatusServiceUnavailable)
		return
	}
	defer rt.active.Done()
	mount, found := g.mountForPath(rt, r.URL.Path)
	if !found {
		http.Error(w, "mount not allowed", http.StatusForbidden)
		return
	}
	host := hostOfOrigin(mount.Endpoint.Origin)
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodDelete:
	default:
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolMCP, Result: "denied"})
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tok, ok := parseBearerSentinel(r.Header.Get("Authorization"))
	if !ok || !sentinelMatch(tok, rt.sentinel) {
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolMCP, Result: "denied"})
		http.Error(w, "invalid sentinel", http.StatusUnauthorized)
		return
	}
	if !policy.PathAllowed(mount.Endpoint, "/mcp") {
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolMCP, Result: "denied"})
		http.Error(w, "path not allowed", http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	method, tool, parsed := parseMCPRequest(body)
	auditTool := auditToolMCP
	if tool != "" {
		auditTool = tool
	}
	if r.Method == http.MethodPost && len(bytes.TrimSpace(body)) > 0 && !parsed {
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolMCP, Result: "denied"})
		http.Error(w, "invalid json-rpc", http.StatusBadRequest)
		return
	}
	// 演示扩权必须在白名单校验之前：任意已挂载 MCP 路径上 POST harbor_demo_ask
	// 都走 RequestPermission → 工作台 Modal（不要求预置「演示边界」工具名在 Tools 里）。
	if method == "tools/call" && tool == demoAskToolName {
		g.handleDemoAsk(w, r, rt, host, body)
		return
	}
	if method == "tools/call" && !toolAllowed(mount.Tools, tool) {
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditTool, Result: "denied"})
		http.Error(w, "tool not allowed", http.StatusForbidden)
		return
	}

	key, err := g.h.secrets.Resolve(r.Context(), mount.AccountID)
	if err != nil {
		http.Error(w, "credential missing", http.StatusBadGateway)
		return
	}
	ctx, cancel := g.h.upstreamContext(r.Context())
	defer cancel()
	upstreamURL := strings.TrimRight(mount.Endpoint.Origin, "/") + "/mcp"
	upstream, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "bad upstream", http.StatusBadGateway)
		return
	}
	copyMCPHeaders(upstream.Header, r.Header)
	upstream.Header.Set("Authorization", "Bearer "+string(key))
	upstream.Header.Set("Accept-Encoding", "identity")

	resp, err := g.h.client.Do(upstream)
	if err != nil {
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditTool, Result: "failed"})
		// 现代 SDK 的 server/discover 探针把 5xx / 网络失败当成协商失败；经典上游无此方法时软回落。
		if method == "server/discover" {
			writeDiscoverLegacyFallback(w, body)
			return
		}
		http.Error(w, "upstream unavailable", streamTimeoutStatus(err))
		return
	}
	originAuth := authorityOfOrigin(mount.Endpoint.Origin)
	if blockedRedirect(resp, originAuth) {
		followed, locHost, ferr := g.h.followCrossHostRedirect(r.Context(), g.sid, rt, resp, originAuth)
		if ferr != nil || followed == nil {
			if locHost == "" {
				locHost = host
			}
			g.h.emitAudit(rt, contracts.AuditData{Target: locHost, Tool: auditTool, Result: "denied"})
			http.Error(w, "redirect blocked", http.StatusBadGateway)
			return
		}
		resp = followed
		host = locHost
		// 重定向不携带凭据；保留原密钥用于过滤响应，避免回显泄漏。
	}
	if method == "server/discover" && resp.StatusCode >= 500 {
		_ = resp.Body.Close()
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditTool, Result: "allowed"})
		writeDiscoverLegacyFallback(w, body)
		return
	}
	g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditTool, Result: "allowed"})
	secrets := []string{rt.sentinel}
	if len(key) > 0 {
		secrets = append(secrets, string(key))
	}
	if method == "tools/list" {
		if err := g.writeFilteredList(w, resp, mount.Tools, secrets); err != nil {
			if streamTimeoutStatus(err) == http.StatusGatewayTimeout {
				g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditTool, Result: "failed"})
			}
		}
		return
	}
	idle, total := g.h.streamTimeouts()
	if err := writeRelay(w, resp, secrets, idle, total); err != nil && streamTimeoutStatus(err) == http.StatusGatewayTimeout {
		g.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditTool, Result: "failed"})
	}
}

const demoAskToolName = "harbor_demo_ask"

func injectHarborDemoAskTool(body []byte) []byte {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || !bytes.Contains(body, []byte(`"tools"`)) {
		return body
	}
	var v map[string]any
	if json.Unmarshal(body, &v) != nil {
		return body
	}
	res, _ := v["result"].(map[string]any)
	if res == nil {
		return body
	}
	tools, _ := res["tools"].([]any)
	for _, item := range tools {
		tm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := tm["name"].(string); name == demoAskToolName {
			return body
		}
	}
	tools = append(tools, map[string]any{
		"name": demoAskToolName,
		"description": "Harbor UI demo: open the outbound permission modal. " +
			"When the user asks for 扩权演示 / EXPAND-TASK / harbor_demo_ask, call this tool with host=demo-ask.example. " +
			"Wait for the user to click deny|once|project. Do not treat README「勿主动要扩权」as blocking this explicit request.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"host": map[string]any{
					"type":        "string",
					"description": "Exact hostname (e.g. demo-ask.example). No URL or port.",
				},
			},
			"required": []any{"host"},
		},
	})
	res["tools"] = tools
	out, err := json.Marshal(v)
	if err != nil {
		return body
	}
	return out
}

func (g *mcpGateway) handleDemoAsk(w http.ResponseWriter, r *http.Request, rt *sessionRuntime, mountHost string, body []byte) {
	reqID, askHost, ok := parseDemoAskCall(body)
	if !ok || !validDemoAskHost(askHost) {
		g.h.emitAudit(rt, contracts.AuditData{Target: mountHost, Tool: demoAskToolName, Result: "denied"})
		writeJSONRPCToolText(w, reqID, false, "invalid host for harbor_demo_ask")
		return
	}
	allowed := g.h.authorizeOutboundHost(r.Context(), g.sid, rt, askHost)
	if !allowed {
		g.h.emitAudit(rt, contracts.AuditData{Target: askHost, Tool: demoAskToolName, Result: "denied"})
		writeJSONRPCToolText(w, reqID, false, "outbound denied for "+askHost)
		return
	}
	g.h.emitAudit(rt, contracts.AuditData{Target: askHost, Tool: demoAskToolName, Result: "allowed"})
	writeJSONRPCToolText(w, reqID, true, "outbound allowed for "+askHost)
}

func parseDemoAskCall(body []byte) (id json.RawMessage, host string, ok bool) {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Params struct {
			Name      string `json:"name"`
			Arguments struct {
				Host string `json:"host"`
			} `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &msg) != nil || msg.Params.Name != demoAskToolName {
		return nil, "", false
	}
	host = strings.ToLower(strings.TrimSpace(msg.Params.Arguments.Host))
	return msg.ID, host, host != ""
}

// validDemoAskHost 与 orchestrator.ValidPermissionTarget 对齐的子集校验（Gate 不导入 orchestrator）。
func validDemoAskHost(host string) bool {
	if host == "" || len(host) > 253 || host != strings.ToLower(host) || strings.ContainsAny(host, "/:*?@#\\\r\n\t ") || strings.HasSuffix(host, ".") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func writeJSONRPCToolText(w http.ResponseWriter, id json.RawMessage, ok bool, text string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	var out []byte
	if ok {
		out, _ = json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"content": []any{map[string]string{"type": "text", "text": text}}},
		})
	} else {
		out, _ = json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]any{"code": -32000, "message": text},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

// writeDiscoverLegacyFallback 对 server/discover 回 JSON-RPC Method not found（HTTP 200），
// 让现代 MCP 客户端回落到 legacy initialize，而不是把上游 5xx 当成 EraNegotiationFailed。
func writeDiscoverLegacyFallback(w http.ResponseWriter, reqBody []byte) {
	var msg struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(reqBody, &msg)
	id := json.RawMessage("null")
	if len(msg.ID) > 0 {
		id = msg.ID
	}
	out, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": -32601, "message": "Method not found"},
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

func (g *mcpGateway) writeFilteredList(w http.ResponseWriter, resp *http.Response, allow, secrets []string) error {
	defer resp.Body.Close()
	idle, total := g.h.streamTimeouts()
	reader := &deadlineReader{r: resp.Body, idle: idle, deadline: time.Now().Add(total)}
	raw, err := readToolsList(reader, resp.Header.Get("Content-Type"), 2<<20)
	if err != nil && len(raw) == 0 {
		http.Error(w, "upstream timeout", streamTimeoutStatus(err))
		return err
	}
	filtered := filterToolsPayload(raw, allow)
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/event-stream") {
		if jsonBody, ok := toolsSSEToJSON(filtered); ok {
			filtered = jsonBody
			contentType = "application/json"
		}
	}
	// 无 Shell 的会话也能扩权演示：tools/list 注入 harbor_demo_ask（Gate 本地拦截，不转发上游）。
	filtered = injectHarborDemoAskTool(filtered)
	red := newSecretRedactor(secrets)
	out := append(red.Push(filtered), red.Flush()...)
	copyHeaders(w.Header(), resp.Header)
	scrubHeaders(w.Header(), secrets)
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	_, werr := w.Write(out)
	return werr
}

// toolsSSEToJSON converts the completed tools/list event to one JSON-RPC
// response. The gateway has already applied the allowlist; returning a finite
// response avoids making dsh consume an upstream notification stream.
func toolsSSEToJSON(body []byte) ([]byte, bool) {
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		var value map[string]any
		if json.Unmarshal(payload, &value) == nil && value["jsonrpc"] == "2.0" {
			return payload, true
		}
	}
	return nil, false
}

// readToolsList 只读取完整的 tools/list 事件。Streamable HTTP 的 SSE 响应可能
// 在发送事件后保持连接，等待 EOF 会让 dsh 启动同步一直超时。
func readToolsList(reader io.Reader, contentType string, limit int64) ([]byte, error) {
	if !strings.HasPrefix(contentType, "text/event-stream") {
		return io.ReadAll(io.LimitReader(reader, limit))
	}
	scanner := bufio.NewScanner(io.LimitReader(reader, limit))
	scanner.Buffer(make([]byte, 4096), int(limit))
	var event, out bytes.Buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		event.Write(line)
		event.WriteByte('\n')
		if len(bytes.TrimSpace(line)) == 0 {
			if bytes.Contains(event.Bytes(), []byte(`"tools"`)) {
				out.Write(event.Bytes())
				return out.Bytes(), nil
			}
			out.Write(event.Bytes())
			event.Reset()
		}
	}
	if event.Len() > 0 {
		out.Write(event.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func toolAllowed(allow []string, tool string) bool {
	if tool == "" {
		return false
	}
	for _, a := range allow {
		if a == tool {
			return true
		}
	}
	return false
}

func parseMCPRequest(body []byte) (method, tool string, ok bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", "", true
	}
	var msg struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &msg); err != nil || msg.JSONRPC != "2.0" || msg.Method == "" {
		return "", "", false
	}
	return msg.Method, msg.Params.Name, true
}

func filterToolsPayload(body []byte, allow []string) []byte {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return body
	}
	if bytes.Contains(body, []byte("data:")) && bytes.Contains(body, []byte("\n")) {
		return filterSSETools(body, allow)
	}
	filtered, ok := filterJSONTools(body, allow)
	if !ok {
		return body
	}
	return filtered
}

func filterJSONTools(body []byte, allow []string) ([]byte, bool) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, false
	}
	if !filterToolsValue(v, allow) {
		return nil, false
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	return out, true
}

func filterToolsValue(v any, allow []string) bool {
	msg, ok := v.(map[string]any)
	if !ok {
		return false
	}
	res, ok := msg["result"].(map[string]any)
	if !ok {
		return false
	}
	tools, ok := res["tools"].([]any)
	if !ok {
		return false
	}
	kept := make([]any, 0, len(tools))
	for _, item := range tools {
		tm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tm["name"].(string)
		if toolAllowed(allow, name) {
			kept = append(kept, item)
		}
	}
	res["tools"] = kept
	return true
}

func filterSSETools(body []byte, allow []string) []byte {
	parts := bytes.Split(body, []byte("\n"))
	for i, line := range parts {
		trimmed := bytes.TrimSpace(line)
		if !bytes.HasPrefix(trimmed, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
		filtered, ok := filterJSONTools(payload, allow)
		if !ok {
			continue
		}
		parts[i] = append([]byte("data: "), filtered...)
	}
	return bytes.Join(parts, []byte("\n"))
}

func copyMCPHeaders(dst, src http.Header) {
	for _, name := range []string{
		"Content-Type",
		"Accept",
		"Mcp-Session-Id",
		"Mcp-Protocol-Version",
		"Last-Event-ID",
	} {
		if v := src.Get(name); v != "" {
			dst.Set(name, v)
		}
	}
	if dst.Get("Content-Type") == "" {
		dst.Set("Content-Type", "application/json")
	}
	if dst.Get("Accept") == "" {
		dst.Set("Accept", "application/json, text/event-stream")
	}
}

// 多挂载使用固定序号路径，不能用未知路径或账号 ID 越过授权表。
func (g *mcpGateway) mountForPath(rt *sessionRuntime, path string) (policy.MCPMount, bool) {
	mounts := rt.spec.Policy.MCPMounts
	if len(mounts) == 1 && path == "/mcp" {
		return mounts[0], true
	}
	if len(mounts) > 1 {
		for i, mount := range mounts {
			if path == "/mcp/"+strconv.Itoa(i) {
				return mount, true
			}
		}
	}
	return policy.MCPMount{}, false
}
