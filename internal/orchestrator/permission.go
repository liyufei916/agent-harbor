package orchestrator

import (
	"context"
	"net"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type permissionRequest struct {
	ctx      context.Context
	decision chan string
}

// RequestPermission 供会话 Gate 回调使用，只等待决策，不修改出站策略。
// Gate 必须重新求值策略、持久化 project 授权并约束 once 的单次作用域。
func (o *Service) RequestPermission(ctx context.Context, id string, request contracts.PermissionNeeded) (string, error) {
	if ctx.Err() != nil {
		return "", publicError("CANCELLED")
	}
	if !ValidPermissionTarget(request.Target) || request.Action != "connect" {
		return "", publicError("INVALID_ARGUMENT")
	}
	o.mu.Lock()
	s, ok := o.sessions[id]
	if !ok {
		o.mu.Unlock()
		return "", publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.State != contracts.StateRunning || s.ctx.Err() != nil {
		o.mu.Unlock()
		return "", publicError("INVALID_ARGUMENT")
	}
	// 不让会话能力令牌进入前端；回调只允许经过校验的单个主机名。
	for _, filter := range s.filters {
		if filter.token != "" && strings.Contains(request.Target, filter.token) {
			o.mu.Unlock()
			return "", publicError("INVALID_ARGUMENT")
		}
	}
	if s.permissions == nil {
		s.permissions = make(map[string]*permissionRequest)
	}
	if s.permissions[request.Target] != nil || len(s.permissions) >= 8 {
		o.mu.Unlock()
		return "", publicError("INVALID_ARGUMENT")
	}
	pending := &permissionRequest{ctx: ctx, decision: make(chan string, 1)}
	s.permissions[request.Target] = pending
	o.emitLocked(s, contracts.Event{Type: contracts.EventTypePermissionNeeded, Permission: &request})
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		if s.permissions[request.Target] == pending {
			delete(s.permissions, request.Target)
		}
		o.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return "", publicError("CANCELLED")
	case <-s.ctx.Done():
		return "", publicError("CANCELLED")
	case decision := <-pending.decision:
		if ctx.Err() != nil || s.ctx.Err() != nil {
			return "", publicError("CANCELLED")
		}
		return decision, nil
	}
}

func (o *Service) ResolvePermission(ctx context.Context, input contracts.ResolvePermissionInput) error {
	if ctx.Err() != nil {
		return publicError("CANCELLED")
	}
	if !ValidPermissionTarget(input.Target) || (input.Decision != "deny" && input.Decision != "once" && input.Decision != "project") {
		return publicError("INVALID_ARGUMENT")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.sessions[input.SessionID]
	if !ok {
		return publicError("SESSION_NOT_FOUND")
	}
	pending := s.permissions[input.Target]
	if s.snapshot.State != contracts.StateRunning || s.ctx.Err() != nil || pending == nil {
		return publicError("INVALID_ARGUMENT")
	}
	if pending.ctx.Err() != nil {
		delete(s.permissions, input.Target)
		return publicError("INVALID_ARGUMENT")
	}
	delete(s.permissions, input.Target)
	pending.decision <- input.Decision
	return nil
}

// ValidPermissionTarget 只接收规范化的精确主机名，拒绝 URL、通配符、端口及本机地址。
func ValidPermissionTarget(host string) bool {
	if host == "" || len(host) > 253 || host != strings.ToLower(host) || strings.ContainsAny(host, "/:*?@#\\\r\n\t ") || strings.HasSuffix(host, ".") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsPrivate() && ip.IsGlobalUnicast() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
