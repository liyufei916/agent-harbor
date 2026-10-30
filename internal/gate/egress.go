package gate

import (
	"fmt"
	"net"
	"strings"
)

// lookupIP 可在单测中替换。
var lookupIP = net.LookupIP

// insecureRedirectOK 仅单测注入（允许 http 跟随）；生产闭包恒 false。
var insecureRedirectOK = func() bool { return false }

// ValidatePublicHostname 要求主机名可解析且全部地址为公网全局单播。
// 拒绝字面量/解析结果中的回环、私网、链路本地、组播、未指定与 CGNAT。
// 生产路径恒不允许回环；测试见 validatePublicHostname。
func ValidatePublicHostname(host string) error {
	_, err := resolvePublicHostname(host, false)
	return err
}

func resolvePublicHostname(host string, allowLoopback bool) ([]net.IP, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || strings.ContainsAny(host, "/:#?@") {
		return nil, fmt.Errorf("gate: invalid egress host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedEgressIP(ip, allowLoopback) {
			return nil, fmt.Errorf("gate: blocked egress address")
		}
		return []net.IP{append(net.IP{}, ip...)}, nil
	}
	ips, err := lookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("gate: dns lookup failed")
	}
	for _, ip := range ips {
		if isBlockedEgressIP(ip, allowLoopback) {
			return nil, fmt.Errorf("gate: host resolves to blocked address")
		}
	}
	result := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		result = append(result, append(net.IP{}, ip...))
	}
	return result, nil
}

func isBlockedEgressIP(ip net.IP, allowLoopback bool) bool {
	if ip == nil {
		return true
	}
	if allowLoopback && ip.IsLoopback() {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// CGNAT 100.64.0.0/10
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
		// 文档/基准 0.0.0.0/8 已由 IsUnspecified 覆盖部分；拒绝 0.x
		if v4[0] == 0 {
			return true
		}
	}
	return false
}
