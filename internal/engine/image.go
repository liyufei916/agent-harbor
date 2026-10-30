package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnsureImage builds agent-harbor/runtime:dev when missing.
// dockerfileDir must contain Dockerfile (typically deploy/docker).
func (c *Colima) EnsureImage(ctx context.Context, dockerfileDir string) error {
	if err := c.Ensure(ctx); err != nil {
		return err
	}
	st, _ := c.Health(ctx)
	if !st.Ready {
		if err := c.Start(ctx); err != nil {
			return err
		}
	}
	docker, err := c.resolveBin("docker")
	if err != nil {
		return err
	}
	image := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_IMAGE"))
	if image == "" {
		image = "agent-harbor/runtime:dev"
	}
	c.report(93, "检查运行时镜像…")
	inspectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	_, err = c.run(inspectCtx, docker, []string{"image", "inspect", "--format", "{{.Id}}", image}, c.dockerEnv())
	cancel()
	if err == nil {
		c.report(100, "运行时镜像已就绪")
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
	c.report(95, "构建运行时镜像（较久）…")
	buildCtx, buildCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer buildCancel()
	args := []string{"build", "-t", image, "-f", df, dir}
	if _, err := c.run(buildCtx, docker, args, c.dockerEnv()); err != nil {
		return errEngine("构建 Harbor 运行时镜像失败。")
	}
	c.report(100, "运行时镜像构建完成")
	return nil
}
