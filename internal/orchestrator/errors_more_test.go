package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
)

func TestPublicAndSafeErrorMapping(t *testing.T) {
	for _, code := range []string{contracts.ErrInvalidArgument, contracts.ErrUpstreamTimeout, contracts.ErrUpstreamUnavailable, contracts.ErrInternal, "UNKNOWN"} {
		got := publicError(code)
		if got == nil || got.Message == "" {
			t.Fatalf("publicError(%q) = %#v", code, got)
		}
		if code == "UNKNOWN" && got.Code != contracts.ErrInternal {
			t.Fatalf("unknown code = %q", got.Code)
		}
	}
	app := contracts.NewAppError(contracts.ErrSessionBusy, "secret details", false)
	if got := safeError(app, contracts.ErrInternal); got.Code != contracts.ErrSessionBusy || got.Message == "secret details" {
		t.Fatalf("safe public error: %#v", got)
	}
	for _, tc := range []struct {
		err  error
		code string
	}{
		{policy.ErrInvalidWorkspace, contracts.ErrWorkspaceDenied},
		{policy.ErrUnknownProfile, contracts.ErrInvalidArgument},
		{policy.ErrHostMismatch, contracts.ErrInvalidArgument},
		{policy.ErrKindMismatch, contracts.ErrInvalidArgument},
		{policy.ErrMissingMCP, contracts.ErrInvalidArgument},
		{policy.ErrInvalidAgent, contracts.ErrInvalidArgument},
		{errors.New("x"), contracts.ErrInternal},
	} {
		if got := safeError(tc.err, contracts.ErrInternal); got.Code != tc.code {
			t.Errorf("safeError(%v) = %s", tc.err, got.Code)
		}
	}
	if got := safeError(context.Canceled, contracts.ErrInternal); got.Code != contracts.ErrInternal {
		t.Fatalf("fallback mapping: %#v", got)
	}
}
