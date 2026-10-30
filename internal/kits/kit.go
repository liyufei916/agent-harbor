// 本文件按 docs/CONTRACT.md v1 定义，不包含业务实现。
package kits

import (
	"context"
	"net"
	"net/url"
	"strconv"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

type KitInfo struct {
	Agent               contracts.AgentInfo
	ReadOnlyPaths       []string // 固定 CLI 与必要运行文件的真实路径
	NeedsChildProcesses bool     // 需要受控子进程时由 Runtime 选择更严格的子进程策略（如 Codex）
	NeedsDescendants    bool     // Agent 会话需白名单内 fork + PTY ioctl，便于多轮与 Stop 杀树；Health 仍 deny
}

type KitInput struct {
	Task         string
	PrivateHome  string
	ModelBaseURL string
	MCPURL       string
	MCPURLs      []string
	Sentinel     string
	// Model：账号探测落盘的模型名；非空时覆盖 Kit 构造时的默认（如 HARBOR_CLAUDE_MODEL）。
	Model string
	// Workspace：Runtime 实际工作区（direct 为源路径，clone 为会话副本）；Agent 须在此目录作业。
	Workspace string
}

type Kit interface {
	Detect(context.Context) (KitInfo, error)
	Build(context.Context, KitInput) (runtime.LaunchSpec, error)
}

// MCPName 生成稳定的受信服务器名，工具命名空间不依赖任务或账号正文。
func MCPName(index int) string {
	if index == 0 {
		return "harbor"
	}
	return "harbor-" + strconv.Itoa(index+1)
}

// MCPGateURLs 兼容单挂载并拒绝歧义、重复或非 Gate 地址。
func MCPGateURLs(input KitInput) ([]string, error) {
	urls := input.MCPURLs
	if input.MCPURL != "" {
		if len(urls) > 0 {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 配置存在歧义。", false)
		}
		urls = []string{input.MCPURL}
	}
	if len(urls) > 8 {
		return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 挂载超出上限。", false)
	}
	seen := map[string]bool{}
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || seen[raw] {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 地址不合法。", false)
		}
		host, port, err := net.SplitHostPort(u.Host)
		number, portErr := strconv.Atoi(port)
		if err != nil || host != "127.0.0.1" || portErr != nil || number < 1 || number > 65535 || strconv.Itoa(number) != port {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 地址不合法。", false)
		}
		seen[raw] = true
	}
	return append([]string{}, urls...), nil
}
