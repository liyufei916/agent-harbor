package gate

import "testing"

func TestUsageScannerReadsAnthropicAndOpenAIFields(t *testing.T) {
	u := &usageScanner{}
	u.Push([]byte(`event: message_delta\ndata: {"usage":{"input_tokens":12,"output_tokens":3}}`))
	in, out := u.Totals()
	if in != 12 || out != 3 {
		t.Fatalf("anthropic got %d/%d", in, out)
	}
	u2 := &usageScanner{}
	u2.Push([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":20}}}`))
	in, out = u2.Totals()
	if in != 100 || out != 20 {
		t.Fatalf("responses got %d/%d", in, out)
	}
	u3 := &usageScanner{}
	u3.Push([]byte(`{"usage":{"prompt_tokens":7,"completion_tokens":9}}`))
	in, out = u3.Totals()
	if in != 7 || out != 9 {
		t.Fatalf("legacy got %d/%d", in, out)
	}
}

func TestMCPProtocolAcceptedList(t *testing.T) {
	for _, v := range []string{"2025-03-26", "2024-11-05", "2025-06-18", "", "2024-11-05", "2026-01-01"} {
		if !mcpProtocolAccepted(v) {
			t.Fatalf("should accept %q", v)
		}
	}
	if mcpProtocolAccepted("not-a-date") {
		t.Fatal("non-date version")
	}
	if mcpProtocolAccepted("v1") {
		t.Fatal("opaque version")
	}
}
