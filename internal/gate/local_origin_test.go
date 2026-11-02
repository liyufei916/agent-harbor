package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/secrets"
)

func TestLocalModelConnectStorePolicyAndProxy(t *testing.T) {
	sse := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool-fixture\",\"name\":\"Read\",\"input\":{}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"file_path\\\":\\\"/workspace/README.md\\\"}\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			io.WriteString(w, `{"data":[]}`)
			return
		}
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-key" && r.Header.Get("X-Api-Key") != "fixture-key" {
			w.WriteHeader(401)
			return
		}
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			// 刻意拆开工具参数和结束事件，验证脱敏缓冲不会吞掉尾部。
			for offset := 0; offset < len(sse); offset += 13 {
				end := offset + 13
				if end > len(sse) {
					end = len(sse)
				}
				io.WriteString(w, sse[offset:end])
				w.(http.Flusher).Flush()
			}
			return
		}
		io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"OK"}]}`)
	}))
	defer upstream.Close()
	ctx := context.Background()
	store := secrets.NewStore()
	pol := policy.NewBuilder(nil)
	connected, err := NewConnections(store, pol).ConnectModel(ctx, contracts.ConnectInput{ProfileID: "custom", Name: "本机", Origin: upstream.URL, Model: "fixture-model", APIKey: "fixture-key", Agents: []string{"claude"}})
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.Lookup(ctx, connected.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := pol.Build(ctx, policy.PolicyRequest{Workspace: t.TempDir(), AgentID: "claude", Model: account})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Model.Origin != upstream.URL {
		t.Fatalf("策略地址被改变：%q", snapshot.Model.Origin)
	}
	h := New(store, pol)
	bind, err := h.Start(ctx, GateSpec{SessionID: "local-origin", ModelAccountID: account.ID, Policy: snapshot}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "local-origin")
	req, err := http.NewRequest(http.MethodPost, bind.ModelBaseURL+"/v1/messages", strings.NewReader(`{"model":"protocol-model","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Api-Key", bind.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "OK") {
		t.Fatalf("转发失败 status=%d body=%s", resp.StatusCode, body)
	}
	streamReq, err := http.NewRequest(http.MethodPost, bind.ModelBaseURL+"/v1/messages", strings.NewReader(`{"model":"protocol-model","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	streamReq.Header.Set("X-Api-Key", bind.Sentinel)
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer streamResp.Body.Close()
	streamBody, err := io.ReadAll(streamResp.Body)
	if err != nil || streamResp.StatusCode != 200 || string(streamBody) != sse {
		t.Fatalf("工具 SSE 被修改或截断：status=%d err=%v", streamResp.StatusCode, err)
	}
}

func TestModelProxyAuditsInterruptedStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		// 声明正文长于实际数据，模拟流式生成过程中上游连接断开。
		fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nContent-Length: 10000\r\n\r\nevent: message_start\ndata: {}\n\n")
	}))
	defer upstream.Close()
	ctx := context.Background()
	store := secrets.NewStore()
	accountID, err := store.Put(ctx, contracts.AccountInfo{Kind: "model", Name: "fixture", ProfileID: "custom", Host: "127.0.0.1"}, []byte("fixture-key"))
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, policy.NewBuilder(nil))
	audits := make(chan contracts.AuditData, 8)
	bind, err := h.Start(ctx, GateSpec{SessionID: "broken-stream", ModelAccountID: accountID, Policy: policy.PolicySnapshot{
		Model: policy.EndpointRule{Origin: upstream.URL, Paths: []string{"/v1/messages"}},
	}}, func(a contracts.AuditData) { audits <- a }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Revoke(ctx, "broken-stream")
	req, _ := http.NewRequest(http.MethodPost, bind.ModelBaseURL+"/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("X-Api-Key", bind.Sentinel)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		select {
		case audit := <-audits:
			if audit.Result == "failed" {
				return
			}
		case <-deadline.C:
			t.Fatal("流式响应中断后未记录 failed 审计")
		}
	}
}
