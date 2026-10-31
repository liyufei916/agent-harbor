package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBridgeInitializeAndHTTPProxy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stdio fixture uses python3")
	}
	py, err := filepath.Abs("testdata/fake_mcp.py")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(py); err != nil {
		t.Fatal(err)
	}
	python := ""
	for _, cand := range []string{"/usr/bin/python3", "/usr/local/bin/python3", "python3"} {
		if path, lookErr := exec.LookPath(cand); lookErr == nil {
			python = path
			break
		}
		if _, statErr := os.Stat(cand); statErr == nil {
			python = cand
			break
		}
	}
	if python == "" {
		t.Skip("python3 not found")
	}
	absPython, err := filepath.Abs(python)
	if err != nil {
		absPython = python
	}
	if !filepath.IsAbs(absPython) {
		if resolved, lookErr := exec.LookPath(python); lookErr == nil {
			absPython = resolved
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	bridge, err := Start(ctx, Spec{Command: absPython, Args: []string{py}})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	if bridge.Origin == "" || len(bridge.Tools) != 1 || bridge.Tools[0] != "echo" {
		t.Fatalf("bridge=%+v", bridge)
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": "echo", "arguments": map[string]any{}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bridge.Origin+"/mcp", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body %s", resp.StatusCode, raw)
	}

	// Codex 等客户端连上后会再发 initialize；须用缓存，不得二次写子进程。
	initBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]string{"name": "codex", "version": "0"},
		},
	})
	initReq, err := http.NewRequestWithContext(ctx, http.MethodPost, bridge.Origin+"/mcp", bytes.NewReader(initBody))
	if err != nil {
		t.Fatal(err)
	}
	initResp, err := http.DefaultClient.Do(initReq)
	if err != nil {
		t.Fatal(err)
	}
	defer initResp.Body.Close()
	initRaw, _ := io.ReadAll(initResp.Body)
	if initResp.StatusCode != 200 {
		t.Fatalf("re-initialize status %d body %s", initResp.StatusCode, initRaw)
	}
	notifyBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})
	notifyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, bridge.Origin+"/mcp", bytes.NewReader(notifyBody))
	if err != nil {
		t.Fatal(err)
	}
	notifyResp, err := http.DefaultClient.Do(notifyReq)
	if err != nil {
		t.Fatal(err)
	}
	notifyResp.Body.Close()
	if notifyResp.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notify status %d", notifyResp.StatusCode)
	}
	// 二次 initialize 后 tools/call 仍应可用（子进程未退出）
	body2, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params":  map[string]any{"name": "echo", "arguments": map[string]any{}},
	})
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, bridge.Origin+"/mcp", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	raw2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != 200 {
		t.Fatalf("after re-init call status %d body %s", resp2.StatusCode, raw2)
	}

	// 现代 SDK era 探针：须 HTTP 200 + JSON-RPC Method not found，勿 5xx / 勿写子进程。
	discoverBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "server-discover-probe-1",
		"method":  "server/discover",
		"params":  map[string]any{},
	})
	discoverReq, err := http.NewRequestWithContext(ctx, http.MethodPost, bridge.Origin+"/mcp", bytes.NewReader(discoverBody))
	if err != nil {
		t.Fatal(err)
	}
	discoverResp, err := http.DefaultClient.Do(discoverReq)
	if err != nil {
		t.Fatal(err)
	}
	defer discoverResp.Body.Close()
	discoverRaw, _ := io.ReadAll(discoverResp.Body)
	if discoverResp.StatusCode != 200 {
		t.Fatalf("server/discover status %d body %s", discoverResp.StatusCode, discoverRaw)
	}
	var discoverOut struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(discoverRaw, &discoverOut) != nil || discoverOut.Error == nil || discoverOut.Error.Code != -32601 {
		t.Fatalf("server/discover want method-not-found, got %s", discoverRaw)
	}
}

func TestEncodeDecodeSecret(t *testing.T) {
	raw, err := EncodeSecret(Spec{Command: "/bin/true", Args: []string{"a"}, Env: map[string]string{"K": "V"}})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := DecodeSecret(raw)
	if err != nil || spec.Command != "/bin/true" || spec.Env["K"] != "V" {
		t.Fatalf("%+v %v", spec, err)
	}
}

func TestBridgeRejectsInvalidSpecsAndSecret(t *testing.T) {
	ctx := context.Background()
	for _, spec := range []Spec{{}, {Command: "relative"}, {Command: "/bin/true", Args: []string{"bad\x00"}}, {Command: "/bin/true", Env: map[string]string{"BAD=KEY": "x"}}} {
		if _, err := Start(ctx, spec); err == nil {
			t.Fatalf("invalid spec accepted: %+v", spec)
		}
	}
	if _, err := DecodeSecret([]byte("bad")); err == nil {
		t.Fatal("invalid secret decoded")
	}
}

func TestBridgeFramingAndToolParsing(t *testing.T) {
	for _, raw := range []string{"", "Content-Length: x\r\n\r\n", "Content-Length: 0\r\n\r\n", "Content-Length: 3\r\n\r\nxy"} {
		if _, err := readFramed(bufio.NewReader(strings.NewReader(raw))); err == nil {
			t.Fatalf("invalid frame accepted: %q", raw)
		}
	}
	payload := "Content-Length: 3\r\nX-Test: ok\r\n\r\nabc"
	got, err := readFramed(bufio.NewReader(strings.NewReader(payload)))
	if err != nil || string(got) != "abc" {
		t.Fatalf("frame=%q err=%v", got, err)
	}
	if got := parseToolNames(json.RawMessage(`{"tools":[{"name":" echo "},{"name":""},{"name":"read"}]}`)); len(got) != 2 || got[0] != "echo" {
		t.Fatalf("tools=%v", got)
	}
	if parseToolNames(json.RawMessage("bad")) != nil {
		t.Fatal("bad tools parsed")
	}
}

func TestBridgeHTTPValidationAndCachedMethods(t *testing.T) {
	b := &Bridge{initResult: json.RawMessage(`{"serverInfo":{"name":"fake"}}`), pending: map[string]chan json.RawMessage{}}
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{"GET", "", http.StatusMethodNotAllowed}, {"POST", "bad", http.StatusBadRequest},
		{"POST", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, http.StatusOK},
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"server/discover"}`, http.StatusOK},
		{"POST", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, http.StatusAccepted},
	} {
		req := httptest.NewRequest(tc.method, "/mcp", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		b.handleHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s %s => %d want %d body=%s", tc.method, tc.body, w.Code, tc.status, w.Body)
		}
	}
	b.closed = true
	if _, err := b.call(context.Background(), "x", nil); err == nil {
		t.Fatal("closed bridge call succeeded")
	}
}

func TestBridgeFramingAndClosedNotifications(t *testing.T) {
	var out bytes.Buffer
	if err := writeFramed(&out, []byte(`{"jsonrpc":"2.0"}`)); err != nil || !strings.Contains(out.String(), "Content-Length:") {
		t.Fatalf("framed=%q err=%v", out.String(), err)
	}
	b := &Bridge{closed: true, pending: map[string]chan json.RawMessage{}}
	if err := b.notify(context.Background(), "x", nil); err == nil {
		t.Fatal("closed notify succeeded")
	}
	ch := make(chan json.RawMessage, 1)
	b = &Bridge{pending: map[string]chan json.RawMessage{"1": ch}}
	b.failAll()
	if _, ok := <-ch; ok {
		t.Fatal("pending channel should be closed")
	}
	if parseToolNames(json.RawMessage(`{"tools":[{},null,{"name":123}]}`)) != nil {
		t.Fatal("invalid tool entries should produce no usable names")
	}
}
