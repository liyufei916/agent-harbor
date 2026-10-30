package dsh

import (
	"context"
	"strings"

	"github.com/liyufei916/agent-harbor/internal/kits"
)

func (k *Kit) detectDockerGuest(ctx context.Context, info kits.KitInfo) (kits.KitInfo, error) {
	info.Agent.Reason = "Docker 镜像内未检测到固定版 dsh；请构建 deploy/docker 镜像。"
	nodeVer, err := kits.ProbeDockerGuest(ctx, kits.GuestNode, []string{"--version"})
	if err != nil || strings.TrimSpace(nodeVer) != SupportedNodeVersion {
		if ctx.Err() != nil {
			return info, kitError("CANCELLED")
		}
		return info, nil
	}
	text, err := kits.ProbeDockerGuest(ctx, kits.GuestNode, []string{kits.GuestDSHScript, "--version"})
	if err != nil || strings.TrimSpace(text) != SupportedVersion {
		return info, nil
	}
	text, err = kits.ProbeDockerGuest(ctx, kits.GuestNode, []string{kits.GuestDSHScript, "--help"})
	if err != nil || !strings.Contains(text, "--profile") || !strings.Contains(text, "headless") {
		return info, nil
	}
	info.Agent.Available = true
	info.Agent.Version = SupportedVersion
	info.Agent.Reason = ""
	info.NeedsDescendants = true
	info.ReadOnlyPaths = []string{kits.GuestNode, kits.GuestDSHRoot}
	return info, nil
}

func dockerPaths() (node, root, script string, err error) {
	return kits.GuestNode, kits.GuestDSHRoot, kits.GuestDSHScript, nil
}
