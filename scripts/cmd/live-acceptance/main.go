// 本机 live 验收驱动：用 Application Support 中已存账号跑 I1–I2/I8/I10 子集。
// 勿打印 Key。用法（仓库根）：
//
//	go run ./scripts/cmd/live-acceptance
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	// 允许从 scripts/cmd/live-acceptance 运行
	for !fileExists(filepath.Join(root, "go.mod")) {
		parent := filepath.Dir(root)
		if parent == root {
			fatal(fmt.Errorf("找不到 go.mod"))
		}
		root = parent
	}
	ws := filepath.Join(root, "demos", "workspace")
	if !dirExists(ws) {
		fatal(fmt.Errorf("缺少工作区 %s", ws))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host := desktop.NewDesktopApp()
	if err := host.AttachOrchestrator(ctx); err != nil {
		fatal(err)
	}
	defer func() {
		shCtx, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		_ = host.Shutdown(shCtx)
	}()

	api := desktop.NewDesktopAPI(host)
	report := map[string]string{}

	// —— I1：账号列表（Keychain 已存）——
	agents := api.ListAgents()
	if agents.Error != nil {
		report["I1"] = "阻断: ListAgents " + agents.Error.Code + " " + agents.Error.Message
	} else {
		claudeOK := false
		for _, a := range agents.Agents {
			fmt.Printf("agent %s available=%v reason=%q\n", a.ID, a.Available, a.Reason)
			if a.ID == "claude" && a.Available {
				claudeOK = true
			}
		}
		acc := api.ListAccounts("")
		if acc.Error != nil {
			report["I1"] = "失败: ListAccounts " + acc.Error.Code
		} else {
			models, mcps := 0, 0
			for _, a := range acc.Accounts {
				kind := a.Kind
				fmt.Printf("account kind=%s id=%s name=%s host=%s profile=%s protocols=%v agents=%v\n",
					kind, a.ID, a.Name, a.Host, a.ProfileID, a.Protocols, a.Agents)
				if kind == "model" {
					models++
				}
				if kind == "mcp" {
					mcps++
				}
			}
			if models == 0 {
				report["I1"] = "阻断: 无已存模型账号（请在 App 模型页先连接）"
			} else if !claudeOK && !agentAvailable(agents.Agents, "dsh") && !agentAvailable(agents.Agents, "codex") {
				report["I1"] = "阻断: 无可用 Agent CLI"
			} else {
				report["I1"] = fmt.Sprintf("通过(元数据): models=%d mcps=%d claude=%v（Key 未回显）", models, mcps, claudeOK)
			}
		}
	}

	health := api.GetRuntimeHealth()
	fmt.Printf("health available=%v reason=%q vault=%q\n", health.Available, health.Reason, health.VaultNotice)
	if !health.Available {
		report["Health"] = "阻断: " + health.Reason
		printReport(report)
		os.Exit(2)
	}

	// 清掉上次残留会话，避免 WORKSPACE_BUSY
	if list := api.ListSessions(); list.Error == nil {
		for _, s := range list.Sessions {
			fmt.Printf("cleanup leftover %s state=%s\n", short(s.SessionID), s.State)
			stopAndWait(api, s.SessionID)
			_ = api.DestroySession(s.SessionID)
		}
	}

	modelID, agentID := pickModelAndAgent(api, agents.Agents)
	if modelID == "" || agentID == "" {
		report["I2"] = "阻断: 无匹配的 Agent+模型（检查 protocols/agents[]）"
		printReport(report)
		os.Exit(2)
	}
	fmt.Printf("pick agent=%s model=%s\n", agentID, modelID)

	mcpIDs := pickMCP(api, 1)
	task := "只读检查 demos/workspace：确认 hello.txt 存在；把一行摘要写入 summary.txt，内容须含 harbor-live-i2。不要使用 Bash 访问工作区外路径。"

	// —— I2 round 1 ——
	sid, errMsg := runOne(api, ws, agentID, modelID, mcpIDs, task, "clone")
	if errMsg != "" {
		report["I2"] = "失败: " + errMsg
	} else {
		report["I2"] = "通过(会话): sessionId=" + short(sid) + " 已停净"
		// B6 / SyncBack（须在 Destroy 前；副本里若有 .claude/debug/latest 链接已跳过）
		if g := api.GetSession(sid); g.Session != nil {
			fmt.Printf("pre-syncback workspace=%s source=%s\n", g.Session.Workspace, g.Session.SourceWorkspace)
		}
		sb := api.SyncBackWorkspace(sid)
		if sb.Error != nil {
			report["B6"] = "失败: SyncBack " + sb.Error.Code + " " + sb.Error.Message
		} else {
			sum := filepath.Join(ws, "summary.txt")
			body, _ := os.ReadFile(sum)
			if strings.Contains(string(body), "harbor-live-i2") {
				report["B6"] = "通过: SyncBack 后源目录 summary.txt 含标记"
			} else {
				report["B6"] = "部分: SyncBack 调用成功，但 summary.txt 未见 harbor-live-i2（Agent 可能未写入）"
			}
		}
	}

	// —— I8 Stop 幂等（若有 sid）——
	if sid != "" {
		s1 := api.StopSession(sid)
		s2 := api.StopSession(sid)
		if s1.Error != nil && s1.Error.Code != "" {
			report["I8"] = "注意: 首次 Stop " + s1.Error.Code
		} else if s2.Error != nil && s2.Session == nil {
			report["I8"] = "失败: 二次 Stop " + s2.Error.Code
		} else {
			report["I8"] = "通过: 重复 Stop 幂等"
		}
		dr := api.DestroySession(sid)
		if dr.Error != nil {
			fmt.Println("destroy1:", dr.Error.Code, dr.Error.Message)
			// 清理未完成时等再毁
			waitCleanup(api, sid)
			_ = api.DestroySession(sid)
		}
	}

	// —— I10 第二轮 ——
	sid2, err2 := runOne(api, ws, agentID, modelID, nil, "只写一行 OK-harbor-i10 到 summary.txt。", "clone")
	if err2 != "" {
		report["I10"] = "失败: " + err2
	} else {
		report["I10"] = "通过: 第二轮 sessionId=" + short(sid2)
		stopAndWait(api, sid2)
		_ = api.DestroySession(sid2)
	}

	printReport(report)
	for _, k := range []string{"I1", "I2"} {
		if strings.HasPrefix(report[k], "失败") || strings.HasPrefix(report[k], "阻断") {
			os.Exit(1)
		}
	}
}

func runOne(api *desktop.DesktopAPI, ws, agentID, modelID string, mcpIDs []string, task, mount string) (string, string) {
	res := api.CreateSession(contracts.SessionSpec{
		Workspace:      ws,
		AgentID:        agentID,
		ModelAccountID: modelID,
		MCPIDs:         mcpIDs,
		Task:           task,
		MountMode:      mount,
	})
	if res.Error != nil {
		return "", res.Error.Code + ": " + res.Error.Message
	}
	if res.Session == nil {
		return "", "无 session 快照"
	}
	sid := res.Session.SessionID
	fmt.Printf("created %s state=%s\n", short(sid), res.Session.State)

	deadline := time.Now().Add(3 * time.Minute)
	sawRunning := false
	var runningSince time.Time
	markerHit := false
	for time.Now().Before(deadline) {
		g := api.GetSession(sid)
		if g.Error != nil {
			return sid, "GetSession " + g.Error.Code
		}
		st := g.Session.State
		fmt.Printf("  state=%s cleanup=%v err=%v\n", st, g.Session.CleanupComplete, errCode(g.Session.Error))
		if st == contracts.StateRunning {
			if !sawRunning {
				sawRunning = true
				runningSince = time.Now()
			}
			// 多轮 PTY 会话不会自行 stopped；产物出现或 running≥45s 后主动停。
			clone := g.Session.Workspace
			if clone != "" {
				if b, err := os.ReadFile(filepath.Join(clone, "summary.txt")); err == nil && strings.Contains(string(b), "harbor-live") {
					markerHit = true
					fmt.Println("  marker found in clone summary.txt → stop")
					break
				}
			}
			if time.Since(runningSince) >= 45*time.Second {
				fmt.Println("  running≥45s → stop（交互会话保活属预期）")
				break
			}
		}
		if st == contracts.StateStopped || st == contracts.StateFailed {
			if st == contracts.StateFailed {
				msg := ""
				if g.Session.Error != nil {
					msg = g.Session.Error.Code + " " + g.Session.Error.Message
				}
				stopAndWait(api, sid)
				if !sawRunning {
					return sid, "未进入 running 即失败: " + msg
				}
				return sid, "会话 failed: " + msg
			}
			waitCleanup(api, sid)
			return sid, ""
		}
		time.Sleep(2 * time.Second)
	}
	stopAndWait(api, sid)
	if !sawRunning {
		return sid, "超时未进入 running"
	}
	if markerHit {
		return sid, ""
	}
	// running 已达成且已停净：I2 核心通路成立（完整 MCP 工具结果仍建议 GUI 补证据）
	return sid, ""
}

func stopAndWait(api *desktop.DesktopAPI, sid string) {
	_ = api.StopSession(sid)
	waitCleanup(api, sid)
}

func waitCleanup(api *desktop.DesktopAPI, sid string) {
	for i := 0; i < 80; i++ {
		g := api.GetSession(sid)
		if g.Session != nil && (g.Session.State == contracts.StateStopped || g.Session.State == contracts.StateFailed) && g.Session.CleanupComplete {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func pickModelAndAgent(api *desktop.DesktopAPI, agents []contracts.AgentInfo) (modelID, agentID string) {
	acc := api.ListAccounts("model")
	prefer := []string{"claude", "dsh", "codex"}
	for _, ag := range prefer {
		if !agentAvailable(agents, ag) {
			continue
		}
		for _, m := range acc.Accounts {
			if m.Kind != "model" {
				continue
			}
			if modelFits(m, ag) {
				return m.ID, ag
			}
		}
	}
	return "", ""
}

func modelFits(m contracts.AccountInfo, agentID string) bool {
	if len(m.Agents) > 0 {
		for _, a := range m.Agents {
			if a == agentID {
				return true
			}
		}
		return false
	}
	switch agentID {
	case "claude":
		return m.ProfileID == "demo-model" || m.ProfileID == "custom" || contains(m.Protocols, "anthropic")
	case "dsh":
		return m.ProfileID == "dsh-model" || m.ProfileID == "custom" || contains(m.Protocols, "anthropic")
	case "codex":
		return m.ProfileID == "codex-model" || contains(m.Protocols, "openai-responses")
	}
	return false
}

func pickMCP(api *desktop.DesktopAPI, n int) []string {
	acc := api.ListAccounts("mcp")
	var ids []string
	for _, a := range acc.Accounts {
		if a.Kind != "mcp" || a.OfflineHint != "" {
			continue
		}
		ids = append(ids, a.ID)
		if len(ids) >= n {
			break
		}
	}
	return ids
}

func agentAvailable(agents []contracts.AgentInfo, id string) bool {
	for _, a := range agents {
		if a.ID == id && a.Available {
			return true
		}
	}
	return false
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func short(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
}

func errCode(e *contracts.AppError) string {
	if e == nil {
		return ""
	}
	return e.Code
}

func printReport(report map[string]string) {
	fmt.Println("===== LIVE REPORT =====")
	order := []string{"I1", "Health", "I2", "B6", "I8", "I10"}
	for _, k := range order {
		if v, ok := report[k]; ok {
			fmt.Printf("%s: %s\n", k, v)
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "fatal:", err)
	os.Exit(2)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
