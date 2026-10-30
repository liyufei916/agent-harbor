package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RuntimeImageName is the default Harbor P2 guest tag.
func RuntimeImageName() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_IMAGE")); v != "" {
		return v
	}
	return "agent-harbor/runtime:dev"
}

// EnsureRuntimeImage builds the guest image when missing.
// dockerBin is an absolute docker CLI path or "docker"; env is optional process env for the build.
func EnsureRuntimeImage(ctx context.Context, dockerBin string, env []string, dockerfileDir string, report ProgressFunc) error {
	image := RuntimeImageName()
	if report != nil {
		report(Progress{Percent: 93, Label: "检查运行时镜像…"})
	}
	run := defaultRun
	inspectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	_, err := run(inspectCtx, dockerBin, []string{"image", "inspect", "--format", "{{.Id}}", image}, env)
	cancel()
	if err == nil {
		if report != nil {
			report(Progress{Percent: 100, Label: "运行时镜像已就绪"})
		}
		return nil
	}
	dir := strings.TrimSpace(dockerfileDir)
	if dir == "" {
		return errEngine("缺少 Dockerfile 目录，无法构建运行时镜像。")
	}
	df := filepath.Join(dir, "Dockerfile")
	if _, err := os.Stat(df); err != nil {
		return errEngine("未找到 Dockerfile，无法构建运行时镜像。")
	}
	if report != nil {
		report(Progress{Percent: 95, Label: "构建运行时镜像（需联网，较久）…"})
	}
	buildCtx, buildCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer buildCancel()
	args := []string{"build", "-t", image, "-f", df, dir}
	if _, err := run(buildCtx, dockerBin, args, env); err != nil {
		return errEngine("构建 Harbor 运行时镜像失败。")
	}
	if report != nil {
		report(Progress{Percent: 100, Label: "运行时镜像构建完成"})
	}
	return nil
}
