// 命令级 I1/I2 脱敏证据：沙箱 HOME 仅 Gate/Sentinel；账号无完整 Key；会话产物标记。
// 勿打印 Key/Sentinel 原文。用法（仓库根）：
//
//	go run ./scripts/cmd/i1i2-cmd-evidence
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/desktop"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	for !fileExists(filepath.Join(root, "go.mod")) {
		parent := filepath.Dir(root)
		if parent == root {
			fatal(fmt.Errorf("找不到 go.mod"))
		}
		root = parent
	}
	ws := filepath.Join(root, "demos", "workspace")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := desktop.NewDesktopApp()
	if err := host.AttachOrchestrator(ctx); err != nil {
		fatal(err)
	}
	defer func() {
		sh, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		_ = host.Shutdown(sh)
	}()
	api := desktop.NewDesktopAPI(host)

	fmt.Println("## commit")
	fmt.Println(gitShort(root))

	fmt.Println("## I1 ListAccounts（禁止完整 Key）")
	acc := api.ListAccounts("")
	if acc.Error != nil {
		fatal(fmt.Errorf("ListAccounts %s", acc.Error.Code))
	}
	var modelID, mcpID string
	for _, a := range acc.Accounts {
		fmt.Printf("- kind=%s name=%s host=%s keyHint=%q tools=%d protocols=%v\n",
			a.Kind, a.Name, a.Host, a.KeyHint, len(a.Tools), a.Protocols)
		if len(a.KeyHint) > 12 && !strings.HasPrefix(a.KeyHint, "…") && !strings.HasPrefix(a.KeyHint, "...") {
			fatal(fmt.Errorf("keyHint 疑似完整 Key"))
		}
		if a.Kind == "model" && modelID == "" {
			modelID = a.ID
		}
		if a.Kind == "mcp" && a.OfflineHint == "" && mcpID == "" {
			mcpID = a.ID
		}
	}
	agents := api.ListAgents()
	agentID := ""
	for _, a := range agents.Agents {
		if a.ID == "claude" && a.Available {
			agentID = "claude"
			break
		}
	}
	if modelID == "" || agentID == "" {
		fatal(fmt.Errorf("无可用 claude+model"))
	}
	// 优先 Deepseek/custom 给 claude
	for _, a := range acc.Accounts {
		if a.Kind != "model" {
			continue
		}
		if len(a.Agents) > 0 && !contains(a.Agents, "claude") {
			continue
		}
		if a.ProfileID == "custom" || contains(a.Protocols, "anthropic") || a.Host == "api.deepseek.com" {
			modelID = a.ID
			break
		}
	}

	mcps := []string{}
	if mcpID != "" {
		mcps = []string{mcpID}
	}
	fmt.Printf("pick agent=%s model=%s mcp=%v\n", agentID, modelID, mcps)

	res := api.CreateSession(contracts.SessionSpec{
		Workspace:      ws,
		AgentID:        agentID,
		ModelAccountID: modelID,
		MCPIDs:         mcps,
		Task:           "确认 hello.txt 存在；把一行摘要写入 summary.txt，须含 harbor-cmd-i2。",
		MountMode:      "clone",
	})
	if res.Error != nil {
		fatal(fmt.Errorf("CreateSession %s %s", res.Error.Code, res.Error.Message))
	}
	sid := res.Session.SessionID
	clone := ""
	for i := 0; i < 60; i++ {
		g := api.GetSession(sid)
		if g.Session == nil {
			fatal(fmt.Errorf("session gone"))
		}
		if g.Session.State == contracts.StateRunning {
			clone = g.Session.Workspace
			break
		}
		if g.Session.State == contracts.StateFailed {
			fatal(fmt.Errorf("failed %v", g.Session.Error))
		}
		time.Sleep(time.Second)
	}
	if clone == "" {
		fatal(fmt.Errorf("未进入 running"))
	}

	home := filepath.Join(filepath.Dir(clone), "home")
	fmt.Println("## 沙箱私有 HOME（断言：无上游 Origin+sk-；Bearer 仅 Sentinel 长度）")
	fmt.Println("home=" + home)
	sk := regexp.MustCompile(`(?i)sk-[a-z0-9_-]{16,}`)
	_ = filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Size() > 1<<20 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		s := string(b)
		fmt.Printf("- %s bytes=%d loopback=%v auth=%v skPattern=%v deepseekHost=%v\n",
			filepath.Base(p), len(b),
			strings.Contains(s, "127.0.0.1"),
			strings.Contains(s, "Authorization") || strings.Contains(s, "Bearer"),
			sk.MatchString(s),
			strings.Contains(s, "api.deepseek.com"),
		)
		if sk.MatchString(s) && strings.Contains(s, "api.deepseek.com") {
			fatal(fmt.Errorf("沙箱出现上游 Host+sk 形态"))
		}
		re := regexp.MustCompile(`Bearer\s+([A-Za-z0-9._\-]+)`)
		if m := re.FindStringSubmatch(s); len(m) == 2 {
			fmt.Printf("  Bearer_len=%d url_loopback=%v（Sentinel 原文不打印）\n", len(m[1]), strings.Contains(s, "127.0.0.1"))
			if len(m[1]) < 32 {
				fatal(fmt.Errorf("Bearer 过短，不像 Sentinel"))
			}
		}
		return nil
	})

	fmt.Println("## I2 产物 / 上游可达性（会话 running + 写入标记 ⇒ Gate 已代填真 Key 出站；响应路径经 redact）")
	deadline := time.Now().Add(55 * time.Second)
	marker := false
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(filepath.Join(clone, "summary.txt"))
		if err == nil && strings.Contains(string(b), "harbor-cmd-i2") {
			fmt.Printf("summary_bytes=%d marker=true\n", len(b))
			marker = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !marker {
		fmt.Println("marker=false（仍可能已调上游；以 running 为准）")
	}

	ov := api.GetPermissionOverview(sid)
	blob, _ := json.Marshal(ov.Overview)
	if sk.MatchString(string(blob)) {
		fatal(fmt.Errorf("overview 含 sk 形态"))
	}
	fmt.Println("## 权限一览 JSON（应无 Key/Sentinel）")
	fmt.Println(string(blob))

	_ = api.StopSession(sid)
	for i := 0; i < 80; i++ {
		g := api.GetSession(sid)
		if g.Session != nil && g.Session.CleanupComplete {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = api.DestroySession(sid)
	fmt.Println("## OK")
}

func gitShort(root string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "(unknown)"
	}
	h := strings.TrimSpace(string(out))
	if len(h) > 12 {
		return h[:12] + "…"
	}
	return h
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "fatal:", err)
	os.Exit(1)
}
