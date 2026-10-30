package runtime

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

// HarborDockerNetwork is an --internal network: no masquerade to the public
// Internet; Gate remains reachable via host.docker.internal:host-gateway.
const HarborDockerNetwork = "agent-harbor-p2"

func dockerMemoryLimit() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_MEMORY")); v != "" {
		return v
	}
	return "2g"
}

func dockerCPULimit() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_CPUS")); v != "" {
		return v
	}
	return "2"
}

func dockerPidsLimit() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_PIDS")); v != "" {
		return v
	}
	return "256"
}

// ensureHarborNetwork creates the internal P2 network if missing (idempotent).
func (r *Docker) ensureHarborNetwork(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := r.runner.Run(probeCtx, "network", "inspect", HarborDockerNetwork); err == nil {
		return nil
	}
	_, err := r.runner.Run(probeCtx, "network", "create",
		"--internal",
		"--label", "agent-harbor=1",
		HarborDockerNetwork,
	)
	return err
}

func appendResourceLimits(args []string) []string {
	if mem := dockerMemoryLimit(); mem != "" {
		args = append(args, "--memory", mem)
	}
	if cpus := dockerCPULimit(); cpus != "" {
		args = append(args, "--cpus", cpus)
	}
	if pids := dockerPidsLimit(); pids != "" {
		if _, err := strconv.Atoi(pids); err == nil {
			args = append(args, "--pids-limit", pids)
		}
	}
	return args
}
