package mcpstdio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
