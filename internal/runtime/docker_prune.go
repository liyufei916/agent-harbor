package runtime

import (
	"context"
	"strings"
	"time"
)

// pruneExitedHarborContainers removes exited containers labeled agent-harbor=1.
// Best-effort: never fails Health. Active (running) session containers are kept.
func (r *Docker) pruneExitedHarborContainers(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := r.runner.Run(probeCtx,
		"ps", "-aq",
		"--filter", "label=agent-harbor=1",
		"--filter", "status=exited",
	)
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}
	for _, id := range strings.Fields(out) {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		rmCtx, rmCancel := context.WithTimeout(ctx, 15*time.Second)
		_, _ = r.runner.Run(rmCtx, "rm", "-f", id)
		rmCancel()
	}
}
