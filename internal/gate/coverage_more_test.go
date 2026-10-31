package gate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDemoAskValidationAndParsing(t *testing.T) {
	valid := []string{"demo-ask.example", "sub.demo-ask.example"}
	for _, host := range valid {
		if !validDemoAskHost(host) {
			t.Fatalf("valid host rejected: %s", host)
		}
	}
	for _, host := range []string{"", "LOCAL.example", "localhost", "example", "127.0.0.1", "a..example", "-bad.example", "bad-.example", "bad/example"} {
		if validDemoAskHost(host) {
			t.Fatalf("invalid host accepted: %s", host)
		}
	}
	body := []byte(`{"jsonrpc":"2.0","id":7,"params":{"name":"harbor_demo_ask","arguments":{"host":"Demo-Ask.Example"}}}`)
	id, host, ok := parseDemoAskCall(body)
	if !ok || host != "demo-ask.example" || string(id) != "7" {
		t.Fatalf("parse=%s %s %v", id, host, ok)
	}
	if _, _, ok := parseDemoAskCall([]byte(`{"params":{"name":"other"}}`)); ok {
		t.Fatal("other tool parsed")
	}
	if _, _, ok := parseDemoAskCall([]byte("bad")); ok {
		t.Fatal("invalid json parsed")
	}
}

func TestWriteJSONRPCToolText(t *testing.T) {
	for _, hasID := range []bool{true, false} {
		w := httptest.NewRecorder()
		var id json.RawMessage
		if hasID {
			id = json.RawMessage(`3`)
		}
		writeJSONRPCToolText(w, id, hasID, "done")
		if !strings.Contains(w.Body.String(), "done") || w.Code != 200 {
			t.Fatalf("response=%d %s", w.Code, w.Body)
		}
	}
}

func TestConnectionErrorHelpers(t *testing.T) {
	for _, raw := range []string{"  'sk-test' ", "\ufeffabc\u200b", "a\n b"} {
		if got := sanitizeAPIKey(raw); strings.ContainsAny(got, "\n\ufeff\u200b'") {
			t.Fatalf("sanitize=%q", got)
		}
	}
	for _, raw := range []string{"invalid proxy server token", "invalid api key", "", "other"} {
		err := authProbeError("api.example", 401, []byte(raw))
		if err == nil || !strings.Contains(err.Error(), "401") {
			t.Fatalf("auth=%v", err)
		}
	}
	if !isUpstreamAuthErr(ErrUpstreamAuth) || isUpstreamAuthErr(nil) {
		t.Fatal("auth classifier")
	}
	if got := wrapResponsesProbeErr(ErrUpstreamTimeout); got == nil || !strings.Contains(got.Error(), "Responses") {
		t.Fatal(got)
	}
	if wrapResponsesProbeErr(nil) != nil {
		t.Fatal("nil wrapped")
	}
	if got := preferProbeErr(ErrUpstreamTimeout, context.Canceled); got == nil {
		t.Fatal("preferred error missing")
	}
	if rejectCodexOnAnthropicOnlyHost("deepseek.com", "", "https://deepseek.com", []string{"codex"}) == nil {
		t.Fatal("codex accepted on DeepSeek")
	}
	if err := rejectCodexOnAnthropicOnlyHost("example.com", "", "https://example.com", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
}

func TestProbeBodyAndRPCDecoding(t *testing.T) {
	good := httptest.NewRequest("GET", "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	good.Header.Set("Content-Type", "application/json")
	if err := decodeRPC(&http.Response{Body: io.NopCloser(good.Body), Header: good.Header}, 1, &map[string]any{}); err != nil {
		t.Fatal(err)
	}
	bad := httptest.NewRequest("GET", "/", strings.NewReader(`{"jsonrpc":"2.0","id":2,"result":null}`))
	if err := decodeRPC(&http.Response{Body: io.NopCloser(bad.Body), Header: make(http.Header)}, 1, &map[string]any{}); err == nil {
		t.Fatal("invalid rpc accepted")
	}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("body"))}
	if raw, err := readProbeBody(resp); err != nil || string(raw) != "body" {
		t.Fatalf("body=%q err=%v", raw, err)
	}
}

func TestSmallConnectionHelpers(t *testing.T) {
	if got := uniqStrings([]string{"a", "a", " b ", ""}); len(got) != 2 {
		t.Fatalf("uniq=%v", got)
	}
	for _, v := range []string{"streamable-http", "sse", "SSE", ""} {
		_ = mcpProtocolAccepted(v)
	}
	if mapProbeErr(context.Background(), context.Canceled) == nil || mapProbeErr(context.Background(), context.DeadlineExceeded) == nil {
		t.Fatal("probe errors not mapped")
	}
	if annotateAuthKeyMeta(nil, "x") != nil {
		t.Fatal("nil auth metadata changed")
	}
}
