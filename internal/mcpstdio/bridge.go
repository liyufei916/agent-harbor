// Package mcpstdio 在宿主机拉起 stdio MCP，并暴露本机回环 HTTP，供现有 MCP Gateway 转发。
// 沙箱执行面只见 Gateway；命令与环境变量留在宿主机。
package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Spec 宿主侧启动参数；Env 仅本机进程可见。
type Spec struct {
	Command string
	Args    []string
	Env     map[string]string
}

// Bridge 持有子进程与回环 HTTP。
type Bridge struct {
	Origin string
	Tools  []string

	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	ln         net.Listener
	server     *http.Server
	nextID     int
	pending    map[string]chan json.RawMessage
	initResult json.RawMessage // Start 已 initialize；Codex/Claude 再发时回缓存，避免二次 init 弄死 stdio 子进程
	closed     bool
}

// Start 启动 stdio 进程、完成 initialize + tools/list，并监听 127.0.0.1:0/mcp。
func Start(ctx context.Context, spec Spec) (*Bridge, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" || !filepath.IsAbs(command) {
		return nil, fmt.Errorf("mcpstdio: command must be absolute path")
	}
	info, err := os.Stat(command)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("mcpstdio: command not executable")
	}
	cmd := exec.CommandContext(ctx, command, spec.Args...)
	cmd.Env = append([]string{}, os.Environ()...)
	for k, v := range spec.Env {
		k = strings.TrimSpace(k)
		if k == "" || strings.ContainsAny(k, "=\x00\n\r") {
			return nil, fmt.Errorf("mcpstdio: invalid env key")
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	b := &Bridge{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
		pending: make(map[string]chan json.RawMessage),
	}
	go b.readLoop()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = b.Close()
		return nil, err
	}
	b.ln = ln
	b.Origin = "http://" + ln.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", b.handleHTTP)
	mux.HandleFunc("/mcp/", b.handleHTTP)
	b.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = b.server.Serve(ln) }()

	initCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	initRaw, err := b.call(initCtx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "agent-harbor", "version": "1"},
	})
	if err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("mcpstdio initialize: %w", err)
	}
	b.initResult = initRaw
	_ = b.notify(initCtx, "notifications/initialized", map[string]any{})
	raw, err := b.call(initCtx, "tools/list", map[string]any{})
	if err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("mcpstdio tools/list: %w", err)
	}
	b.Tools = parseToolNames(raw)
	if len(b.Tools) == 0 {
		_ = b.Close()
		return nil, fmt.Errorf("mcpstdio: no tools")
	}
	return b, nil
}

func (b *Bridge) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Method == "" {
		http.Error(w, "invalid json-rpc", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var params any
	if len(envelope.Params) > 0 {
		_ = json.Unmarshal(envelope.Params, &params)
	}
	if envelope.ID == nil || string(envelope.ID) == "null" {
		// 已在 Start 完成 initialized；上游客户端（如 Codex rmcp）再通知时吞掉，勿写子进程。
		if envelope.Method == "notifications/initialized" || envelope.Method == "initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_ = b.notify(ctx, envelope.Method, params)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if envelope.Method == "initialize" && len(b.initResult) > 0 {
		out, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(envelope.ID),
			"result":  json.RawMessage(b.initResult),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
		return
	}
	// 现代 MCP SDK 会先 POST server/discover 做 era 协商；经典 stdio 子进程不认此方法。
	// 若原样转发，子进程常以 5xx/断管回应，SDK 会把 HTTP 5xx 判为协商失败并退出。
	// 回 JSON-RPC Method not found（HTTP 200）可让客户端回落到 legacy initialize。
	if envelope.Method == "server/discover" {
		writeJSONRPCMethodNotFound(w, envelope.ID)
		return
	}
	result, err := b.call(ctx, envelope.Method, params)
	if err != nil {
		msg := "stdio mcp: " + err.Error()
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "broken pipe") || strings.Contains(low, "file already closed") {
			msg = "stdio mcp process closed"
		}
		http.Error(w, msg, http.StatusBadGateway)
		return
	}
	out, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(envelope.ID),
		"result":  json.RawMessage(result),
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

func (b *Bridge) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, fmt.Errorf("bridge closed")
	}
	b.nextID++
	id := strconv.Itoa(b.nextID)
	ch := make(chan json.RawMessage, 1)
	b.pending[id] = ch
	msg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	err := writeFramed(b.stdin, msg)
	b.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, ctx.Err()
	case raw := <-ch:
		return raw, nil
	}
}

func (b *Bridge) notify(ctx context.Context, method string, params any) error {
	_ = ctx
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return fmt.Errorf("bridge closed")
	}
	msg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	return writeFramed(b.stdin, msg)
}

func (b *Bridge) readLoop() {
	for {
		payload, err := readFramed(b.stdout)
		if err != nil {
			b.failAll()
			return
		}
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(payload, &envelope) != nil || envelope.ID == nil {
			continue
		}
		id := strings.Trim(string(envelope.ID), `"`)
		b.mu.Lock()
		ch := b.pending[id]
		delete(b.pending, id)
		b.mu.Unlock()
		if ch == nil {
			continue
		}
		if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			ch <- json.RawMessage(`{"error":true}`)
			continue
		}
		if len(envelope.Result) == 0 {
			envelope.Result = json.RawMessage("{}")
		}
		ch <- envelope.Result
	}
}

func (b *Bridge) failAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, ch := range b.pending {
		delete(b.pending, id)
		close(ch)
	}
}

// Close 停止 HTTP 与子进程。
func (b *Bridge) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	if b.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = b.server.Shutdown(ctx)
		cancel()
	}
	if b.ln != nil {
		_ = b.ln.Close()
	}
	if b.stdin != nil {
		_ = b.stdin.Close()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_, _ = b.cmd.Process.Wait()
	}
	b.failAll()
	return nil
}

func writeFramed(w io.Writer, msg []byte) error {
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(msg), msg)
	return err
}

func readFramed(r *bufio.Reader) ([]byte, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("Content-Length:"):]))
			if err != nil || n < 0 || n > 1<<20 {
				return nil, fmt.Errorf("bad content-length")
			}
			length = n
		}
	}
	if length == 0 {
		return nil, fmt.Errorf("missing content-length")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeJSONRPCMethodNotFound(w http.ResponseWriter, id json.RawMessage) {
	out, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error":   map[string]any{"code": -32601, "message": "Method not found"},
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

func parseToolNames(raw json.RawMessage) []string {
	var body struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return nil
	}
	out := make([]string, 0, len(body.Tools))
	for _, t := range body.Tools {
		name := strings.TrimSpace(t.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// SecretPayload 写入 Keychain / 文件库的 JSON。
type SecretPayload struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func EncodeSecret(spec Spec) ([]byte, error) {
	return json.Marshal(SecretPayload{Command: spec.Command, Args: append([]string(nil), spec.Args...), Env: spec.Env})
}

func DecodeSecret(raw []byte) (Spec, error) {
	var p SecretPayload
	if err := json.Unmarshal(bytes.TrimSpace(raw), &p); err != nil {
		return Spec{}, err
	}
	return Spec{Command: p.Command, Args: p.Args, Env: p.Env}, nil
}
