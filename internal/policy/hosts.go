package policy

import (
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// RememberHost 记录项目或会话级已允许主机。不含密钥。
func (b *Builder) RememberHost(host, workspace, scope string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	workspace = strings.TrimSpace(workspace)
	scope = strings.TrimSpace(scope)
	if host == "" || strings.Contains(host, "/") || (scope != "project" && scope != "session") {
		return ErrHostMismatch
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range b.hosts {
		if item.Host == host && item.Workspace == workspace && item.Scope == scope {
			return nil
		}
	}
	b.hosts = append(b.hosts, contracts.RememberedHost{Host: host, Workspace: workspace, Scope: scope})
	return nil
}

// ListRememberedHosts 按工作区过滤；workspace 为空时返回全部。
func (b *Builder) ListRememberedHosts(workspace string) []contracts.RememberedHost {
	workspace = strings.TrimSpace(workspace)
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]contracts.RememberedHost, 0)
	for _, item := range b.hosts {
		if workspace == "" || item.Workspace == workspace {
			out = append(out, item)
		}
	}
	return out
}

// RevokeRememberedHost 删除匹配项。不存在时也成功。
func (b *Builder) RevokeRememberedHost(host, workspace string) {
	host = strings.ToLower(strings.TrimSpace(host))
	workspace = strings.TrimSpace(workspace)
	b.mu.Lock()
	defer b.mu.Unlock()
	kept := b.hosts[:0]
	for _, item := range b.hosts {
		if item.Host == host && item.Workspace == workspace {
			continue
		}
		kept = append(kept, item)
	}
	b.hosts = kept
}

// AllowedHostNames 返回固定 profile 与已记住主机，供权限一览使用。
func (b *Builder) AllowedHostNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(host string) {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" || seen[host] {
			return
		}
		seen[host] = true
		out = append(out, host)
	}
	for _, prof := range b.profiles {
		add(prof.Host)
	}
	for _, item := range b.ListRememberedHosts("") {
		add(item.Host)
	}
	if out == nil {
		out = []string{}
	}
	return out
}
