package contracts

import "testing"

func TestFormatMCPSummarySidesAndTools(t *testing.T) {
	got := FormatMCPSummary(AccountInfo{Name: "wiki", AuthMode: "token", Tools: []string{"search", "get"}})
	if got != "wiki · 远程 · Token 在宿主机 Gate · 允许工具: search, get" {
		t.Fatalf("token: %q", got)
	}
	got = FormatMCPSummary(AccountInfo{Name: "fs", Transport: "stdio", Tools: []string{"echo"}})
	if got != "fs · 宿主 stdio · 进程与密钥在宿主机 · 允许工具: echo" {
		t.Fatalf("stdio: %q", got)
	}
	got = FormatMCPSummary(AccountInfo{ID: "mcp_x", AuthMode: "oauth"})
	if got != "mcp_x · 远程 OAuth · Token 在宿主机 Gate" {
		t.Fatalf("oauth: %q", got)
	}
}
