package engine

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// SystemDocker uses the host docker CLI and default DOCKER_HOST / desktop daemon.
// Advanced fallback when the user already has Docker; not the product primary path.
type SystemDocker struct {
	lookPath func(string) (string, error)
	run      func(ctx context.Context, name string, args ...string) (string, error)
}

func NewSystemDocker() *SystemDocker {
	return &SystemDocker{
		lookPath: exec.LookPath,
		run: func(ctx context.Context, name string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			out := strings.TrimSpace(stdout.String())
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = out
				}
				if msg != "" {
					return out, err
				}
				return out, err
			}
			return out, nil
		},
	}
}

func (s *SystemDocker) Ensure(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, err := s.lookPath("docker"); err != nil {
		return errEngine("未找到 docker CLI。可改用 Harbor 托管引擎，或安装 Docker 后重试。")
	}
	return nil
}

func (s *SystemDocker) Start(ctx context.Context) error {
	// System daemon is owned by Desktop/Colima user installs; Harbor does not start it.
	return s.Ensure(ctx)
}

func (s *SystemDocker) Stop(context.Context) error {
	return nil
}

func (s *SystemDocker) Health(ctx context.Context) (Status, error) {
	if err := s.Ensure(ctx); err != nil {
		return Status{Ready: false, Source: SourceSystemDocker, Message: err.Error()}, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.run(probeCtx, "docker", "info", "--format", "{{.ServerVersion}}"); err != nil {
		return Status{
			Ready:   false,
			Source:  SourceSystemDocker,
			Message: "本机 Docker daemon 不可用。",
		}, nil
	}
	return Status{Ready: true, Source: SourceSystemDocker, Message: "本机 Docker 可用"}, nil
}

var _ Engine = (*SystemDocker)(nil)
