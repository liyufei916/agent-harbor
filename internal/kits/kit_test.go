package kits

import "testing"

func TestMCPNameAndGateURLs(t *testing.T) {
	if MCPName("", 0) != "harbor" || MCPName("", 2) != "harbor-3" {
		t.Fatalf("empty id should keep index fallback names")
	}
	a0 := MCPName("acct-alpha", 0)
	a2 := MCPName("acct-alpha", 2)
	if a0 != a2 || !stringsHasPrefix(a0, "harbor-") {
		t.Fatalf("same account must keep stable name across index: %q vs %q", a0, a2)
	}
	b0 := MCPName("acct-beta", 0)
	if a0 == b0 {
		t.Fatalf("different accounts must differ: %q", a0)
	}
	got, err := MCPGateURLs(KitInput{MCPURL: "http://127.0.0.1:8080"})
	if err != nil || len(got) != 1 || got[0] != "http://127.0.0.1:8080" {
		t.Fatalf("single URL: %#v, %v", got, err)
	}
	stable, err := MCPGateURLs(KitInput{
		MCPURLs: []string{"http://127.0.0.1:41002/mcp/0", "http://127.0.0.1:41003/mcp/1"},
		MCPIDs:  []string{"id-b", "id-a"},
	})
	if err != nil || len(stable) != 2 {
		t.Fatalf("stable ids: %#v %v", stable, err)
	}
	if MCPServerName(KitInput{MCPIDs: []string{"id-b", "id-a"}}, 0) == MCPName("", 0) {
		t.Fatal("account-backed name should not equal index fallback for first slot")
	}
	// 交换 URL 顺序但 ID 仍绑原账号 → 名随 ID，不随下标
	n0 := MCPServerName(KitInput{MCPIDs: []string{"id-b", "id-a"}}, 0)
	n1 := MCPServerName(KitInput{MCPIDs: []string{"id-a", "id-b"}}, 1)
	if n0 != MCPName("id-b", 0) || n1 != MCPName("id-b", 1) || n0 != n1 {
		t.Fatalf("id-b must keep one name: n0=%q n1=%q", n0, n1)
	}
	for name, in := range map[string]KitInput{
		"ambiguous": {MCPURL: "http://127.0.0.1:1", MCPURLs: []string{"http://127.0.0.1:2"}},
		"https":     {MCPURL: "https://127.0.0.1:1"},
		"query":     {MCPURL: "http://127.0.0.1:1?q=1"},
		"host":      {MCPURL: "http://localhost:1"},
		"port":      {MCPURL: "http://127.0.0.1:0"},
		"duplicate": {MCPURLs: []string{"http://127.0.0.1:1", "http://127.0.0.1:1"}},
		"id len":    {MCPURLs: []string{"http://127.0.0.1:1"}, MCPIDs: []string{"a", "b"}},
		"dup name":  {MCPURLs: []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, MCPIDs: []string{"same", "same"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := MCPGateURLs(in); err == nil {
				t.Fatal("invalid MCP URL accepted")
			}
		})
	}
	tooMany := KitInput{}
	for i := 1; i <= 9; i++ {
		tooMany.MCPURLs = append(tooMany.MCPURLs, "http://127.0.0.1:"+string(rune('0'+i)))
	}
	if _, err := MCPGateURLs(tooMany); err == nil {
		t.Fatal("too many MCP URLs accepted")
	}
}

func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
