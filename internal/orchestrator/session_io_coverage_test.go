package orchestrator

import (
	"context"
	"github.com/liyufei916/agent-harbor/internal/contracts"
	"testing"
)

func TestResizePtyValidationAndSuccess(t *testing.T) {
	f := newFixture()
	f.interactive = true
	o := setup(t, f, Timeouts{}, nil)
	s := create(t, o)
	awaitState(t, o, s.SessionID, contracts.StateRunning)
	for _, in := range []contracts.PtyInput{{}, {SessionID: "missing", Cols: 80, Rows: 24}, {SessionID: s.SessionID, Cols: 0, Rows: 24}} {
		if err := o.ResizePty(context.Background(), in); err == nil {
			t.Fatalf("invalid resize accepted: %+v", in)
		}
	}
	if err := o.ResizePty(context.Background(), contracts.PtyInput{SessionID: s.SessionID, Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	_, _ = o.Stop(context.Background(), s.SessionID)
	await(t, o, s.SessionID, func(s contracts.SessionSnapshot) bool { return s.CleanupComplete })
}
