package gate

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/policy"
)

func TestMCPPayloadHelpers(t *testing.T) {
	if !toolAllowed([]string{"read"}, "read") || toolAllowed([]string{"read"}, "write") || toolAllowed(nil, "") {
		t.Fatal("tool allowlist mismatch")
	}
	method, tool, ok := parseMCPRequest([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read"}}`))
	if !ok || method != "tools/call" || tool != "read" {
		t.Fatalf("parsed request: %q %q %v", method, tool, ok)
	}
	for _, body := range [][]byte{nil, []byte(`bad`), []byte(`{"method":"missing-rpc"}`)} {
		if _, _, ok := parseMCPRequest(body); body != nil && ok {
			t.Fatalf("invalid request accepted: %q", body)
		}
	}
	body := []byte(`{"jsonrpc":"2.0","result":{"tools":[{"name":"read"},{"name":"write"}]}}`)
	filtered := filterToolsPayload(body, []string{"read"})
	if bytes.Contains(filtered, []byte(`"write"`)) || !bytes.Contains(filtered, []byte(`"read"`)) {
		t.Fatalf("tools not filtered: %s", filtered)
	}
	sse := []byte("event: message\ndata: " + string(body) + "\n\n")
	filteredSSE := filterToolsPayload(sse, []string{"read"})
	if bytes.Contains(filteredSSE, []byte(`"write"`)) {
		t.Fatalf("SSE tools not filtered: %s", filteredSSE)
	}
	if got := filterToolsPayload([]byte("invalid"), nil); string(got) != "invalid" {
		t.Fatalf("invalid payload changed: %s", got)
	}
	if got, ok := toolsSSEToJSON([]byte("data: {\"jsonrpc\":\"2.0\"}\n")); !ok || len(got) == 0 {
		t.Fatalf("SSE conversion failed: %s %v", got, ok)
	}
}

func TestMCPHeadersAndMountSelection(t *testing.T) {
	dst := make(http.Header)
	src := http.Header{"Content-Type": []string{"application/json"}, "Mcp-Session-Id": []string{"sid"}}
	copyMCPHeaders(dst, src)
	if dst.Get("Content-Type") != "application/json" || dst.Get("Accept") == "" || dst.Get("Mcp-Session-Id") != "sid" {
		t.Fatalf("headers: %#v", dst)
	}
	if got, err := readToolsList(bytes.NewBufferString(`{"result":{"tools":[]}}`), "application/json", 1024); err != nil || len(got) == 0 {
		t.Fatalf("read JSON tools: %s %v", got, err)
	}
	if _, err := readToolsList(errorReader{}, "application/json", 1024); err == nil {
		t.Fatal("reader error ignored")
	}
	rt := &sessionRuntime{spec: GateSpec{Policy: policy.PolicySnapshot{MCPMounts: []policy.MCPMount{{AccountID: "a"}, {AccountID: "b"}}}}}
	ln := httptest.NewRecorder()
	_ = ln
	g := &mcpGateway{}
	if _, ok := g.mountForPath(rt, "/mcp"); ok {
		t.Fatal("ambiguous single path accepted for multiple mounts")
	}
	if mount, ok := g.mountForPath(rt, "/mcp/1"); !ok || mount.AccountID != "b" {
		t.Fatalf("mount selection: %#v %v", mount, ok)
	}
	if _, ok := g.mountForPath(rt, "/mcp/9"); ok {
		t.Fatal("unknown mount accepted")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
