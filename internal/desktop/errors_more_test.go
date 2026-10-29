package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestMapErrorCoversPublicAndDependencyErrors(t *testing.T) {
	public := contracts.NewAppError(contracts.ErrTargetDenied, "denied", true)
	if got := mapError(public); got.Code != contracts.ErrTargetDenied || !got.Retryable {
		t.Fatalf("public error mapping: %#v", got)
	}
	cases := []struct {
		err  error
		code string
	}{
		{context.Canceled, contracts.ErrCancelled}, {context.DeadlineExceeded, contracts.ErrUpstreamTimeout},
		{gate.ErrInvalidConnectInput, contracts.ErrInvalidArgument}, {gate.ErrUpstreamAuth, contracts.ErrUpstreamAuthFailed},
		{gate.ErrUpstreamTimeout, contracts.ErrUpstreamTimeout}, {gate.ErrUpstreamDown, contracts.ErrUpstreamUnavailable},
		{gate.ErrCancelled, contracts.ErrCancelled}, {secrets.ErrNotFound, contracts.ErrAccountNotFound},
		{policy.ErrInvalidWorkspace, contracts.ErrWorkspaceDenied}, {policy.ErrHostMismatch, contracts.ErrInvalidArgument},
		{errors.New(contracts.ErrSessionBusy), contracts.ErrSessionBusy}, {errors.New("unexpected"), contracts.ErrInternal},
	}
	for _, tc := range cases {
		if got := mapError(tc.err); got.Code != tc.code {
			t.Errorf("mapError(%v) code = %s, want %s", tc.err, got.Code, tc.code)
		}
	}
	if mapError(nil) != nil {
		t.Fatal("nil error did not map to nil")
	}
}

func TestWorkspaceResult(t *testing.T) {
	if got := workspaceResult("", nil); !got.Cancelled || got.Error != nil {
		t.Fatalf("cancel result: %#v", got)
	}
	if got := workspaceResult("/tmp/work", nil); got.Path != "/tmp/work" || got.Cancelled {
		t.Fatalf("success result: %#v", got)
	}
	if got := workspaceResult("", errors.New("failed")); got.Error == nil {
		t.Fatalf("error result: %#v", got)
	}
}
