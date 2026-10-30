package desktop

import (
	"context"
	"os"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/kits/claude"
	"github.com/liyufei916/agent-harbor/internal/kits/codex"
	"github.com/liyufei916/agent-harbor/internal/kits/dsh"
	"github.com/liyufei916/agent-harbor/internal/orchestrator"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

// AttachOrchestrator 用研发 A 的 Runtime、Claude/Codex/dsh Kit 与 Orchestrator 接上本进程的 Gate/Policy/Secrets。
// appCtx 是应用级 context，会话任务从它派生；不要传入已取消的请求 context。
// 模型名来自 HARBOR_CLAUDE_MODEL，连接配置必须显式登记；检测仍可在未配置账号时独立运行。
func (a *App) AttachOrchestrator(appCtx context.Context) error {
	if appCtx == nil {
		appCtx = context.Background()
	}
	model := strings.TrimSpace(os.Getenv("HARBOR_CLAUDE_MODEL"))
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	kit, err := claude.New(os.Getenv("HARBOR_CLAUDE_BIN"), model)
	if err != nil {
		return err
	}
	dshModel := strings.TrimSpace(os.Getenv("HARBOR_DSH_MODEL"))
	if dshModel == "" {
		dshModel = "deepseek-flash"
	}
	dshKit, err := dsh.NewConversation(os.Getenv("HARBOR_DSH_NODE"), os.Getenv("HARBOR_DSH_INSTALLATION"), dshModel)
	if err != nil {
		return err
	}
	codexModel := strings.TrimSpace(os.Getenv("HARBOR_CODEX_MODEL"))
	if codexModel == "" {
		codexModel = "gpt-5-codex"
	}
	codexKit, err := codex.New(os.Getenv("HARBOR_CODEX_BIN"), codexModel)
	if err != nil {
		return err
	}
	agentKits := map[string]kits.Kit{"claude": kit, "codex": codexKit, "dsh": dshKit}
	reg, err := kits.NewRegistry(agentKits)
	if err != nil {
		return err
	}
	rt := runtime.NewSelectedWithBackend(a.getPrefs().IsolationBackend)
	svc, err := orchestrator.New(appCtx, orchestrator.Dependencies{
		Runtime: rt,
		Kits:    agentKits,
		Catalog: a.secrets,
		Policy:  a.policy,
		Gate:    a.gate,
	}, a.forwardEvent, orchestrator.Timeouts{})
	if err != nil {
		return err
	}
	a.SetRuntime(rt)
	a.SetAgentDetector(registryDetector{reg: reg})
	a.SetSessionService(svc)
	return nil
}

type registryDetector struct {
	reg *kits.Registry
}

func (d registryDetector) ListAgents(ctx context.Context) ([]contracts.AgentInfo, error) {
	infos, err := d.reg.DetectAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.AgentInfo, 0, len(infos))
	for _, info := range infos {
		out = append(out, info.Agent)
	}
	return out, nil
}
