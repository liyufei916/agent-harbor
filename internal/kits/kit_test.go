package kits

import "testing"

func TestMCPNameAndGateURLs(t *testing.T) {
	if MCPName(0) != "harbor" || MCPName(2) != "harbor-3" {
		t.Fatalf("unexpected MCP names")
	}
	got, err := MCPGateURLs(KitInput{MCPURL: "http://127.0.0.1:8080"})
	if err != nil || len(got) != 1 || got[0] != "http://127.0.0.1:8080" {
		t.Fatalf("single URL: %#v, %v", got, err)
	}
	for name, in := range map[string]KitInput{
		"ambiguous": {MCPURL: "http://127.0.0.1:1", MCPURLs: []string{"http://127.0.0.1:2"}},
		"https":     {MCPURL: "https://127.0.0.1:1"},
		"query":     {MCPURL: "http://127.0.0.1:1?q=1"},
		"host":      {MCPURL: "http://localhost:1"},
		"port":      {MCPURL: "http://127.0.0.1:0"},
		"duplicate": {MCPURLs: []string{"http://127.0.0.1:1", "http://127.0.0.1:1"}},
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
