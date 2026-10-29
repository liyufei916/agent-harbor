package gate

import (
	"context"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// SwitchModel 原子替换兼容账号，路由与模型名保持为该会话的固定 profile。
// 本方法仅访问内存目录，不发网络请求、不调用用户回调。
func (h *Harbor) SwitchModel(ctx context.Context, sessionID, accountID string) error {
	if ctx.Err() != nil {
		return contracts.NewAppError(contracts.ErrCancelled, "操作已取消。", false)
	}
	if accountID == "" {
		return contracts.NewAppError(contracts.ErrInvalidArgument, "模型账号不能为空。", false)
	}
	next, err := h.secrets.Lookup(ctx, accountID)
	if err != nil {
		return contracts.NewAppError(contracts.ErrAccountNotFound, "模型账号不存在。", false)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.revoked || rt.ctx.Err() != nil {
		return contracts.NewAppError(contracts.ErrSessionNotFound, "会话不存在或已停止。", false)
	}
	if ctx.Err() != nil {
		return contracts.NewAppError(contracts.ErrCancelled, "操作已取消。", false)
	}
	current, err := h.secrets.Lookup(ctx, rt.spec.ModelAccountID)
	if err != nil {
		return contracts.NewAppError(contracts.ErrAccountNotFound, "原模型账号不存在。", false)
	}
	if current.Kind != "model" || next.Kind != "model" || current.ProfileID != next.ProfileID || !strings.EqualFold(next.Host, hostOfOrigin(rt.spec.Policy.Model.Origin)) {
		return contracts.NewAppError(contracts.ErrInvalidArgument, "模型账号与当前会话的协议或上游不兼容。", false)
	}
	rt.spec.ModelAccountID = accountID
	// 同步策略中的模型名，供出站改写（否则仍用开会话时的默认名）。
	if m := strings.TrimSpace(next.Model); m != "" {
		rt.spec.Policy.Model.Model = m
	}
	return nil
}

// 与切换和撤销共用准入锁，在请求进入时原子捕获账号。
func (h *Harbor) acquireModelSession(sessionID string) (*sessionRuntime, string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.sessions[sessionID]
	if !ok || rt.revoked || rt.ctx.Err() != nil {
		return nil, "", false
	}
	rt.active.Add(1)
	return rt, rt.spec.ModelAccountID, true
}
