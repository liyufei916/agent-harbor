package contracts

import "testing"

func TestAppErrorAndConstructor(t *testing.T) {
	err := NewAppError(ErrInvalidArgument, "bad input", true)
	if err.Code != ErrInvalidArgument || err.Message != "bad input" || !err.Retryable {
		t.Fatalf("unexpected app error: %#v", err)
	}
	if err.Error() != "bad input" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestEventValidateAllPayloadKinds(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
	}{
		{"audit", Event{SessionID: "s", Seq: 1, Type: EventTypeAudit, Audit: &AuditData{Target: "example.com", Tool: "read", Result: "allowed"}}},
		{"permission", Event{SessionID: "s", Seq: 1, Type: EventTypePermissionNeeded, Permission: &PermissionNeeded{Target: "example.com", Action: "connect"}}},
		{"pty", Event{SessionID: "s", Seq: 1, Type: EventTypePty, Pty: &OutputData{Stream: "stdout", Text: "x"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.ev.Validate(); err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
		})
	}
	for _, ev := range []Event{
		{SessionID: "", Seq: 1, Type: EventTypeOutput, Output: &OutputData{Stream: "stdout", Text: "x"}},
		{SessionID: "s", Seq: 0, Type: EventTypeOutput, Output: &OutputData{Stream: "stdout", Text: "x"}},
		{SessionID: "s", Seq: 1, Type: "unknown", Output: &OutputData{Stream: "stdout", Text: "x"}},
	} {
		if ev.Validate() == nil {
			t.Fatalf("invalid event accepted: %#v", ev)
		}
	}
}
