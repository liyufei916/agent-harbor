// 本文件按 docs/CONTRACT.md v1 定义，不包含业务实现。
package kits

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"strconv"
	"strings"

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
	// MCPIDs 与 MCPURLs 同序的账号 ID；用于稳定 MCP 服务器名（与勾选顺序无关）。
	MCPIDs   []string
	Sentinel string
	// Model：账号探测落盘的上游模型名（Gate 改写出站 JSON）。Claude CLI 的 --model 另用 wireModel，勿直接传非 Anthropic ID。
	Model string
	// Workspace：Runtime 实际工作区（direct 为源路径，clone 为会话副本）；Agent 须在此目录作业。
	Workspace string
}

type Kit interface {
	Detect(context.Context) (KitInfo, error)
	Build(context.Context, KitInput) (runtime.LaunchSpec, error)
}

// MCPName 按账号 ID 派生稳定的受信服务器名（工具命名空间不随挂载勾选顺序变化）。
// accountID 为空时回退按 index 命名（仅单测/无 ID 兼容路径）。
func MCPName(accountID string, index int) string {
	id := strings.TrimSpace(accountID)
	if id == "" {
		if index == 0 {
			return "harbor"
		}
		return "harbor-" + strconv.Itoa(index+1)
	}
	sum := sha256.Sum256([]byte(id))
	return "harbor-" + hex.EncodeToString(sum[:8])
}

// MCPServerName 取第 index 个挂载的服务器名（优先 MCPIDs[index]）。
func MCPServerName(input KitInput, index int) string {
	id := ""
	if index >= 0 && index < len(input.MCPIDs) {
		id = input.MCPIDs[index]
	}
	return MCPName(id, index)
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
	if n := len(input.MCPIDs); n > 0 && n != len(urls) {
		return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 账号与地址数量不一致。", false)
	}
	seen := map[string]bool{}
	seenName := map[string]bool{}
	for i, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || seen[raw] {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 地址不合法。", false)
		}
		host, port, err := net.SplitHostPort(u.Host)
		number, portErr := strconv.Atoi(port)
		if err != nil || host != "127.0.0.1" || portErr != nil || number < 1 || number > 65535 || strconv.Itoa(number) != port {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 地址不合法。", false)
		}
		name := MCPServerName(input, i)
		if seenName[name] {
			return nil, contracts.NewAppError(contracts.ErrInvalidArgument, "MCP 服务器名冲突。", false)
		}
		seen[raw] = true
		seenName[name] = true
	}
	return append([]string{}, urls...), nil
}
