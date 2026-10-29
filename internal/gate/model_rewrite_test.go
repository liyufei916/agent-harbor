package gate

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/policy"
)

func TestNormalizeModelRequestPathMapsCodexResponses(t *testing.T) {
	rule := policy.EndpointRule{Paths: []string{"/v1/responses"}}
	if got := normalizeModelRequestPath(rule, "/responses"); got != "/v1/responses" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeModelRequestPath(rule, "/v1/responses"); got != "/v1/responses" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeModelRequestPath(policy.EndpointRule{Paths: []string{"/v1/messages"}}, "/responses"); got != "/responses" {
		t.Fatalf("未放行 /v1/responses 时不应改写: %q", got)
	}
}

func TestRewriteJSONModelField(t *testing.T) {
	in := `{"model":"claude-sonnet-4-5","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	r, ok := rewriteJSONModelField(bytes.NewReader([]byte(in)), "gpt-5.6-sol")
	if !ok {
		t.Fatal("rewrite failed")
	}
	raw, _ := io.ReadAll(r)
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["model"] != "gpt-5.6-sol" {
		t.Fatalf("model=%v body=%s", obj["model"], raw)
	}
	if _, ok := obj["messages"]; !ok {
		t.Fatal("messages dropped")
	}
}
