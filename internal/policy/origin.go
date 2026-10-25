package policy

import (
	"net"
	"net/url"
	"strings"
)

// ParseCustomOrigin 只接受 https 的主机级地址，拒绝路径、查询和用户信息。
func ParseCustomOrigin(raw string) (origin, host string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", "", ErrHostMismatch
	}
	name := strings.ToLower(u.Hostname())
	if name == "" {
		return "", "", ErrHostMismatch
	}
	host = name
	if port := u.Port(); port != "" && port != "443" {
		host = net.JoinHostPort(name, port)
	}
	return "https://" + strings.ToLower(u.Host), host, nil
}

// OriginFromAccountHost 从已保存的主机字段还原自定义 origin。
func OriginFromAccountHost(host string) (origin, normalized string, err error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || strings.Contains(host, "/") || strings.Contains(host, "@") {
		return "", "", ErrHostMismatch
	}
	return ParseCustomOrigin("https://" + host)
}
