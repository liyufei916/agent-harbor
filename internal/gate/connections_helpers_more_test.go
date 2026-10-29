package gate

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMapProbeError(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
		err  error
		want error
	}{
		{"cancel", context.Background(), context.Canceled, ErrCancelled},
		{"deadline", context.Background(), context.DeadlineExceeded, ErrUpstreamTimeout},
		{"ctx deadline", deadlineContext(), errors.New("failed"), ErrUpstreamTimeout},
		{"text timeout", context.Background(), errors.New("request timeout"), ErrUpstreamTimeout},
		{"other", context.Background(), errors.New("connection reset"), ErrUpstreamDown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mapProbeErr(tc.ctx, tc.err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapProbeErr() = %v, want %v", got, tc.want)
			}
		})
	}
}

func deadlineContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), nowMinusSecond())
	cancel()
	return ctx
}

func nowMinusSecond() time.Time { return time.Now().Add(-time.Second) }
