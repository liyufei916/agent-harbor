package claude

import (
	"context"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/kits"
)

func (k *Kit) detectDockerGuest(ctx context.Context, info kits.KitInfo) (kits.KitInfo, error) {
	info.Agent.Reason = "Docker 镜像内未检测到 Claude Code；请构建 deploy/docker 镜像。"
	versionOut, err := kits.ProbeDockerGuest(ctx, kits.GuestClaude, []string{"--version"})
	if err != nil {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		return info, nil
	}
	version, ok := parseClaudeVersion(versionOut)
	if !ok {
		info.Agent.Reason = "镜像内 Claude 版本输出无法识别。"
		return info, nil
	}
	info.Agent.Version = version
	help, err := kits.ProbeDockerGuest(ctx, kits.GuestClaude, []string{"--help"})
	if err != nil {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		info.Agent.Reason = "已安装 " + version + "，镜像内 --help 探测失败。"
		return info, nil
	}
	var missing []string
	for _, flag := range requiredHelpFlags {
		if !strings.Contains(help, flag) {
			missing = append(missing, flag)
		}
	}
	if len(missing) > 0 {
		info.Agent.Reason = "镜像内 Claude " + version + " 缺少：" + strings.Join(missing, ", ") + "。"
		return info, nil
	}
	info.Agent.Available, info.Agent.Reason = true, ""
	info.NeedsDescendants = true
	info.ReadOnlyPaths = []string{kits.GuestClaude}
	return info, nil
}

func dockerLaunch() launchSpec {
	return launchSpec{
		Command:  kits.GuestClaude,
		ReadOnly: []string{kits.GuestClaude},
		Label:    kits.GuestClaude + " (docker guest)",
	}
}
