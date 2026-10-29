package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type detectorStub struct {
	items []contracts.AgentInfo
	err   error
}

func (d detectorStub) ListAgents(context.Context) ([]contracts.AgentInfo, error) {
	return d.items, d.err
}

func TestDesktopAgentAndEventBranches(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	if got := api.ListAgents(); got.Error != nil || len(got.Agents) != 0 {
		t.Fatalf("no detector: %#v", got)
	}
	app.SetAgentDetector(detectorStub{err: errors.New("probe failed")})
	if got := api.ListAgents(); got.Error == nil || got.Error.Code != contracts.ErrInternal {
		t.Fatalf("detector error: %#v", got)
	}
	app.SetAgentDetector(detectorStub{items: []contracts.AgentInfo{{ID: "claude"}}})
	if got := api.ListAgents(); got.Error != nil || len(got.Agents) != 1 {
		t.Fatalf("detector success: %#v", got)
	}
	app.EmitHarborEvent(contracts.Event{SessionID: "s", Seq: 1, Type: contracts.EventTypeOutput})
	app.forwardEvent(contracts.Event{SessionID: "s", Seq: 2, Type: contracts.EventTypeOutput})
}
