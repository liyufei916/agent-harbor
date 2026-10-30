package gate

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// authorizeOutboundHost 判定未授权主机：已记住 → 放行；否则 RequestPermission；deny/缺回调 → 拒绝。
func (h *Harbor) authorizeOutboundHost(ctx context.Context, sid string, rt *sessionRuntime, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if h.HasExtraHost(sid, host) {
		return true
	}
	if rt == nil || rt.spec.RequestPermission == nil {
		return false
	}
	decision, err := rt.spec.RequestPermission(ctx, contracts.PermissionNeeded{
		Target: host,
		Action: "connect",
	})
	if err != nil || (decision != "once" && decision != "project") {
		return false
	}
	_ = h.AllowExtraHost(sid, host, decision)
	if decision == "once" {
		_ = h.HasExtraHost(sid, host)
	}
	return true
}

// followCrossHostRedirect 在用户扩权且 DNS/IP 通过后跟随一跳；不注入原账号凭据。
func (h *Harbor) followCrossHostRedirect(ctx context.Context, sid string, rt *sessionRuntime, resp *http.Response, originAuth string) (*http.Response, string, error) {
	locURL, err := redirectAbsoluteURL(resp)
	defer resp.Body.Close()
	if err != nil || locURL == nil || locURL.Host == "" || locURL.User != nil {
		return nil, "", fmt.Errorf("gate: bad redirect")
	}
	locAuth := strings.ToLower(locURL.Host)
	locHost := strings.ToLower(locURL.Hostname())
	if strings.EqualFold(locAuth, originAuth) {
		return nil, locHost, fmt.Errorf("gate: same-origin redirect not blocked")
	}

	if !h.authorizeOutboundHost(ctx, sid, rt, locHost) {
		return nil, locHost, fmt.Errorf("gate: redirect not permitted")
	}
	ips, err := resolvePublicHostname(locHost, false)
	if err != nil {
		return nil, locHost, err
	}
	// 生产仅 https（默认/443）；单测注入 insecureRedirectOK 可放宽到 http（httptest）。
	if locURL.Scheme != "https" && !(locURL.Scheme == "http" && insecureRedirectOK()) {
		return nil, locHost, fmt.Errorf("gate: redirect scheme denied")
	}
	if port := locURL.Port(); port != "" && port != "443" {
		if !(insecureRedirectOK() && locURL.Scheme == "http") {
			return nil, locHost, fmt.Errorf("gate: redirect port denied")
		}
	}

	method := http.MethodGet
	if resp.Request != nil {
		method = redirectFollowMethod(resp.StatusCode, resp.Request.Method)
	}
	followReq, err := http.NewRequestWithContext(ctx, method, locURL.String(), nil)
	if err != nil {
		return nil, locHost, err
	}
	followReq.Header.Set("Accept-Encoding", "identity")
	if resp.Request != nil {
		if ua := resp.Request.Header.Get("User-Agent"); ua != "" {
			followReq.Header.Set("User-Agent", ua)
		}
	}

	// 只拨号本次解析并校验的 IP，禁止代理环境或再次 DNS 解析绕过私网拒绝。
	base, ok := h.client.Transport.(*http.Transport)
	if !ok {
		return nil, locHost, fmt.Errorf("gate: redirect transport cannot enforce address pinning")
	}
	transport := base.Clone()
	transport.Proxy = nil
	transport.DisableKeepAlives = true
	transport.DialTLSContext, transport.DialTLS = nil, nil
	dial := base.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	transport.DialContext = func(call context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || !strings.EqualFold(host, locHost) {
			return nil, fmt.Errorf("gate: unexpected redirect authority")
		}
		var last error
		for _, ip := range ips {
			connection, err := dial(call, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return connection, nil
			}
			last = err
			if call.Err() != nil {
				return nil, call.Err()
			}
		}
		return nil, last
	}
	client := *h.client
	client.Transport = transport
	defer transport.CloseIdleConnections()
	next, err := client.Do(followReq)
	if err != nil {
		return nil, locHost, err
	}
	if blockedRedirect(next, locAuth) {
		next.Body.Close()
		return nil, locHost, fmt.Errorf("gate: multi-hop redirect denied")
	}
	return next, locHost, nil
}

func blockedRedirect(resp *http.Response, allowedAuthority string) bool {
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return false
	}
	u, err := redirectAbsoluteURL(resp)
	if err != nil || u == nil || u.Host == "" {
		return false
	}
	return !strings.EqualFold(u.Host, allowedAuthority)
}

func redirectAbsoluteURL(resp *http.Response) (*url.URL, error) {
	loc := resp.Header.Get("Location")
	if loc == "" || resp.Request == nil || resp.Request.URL == nil {
		return nil, fmt.Errorf("missing location")
	}
	return resp.Request.URL.Parse(loc)
}

func redirectFollowMethod(status int, original string) string {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther:
		return http.MethodGet
	default:
		if original == "" {
			return http.MethodGet
		}
		return original
	}
}

func hostOfOrigin(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return origin
	}
	return u.Hostname()
}

func authorityOfOrigin(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(origin))
	}
	return strings.ToLower(u.Host)
}
