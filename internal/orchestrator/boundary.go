package orchestrator

import (
	"context"
	"net/url"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
)

func sessionBoundary(p policy.PolicySnapshot, accounts []contracts.AccountInfo) contracts.PermissionOverview {
	result := contracts.PermissionOverview{AllowedHosts: []string{}, MCPSummary: []string{}, DeniedPathCategories: []string{"系统目录", "SSH 密钥与云厂商凭证", "其它项目路径"}}
	seen := map[string]bool{}
	add := func(origin string) {
		u, err := url.Parse(origin)
		if err == nil && u.Hostname() != "" && !seen[u.Hostname()] {
			seen[u.Hostname()] = true
			result.AllowedHosts = append(result.AllowedHosts, u.Hostname())
		}
	}
	add(p.Model.Origin)
	for _, mount := range p.MCPMounts {
		add(mount.Endpoint.Origin)
	}
	for _, account := range accounts {
		result.MCPSummary = append(result.MCPSummary, contracts.FormatMCPSummary(account))
	}
	return result
}

// Boundary 只报告本会话实际授权的资源，不把其它已连接账号当作已挂载。
func (o *Service) Boundary(ctx context.Context, id string) (contracts.PermissionOverview, error) {
	if ctx.Err() != nil {
		return contracts.PermissionOverview{}, publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.sessions[id]
	if !ok {
		return contracts.PermissionOverview{}, publicError("SESSION_NOT_FOUND")
	}
	result := s.boundary
	result.SessionID, result.Workspace = id, s.snapshot.Workspace
	result.AllowedHosts = append([]string{}, result.AllowedHosts...)
	result.MCPSummary = append([]string{}, result.MCPSummary...)
	result.DeniedPathCategories = append([]string{}, result.DeniedPathCategories...)
	return result, nil
}

// ProjectWorkspace 是宿主机内部的规则作用域，clone 副本不能作为项目标识。
func (o *Service) ProjectWorkspace(ctx context.Context, id string) (string, error) {
	if ctx.Err() != nil {
		return "", publicError("CANCELLED")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.sessions[id]
	if !ok {
		return "", publicError("SESSION_NOT_FOUND")
	}
	return s.spec.Workspace, nil
}
