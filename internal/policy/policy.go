// Package policy 将已确认资源转为不可变会话授权快照。
// 负责人：研发 B。不负责网络转发或进程隔离。
package policy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
}

// PolicyRequest 是编排层确认后的非敏感输入。
type PolicyRequest struct {
	Workspace string
	AgentID   string
	Model     contracts.AccountInfo
	MCP       contracts.AccountInfo
}

// PolicySnapshot 是会话期不可变授权。
type PolicySnapshot struct {
	Workspace string
	MountMode string
	Model     EndpointRule
	MCP       EndpointRule
	MCPID     string
	Tools     []string
}

// Builder 构造会话策略快照。
type Builder struct {
	profiles map[string]FixedProfile
	mu       sync.Mutex
	hosts    []contracts.RememberedHost
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
	modelProf, err := b.accountProfile(req.Model, "model")
	if err != nil {
		return PolicySnapshot{}, err
	}
	if req.AgentID == "dsh" && req.Model.ProfileID != ProfileDSHModel {
		return PolicySnapshot{}, ErrKindMismatch
	}
	if req.AgentID == "claude" && req.Model.ProfileID != ProfileDemoModel && req.Model.ProfileID != ProfileCustom {
		return PolicySnapshot{}, ErrKindMismatch
	}
	if !ValidBasePath(modelProf.BasePath) {
		return PolicySnapshot{}, ErrUnknownProfile
	}
	mcpProf, err := b.accountProfile(req.MCP, "mcp")
	if err != nil {
		return PolicySnapshot{}, err
	}
	if req.MCP.ID == "" {
		return PolicySnapshot{}, ErrMissingMCP
	}
	if !sameHost(req.Model.Host, modelProf.Host) || !sameHost(req.MCP.Host, mcpProf.Host) {
		return PolicySnapshot{}, ErrHostMismatch
	}
	return PolicySnapshot{
		Workspace: ws,
		MountMode: "direct",
		Model: EndpointRule{
			Origin:   modelProf.Origin,
			BasePath: modelProf.BasePath,
			Paths:    append([]string{}, modelProf.Paths...),
		},
		MCP: EndpointRule{
			Origin: mcpProf.Origin,
			Paths:  append([]string{}, mcpProf.Paths...),
		},
		MCPID: req.MCP.ID,
		Tools: append([]string{}, mcpProf.Tools...),
	}, nil
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

func (b *Builder) accountProfile(info contracts.AccountInfo, kind string) (FixedProfile, error) {
	if strings.TrimSpace(info.Kind) != kind {
		if kind == "mcp" {
			return FixedProfile{}, ErrMissingMCP
		}
		return FixedProfile{}, ErrKindMismatch
	}
	if info.ProfileID == ProfileCustom {
		origin, host, err := OriginFromAccountHost(info.Host)
		if err != nil {
			return FixedProfile{}, err
		}
		prof := FixedProfile{ID: ProfileCustom, Kind: kind, Origin: origin, Host: host, Protocol: "anthropic", Paths: []string{"/v1/messages"}}
		if kind == "mcp" {
			prof.Protocol = "streamable-http"
			prof.Paths = []string{"/mcp"}
		}
		return prof, nil
	}
	prof, ok := b.profiles[info.ProfileID]
	if !ok || prof.Kind != kind {
		return FixedProfile{}, ErrUnknownProfile
	}
	return prof, nil
}

func sameHost(got, want string) bool {
	got = strings.ToLower(strings.TrimSpace(got))
	want = strings.ToLower(strings.TrimSpace(want))
	return got != "" && got == want
}
