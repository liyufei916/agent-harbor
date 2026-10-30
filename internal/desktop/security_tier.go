package desktop

import (
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

var tierStandardHosts = []string{
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"pypi.org",
	"files.pythonhosted.org",
	"proxy.golang.org",
	"sum.golang.org",
	"crates.io",
	"static.crates.io",
	"rubygems.org",
}

var tierRelaxedExtra = []string{
	"github.com",
	"api.github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
	"bitbucket.org",
	"objects.githubusercontent.com",
}

func normalizeSecurityTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "strict":
		return "strict"
	case "relaxed":
		return "relaxed"
	default:
		return "standard"
	}
}

func hostsForSecurityTier(tier string) []string {
	switch normalizeSecurityTier(tier) {
	case "strict":
		return nil
	case "relaxed":
		out := append([]string{}, tierStandardHosts...)
		return append(out, tierRelaxedExtra...)
	default:
		return append([]string{}, tierStandardHosts...)
	}
}

// mergeRememberedWithTier 将档位种子主机并入记住表（project 作用域），不覆盖用户已有条目。
func mergeRememberedWithTier(existing []contracts.RememberedHost, workspace, tier string) []contracts.RememberedHost {
	seen := map[string]struct{}{}
	out := make([]contracts.RememberedHost, 0, len(existing)+16)
	for _, h := range existing {
		host := strings.ToLower(strings.TrimSpace(h.Host))
		if host == "" {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, h)
	}
	ws := strings.TrimSpace(workspace)
	for _, host := range hostsForSecurityTier(tier) {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, contracts.RememberedHost{
			Host:      host,
			Workspace: ws,
			Scope:     "project",
		})
	}
	return out
}
