package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestMCPConnectRequiresHandshakeAndSelectedTools(t *testing.T) {
	for _, scenario := range []string{"json", "sse", "missing-tool", "rpc-error", "wrong-id", "auth", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			var methods []string
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture-token" {
					t.Error("认证头错误")
				}
				if scenario == "auth" {
					w.WriteHeader(401)
					return
				}
				if scenario == "redirect" {
					w.Header().Set("Location", "/other")
					w.WriteHeader(307)
					return
				}
				if r.Method == "DELETE" {
					methods = append(methods, "DELETE")
					w.WriteHeader(204)
					return
				}
				var req struct {
					Method string `json:"method"`
					ID     int    `json:"id"`
					Params struct {
						Cursor string `json:"cursor"`
					} `json:"params"`
				}
				if json.NewDecoder(r.Body).Decode(&req) != nil {
					t.Error("无效请求")
				}
				methods = append(methods, req.Method)
				if req.Method != "initialize" && r.Header.Get("Mcp-Session-Id") != "fixture-session" {
					t.Error("丢失上游会话标识")
				}
				if req.Method == "notifications/initialized" {
					w.WriteHeader(202)
					return
				}
				var result any
				switch req.Method {
				case "initialize":
					w.Header().Set("Mcp-Session-Id", "fixture-session")
					result = map[string]any{"protocolVersion": mcpProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "fixture", "version": "1"}}
				case "tools/list":
					name := "approved"
					if scenario == "missing-tool" {
						name = "other"
					}
					if req.Params.Cursor == "" {
						result = map[string]any{"tools": []any{}, "nextCursor": "next"}
					} else {
						result = map[string]any{"tools": []any{map[string]string{"name": name}}, "nextCursor": ""}
					}
				default:
					t.Error("连接测试不应调用工具或未知方法")
				}
				id := req.ID
				if scenario == "wrong-id" {
					id += 100
				}
				msg := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
				if scenario == "rpc-error" {
					delete(msg, "result")
					msg["error"] = map[string]any{"code": -32603, "message": "fixture"}
				}
				raw, _ := json.Marshal(msg)
				if scenario == "sse" {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", raw)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.Write(raw)
				}
			}))
			defer up.Close()
			store := secrets.NewStore()
			p := policy.NewBuilder(map[string]policy.FixedProfile{"mcp": {ID: "mcp", Kind: "mcp", Protocol: "streamable-http", Origin: up.URL, Host: "127.0.0.1", TestPath: "/mcp", Tools: []string{"approved"}, AllowInsecureLoopback: true}})
			account, err := NewConnections(store, p).ConnectMcp(context.Background(), contracts.ConnectInput{ProfileID: "mcp", Name: "fixture", APIKey: "fixture-token"})
			success := scenario == "json" || scenario == "sse"
			if success {
				if err != nil || account.ID == "" || store.Count() != 1 {
					t.Fatalf("%+v %v", account, err)
				}
				if strings.Join(methods, ",") != "initialize,notifications/initialized,tools/list,tools/list,DELETE" {
					t.Fatalf("握手顺序 %v", methods)
				}
			} else if err == nil || store.Count() != 0 {
				t.Fatalf("无效上游仍保存账号 %v", err)
			}
		})
	}
}

// 回归：自定义 MCP（填 Origin）必须落盘 tools/list；否则开会话 Policy.Build → INVALID_ARGUMENT。
func TestCustomMCPConnectPersistsDiscoveredTools(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(204)
			return
		}
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "notifications/initialized" {
			w.WriteHeader(202)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "custom-session")
			result = map[string]any{"protocolVersion": mcpProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "custom", "version": "1"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]string{"name": "resolve-library-id"}, map[string]string{"name": "query-docs"}}}
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer up.Close()

	store := secrets.NewStore()
	conn := NewConnections(store, policy.NewBuilder(map[string]policy.FixedProfile{}))
	account, err := conn.ConnectMcp(context.Background(), contracts.ConnectInput{
		ProfileID: policy.ProfileCustom,
		Name:      "CTX7",
		Origin:    up.URL + "/mcp",
		APIKey:    "fixture-token",
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if account.ProfileID != policy.ProfileCustom {
		t.Fatalf("profile=%s", account.ProfileID)
	}
	if len(account.Tools) != 2 || account.Tools[0] != "resolve-library-id" {
		t.Fatalf("tools not persisted: %+v", account.Tools)
	}
	got, err := store.Lookup(context.Background(), account.ID)
	if err != nil || len(got.Tools) != 2 {
		t.Fatalf("store tools=%v err=%v", got.Tools, err)
	}
}

func TestModelRejectsHTMLAndWrongProtocol(t *testing.T) {
	for _, body := range []string{"<html>ok</html>", `{"ok":true}`, `{"type":"message","content":[]}`} {
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		store := secrets.NewStore()
		p := policy.NewBuilder(map[string]policy.FixedProfile{"m": {ID: "m", Kind: "model", Protocol: "anthropic", Model: "fixture", Origin: up.URL, Host: "127.0.0.1", TestPath: "/v1/messages", AllowInsecureLoopback: true}})
		_, err := NewConnections(store, p).ConnectModel(context.Background(), contracts.ConnectInput{ProfileID: "m", Name: "fixture", APIKey: "fixture"})
		up.Close()
		if err == nil || store.Count() != 0 {
			t.Fatal("错误协议被判成功")
		}
	}
}

func TestOpenAIResponsesModelConnectionUsesBearerAndResponseShape(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Errorf("Codex Responses 请求错误: path=%s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"resp_fixture","object":"response"}`)
	}))
	defer up.Close()
	store := secrets.NewStore()
	p := policy.NewBuilder(map[string]policy.FixedProfile{"codex": {ID: "codex", Kind: "model", Protocol: "openai-responses", Model: "gpt-5-codex", Origin: up.URL, Host: "127.0.0.1", TestPath: "/v1/responses", AllowInsecureLoopback: true}})
	account, err := NewConnections(store, p).ConnectModel(context.Background(), contracts.ConnectInput{ProfileID: "codex", Name: "Codex", APIKey: "fixture-key"})
	if err != nil || account.ID == "" || store.Count() != 1 {
		t.Fatalf("Codex Responses 连接失败: %+v %v", account, err)
	}
}

func TestRejectCodexOnDeepSeekAnthropicOrigin(t *testing.T) {
	store := secrets.NewStore()
	p := policy.NewBuilder(nil)
	_, err := NewConnections(store, p).ConnectModel(context.Background(), contracts.ConnectInput{
		ProfileID: "custom",
		Name:      "我的 DeepSeek",
		APIKey:    "sk-fixture-not-sent",
		Origin:    "https://api.deepseek.com/anthropic",
		Model:     "deepseek-chat",
		Agents:    []string{"codex"},
	})
	if err == nil || store.Count() != 0 {
		t.Fatal("DeepSeek+Codex 不应保存")
	}
	ae, ok := err.(*contracts.AppError)
	if !ok || ae.Code != contracts.ErrInvalidArgument || !strings.Contains(ae.Message, "Codex") {
		t.Fatalf("got %#v", err)
	}
}

func TestOpenAIResponsesProbeFailureSurfacesAppError(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error":"no responses"}`)
	}))
	defer up.Close()
	store := secrets.NewStore()
	p := policy.NewBuilder(map[string]policy.FixedProfile{
		"codex-model": {
			ID: "codex-model", Kind: "model", Protocol: "openai-responses", Model: "gpt-5-codex",
			Origin: up.URL, Host: "127.0.0.1", Paths: []string{"/v1/responses"}, TestPath: "/v1/responses",
			AllowInsecureLoopback: true,
		},
	})
	_, err := NewConnections(store, p).ConnectModel(context.Background(), contracts.ConnectInput{
		ProfileID: "codex-model", Name: "Codex", APIKey: "fixture-key", Agents: []string{"codex"},
	})
	if err == nil || store.Count() != 0 {
		t.Fatal("Responses 失败仍保存了账号")
	}
	ae, ok := err.(*contracts.AppError)
	if !ok || ae.Code != contracts.ErrUpstreamUnavailable || !strings.Contains(ae.Message, "/v1/responses") {
		t.Fatalf("期望带 /v1/responses 的 UPSTREAM_UNAVAILABLE，got %#v", err)
	}
}
