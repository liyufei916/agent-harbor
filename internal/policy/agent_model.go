package policy

import (
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// DefaultAgentsForProfile 未显式勾选 Agents 时的单绑默认。
func DefaultAgentsForProfile(profileID string) []string {
	switch strings.TrimSpace(profileID) {
	case ProfileCodexModel:
		return []string{"codex"}
	case ProfileDSHModel:
		return []string{"dsh"}
	case ProfileDemoModel, ProfileCustom:
		return []string{"claude"}
	default:
		return nil
	}
}

// NormalizeModelAgents 去重、只保留已知 Agent；空输入按 profile 默认。
func NormalizeModelAgents(profileID string, agents []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "claude" && id != "codex" && id != "dsh" {
			return
		}
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, a := range agents {
		add(a)
	}
	if len(out) == 0 {
		for _, a := range DefaultAgentsForProfile(profileID) {
			add(a)
		}
	}
	return out
}

// AccountAllowsAgent 判断模型账号是否可供该 Agent 开会话。
func AccountAllowsAgent(account contracts.AccountInfo, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	agents := append([]string(nil), account.Agents...)
	if len(agents) == 0 {
		agents = DefaultAgentsForProfile(account.ProfileID)
	}
	if len(agents) == 0 {
		// 未知/测试 profile 且未声明 agents：不在此拦截（兼容单测假账号）。
		return true
	}
	for _, a := range agents {
		if strings.TrimSpace(a) == agentID {
			return true
		}
	}
	return false
}

// ModelProfileForAgent 按会话 Agent 从账号元信息合成出站 FixedProfile。
// 多 Agent 共用同一 Host/Key：Claude/dsh 走 Anthropic Messages，Codex 走 Responses。
func ModelProfileForAgent(agentID string, account contracts.AccountInfo, registered map[string]FixedProfile) (FixedProfile, error) {
	if !AccountAllowsAgent(account, agentID) {
		return FixedProfile{}, ErrKindMismatch
	}
	host := strings.TrimSpace(account.Host)
	if host == "" {
		return FixedProfile{}, ErrUnknownProfile
	}
	modelName := strings.TrimSpace(account.Model)
	basePath := strings.TrimRight(strings.TrimSpace(account.BasePath), "/")

	// 仍挂固定 profile 且仅单 Agent 时，优先用登记表（含 env Origin）。
	if len(NormalizeModelAgents(account.ProfileID, account.Agents)) == 1 {
		switch agentID {
		case "claude":
			if account.ProfileID == ProfileDemoModel {
				if p, ok := registered[ProfileDemoModel]; ok && p.Kind == "model" {
					return p, nil
				}
			}
		case "dsh":
			if account.ProfileID == ProfileDSHModel {
				if p, ok := registered[ProfileDSHModel]; ok && p.Kind == "model" {
					return p, nil
				}
			}
		case "codex":
			if account.ProfileID == ProfileCodexModel {
				if p, ok := registered[ProfileCodexModel]; ok && p.Kind == "model" {
					return p, nil
				}
			}
		}
	}

	switch agentID {
	case "claude", "dsh":
		origin := "https://" + host
		if basePath != "" {
			origin = origin + basePath
		}
		custom, err := CustomModelProfile(origin, modelName)
		if err != nil {
			return FixedProfile{}, ErrUnknownProfile
		}
		if basePath != "" {
			if !ValidBasePath(basePath) {
				return FixedProfile{}, ErrUnknownProfile
			}
			custom.BasePath = basePath
		}
		if !custom.ValidConnection() {
			return FixedProfile{}, ErrUnknownProfile
		}
		return custom, nil
	case "codex":
		if modelName == "" {
			modelName = "gpt-5-codex"
		}
		p := FixedProfile{
			ID:       ProfileCodexModel,
			Kind:     "model",
			Protocol: "openai-responses",
			Model:    modelName,
			Origin:   "https://" + host,
			Host:     strings.ToLower(host),
			BasePath: "",
			Paths:    []string{"/v1/responses"},
			TestPath: "/v1/responses",
		}
		if !p.ValidConnection() {
			return FixedProfile{}, ErrUnknownProfile
		}
		return p, nil
	default:
		return FixedProfile{}, ErrInvalidAgent
	}
}
