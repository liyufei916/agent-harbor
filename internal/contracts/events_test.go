package contracts

import "testing"

func TestNewAppErrorImplementsErrorContract(t *testing.T) {
	err := NewAppError(ErrTargetDenied, "目标被拒绝", true)
	if err.Code != ErrTargetDenied || err.Message != "目标被拒绝" || !err.Retryable {
		t.Fatalf("错误字段不完整: %+v", err)
	}
	if err.Error() != err.Message {
		t.Fatalf("Error() 应返回公开消息: %q", err.Error())
	}
}

func TestEventValidateRequiresOneTypedPayload(t *testing.T) {
	base := Event{SessionID: "session", Seq: 1}
	if base.Validate() == nil {
		t.Fatal("空载荷事件通过")
	}
	state := base
	state.Type = "state_changed"
	state.State = &SessionSnapshot{SessionID: "session", Seq: 1, State: StateRunning}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	duplicate := state
	duplicate.Output = &OutputData{Stream: "stdout", Text: "hello"}
	if duplicate.Validate() == nil {
		t.Fatal("多载荷事件通过")
	}
	wrongSeq := state
	wrongSeq.State.Seq = 2
	if wrongSeq.Validate() == nil {
		t.Fatal("状态水位不一致通过")
	}
}

func TestEventValidateRejectsInvalidOutputAuditAndError(t *testing.T) {
	output := Event{SessionID: "session", Seq: 2, Type: "output", Output: &OutputData{Stream: "file", Text: "x"}}
	if output.Validate() == nil {
		t.Fatal("非法输出流通过")
	}
	audit := Event{SessionID: "session", Seq: 3, Type: "audit", Audit: &AuditData{Target: "example.com", Tool: "read", Result: "unknown"}}
	if audit.Validate() == nil {
		t.Fatal("非法审计结果通过")
	}
	errEvent := Event{SessionID: "session", Seq: 4, Type: "error", Error: &AppError{Code: "X"}}
	if errEvent.Validate() == nil {
		t.Fatal("不完整错误事件通过")
	}
}
