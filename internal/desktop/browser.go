package desktop

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// openSystemBrowser 尽力打开系统浏览器；失败不影响 OAuth pending。
// HARBOR_MCP_OAUTH_NO_BROWSER=1 时跳过（证据脚本 / CI 自点 authorize）。
func openSystemBrowser(rawURL string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return
	}
	if strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_NO_BROWSER")) == "1" {
		return
	}
	go func() {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", rawURL)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
		default:
			cmd = exec.Command("xdg-open", rawURL)
		}
		_ = cmd.Start()
	}()
}
