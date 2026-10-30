package runtime

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HarborDockerNetwork is the P2 session network. Guests reach Gate only via
// harbor.gate:host-gateway. Do NOT use Docker --internal: on Desktop it also
// blackholes host-gateway, so MCP/model proxies become Network unreachable.
const HarborDockerNetwork = "agent-harbor-p2"

var (
	dockerMemoryRe = regexp.MustCompile(`(?i)^[1-9][0-9]{0,8}[bkmgt]?$`)
	dockerCPUsRe   = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,3})?$`)
)

func dockerMemoryLimit() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_MEMORY")); dockerMemoryRe.MatchString(v) {
		return strings.ToLower(v)
	}
	return "2g"
}

func dockerCPULimit() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_CPUS")); dockerCPUsRe.MatchString(v) {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 64 {
			return v
		}
	}
	return "2"
}

func dockerPidsLimit() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_PIDS")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 16 && n <= 100000 {
			return v
		}
	}
	return "256"
}

// ensureHarborNetwork creates the P2 network if missing, and migrates legacy
// --internal networks that block harbor.gate (idempotent when already correct).
func (r *Docker) ensureHarborNetwork(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	internalFlag, err := r.runner.Run(probeCtx, "network", "inspect", "-f", "{{.Internal}}", HarborDockerNetwork)
	if err != nil {
		_, err = r.runner.Run(probeCtx, "network", "create",
			"--label", "agent-harbor=1",
			HarborDockerNetwork,
		)
		return err
	}
	if strings.EqualFold(strings.TrimSpace(internalFlag), "true") {
		if _, rmErr := r.runner.Run(probeCtx, "network", "rm", HarborDockerNetwork); rmErr != nil {
			return isolationUnavailable("Harbor 网络仍为 --internal（会阻断 harbor.gate）。请先停止全部 Docker 会话，或手动执行：docker network rm " + HarborDockerNetwork)
		}
		_, err = r.runner.Run(probeCtx, "network", "create",
			"--label", "agent-harbor=1",
			HarborDockerNetwork,
		)
		return err
	}
	return nil
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
