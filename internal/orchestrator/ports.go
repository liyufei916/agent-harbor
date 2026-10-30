// Package orchestrator 统一管理会话，不持有真实上游凭据。
package orchestrator

import (
	"context"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

type SessionService interface {
	Health(context.Context) error
	Boundary(context.Context, string) (contracts.PermissionOverview, error)
	WithConnection(context.Context, func(context.Context) error) error
	Create(context.Context, contracts.SessionSpec) (contracts.SessionSnapshot, error)
	Get(context.Context, string) (contracts.SessionSnapshot, error)
	Stop(context.Context, string) (contracts.SessionSnapshot, error)
	List(context.Context) ([]contracts.SessionSummary, error)
	Destroy(context.Context, string) (contracts.SessionSnapshot, error)
	SyncBack(context.Context, string) (contracts.SessionSnapshot, error)
	// RequestPermission / ProjectWorkspace：扩权 ask 与记住主机写回所需。
	RequestPermission(context.Context, string, contracts.PermissionNeeded) (string, error)
	ProjectWorkspace(context.Context, string) (string, error)
	ResolvePermission(context.Context, contracts.ResolvePermissionInput) error
	SwitchModel(context.Context, contracts.SwitchModelInput) (contracts.SessionSnapshot, error)
	WritePty(context.Context, contracts.PtyInput) error
	ResizePty(context.Context, contracts.PtyInput) error
	Shutdown(context.Context) error
}

// PermissionAsker 桌面 PromptPermission 的可选能力口（避免匿名类型断言）。
type PermissionAsker interface {
	RequestPermission(context.Context, string, contracts.PermissionNeeded) (string, error)
}

// ProjectWorkspaceReader 扩权 project 范围写回记住主机时解析源工作区。
type ProjectWorkspaceReader interface {
	ProjectWorkspace(context.Context, string) (string, error)
}

type AccountCatalog interface {
	Lookup(context.Context, string) (contracts.AccountInfo, error)
}

type PolicyBuilder interface {
	Build(context.Context, policy.PolicyRequest) (policy.PolicySnapshot, error)
}

type Gate interface {
	Start(context.Context, gate.GateSpec, func(contracts.AuditData), func(error)) (gate.GateBinding, error)
	Revoke(context.Context, string) error
}

type Dependencies struct {
	Runtime runtime.Runtime
	Kits    map[string]kits.Kit
	Catalog AccountCatalog
	Policy  PolicyBuilder
	Gate    Gate
}

// Timeouts 可在测试中缩短；生产装配传零值采用契约预算。
type Timeouts struct {
	Provision time.Duration
	Session   time.Duration
	Cleanup   time.Duration
}
