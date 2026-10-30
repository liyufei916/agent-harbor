// Package policy 将已确认资源转为不可变会话授权快照。
// 负责人：研发 B。不负责网络转发或进程隔离。
package policy

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

var (
	ErrInvalidWorkspace = errors.New("policy: workspace denied")
	ErrInvalidAgent     = errors.New("policy: agent required")
	ErrUnknownProfile   = errors.New("policy: unknown profile")
	ErrKindMismatch     = errors.New("policy: account kind mismatch")
	ErrMissingMCP       = errors.New("policy: mcp required")
	ErrHostMismatch     = errors.New("policy: account host mismatch")
)

// EndpointRule 绑定固定 origin 与允许路径。
type EndpointRule struct {
	Origin   string
	BasePath string
	Paths    []string
	// Model：账号落盘的上游模型名；Gate 出站可按此改写请求体，避免 Agent 默认名越权。
	Model string
}

// PolicyRequest 是编排层确认后的非敏感输入。
type PolicyRequest struct {
	Workspace string
	AgentID   string
	MountMode string
	Model     contracts.AccountInfo
	MCP       contracts.AccountInfo // 兼容旧单挂载
	MCPs      []contracts.AccountInfo
}

// MCPMount 将单个授权账号、上游和工具名单绑定在一起。
type MCPMount struct {
	AccountID string
	Endpoint  EndpointRule
	Tools     []string
}

// PolicySnapshot 是会话期不可变授权。
type PolicySnapshot struct {
	Workspace string
	MountMode string
	Model     EndpointRule
	MCP       EndpointRule
	MCPID     string
	Tools     []string
	MCPMounts []MCPMount
}

// Builder 构造会话策略快照。
type Builder struct {
	profiles map[string]FixedProfile
}

// NewBuilder 使用默认固定 profile；可注入自定义 map 便于测试。
func NewBuilder(profiles map[string]FixedProfile) *Builder {
	if profiles == nil {
		profiles = DefaultProfiles()
	}
	return &Builder{profiles: profiles}
}

// Profile 返回固定 profile；供 Gate 连接测试使用。
func (b *Builder) Profile(id string) (FixedProfile, bool) {
	p, ok := b.profiles[id]
	return p, ok
}

// Build 规范化工作区并绑定固定上游。
func (b *Builder) Build(_ context.Context, req PolicyRequest) (PolicySnapshot, error) {
	ws, err := NormalizeWorkspace(req.Workspace)
	if err != nil {
		return PolicySnapshot{}, err
	}
	if err := denySensitiveWorkspace(ws); err != nil {
		return PolicySnapshot{}, err
	}
	if strings.TrimSpace(req.AgentID) == "" {
		return PolicySnapshot{}, ErrInvalidAgent
	}
	if req.Model.Kind != "model" {
		return PolicySnapshot{}, ErrKindMismatch
	}
	modelProf, err := ModelProfileForAgent(req.AgentID, req.Model, b.profiles)
	if err != nil {
		return PolicySnapshot{}, err
	}
	if !ValidBasePath(modelProf.BasePath) {
		return PolicySnapshot{}, ErrUnknownProfile
	}
	if !sameHost(req.Model.Host, modelProf.Host) {
		return PolicySnapshot{}, ErrHostMismatch
	}
	accounts := req.MCPs
	if req.MCP.ID != "" {
		if len(accounts) > 0 {
			return PolicySnapshot{}, ErrMissingMCP
		}
		accounts = []contracts.AccountInfo{req.MCP}
	}
	if len(accounts) > 8 {
		return PolicySnapshot{}, ErrMissingMCP
	}
	mounts := make([]MCPMount, 0, len(accounts))
	seen := map[string]bool{}
	for _, account := range accounts {
		if account.Kind != "mcp" || strings.TrimSpace(account.ID) == "" || seen[account.ID] {
			return PolicySnapshot{}, ErrMissingMCP
		}
		seen[account.ID] = true
		var prof FixedProfile
		if account.ProfileID == ProfileCustom {
			var custom FixedProfile
			var err error
			if strings.TrimSpace(account.Transport) == "stdio" {
				// Host 形如 127.0.0.1:port；仅允许回环 http，由宿主 stdio 桥提供。
				hostPort := strings.TrimSpace(account.Host)
				hostname := hostPort
				if h, _, splitErr := net.SplitHostPort(hostPort); splitErr == nil {
					hostname = h
				}
				ip := net.ParseIP(hostname)
				if ip == nil || !ip.IsLoopback() {
					return PolicySnapshot{}, ErrUnknownProfile
				}
				custom = FixedProfile{
					ID:                    ProfileCustom,
					Kind:                  "mcp",
					Protocol:              "streamable-http",
					Origin:                "http://" + hostPort,
					Host:                  hostname,
					Paths:                 []string{"/mcp"},
					TestPath:              "/mcp",
					AllowInsecureLoopback: true,
				}
				if !custom.ValidConnection() {
					return PolicySnapshot{}, ErrUnknownProfile
				}
			} else {
				origin := "https://" + strings.TrimSpace(account.Host)
				if bp := strings.TrimRight(strings.TrimSpace(account.BasePath), "/"); bp != "" {
					origin = origin + bp
				}
				custom, err = CustomMCPProfile(origin)
				if err != nil || !custom.ValidConnection() {
					return PolicySnapshot{}, ErrUnknownProfile
				}
			}
			if len(account.Tools) == 0 {
				// 旧版连接未落盘 tools/list；须在「工具」页删除后重新保存 MCP。
				return PolicySnapshot{}, ErrMissingMCP
			}
			custom.Tools = append([]string(nil), account.Tools...)
			prof = custom
		} else {
			var ok bool
			prof, ok = b.profiles[account.ProfileID]
			if !ok || prof.Kind != "mcp" {
				return PolicySnapshot{}, ErrUnknownProfile
			}
			if !sameHost(account.Host, prof.Host) {
				return PolicySnapshot{}, ErrHostMismatch
			}
		}
		mounts = append(mounts, MCPMount{AccountID: account.ID, Endpoint: EndpointRule{Origin: prof.Origin, Paths: append([]string{}, prof.Paths...)}, Tools: append([]string{}, prof.Tools...)})
	}
	mountMode := strings.TrimSpace(req.MountMode)
	if mountMode == "" {
		mountMode = "direct"
	}
	if mountMode != "direct" && mountMode != "clone" {
		return PolicySnapshot{}, ErrInvalidWorkspace
	}
	snapshot := PolicySnapshot{
		Workspace: ws,
		MountMode: mountMode,
		Model: EndpointRule{
			Origin:   modelProf.Origin,
			BasePath: modelProf.BasePath,
			Paths:    append([]string{}, modelProf.Paths...),
			Model:    strings.TrimSpace(modelProf.Model),
		},
		MCPMounts: mounts,
	}
	if len(mounts) == 1 {
		snapshot.MCP, snapshot.MCPID, snapshot.Tools = mounts[0].Endpoint, mounts[0].AccountID, append([]string{}, mounts[0].Tools...)
	}
	return snapshot, nil
}

// NormalizeWorkspace 返回绝对清理路径。
func NormalizeWorkspace(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ErrInvalidWorkspace
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// 目录可能尚未完全可读；仍返回 Clean 后的绝对路径，由 Runtime 再确认。
		if os.IsNotExist(err) {
			return "", ErrInvalidWorkspace
		}
		return filepath.Clean(abs), nil
	}
	return filepath.Clean(resolved), nil
}

func denySensitiveWorkspace(ws string) error {
	home, _ := os.UserHomeDir()
	clean := filepath.Clean(ws)
	if home != "" {
		home = filepath.Clean(home)
		if clean == home {
			return ErrInvalidWorkspace
		}
		for _, name := range []string{".ssh", ".aws", ".gnupg", "Library/Keychains"} {
			if strings.HasPrefix(clean, filepath.Join(home, name)) {
				return ErrInvalidWorkspace
			}
		}
	}
	for _, root := range []string{"/", "/etc", "/System", "/usr", "/bin", "/sbin", "/var", "/private"} {
		if clean == root {
			return ErrInvalidWorkspace
		}
	}
	return nil
}

// PathAllowed 判断请求路径是否落在规则内（末尾 / 表示前缀，否则精确匹配；忽略查询串）。
func PathAllowed(rule EndpointRule, path string) bool {
	if path == "" {
		path = "/"
	}
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	for _, p := range rule.Paths {
		if p == "" {
			continue
		}
		// 单独的 "/" 只表示根路径，不能变成全路径放行。
		if p == "/" {
			if path == "/" {
				return true
			}
			continue
		}
		if path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}

func sameHost(got, want string) bool {
	got = strings.ToLower(strings.TrimSpace(got))
	want = strings.ToLower(strings.TrimSpace(want))
	return got != "" && got == want
}
