package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
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
	r, err := rewriteJSONModelField(bytes.NewReader([]byte(in)), "gpt-5.6-sol")
	if err != nil {
		t.Fatal(err)
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

func TestRewriteJSONModelFieldRejectsOversize(t *testing.T) {
	// 旧上限 2MiB 会静默截断；现应仍可改写，避免上游 JSON EOF @ column 2097152。
	payload := `{"model":"x","pad":"` + strings.Repeat("a", (2<<20)+64) + `"}`
	r, err := rewriteJSONModelField(strings.NewReader(payload), "y")
	if err != nil {
		t.Fatalf("2MiB+ 应仍可改写（上限已抬到 32MiB）: %v", err)
	}
	raw, _ := io.ReadAll(r)
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["model"] != "y" {
		t.Fatalf("model=%v", obj["model"])
	}

	huge := make([]byte, maxModelProxyRewriteBody+1)
	_, err = rewriteJSONModelField(bytes.NewReader(huge), "y")
	if !errors.Is(err, errModelProxyBodyTooLarge) {
		t.Fatalf("want errModelProxyBodyTooLarge, got %v", err)
	}
}
