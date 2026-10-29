package gate

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSentinelAndRedirectHelpers(t *testing.T) {
	token, err := newSentinel()
	if err != nil || !looksLikeSentinel(token) {
		t.Fatalf("sentinel: %q %v", token, err)
	}
	for _, raw := range []string{"", "Basic x", "Bearer harbor_sentinel_short", "harbor_sentinel_short"} {
		if _, ok := parseBearerSentinel(raw); ok {
			t.Fatalf("invalid sentinel accepted: %q", raw)
		}
	}
	if !sentinelMatch(token, token) || sentinelMatch(token, token+"x") || sentinelMatch("", token) {
		t.Fatal("sentinel comparison mismatch")
	}
	for _, tc := range []struct {
		status       int
		method, want string
	}{
		{301, http.MethodPost, http.MethodGet}, {302, http.MethodPut, http.MethodGet}, {303, http.MethodPatch, http.MethodGet}, {307, http.MethodPost, http.MethodPost}, {308, "", http.MethodGet},
	} {
		if got := redirectFollowMethod(tc.status, tc.method); got != tc.want {
			t.Errorf("redirectFollowMethod = %q, want %q", got, tc.want)
		}
	}
	if hostOfOrigin("https://Example.com/path") != "Example.com" || authorityOfOrigin("https://Example.com:443") != "example.com:443" {
		t.Fatal("origin helper mismatch")
	}
}

func TestStreamTimeoutAndRelayRedaction(t *testing.T) {
	h := &Harbor{idleTimeout: 0, totalTimeout: 0}
	idle, total := h.streamTimeouts()
	if idle <= 0 || total <= 0 {
		t.Fatal("default timeouts missing")
	}
	ctx, cancel := h.upstreamContext(nil)
	cancel()
	if ctx.Err() == nil {
		t.Fatal("upstream context not cancellable")
	}
	if streamTimeoutStatus(errStreamIdle) != http.StatusGatewayTimeout || streamTimeoutStatus(io.ErrUnexpectedEOF) != http.StatusBadGateway {
		t.Fatal("timeout status mismatch")
	}
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Key": []string{"secret-value"}}, Body: io.NopCloser(bytes.NewBufferString("hello secret-value"))}
	rec := httptest.NewRecorder()
	if err := relayResponse(rec, resp, []string{"secret-value"}, time.Second, time.Second); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("secret-value")) || !bytes.Contains(rec.Body.Bytes(), []byte("[redacted]")) {
		t.Fatalf("secret leaked: %s", rec.Body.Bytes())
	}
	_ = context.Background()
}
