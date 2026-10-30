package contracts

import "strings"

// FormatMCPSummary 权限一览用：MCP 名 + 密钥所在侧 + 允许工具（无 Key/Sentinel）。
func FormatMCPSummary(account AccountInfo) string {
	name := account.Name
	if name == "" {
		name = account.ID
	}
	side := "远程 · Token 在宿主机 Gate"
	if strings.EqualFold(strings.TrimSpace(account.Transport), "stdio") {
		side = "宿主 stdio · 进程与密钥在宿主机"
	} else if strings.EqualFold(strings.TrimSpace(account.AuthMode), "oauth") {
		side = "远程 OAuth · Token 在宿主机 Gate"
	}
	parts := []string{name, side}
	if tools := account.Tools; len(tools) > 0 {
		parts = append(parts, "允许工具: "+strings.Join(tools, ", "))
	}
	return strings.Join(parts, " · ")
}
