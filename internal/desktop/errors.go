package desktop

import (
	"context"
	"errors"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func mapError(err error) *contracts.AppError {
	if err == nil {
		return nil
	}
	var pub *contracts.AppError
	if errors.As(err, &pub) && pub != nil && pub.Code != "" {
		return contracts.NewAppError(pub.Code, pub.Message, pub.Retryable)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return contracts.NewAppError(contracts.ErrCancelled, "操作已取消", false)
	case errors.Is(err, context.DeadlineExceeded):
		return contracts.NewAppError(contracts.ErrUpstreamTimeout, "上游连接超时", true)
	case errors.Is(err, gate.ErrInvalidConnectInput):
		return contracts.NewAppError(contracts.ErrInvalidArgument, "输入不合法", false)
	case errors.Is(err, gate.ErrUpstreamAuth):
		return contracts.NewAppError(contracts.ErrUpstreamAuthFailed, "上游认证失败", false)
	case errors.Is(err, gate.ErrUpstreamTimeout):
		return contracts.NewAppError(contracts.ErrUpstreamTimeout, "上游连接超时", true)
	case errors.Is(err, gate.ErrUpstreamDown):
		return contracts.NewAppError(contracts.ErrUpstreamUnavailable, "上游不可用", true)
	case errors.Is(err, gate.ErrCancelled):
		return contracts.NewAppError(contracts.ErrCancelled, "操作已取消", false)
	case errors.Is(err, secrets.ErrNotFound):
		return contracts.NewAppError(contracts.ErrAccountNotFound, "账号不存在", false)
	case errors.Is(err, secrets.ErrInvalidInput), errors.Is(err, secrets.ErrEmptyKey):
		return contracts.NewAppError(contracts.ErrInvalidArgument, "输入不合法", false)
	case errors.Is(err, errOrchestratorDown):
		return contracts.NewAppError(contracts.ErrOrchestratorUnavailable, "编排服务尚未就绪", false)
	case errors.Is(err, policy.ErrInvalidWorkspace):
		return contracts.NewAppError(contracts.ErrWorkspaceDenied, "工作区不被允许", false)
	case errors.Is(err, policy.ErrHostMismatch):
		return contracts.NewAppError(contracts.ErrInvalidArgument, "账号与固定上游不一致", false)
	}
	switch err.Error() {
	case contracts.ErrInvalidArgument:
		return contracts.NewAppError(contracts.ErrInvalidArgument, "输入不合法", false)
	case contracts.ErrSessionBusy:
		return contracts.NewAppError(contracts.ErrSessionBusy, "已有会话进行中", false)
	case contracts.ErrSessionNotFound:
		return contracts.NewAppError(contracts.ErrSessionNotFound, "会话不存在", false)
	case contracts.ErrAgentUnsupported:
		return contracts.NewAppError(contracts.ErrAgentUnsupported, "Agent 不可用", false)
	case contracts.ErrIsolationUnavailable:
		return contracts.NewAppError(contracts.ErrIsolationUnavailable, "隔离能力不可用", false)
	case contracts.ErrWorkspaceDenied:
		return contracts.NewAppError(contracts.ErrWorkspaceDenied, "工作区不被允许", false)
	case contracts.ErrCleanupFailed:
		return contracts.NewAppError(contracts.ErrCleanupFailed, "清理未完成", false)
	case contracts.ErrCancelled:
		return contracts.NewAppError(contracts.ErrCancelled, "操作已取消", false)
	default:
		return contracts.NewAppError(contracts.ErrInternal, "内部错误", false)
	}
}
