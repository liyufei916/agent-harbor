package gate

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPGatewayPureValidationBranches(t *testing.T) {
	for _, host := range []string{"", "LOCAL.example", "localhost", "a.localhost", "example", "127.0.0.1", "-bad.example", "bad-.example", "bad..example", "bad/path.example", "good.example"} {
		got := validDemoAskHost(host)
		if host == "good.example" && !got {
			t.Fatalf("valid host rejected: %q", host)
		}
		if host != "good.example" && got {
			t.Fatalf("invalid host accepted: %q", host)
		}
	}
	if toolAllowed([]string{"read"}, "") || toolAllowed([]string{"read"}, "write") || !toolAllowed([]string{"read"}, "read") {
		t.Fatal("tool allowlist branches incorrect")
	}
	for _, body := range [][]byte{nil, []byte("{}"), []byte(`{"jsonrpc":"1.0","method":"x"}`), []byte(`{"jsonrpc":"2.0"}`), []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read"}}`)} {
		method, tool, ok := parseMCPRequest(body)
		if len(bytes.TrimSpace(body)) == 0 && (!ok || method != "" || tool != "") {
			t.Fatalf("empty request=%q %q %q %v", body, method, tool, ok)
		}
	}
}

func TestMCPGatewayFilteringAndFallbackShapes(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","result":{"tools":[{"name":"read"},{"name":"write"},{"bad":true}]}}`)
	filtered := filterToolsPayload(body, []string{"read"})
	if !bytes.Contains(filtered, []byte(`"read"`)) || bytes.Contains(filtered, []byte(`"write"`)) {
		t.Fatalf("filtered=%s", filtered)
	}
	sse := []byte("event: message\ndata: " + string(body) + "\n\n")
	if out := filterToolsPayload(sse, []string{"read"}); !bytes.Contains(out, []byte(`"read"`)) || bytes.Contains(out, []byte(`"write"`)) {
		t.Fatalf("sse filtered=%s", out)
	}
	if got, ok := toolsSSEToJSON(sse); !ok || !bytes.Contains(got, []byte(`"jsonrpc"`)) {
		t.Fatalf("sse conversion=%s %v", got, ok)
	}
	if _, ok := toolsSSEToJSON([]byte("data: nope\n")); ok {
		t.Fatal("invalid SSE converted")
	}
	rr := httptest.NewRecorder()
	writeDiscoverLegacyFallback(rr, []byte(`{"jsonrpc":"2.0","id":7}`))
	var response map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil || response["error"] == nil {
		t.Fatalf("fallback=%s err=%v", rr.Body.Bytes(), err)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Fatal("fallback content type missing")
	}
}
