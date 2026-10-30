package codex

import (
	"context"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/kits"
)

func (k *Kit) detectDockerGuest(ctx context.Context, info kits.KitInfo) (kits.KitInfo, error) {
	info.Agent.Reason = "Docker 镜像内未检测到 Codex；请构建 deploy/docker 镜像。"
	version, err := kits.ProbeDockerGuest(ctx, kits.GuestCodex, []string{"--version"})
	if err != nil {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		return info, nil
	}
	version = strings.TrimSpace(version)
	if version == "" || !strings.Contains(strings.ToLower(version), "codex") {
		info.Agent.Reason = "镜像内 Codex 版本输出无法识别。"
		return info, nil
	}
	help, err := kits.ProbeDockerGuest(ctx, kits.GuestCodex, []string{"exec", "--help"})
	if err != nil {
		return info, nil
	}
	for _, flag := range []string{"--json", "--ephemeral", "--skip-git-repo-check"} {
		if !strings.Contains(help, flag) {
			info.Agent.Reason = "镜像内 Codex 缺少受限 exec 参数。"
			return info, nil
		}
	}
	info.Agent.Available, info.Agent.Version, info.Agent.Reason = true, version, ""
	info.ReadOnlyPaths = []string{kits.GuestCodex, kits.GuestNode}
	return info, nil
}

// dockerLaunch：镜像内 Codex 为 npm 全局脚本，须 node + 脚本路径才能走 turn_bridge 多轮。
// 若 Command=GuestCodex 且 Args 空，Build 会落入单轮 `codex exec`，进程退出即 stopped，无法交互。
func dockerLaunch() launchSpec {
	return launchSpec{
		Command:  kits.GuestNode,
		Args:     []string{kits.GuestCodex},
		ReadOnly: []string{kits.GuestCodex, kits.GuestNode},
		Label:    kits.GuestCodex,
		Node:     kits.GuestNode,
	}
}
