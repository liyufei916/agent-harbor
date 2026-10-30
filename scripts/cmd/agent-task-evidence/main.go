// 按 Agent 跑一条真实短任务，产出可粘贴的脱敏证据行（codex / dsh / claude）。
// 勿打印 Key。用法（仓库根）：
//
//	go run ./scripts/cmd/agent-task-evidence                 # 默认：本机可用的 codex+dsh（有则跑）
//	go run ./scripts/cmd/agent-task-evidence -agents=codex
//	HARBOR_AGENTS=claude,codex,dsh go run ./scripts/cmd/agent-task-evidence
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/desktop"
)

func main() {
	agentsFlag := flag.String("agents", envOr("HARBOR_AGENTS", "codex,dsh"), "逗号分隔：claude,codex,dsh")
	timeout := flag.Duration("timeout", 90*time.Second, "单 Agent 等待 marker 上限")
	flag.Parse()

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

	want := splitCSV(*agentsFlag)
	if len(want) == 0 {
		fatal(fmt.Errorf("未指定 Agent"))
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

	health := api.GetRuntimeHealth()
	fmt.Printf("health available=%v isolation=%q\n", health.Available, health.IsolationLevel)
	if !health.Available {
		fatal(fmt.Errorf("隔离不可用: %s", health.Reason))
	}

	agents := api.ListAgents()
	if agents.Error != nil {
		fatal(fmt.Errorf("ListAgents: %s", agents.Error.Code))
	}
	for _, a := range agents.Agents {
		fmt.Printf("agent %s available=%v reason=%q\n", a.ID, a.Available, a.Reason)
	}

	// 清残留
	if list := api.ListSessions(); list.Error == nil {
		for _, s := range list.Sessions {
			fmt.Printf("cleanup leftover %s state=%s\n", short(s.SessionID), s.State)
			_ = api.StopSession(s.SessionID)
			waitCleanup(api, s.SessionID)
			_ = api.DestroySession(s.SessionID)
		}
	}

	fmt.Println("===== AGENT TASK EVIDENCE =====")
	exitBad := false
	for _, ag := range want {
		line := runOneAgent(api, ws, agents.Agents, ag, *timeout)
		fmt.Println(line)
		if strings.HasPrefix(line, ag+": 失败") || strings.HasPrefix(line, ag+": 阻断") {
			exitBad = true
		}
	}
	if exitBad {
		os.Exit(1)
	}
}

func runOneAgent(api *desktop.DesktopAPI, ws string, agents []contracts.AgentInfo, agentID string, timeout time.Duration) string {
	if !agentAvailable(agents, agentID) {
		return agentID + ": 阻断: CLI 不可用（Detect 未绿）"
	}
	modelID := pickModel(api, agentID)
	if modelID == "" {
		return agentID + ": 阻断: 无匹配模型账号（检查 profileId / agents[] / protocols）"
	}

	marker := fmt.Sprintf("harbor-agent-%s-%d", agentID, time.Now().Unix()%100000)
	task := fmt.Sprintf(
		"只读检查 demos/workspace：确认 hello.txt 存在；把一行摘要写入 summary.txt，内容须含 %s。不要使用 Bash 访问工作区外路径。",
		marker,
	)

	res := api.CreateSession(contracts.SessionSpec{
		Workspace:      ws,
		AgentID:        agentID,
		ModelAccountID: modelID,
		Task:           task,
		MountMode:      "clone",
	})
	if res.Error != nil {
		return fmt.Sprintf("%s: 失败: Create %s %s", agentID, res.Error.Code, res.Error.Message)
	}
	if res.Session == nil {
		return agentID + ": 失败: Create 无会话"
	}
	sid := res.Session.SessionID
	clone := res.Session.Workspace

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		g := api.GetSession(sid)
		if g.Error != nil {
			_ = api.DestroySession(sid)
			return fmt.Sprintf("%s: 失败: GetSession %s", agentID, g.Error.Code)
		}
		if g.Session == nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		st := g.Session.State
		if st == contracts.StateRunning {
			if g.Session.Workspace != "" {
				clone = g.Session.Workspace
			}
			break
		}
		if st == contracts.StateFailed || st == contracts.StateStopped {
			code, msg := "", ""
			if g.Session.Error != nil {
				code = g.Session.Error.Code
				msg = strings.TrimSpace(g.Session.Error.Message)
			}
			_ = api.DestroySession(sid)
			if msg != "" {
				return fmt.Sprintf("%s: 失败: 未进 running（%s %s: %s）", agentID, st, code, msg)
			}
			return fmt.Sprintf("%s: 失败: 未进 running（%s %s）", agentID, st, code)
		}
		time.Sleep(300 * time.Millisecond)
	}
	g := api.GetSession(sid)
	if g.Session == nil || g.Session.State != contracts.StateRunning {
		stopDestroy(api, sid)
		return agentID + ": 失败: 超时未进 running"
	}

	hit := waitMarker(clone, marker, timeout)
	_ = api.StopSession(sid)
	waitCleanup(api, sid)
	sb := api.SyncBackWorkspace(sid)
	syncNote := "SyncBack=ok"
	if sb.Error != nil {
		syncNote = "SyncBack=" + sb.Error.Code
	}
	_ = api.DestroySession(sid)

	srcHit := false
	if b, err := os.ReadFile(filepath.Join(ws, "summary.txt")); err == nil && strings.Contains(string(b), marker) {
		srcHit = true
	}
	if !hit {
		return fmt.Sprintf("%s: 失败: session=%s model=%s 未在 clone 见到 marker（%s）", agentID, short(sid), short(modelID), syncNote)
	}
	return fmt.Sprintf("%s: 通过: session=%s model=%s marker=%s srcSummary=%v %s",
		agentID, short(sid), short(modelID), marker, srcHit, syncNote)
}

func waitMarker(clone, marker string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(filepath.Join(clone, "summary.txt"))
		if err == nil && strings.Contains(string(b), marker) {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func stopDestroy(api *desktop.DesktopAPI, sid string) {
	_ = api.StopSession(sid)
	waitCleanup(api, sid)
	_ = api.DestroySession(sid)
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

func pickModel(api *desktop.DesktopAPI, agentID string) string {
	acc := api.ListAccounts("model")
	if acc.Error != nil {
		return ""
	}
	for _, m := range acc.Accounts {
		if m.Kind != "model" {
			continue
		}
		if modelFits(m, agentID) {
			return m.ID
		}
	}
	return ""
}

func modelFits(m contracts.AccountInfo, agentID string) bool {
	// 与 policy.AccountAllowsAgent 对齐：空 Agents 时 custom/demo→仅 claude，dsh-model→仅 dsh。
	agents := append([]string(nil), m.Agents...)
	if len(agents) == 0 {
		switch m.ProfileID {
		case "dsh-model":
			agents = []string{"dsh"}
		case "codex-model":
			agents = []string{"codex"}
		case "demo-model", "custom", "":
			agents = []string{"claude"}
		}
	}
	listed := false
	for _, a := range agents {
		if a == agentID {
			listed = true
			break
		}
	}
	if !listed {
		return false
	}
	switch agentID {
	case "claude", "dsh":
		return m.ProfileID == "dsh-model" || m.ProfileID == "demo-model" || m.ProfileID == "custom" ||
			contains(m.Protocols, "anthropic") || len(m.Protocols) == 0
	case "codex":
		return m.ProfileID == "codex-model" || contains(m.Protocols, "openai-responses")
	}
	return false
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

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func short(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
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
