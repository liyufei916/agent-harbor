package orchestrator

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

func (o *Service) WritePty(ctx context.Context, input contracts.PtyInput) error {
	if ctx.Err() != nil {
		return publicError("CANCELLED")
	}
	if input.SessionID == "" || !utf8.ValidString(input.Data) || len(input.Data) == 0 || len(input.Data) > 32768 || strings.ContainsRune(input.Data, '\x00') {
		return publicError("INVALID_ARGUMENT")
	}
	o.mu.Lock()
	s, ok := o.sessions[input.SessionID]
	if !ok {
		o.mu.Unlock()
		return publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.State != contracts.StateRunning || s.process == nil {
		o.mu.Unlock()
		return publicError("INVALID_ARGUMENT")
	}
	if !s.interactive {
		o.mu.Unlock()
		return publicError("UNIMPLEMENTED")
	}
	process := s.process
	o.mu.Unlock()
	interactive, ok := process.(runtime.InteractiveProcess)
	if !ok {
		return publicError("UNIMPLEMENTED")
	}
	if err := interactive.Write(ctx, []byte(input.Data)); err != nil {
		// 写入中断可能留下半条协议消息，必须清理会话，不能继续接收下一轮。
		var app *contracts.AppError
		if !errors.As(err, &app) || app.Code != "INVALID_ARGUMENT" {
			o.fail(s, err, "INTERNAL_ERROR")
		}
		return safeError(err, "INTERNAL_ERROR")
	}
	return nil
}

func (o *Service) ResizePty(ctx context.Context, input contracts.PtyInput) error {
	if ctx.Err() != nil {
		return publicError("CANCELLED")
	}
	if input.SessionID == "" {
		return publicError("INVALID_ARGUMENT")
	}
	o.mu.Lock()
	s, ok := o.sessions[input.SessionID]
	if !ok {
		o.mu.Unlock()
		return publicError("SESSION_NOT_FOUND")
	}
	if s.snapshot.State != contracts.StateRunning || s.process == nil || !s.interactive {
		o.mu.Unlock()
		return publicError("INVALID_ARGUMENT")
	}
	process := s.process
	o.mu.Unlock()
	interactive, ok := process.(runtime.InteractiveProcess)
	if !ok {
		return publicError("UNIMPLEMENTED")
	}
	if err := interactive.Resize(input.Cols, input.Rows); err != nil {
		return safeError(err, "UNIMPLEMENTED")
	}
	return nil
}

func (o *Service) audit(s *session, data contracts.AuditData) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.snapshot.State != contracts.StateProvisioning && s.snapshot.State != contracts.StateRunning {
		return
	}
	if s.filters == nil {
		return
	}
	// Gate 必须提供脱敏主机名；拒绝整条 URL 或异常结果。
	if strings.ContainsAny(data.Target, "/?@#\r\n") || (data.Result != "allowed" && data.Result != "denied" && data.Result != "failed") {
		return
	}
	for _, filter := range s.filters {
		if strings.Contains(data.Target, filter.token) || strings.Contains(data.Tool, filter.token) {
			return
		}
		break
	}
	o.emitLocked(s, contracts.Event{Type: "audit", Audit: &data})
}

func (o *Service) output(s *session, data contracts.OutputData) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.snapshot.State == contracts.StateStopped || s.snapshot.State == contracts.StateFailed {
		return
	}
	filter := s.filters[data.Stream]
	if filter == nil {
		return
	}
	text := filter.write(data.Text)
	chunks(text, func(text string) {
		data := contracts.OutputData{Stream: data.Stream, Text: text}
		if s.outputReady {
			o.outputLocked(s, data)
			return
		}
		s.early = append(s.early, data)
		s.earlyBytes += len(text)
		for s.earlyBytes > outputLimit {
			s.earlyBytes -= len(s.early[0].Text)
			s.early[0] = contracts.OutputData{}
			s.early = s.early[1:]
			s.earlyTruncated = true
		}
	})
}

func (o *Service) outputLocked(s *session, data contracts.OutputData) {
	if s.interactive {
		o.emitLocked(s, contracts.Event{Type: contracts.EventTypePty, Pty: &data})
		return
	}
	o.emitLocked(s, contracts.Event{Type: contracts.EventTypeOutput, Output: &data})
}

func (o *Service) drainEarlyLocked(s *session) {
	if s.earlyTruncated {
		o.outputLocked(s, contracts.OutputData{Stream: "stderr", Text: "[输出已截断]\n"})
	}
	for _, data := range s.early {
		o.outputLocked(s, data)
	}
	s.early, s.earlyBytes, s.earlyTruncated = nil, 0, false
}
