package kits

import (
	"context"

	"github.com/liyufei916/agent-harbor/internal/runtime"
)

// DockerGuest is true when the desktop selected the P2 Docker Runtime.
func DockerGuest() bool { return runtime.DockerEnabled() }

// ProbeDockerGuest runs a guest-image CLI probe (Linux container).
func ProbeDockerGuest(ctx context.Context, command string, args []string) (string, error) {
	return runtime.ProbeDockerGuest(ctx, command, args)
}

// Fixed guest paths baked into deploy/docker/Dockerfile.
const (
	GuestHome       = "/home/harbor" // 与 runtime dockerGuestHome 对齐
	GuestNode       = "/usr/local/bin/node"
	GuestClaude     = "/usr/local/bin/claude"
	GuestCodex      = "/usr/local/bin/codex"
	GuestDSHRoot    = "/opt/harbor/dsh-0.2.1-rc.2"
	GuestDSHScript  = GuestDSHRoot + "/node_modules/@deepseek-ai/dsh/lib/bin.js"
	GuestDSHNodeBin = GuestNode // image Node; version must match dsh SupportedNodeVersion
)
