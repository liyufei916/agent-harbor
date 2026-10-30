package engine

import (
	"context"
)

// EnsureImage builds agent-harbor/runtime:dev when missing (Harbor Colima path).
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
	run := c.run
	if run == nil {
		run = defaultRun
	}
	return ensureRuntimeImage(ctx, run, docker, c.dockerEnv(), dockerfileDir, c.OnProgress)
}

// EnsureImage builds the Harbor guest image using the host docker CLI (本机 Docker).
func (s *SystemDocker) EnsureImage(ctx context.Context, dockerfileDir string) error {
	if err := s.Ensure(ctx); err != nil {
		return err
	}
	st, _ := s.Health(ctx)
	if !st.Ready {
		return errEngine("本机 Docker daemon 不可用，无法构建运行时镜像。")
	}
	return EnsureRuntimeImage(ctx, "docker", nil, dockerfileDir, nil)
}
