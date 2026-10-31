// 非 context7 远程 HTTP MCP 接通证据：本机受控 HTTP 回环上游 → Connections.ConnectMcp → demos/evidence。
// 产品主路径仍是公网 HTTPS；回环仅作命令级接通证明（policy AllowInsecureLoopback）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/gate"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

const evidenceTool = "harbor_evidence_echo"

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(err)
	}
	defer ln.Close()
	srv := &http.Server{Handler: http.HandlerFunc(mcpHandler)}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	origin := "http://" + ln.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := secrets.NewStore()
	conn := gate.NewConnections(store, policy.NewBuilder(nil))
	meta, err := conn.ConnectMcp(ctx, contracts.ConnectInput{
		Name:      "harbor-evidence-mcp",
		ProfileID: "custom",
		APIKey:    "evidence-token",
		Origin:    origin,
	})
	if err != nil {
		fatal(err)
	}

	outDir := filepath.Join(root, "demos", "evidence")
	_ = os.MkdirAll(outDir, 0755)
	stamp := time.Now().Format("2006-01-02")
	path := filepath.Join(outDir, "mcp-remote-non-context7-"+stamp+".txt")
	tools := strings.Join(meta.Tools, ",")
	body := fmt.Sprintf(`# 非 context7 远程 HTTP MCP 接通证据
# 日期：%s
# 命令：go run ./scripts/cmd/mcp-remote-evidence
# 上游：本机受控 Streamable HTTP（http://127.0.0.1 · AllowInsecureLoopback）
# 说明：证明非 Context7 的远程 HTTP ConnectMcp 路径；公网用户仍用 HTTPS Origin。

ORIGIN=%s
accountId=%s
host=%s
transport=%s
authMode=%s
tools=%s
keyHint=%s
PASS: ConnectMcp non-context7 remote HTTP
`, stamp, origin, meta.ID, meta.Host, meta.Transport, meta.AuthMode, tools, meta.KeyHint)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		fatal(err)
	}
	fmt.Print(body)
	fmt.Println("wrote", path)
}

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/mcp") {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch msg.Method {
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
		return
	case "initialize":
		writeJSON(w, map[string]any{
			"jsonrpc": "2.0",
			"id":      jsonRawOrOne(msg.ID),
			"result": map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "harbor-evidence-mcp", "version": "0.1.0"},
			},
		})
	case "tools/list":
		writeJSON(w, map[string]any{
			"jsonrpc": "2.0",
			"id":      jsonRawOrOne(msg.ID),
			"result": map[string]any{
				"tools": []map[string]any{{
					"name":        evidenceTool,
					"description": "Evidence-only echo (non-context7 remote HTTP).",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
				}},
			},
		})
	default:
		writeJSON(w, map[string]any{
			"jsonrpc": "2.0",
			"id":      jsonRawOrOne(msg.ID),
			"error":   map[string]any{"code": -32601, "message": "Method not found"},
		})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func jsonRawOrOne(id json.RawMessage) any {
	if len(id) == 0 {
		return 1
	}
	var v any
	if json.Unmarshal(id, &v) == nil {
		return v
	}
	return 1
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "FAIL:", err)
	os.Exit(1)
}
