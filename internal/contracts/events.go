// 本文件按 docs/CONTRACT.md v1 定义事件形状，并附带公开错误码常量。
package contracts

import (
	"errors"
	"strings"
)

// HarborEventName 与 EventName 相同，供桌面桥注册固定通道。
const HarborEventName = EventName

// 事件 type 取值。
const (
	EventTypeStateChanged     = "state_changed"
	EventTypeOutput           = "output"
	EventTypeAudit            = "audit"
	EventTypeError            = "error"
	EventTypePermissionNeeded = "permission_needed"
	EventTypePty              = "pty"
)

// 公开错误码（与 docs/CONTRACT.md v1.2 一致）。
const (
	ErrInvalidArgument         = "INVALID_ARGUMENT"
	ErrAccountNotFound         = "ACCOUNT_NOT_FOUND"
	ErrSessionBusy             = "SESSION_BUSY"
	ErrSessionNotFound         = "SESSION_NOT_FOUND"
	ErrAgentUnsupported        = "AGENT_UNSUPPORTED"
	ErrIsolationUnavailable    = "ISOLATION_UNAVAILABLE"
	ErrWorkspaceDenied         = "WORKSPACE_DENIED"
	ErrUpstreamAuthFailed      = "UPSTREAM_AUTH_FAILED"
	ErrUpstreamTimeout         = "UPSTREAM_TIMEOUT"
	ErrUpstreamUnavailable     = "UPSTREAM_UNAVAILABLE"
	ErrTargetDenied            = "TARGET_DENIED"
	ErrInvalidSentinel         = "INVALID_SENTINEL"
	ErrAgentExitFailed         = "AGENT_EXIT_FAILED"
	ErrCancelled               = "CANCELLED"
	ErrCleanupFailed           = "CLEANUP_FAILED"
	ErrInternal                = "INTERNAL_ERROR"
	ErrOrchestratorUnavailable = "ORCHESTRATOR_UNAVAILABLE"
	ErrSessionLimit            = "SESSION_LIMIT"
	ErrWorkspaceBusy           = "WORKSPACE_BUSY"
	ErrUnimplemented           = "UNIMPLEMENTED"
)

// NewAppError 构造公开错误；message 应为固定模板。
func NewAppError(code, message string, retryable bool) *AppError {
	return &AppError{Code: code, Message: message, Retryable: retryable}
}

// OutputData 是 stdout/stderr 文本块。
type OutputData struct {
	Stream string `json:"stream"` // stdout 或 stderr
	Text   string `json:"text"`
}

// AuditData 是出站审计摘要，不含认证头或正文。
type AuditData struct {
	Target       string `json:"target"` // 主机名，不包含 URL 查询串
	Tool         string `json:"tool"`
	Result       string `json:"result"` // allowed、denied 或 failed
	InputTokens  int64  `json:"inputTokens,omitempty"`
	OutputTokens int64  `json:"outputTokens,omitempty"`
}

// PermissionNeeded 是扩权挂起载荷（不含密钥）。
type PermissionNeeded struct {
	Target string `json:"target"`
	Action string `json:"action"`
}

// Event 通过固定事件名 harbor:event 传递；五类载荷恰有一项非 nil。
type Event struct {
	SessionID  string            `json:"sessionId"`
	Seq        uint64            `json:"seq"`
	Type       string            `json:"type"`
	State      *SessionSnapshot  `json:"state"`
	Output     *OutputData       `json:"output"`
	Audit      *AuditData        `json:"audit"`
	Error      *AppError         `json:"error"`
	Permission *PermissionNeeded `json:"permission"`
	Pty        *OutputData       `json:"pty"`
}

// Validate 只校验公开事件的数据形状，不执行策略、文件或网络检查。
func (e Event) Validate() error {
	if e.SessionID == "" || e.Seq == 0 {
		return errors.New("event identity is required")
	}
	payloads := 0
	if e.State != nil {
		payloads++
	}
	if e.Output != nil {
		payloads++
	}
	if e.Audit != nil {
		payloads++
	}
	if e.Error != nil {
		payloads++
	}
	if e.Permission != nil {
		payloads++
	}
	if e.Pty != nil {
		payloads++
	}
	if payloads != 1 {
		return errors.New("event must contain exactly one payload")
	}
	switch e.Type {
	case EventTypeStateChanged:
		if e.State == nil || e.State.SessionID != e.SessionID || e.State.Seq != e.Seq {
			return errors.New("invalid state event")
		}
	case EventTypeOutput:
		if e.Output == nil || (e.Output.Stream != "stdout" && e.Output.Stream != "stderr") || e.Output.Text == "" || strings.ContainsRune(e.Output.Text, '\x00') {
			return errors.New("invalid output event")
		}
	case EventTypeAudit:
		if e.Audit == nil || e.Audit.Target == "" || e.Audit.Tool == "" || (e.Audit.Result != "allowed" && e.Audit.Result != "denied" && e.Audit.Result != "failed") {
			return errors.New("invalid audit event")
		}
	case EventTypeError:
		if e.Error == nil || e.Error.Code == "" || e.Error.Message == "" {
			return errors.New("invalid error event")
		}
	case EventTypePermissionNeeded:
		if e.Permission == nil || e.Permission.Target == "" || e.Permission.Action == "" {
			return errors.New("invalid permission event")
		}
	case EventTypePty:
		if e.Pty == nil || (e.Pty.Stream != "stdout" && e.Pty.Stream != "stderr") || e.Pty.Text == "" || strings.ContainsRune(e.Pty.Text, '\x00') {
			return errors.New("invalid pty event")
		}
	default:
		return errors.New("unknown event type")
	}
	return nil
}
