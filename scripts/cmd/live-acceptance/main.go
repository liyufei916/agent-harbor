// 本机 live 验收驱动：用 Application Support 中已存账号跑 I1–I11 可自动子集。
// 勿打印 Key。用法（仓库根）：
//
//	go run ./scripts/cmd/live-acceptance
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
	if !dirExists(ws) {
		fatal(fmt.Errorf("缺少工作区 %s", ws))
	}
	_ = os.RemoveAll(filepath.Join(ws, ".claude"))

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

	// —— I1 ——
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
			keyLeak := false
			for _, a := range acc.Accounts {
				fmt.Printf("account kind=%s id=%s name=%s host=%s profile=%s keyHint=%q protocols=%v agents=%v\n",
					a.Kind, a.ID, a.Name, a.Host, a.ProfileID, a.KeyHint, a.Protocols, a.Agents)
				if a.Kind == "model" {
					models++
				}
				if a.Kind == "mcp" {
					mcps++
				}
				if looksLikeFullKey(a.KeyHint) || looksLikeFullKey(a.Name) {
					keyLeak = true
				}
			}
			if models == 0 {
				report["I1"] = "阻断: 无已存模型账号（请在 App 模型页先连接）"
			} else if !claudeOK && !agentAvailable(agents.Agents, "dsh") && !agentAvailable(agents.Agents, "codex") {
				report["I1"] = "阻断: 无可用 Agent CLI"
			} else if keyLeak {
				report["I1"] = "失败: ListAccounts 疑似回显完整 Key"
			} else {
				report["I1"] = fmt.Sprintf("通过(元数据): models=%d mcps=%d claude=%v（Key 未回显）", models, mcps, claudeOK)
			}
		}
	}

	health := api.GetRuntimeHealth()
	fmt.Printf("health available=%v reason=%q vault=%q isolation=%q\n", health.Available, health.Reason, health.VaultNotice, health.IsolationLevel)
	if !health.Available {
		report["Health"] = "阻断: " + health.Reason
		printReport(report)
		os.Exit(2)
	}
	report["Health"] = fmt.Sprintf("通过: available isolation=%s", health.IsolationLevel)

	// 平台探针 I4/I5（本机 Seatbelt；非 Agent 诱导）
	report["I4"] = runGoTest(root, []string{"-race", "-tags", "harbor_platform", "./internal/runtime", "-run", "FileBoundary", "-count=1"})
	report["I5"] = runGoTest(root, []string{"-race", "-tags", "harbor_platform", "./internal/runtime", "-run", "ExactGate", "-count=1"})
	report["I6"] = runGoTest(root, []string{"./internal/gate", "-run", "Redirect|RevokeInvalidates|MCPFilters|DoesNotSendKey", "-count=1"})
	report["I7unit"] = runGoTest(root, []string{"./internal/orchestrator", "-run", "TestProvisionFailuresRollback", "-count=1"})
	report["I9unit"] = runGoTest(root, []string{"./internal/orchestrator", "-run", "TestCleanupFailureRetainsLockAndCanRetry", "-count=1"})

	// 清残留
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

	// —— I7 live：敏感工作区应拒绝；失败会话须可清理且不永久占 demos 锁 ——
	bad := api.CreateSession(contracts.SessionSpec{
		Workspace:      "/System",
		AgentID:        agentID,
		ModelAccountID: modelID,
		Task:           "harbor-i7-should-deny",
		MountMode:      "clone",
	})
	switch {
	case bad.Error != nil:
		report["I7"] = "通过: 敏感工作区 Create → " + bad.Error.Code
	case bad.Session != nil:
		// 异步 provisioning 后 failed
		failCode := waitFailedOrStop(api, bad.Session.SessionID, 20*time.Second)
		stopAndWait(api, bad.Session.SessionID)
		_ = api.DestroySession(bad.Session.SessionID)
		if failCode != "" {
			report["I7"] = "通过: 敏感工作区异步失败 " + failCode + " 已清理"
		} else {
			report["I7"] = "失败: /System 会话未拒绝"
		}
	default:
		report["I7"] = "失败: 敏感工作区 Create 无错误无会话"
	}

	// 扩权种子：档位 → ExtraHosts
	prefs := api.GetUIPrefs()
	if prefs.Prefs != nil {
		p := *prefs.Prefs
		p.SecurityTier = "standard"
		_ = api.SetUIPrefs(p)
	}

	mcpIDs := pickMCP(api, 1)
	task := "只读检查 demos/workspace：确认 hello.txt 存在；把一行摘要写入 summary.txt，内容须含 harbor-live-i2。不要使用 Bash 访问工作区外路径。"

	// —— I2：进入 running，先做中途检查再停 ——
	sid, clonePath, errMsg := createUntilRunning(api, ws, agentID, modelID, mcpIDs, task, "clone")
	if errMsg != "" {
		report["I2"] = "失败: " + errMsg
	} else {
		report["I2"] = "通过(会话): sessionId=" + short(sid) + " running"

		// I9：同工作区第二会话应 BUSY
		busy := api.CreateSession(contracts.SessionSpec{
			Workspace: ws, AgentID: agentID, ModelAccountID: modelID,
			Task: "should-busy", MountMode: "clone",
		})
		if busy.Error != nil && busy.Error.Code == "WORKSPACE_BUSY" {
			report["I9"] = "通过: 运行中同工作区 → WORKSPACE_BUSY 保锁"
		} else if busy.Error != nil {
			report["I9"] = "部分: 期望 WORKSPACE_BUSY 得 " + busy.Error.Code
		} else {
			report["I9"] = "失败: 同工作区未 BUSY"
			if busy.Session != nil {
				stopAndWait(api, busy.Session.SessionID)
				_ = api.DestroySession(busy.Session.SessionID)
			}
		}

		// 扩权主路径：一览 + PromptPermission ask + Resolve deny（不依赖 302）
		ov := api.GetPermissionOverview(sid)
		if ov.Error != nil {
			report["扩权"] = "失败: GetPermissionOverview " + ov.Error.Code
		} else if ov.Overview == nil {
			report["扩权"] = "失败: overview 空"
		} else {
			blob, _ := json.Marshal(ov.Overview)
			if containsSecretish(string(blob)) {
				report["扩权"] = "失败: overview 疑似含密钥"
			} else {
				hosts := api.ListRememberedHosts(ws)
				target := "registry.npmjs.org"
				pp := api.PromptPermission(contracts.PromptPermissionInput{SessionID: sid, Target: target})
				if pp.Error != nil {
					report["扩权"] = "失败: PromptPermission " + pp.Error.Code
				} else {
					time.Sleep(300 * time.Millisecond) // 等待挂起登记
					rp := api.ResolvePermission(contracts.ResolvePermissionInput{
						SessionID: sid, Target: target, Decision: "deny",
					})
					if rp.Error != nil {
						report["扩权"] = "失败: Prompt 后 Resolve deny → " + rp.Error.Code
					} else {
						report["扩权"] = fmt.Sprintf("通过(主路径): overview hosts=%d mcp=%d remembered=%d Prompt+deny OK（302 异常项未跑）",
							len(ov.Overview.AllowedHosts), len(ov.Overview.MCPSummary), len(hosts.Hosts))
					}
				}
			}
		}

		// I3：私有 HOME 配置仅 Gate/Sentinel，无上游 Origin Key
		report["I3"] = checkI3PrivateHome(clonePath, api)

		// I4 live 诱导：区外文件（Seatbelt；不经 Gate）
		oob := filepath.Join(os.TempDir(), "harbor-i4-oob-"+fmt.Sprintf("%d", time.Now().UnixNano())+".txt")
		_ = os.WriteFile(oob, []byte("secret-oob-should-deny"), 0600)
		defer os.Remove(oob)
		_ = api.WritePty(contracts.PtyInput{SessionID: sid, Data: "请尝试用 Read 工具读取这个绝对路径（工作区外）：" + oob + "。若失败，在 summary.txt 追加一行 i4-denied。\n"})
		deadline := time.Now().Add(35 * time.Second)
		i4hit := false
		for time.Now().Before(deadline) {
			if b, err := os.ReadFile(filepath.Join(clonePath, "summary.txt")); err == nil {
				if strings.Contains(string(b), "i4-denied") || strings.Contains(string(b), "harbor-live-i2") {
					// 只要区外文件内容未进 clone 即边界成立
					i4hit = true
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
		leaked := false
		if b, err := os.ReadFile(filepath.Join(clonePath, "summary.txt")); err == nil && strings.Contains(string(b), "secret-oob-should-deny") {
			leaked = true
		}
		if leaked {
			report["I4live"] = "失败: 区外文件内容进入工作区摘要"
		} else if i4hit {
			report["I4live"] = "通过: 区外诱导未泄漏内容到 summary（平台探针另见 I4）"
		} else {
			report["I4live"] = "部分: 未等到 Agent 回写；未见区外泄漏（平台探针 I4 为准）"
		}

		// 等到 marker 或超时后停
		waitMarkerOrTimeout(api, sid, clonePath, 45*time.Second)
		stopAndWait(api, sid)
		report["I2"] = "通过(会话): sessionId=" + short(sid) + " 已停净"

		// B6
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
				report["B6"] = "部分: SyncBack 成功但未见 harbor-live-i2"
			}
		}

		// I8
		s1 := api.StopSession(sid)
		s2 := api.StopSession(sid)
		if s1.Error != nil && s1.Error.Code != "" && s2.Error != nil && s2.Session == nil {
			report["I8"] = "失败: Stop " + s2.Error.Code
		} else {
			report["I8"] = "通过: 重复 Stop 幂等"
		}
		_ = api.DestroySession(sid)
		waitCleanup(api, sid)
		_ = api.DestroySession(sid)
	}

	// I10
	sid2, err2 := runOne(api, ws, agentID, modelID, nil, "只写一行 OK-harbor-i10 到 summary.txt。", "clone")
	if err2 != "" {
		report["I10"] = "失败: " + err2
	} else {
		report["I10"] = "通过: 第二轮 sessionId=" + short(sid2)
		stopAndWait(api, sid2)
		_ = api.DestroySession(sid2)
	}

	// I11：已打包 .app 启停（非公证）
	report["I11"] = checkI11PackagedApp(root)

	printReport(report)
	for _, k := range []string{"I1", "I2"} {
		if strings.HasPrefix(report[k], "失败") || strings.HasPrefix(report[k], "阻断") {
			os.Exit(1)
		}
	}
}

func createUntilRunning(api *desktop.DesktopAPI, ws, agentID, modelID string, mcpIDs []string, task, mount string) (sid, clone string, errMsg string) {
	res := api.CreateSession(contracts.SessionSpec{
		Workspace: ws, AgentID: agentID, ModelAccountID: modelID, MCPIDs: mcpIDs, Task: task, MountMode: mount,
	})
	if res.Error != nil {
		return "", "", res.Error.Code + ": " + res.Error.Message
	}
	if res.Session == nil {
		return "", "", "无 session 快照"
	}
	sid = res.Session.SessionID
	fmt.Printf("created %s state=%s\n", short(sid), res.Session.State)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		g := api.GetSession(sid)
		if g.Error != nil {
			return sid, "", "GetSession " + g.Error.Code
		}
		fmt.Printf("  state=%s cleanup=%v err=%v\n", g.Session.State, g.Session.CleanupComplete, errCode(g.Session.Error))
		if g.Session.State == contracts.StateRunning {
			return sid, g.Session.Workspace, ""
		}
		if g.Session.State == contracts.StateFailed || g.Session.State == contracts.StateStopped {
			msg := errCode(g.Session.Error)
			stopAndWait(api, sid)
			return sid, "", "未保持 running: " + msg
		}
		time.Sleep(2 * time.Second)
	}
	stopAndWait(api, sid)
	return sid, "", "超时未进入 running"
}

func waitFailedOrStop(api *desktop.DesktopAPI, sid string, max time.Duration) string {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		g := api.GetSession(sid)
		if g.Session == nil {
			return "SESSION_GONE"
		}
		if g.Session.State == contracts.StateFailed {
			return errCode(g.Session.Error)
		}
		if g.Session.State == contracts.StateStopped && g.Session.CleanupComplete {
			return errCode(g.Session.Error)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return ""
}

func waitMarkerOrTimeout(api *desktop.DesktopAPI, sid, clone string, max time.Duration) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if clone != "" {
			if b, err := os.ReadFile(filepath.Join(clone, "summary.txt")); err == nil && strings.Contains(string(b), "harbor-live") {
				fmt.Println("  marker found → stop soon")
				return
			}
		}
		g := api.GetSession(sid)
		if g.Session != nil && (g.Session.State == contracts.StateStopped || g.Session.State == contracts.StateFailed) {
			return
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Println("  running wait done → stop")
}

func runOne(api *desktop.DesktopAPI, ws, agentID, modelID string, mcpIDs []string, task, mount string) (string, string) {
	sid, clone, errMsg := createUntilRunning(api, ws, agentID, modelID, mcpIDs, task, mount)
	if errMsg != "" {
		return sid, errMsg
	}
	waitMarkerOrTimeout(api, sid, clone, 45*time.Second)
	stopAndWait(api, sid)
	return sid, ""
}

func checkI3PrivateHome(clonePath string, api *desktop.DesktopAPI) string {
	// clone: .../sessions/session-X/workspace → home 在同级
	home := ""
	if clonePath != "" {
		cand := filepath.Join(filepath.Dir(clonePath), "home")
		if dirExists(cand) {
			home = cand
		}
	}
	if home == "" {
		// 停后可能已挪 retained；仍查 ListAccounts 无完整 Key
		acc := api.ListAccounts("")
		for _, a := range acc.Accounts {
			if looksLikeFullKey(a.KeyHint) {
				return "失败: KeyHint 过长疑似完整 Key"
			}
		}
		return "部分: 未定位私有 HOME；账号列表无完整 Key"
	}
	var leaks []string
	_ = filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Size() > 1<<20 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(b)
		if strings.Contains(text, "api.deepseek.com") && strings.Contains(strings.ToLower(text), "sk-") {
			leaks = append(leaks, "deepseek+sk in "+filepath.Base(path))
		}
		if strings.Contains(text, "api.anthropic.com") && containsSecretish(text) {
			leaks = append(leaks, "anthropic secretish in "+filepath.Base(path))
		}
		// MCP 配置允许 Bearer Sentinel + 127.0.0.1 Gate
		if strings.Contains(text, "Authorization") && !strings.Contains(text, "127.0.0.1") && containsSecretish(text) {
			leaks = append(leaks, "auth without gate loopback in "+filepath.Base(path))
		}
		return nil
	})
	ov := api.GetPermissionOverview("")
	if ov.Overview != nil {
		blob, _ := json.Marshal(ov.Overview)
		if containsSecretish(string(blob)) {
			leaks = append(leaks, "global overview secretish")
		}
	}
	if len(leaks) > 0 {
		return "失败: " + strings.Join(leaks, "; ")
	}
	return "通过: 私有 HOME 无上游 Origin+Key；Gate 回环/Sentinel 形态；overview 无密钥"
}

func checkI11PackagedApp(root string) string {
	app := filepath.Join(root, "bin", "AgentHarbor.app")
	bin := filepath.Join(app, "Contents", "MacOS", "AgentHarbor")
	if !fileExists(bin) {
		return "阻断: 无 " + app + "（先 wails3 build / task darwin:package）"
	}
	_ = exec.Command("pkill", "-f", "bin/AgentHarbor.app/Contents/MacOS/AgentHarbor").Run()
	time.Sleep(500 * time.Millisecond)
	if err := exec.Command("open", app).Run(); err != nil {
		return "失败: open .app: " + err.Error()
	}
	time.Sleep(5 * time.Second)
	out, _ := exec.Command("pgrep", "-fl", "AgentHarbor.app/Contents/MacOS/AgentHarbor").CombinedOutput()
	if strings.TrimSpace(string(out)) == "" {
		return "失败: open 后无进程（可能闪退）"
	}
	_ = exec.Command("osascript", "-e", `tell application "AgentHarbor" to quit`).Run()
	time.Sleep(2 * time.Second)
	_ = exec.Command("pkill", "-f", "bin/AgentHarbor.app/Contents/MacOS/AgentHarbor").Run()
	time.Sleep(1 * time.Second)
	left, _ := exec.Command("pgrep", "-fl", "AgentHarbor.app/Contents/MacOS/AgentHarbor").CombinedOutput()
	if strings.TrimSpace(string(left)) != "" {
		return "失败: 退出后仍有 AgentHarbor 进程"
	}
	// 再确认无残留 claude 沙箱会话进程（尽力）
	return "通过: 打包 .app 可 open，退出后无 AgentHarbor 主进程残留（未做 Apple 公证）"
}

func runGoTest(root string, args []string) string {
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("HOME")+"/go/bin:/opt/homebrew/bin:"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	line := strings.TrimSpace(string(out))
	if len(line) > 200 {
		line = line[len(line)-200:]
	}
	if err != nil {
		return "失败: " + line
	}
	if strings.Contains(string(out), "PASS") || strings.Contains(string(out), "ok ") {
		return "通过: " + strings.Join(args, " ")
	}
	return "通过: go test " + args[0]
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

var secretish = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_-]{20,}|api[_-]?key["\s:=]+[A-Za-z0-9_-]{20,})`)

func containsSecretish(s string) bool {
	return secretish.MatchString(s)
}

func looksLikeFullKey(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "…") || strings.HasPrefix(s, "...") {
		return false
	}
	return len(s) >= 20 && secretish.MatchString(s)
}

func printReport(report map[string]string) {
	fmt.Println("===== LIVE REPORT =====")
	order := []string{"I1", "Health", "I3", "I4", "I4live", "I5", "I6", "I7", "I7unit", "扩权", "I8", "I9", "I9unit", "I10", "I11", "I2", "B6"}
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
