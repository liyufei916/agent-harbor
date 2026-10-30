package desktop

import (
	"context"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type partialSession struct{}

func (partialSession) WithConnection(context.Context, func(context.Context) error) error {
	return nil
}
func (partialSession) Create(context.Context, contracts.SessionSpec) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{}, nil
}
func (partialSession) Get(context.Context, string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{}, nil
}
func (partialSession) Stop(context.Context, string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{}, nil
}
func (partialSession) List(context.Context) ([]contracts.SessionSummary, error) {
	return nil, nil
}
func (partialSession) Destroy(context.Context, string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{}, nil
}
func (partialSession) SyncBack(context.Context, string) (contracts.SessionSnapshot, error) {
	return contracts.SessionSnapshot{}, nil
}
func (partialSession) Shutdown(context.Context) error { return nil }

func TestRequireAttachedSessionServiceRejectsPartial(t *testing.T) {
	if err := requireAttachedSessionService(partialSession{}); err == nil {
		t.Fatal("expected partial session to fail capability assert")
	}
}

func TestAttachOrchestratorPassesCapabilityAssert(t *testing.T) {
	app := NewApp()
	if err := app.AttachOrchestrator(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := requireAttachedSessionService(app.sessionRef()); err != nil {
		t.Fatal(err)
	}
}
