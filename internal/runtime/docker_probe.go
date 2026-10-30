package runtime

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// ProbeDockerGuest runs command+args inside a throwaway container from DockerImage().
// Used by Kits when HARBOR_RUNTIME selects Docker (host macOS binaries cannot run in Linux guests).
func ProbeDockerGuest(ctx context.Context, command string, args []string) (string, error) {
	if ctx.Err() != nil {
		return "", runtimeError("CANCELLED")
	}
	if strings.TrimSpace(command) == "" || !IsGuestImagePath(command) {
		return "", runtimeError("AGENT_UNSUPPORTED")
	}
	for _, a := range args {
		if strings.ContainsRune(a, 0) {
			return "", runtimeError("INVALID_ARGUMENT")
		}
	}
	bin, err := exec.LookPath("docker")
	if err != nil {
		return "", isolationUnavailable("未找到 docker CLI，已拒绝启动。")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	image := DockerImage()
	inspect := exec.CommandContext(probeCtx, bin, "image", "inspect", "--format", "{{.Id}}", image)
	if err := inspect.Run(); err != nil {
		return "", isolationUnavailable("Harbor 运行时镜像不存在，已拒绝启动。请先构建 agent-harbor/runtime:dev。")
	}
	runArgs := []string{
		"run", "--rm",
		"--network", "none",
		"--user", "65534:65534",
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=32m",
		"--entrypoint", command,
		image,
	}
	runArgs = append(runArgs, args...)
	cmd := exec.CommandContext(probeCtx, bin, runArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.ToLower(stderr.String() + stdout.String())
		if strings.Contains(detail, "cannot connect") || strings.Contains(detail, "is the docker daemon running") {
			return "", isolationUnavailable("Docker daemon 不可用，已拒绝启动。")
		}
		return "", runtimeError("AGENT_UNSUPPORTED")
	}
	return stdout.String(), nil
}
