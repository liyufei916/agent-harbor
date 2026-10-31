package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
)

// maxModelProxyRewriteBody 是改写顶层 model 时允许缓冲的请求体上限。
// 旧值 2MiB 会在多图/tool_result 场景截断 JSON（Anthropic 报 EOF @ column 2097152）。
const maxModelProxyRewriteBody = 32 << 20

var errModelProxyBodyTooLarge = errors.New("model proxy request body too large")

// modelProxy 是固定模型 HTTPS 上游的显式 Base URL 反向代理。
// 不实现通用 CONNECT / HTTPS MITM。
type modelProxy struct {
	h      *Harbor
	sid    string
	ln     net.Listener
	server *http.Server
}

func newModelProxy(h *Harbor, sessionID string, ln net.Listener) *modelProxy {
	p := &modelProxy{h: h, sid: sessionID, ln: ln}
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handle)
	p.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return p
}

func (p *modelProxy) close() {
	// 取消时强制关闭活动连接；Revoke 另行等待全部 handler 与 Serve 退出。
	_ = p.server.Close()
	_ = p.ln.Close()
}

func (p *modelProxy) serve(ctx context.Context) {

	err := p.server.Serve(p.ln)
	if err != nil && err != http.ErrServerClosed {
		rt, _ := p.h.getSession(p.sid)
		p.h.emitFatal(rt, err)
	}
}

func (p *modelProxy) handle(w http.ResponseWriter, r *http.Request) {
	rt, modelAccountID, ok := p.h.acquireModelSession(p.sid)
	if !ok {
		http.Error(w, "session gone", http.StatusServiceUnavailable)
		return
	}
	defer rt.active.Done()
	originAuth := authorityOfOrigin(rt.spec.Policy.Model.Origin)
	host := hostOfOrigin(rt.spec.Policy.Model.Origin)
	reqPath := normalizeModelRequestPath(rt.spec.Policy.Model, r.URL.Path)
	if !policy.PathAllowed(rt.spec.Policy.Model, reqPath) {
		p.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolModel, Result: "denied"})
		http.Error(w, "path not allowed", http.StatusForbidden)
		return
	}
	if !p.authorize(r, rt) {
		p.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolModel, Result: "denied"})
		http.Error(w, "invalid sentinel", http.StatusUnauthorized)
		return
	}

	if !policy.ValidBasePath(rt.spec.Policy.Model.BasePath) {
		http.Error(w, "invalid model route", http.StatusForbidden)
		return
	}
	modelURL := *r.URL
	modelURL.Path = policy.JoinBasePath(rt.spec.Policy.Model.BasePath, reqPath)
	upstreamURL, err := joinOrigin(rt.spec.Policy.Model.Origin, &modelURL)
	if err != nil {
		http.Error(w, "bad upstream", http.StatusBadGateway)
		return
	}
	ctx, cancel := p.h.upstreamContext(r.Context())
	defer cancel()
	body := io.Reader(r.Body)
	// 账号配置了模型名时，改写出站 JSON 的 model，避免 Claude 默认 claude-sonnet-* 打到只允许 GPT 的 Key。
	wantModel := strings.TrimSpace(rt.spec.Policy.Model.Model)
	if wantModel == "" {
		if acc, err := p.h.secrets.Lookup(r.Context(), modelAccountID); err == nil {
			wantModel = strings.TrimSpace(acc.Model)
		}
	}
	if r.Method == http.MethodPost && wantModel != "" && r.Body != nil {
		rewritten, err := rewriteJSONModelField(r.Body, wantModel)
		if err != nil {
			if errors.Is(err, errModelProxyBodyTooLarge) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad request body", http.StatusBadRequest)
			return
		}
		body = rewritten
		r.Body = io.NopCloser(bytes.NewReader(nil))
	}
	outReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadGateway)
		return
	}
	copyHeaders(outReq.Header, r.Header)
	outReq.Header.Set("Accept-Encoding", "identity")
	if buf, ok := body.(*bytes.Reader); ok {
		outReq.ContentLength = int64(buf.Len())
		outReq.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	}
	key, err := p.h.secrets.Resolve(r.Context(), modelAccountID)
	if err != nil {
		http.Error(w, "credential missing", http.StatusBadGateway)
		return
	}
	p.applyModelKey(outReq, string(key))

	resp, err := p.h.client.Do(outReq)
	if err != nil {
		p.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolModel, Result: "failed"})
		http.Error(w, "upstream unavailable", streamTimeoutStatus(err))
		return
	}
	if blockedRedirect(resp, originAuth) {
		followed, locHost, ferr := p.h.followCrossHostRedirect(r.Context(), p.sid, rt, resp, originAuth)
		if ferr != nil || followed == nil {
			if locHost == "" {
				locHost = host
			}
			p.h.emitAudit(rt, contracts.AuditData{Target: locHost, Tool: auditToolModel, Result: "denied"})
			http.Error(w, "redirect blocked", http.StatusBadGateway)
			return
		}
		resp = followed
		host = locHost
		// 不向重定向目标注入凭据，但响应脱敏仍覆盖原账号和 Sentinel。
	}
	p.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolModel, Result: "allowed"})
	idle, total := p.h.streamTimeouts()
	secretsToRedact := []string{rt.sentinel}
	if len(key) > 0 {
		secretsToRedact = append(secretsToRedact, string(key))
	}
	scan := &usageScanner{}
	if err := writeRelayScan(w, resp, secretsToRedact, idle, total, scan); err != nil {
		if streamTimeoutStatus(err) == http.StatusGatewayTimeout {
			p.h.emitAudit(rt, contracts.AuditData{Target: host, Tool: auditToolModel, Result: "failed"})
		}
		return
	}
	if in, out := scan.Totals(); in > 0 || out > 0 {
		p.h.emitAudit(rt, contracts.AuditData{
			Target: host, Tool: auditToolModel, Result: "allowed",
			InputTokens: in, OutputTokens: out,
		})
	}
}

func (p *modelProxy) authorize(r *http.Request, rt *sessionRuntime) bool {
	if tok, ok := parseBearerSentinel(r.Header.Get("Authorization")); ok && sentinelMatch(tok, rt.sentinel) {
		return true
	}
	for _, name := range []string{"X-Api-Key", "Api-Key", "x-api-key"} {
		if looksLikeSentinel(r.Header.Get(name)) && sentinelMatch(strings.TrimSpace(r.Header.Get(name)), rt.sentinel) {
			return true
		}
	}
	return false
}

func (p *modelProxy) applyModelKey(req *http.Request, keyStr string) {
	if auth := req.Header.Get("Authorization"); looksLikeSentinel(auth) {
		req.Header.Set("Authorization", "Bearer "+keyStr)
	}
	for _, name := range []string{"X-Api-Key", "Api-Key", "x-api-key"} {
		if looksLikeSentinel(req.Header.Get(name)) {
			req.Header.Set(name, keyStr)
		}
	}
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+keyStr)
	}
}

// normalizeModelRequestPath：Codex 在 base_url 无 /v1 时会 POST /responses；
// 若策略已放行 /v1/responses，则归一到该路径再做白名单与上游拼接。
func normalizeModelRequestPath(rule policy.EndpointRule, path string) string {
	if path == "/responses" && policy.PathAllowed(rule, "/v1/responses") {
		return "/v1/responses"
	}
	return path
}

// rewriteJSONModelField 将 JSON 对象顶层 "model" 改为账号模型名；非 JSON 则原样透传。
// 超过 maxModelProxyRewriteBody 时返回 errModelProxyBodyTooLarge（禁止截断后转发）。
func rewriteJSONModelField(r io.Reader, model string) (*bytes.Reader, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxModelProxyRewriteBody+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxModelProxyRewriteBody {
		return nil, errModelProxyBodyTooLarge
	}
	if len(raw) == 0 {
		return bytes.NewReader(nil), nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return bytes.NewReader(raw), nil
	}
	enc, err := json.Marshal(model)
	if err != nil {
		return bytes.NewReader(raw), nil
	}
	obj["model"] = enc
	out, err := json.Marshal(obj)
	if err != nil {
		return bytes.NewReader(raw), nil
	}
	return bytes.NewReader(out), nil
}

func joinOrigin(origin string, u *url.URL) (string, error) {
	base, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	rel := &url.URL{Path: u.Path, RawQuery: u.RawQuery}
	return base.ResolveReference(rel).String(), nil
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
