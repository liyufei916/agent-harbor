package gate

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestMapProbeErrClassifiesCancellationTimeoutAndUpstreamFailure(t *testing.T) {
	if got := mapProbeErr(context.Background(), context.Canceled); got != ErrCancelled {
		t.Fatalf("cancel=%v", got)
	}
	if got := mapProbeErr(context.Background(), context.DeadlineExceeded); got != ErrUpstreamTimeout {
		t.Fatalf("deadline=%v", got)
	}
	if got := mapProbeErr(context.Background(), errors.New("dial timeout")); got != ErrUpstreamTimeout {
		t.Fatalf("timeout=%v", got)
	}
	if got := mapProbeErr(context.Background(), errors.New("connection reset")); !errors.Is(got, ErrUpstreamDown) {
		t.Fatalf("upstream=%v", got)
	}
}

func TestReviewConnectRejectsUnauthenticated404(t *testing.T) {
	up := httptest.NewServer(http.NotFoundHandler())
	defer up.Close()
	s := secrets.NewStore()
	p := policy.NewBuilder(map[string]policy.FixedProfile{"m": {ID: "m", Kind: "model", Protocol: "anthropic", Model: "fixture", AllowInsecureLoopback: true, Origin: up.URL, Host: "127.0.0.1", TestPath: "/missing"}})
	_, err := NewConnections(s, p).ConnectModel(context.Background(), contracts.ConnectInput{ProfileID: "m", Name: "review", APIKey: "review-fixture-only"})
	if err == nil || s.Count() != 0 {
		t.Fatalf("未验证认证的 404 被判成功，账号数=%d，error=%v", s.Count(), err)
	}
}

func TestReviewConnectDoesNotSendKeyToRedirectOrigin(t *testing.T) {
	var received atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
	}))
	defer other.Close()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer up.Close()
	p := policy.NewBuilder(map[string]policy.FixedProfile{"m": {ID: "m", Kind: "model", Protocol: "anthropic", Model: "fixture", AllowInsecureLoopback: true, Origin: up.URL, Host: "127.0.0.1", TestPath: "/"}})
	_, _ = NewConnections(secrets.NewStore(), p).ConnectModel(context.Background(), contracts.ConnectInput{ProfileID: "m", Name: "review", APIKey: "review-fixture-only"})
	if received.Load() {
		t.Fatal("不同端口的重定向目的 origin 收到了测试认证头")
	}
}

func TestCanceledConnectionDoesNotStoreCandidate(t *testing.T) {
	for _, kind := range []string{"model", "mcp"} {
		t.Run(kind, func(t *testing.T) {
			entered := make(chan struct{})
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(entered)
				<-r.Context().Done()
			}))
			defer up.Close()
			prof := policy.FixedProfile{ID: "fixture", Kind: kind, Protocol: "anthropic", Model: "fixture", AllowInsecureLoopback: true, Origin: up.URL, Host: "127.0.0.1", TestPath: "/v1/messages", Tools: []string{"approved"}}
			if kind == "mcp" {
				prof.Protocol = "streamable-http"
				prof.TestPath = "/mcp"
			}
			store := secrets.NewStore()
			conn := NewConnections(store, policy.NewBuilder(map[string]policy.FixedProfile{"fixture": prof}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := conn.connect(ctx, contracts.ConnectInput{ProfileID: "fixture", Name: "fixture", APIKey: "fixture"}, kind)
				done <- err
			}()
			<-entered
			cancel()
			if err := <-done; err != ErrCancelled || store.Count() != 0 {
				t.Fatalf("候选凭据残留：%v count=%d", err, store.Count())
			}
		})
	}
}
