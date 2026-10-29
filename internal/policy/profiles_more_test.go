package policy

import (
	"os"
	"testing"
)

func TestCustomProfilesAndModelCandidates(t *testing.T) {
	t.Setenv("HARBOR_CLAUDE_MODEL", "env-model")
	p, err := CustomModelProfile("https://api.deepseek.com/anthropic/", "")
	if err != nil || p.BasePath != "/anthropic" || p.Model != "env-model" || p.Host != "api.deepseek.com" {
		t.Fatalf("custom model profile: %#v, %v", p, err)
	}
	cands, err := CustomModelProbeCandidates("https://api.deepseek.com", "")
	if err != nil || len(cands) < 2 {
		t.Fatalf("deepseek candidates: %#v, %v", cands, err)
	}
	if _, err := CustomModelProfile("https://user:pass@example.com", "m"); err == nil {
		t.Fatal("userinfo accepted")
	}
	if _, err := CustomModelProfile("https://example.com/a//b", "m"); err == nil {
		t.Fatal("ambiguous base path accepted")
	}
	if got := defaultModelForHost("example.com", " explicit "); got != "explicit" {
		t.Fatalf("explicit model = %q", got)
	}
	if got := defaultModelCandidates("deepseek.com", "deepseek-chat"); len(got) != 3 {
		t.Fatalf("candidate dedupe = %#v", got)
	}
	_ = os.Unsetenv("HARBOR_CLAUDE_MODEL")
}

func TestValidConnectionAndEnvironmentProfiles(t *testing.T) {
	b := NewBuilder(map[string]FixedProfile{"x": {ID: "x", Kind: "model"}})
	if _, ok := b.Profile("missing"); ok {
		t.Fatal("unknown profile found")
	}
	if p, ok := b.Profile("x"); !ok || p.ID != "x" {
		t.Fatalf("profile lookup: %#v %v", p, ok)
	}
	valid := FixedProfile{Kind: "model", Protocol: "anthropic", Model: "m", Origin: "https://example.com", Host: "example.com", TestPath: "/v1/messages"}
	if !valid.ValidConnection() {
		t.Fatal("valid profile rejected")
	}
	for _, mutate := range []func(*FixedProfile){
		func(p *FixedProfile) { p.Host = "other.example.com" },
		func(p *FixedProfile) { p.Origin = "http://example.com" },
		func(p *FixedProfile) { p.TestPath = "v1/messages" },
		func(p *FixedProfile) { p.Model = "" },
	} {
		bad := valid
		mutate(&bad)
		if bad.ValidConnection() {
			t.Fatalf("invalid profile accepted: %#v", bad)
		}
	}
	t.Setenv("HARBOR_CLAUDE_MODEL_ORIGIN", "https://model.example.com/")
	t.Setenv("HARBOR_CLAUDE_MODEL_BASE_PATH", "/anthropic/")
	t.Setenv("HARBOR_DEMO_MCP_ORIGIN", "https://mcp.example.com/")
	t.Setenv("HARBOR_DEMO_MCP_TOOLS", " read, write ,, ")
	profiles := DefaultProfiles()
	if profiles[ProfileDemoModel].Origin != "https://model.example.com" || profiles[ProfileDemoModel].BasePath != "/anthropic" {
		t.Fatalf("environment profile: %#v", profiles[ProfileDemoModel])
	}
	if len(profiles[ProfileDemoMCP].Tools) != 2 {
		t.Fatalf("MCP tools: %#v", profiles[ProfileDemoMCP].Tools)
	}
}
