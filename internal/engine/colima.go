package engine

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

// Colima is the product-primary managed engine (B→C).
// Uses PATH or Root/bin; Ensure may auto-download pinned binaries (M2c).
type Colima struct {
	Root string // Application Support/.../engine; empty → default

	// OnProgress optional UI callback during Ensure/Start/EnsureImage (0–100).
	OnProgress ProgressFunc

	lookPath func(string) (string, error)
	run      func(ctx context.Context, name string, args []string, env []string) (string, error)
	fetch    fetchFunc
}

func (c *Colima) report(percent int, label string) {
	if c == nil || c.OnProgress == nil {
		return
	}
	c.OnProgress(Progress{Percent: clampPercent(percent), Label: label})
}

func NewColima() *Colima {
	return &Colima{
		lookPath: exec.LookPath,
		run:      defaultRun,
	}
}

func defaultRun(ctx context.Context, name string, args []string, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
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
			return out, errEngine(msg)
		}
		return out, err
	}
	return out, nil
}

func (c *Colima) baseEnv() []string {
	env := append([]string{}, os.Environ()...)
	home := c.colimaHomeDir()
	limaBin := filepath.Join(c.root(), "lima", "bin")
	bin := c.binDir() + string(os.PathListSeparator) + limaBin
	pathSet := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + bin + string(os.PathListSeparator) + strings.TrimPrefix(e, "PATH=")
			pathSet = true
			break
		}
	}
	if !pathSet {
		env = append(env, "PATH="+bin)
	}
	env = append(env,
		"COLIMA_HOME="+home,
		"COLIMA_PROFILE="+ProfileName,
	)
	return env
}

func (c *Colima) dockerEnv() []string {
	return append(c.baseEnv(), "DOCKER_HOST="+c.DockerHost())
}

func (c *Colima) Ensure(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	c.report(1, "准备引擎目录…")
	if err := os.MkdirAll(c.binDir(), 0700); err != nil {
		return errEngine("无法创建引擎目录。")
	}
	if err := os.MkdirAll(c.colimaHomeDir(), 0700); err != nil {
		return errEngine("无法创建 Colima 数据目录。")
	}
	needDownload := false
	for _, name := range []string{"colima", "docker", "limactl"} {
		if _, err := c.resolveBin(name); err != nil {
			needDownload = true
			break
		}
	}
	if needDownload {
		c.report(3, "开始下载引擎组件…")
		if err := c.ensurePinnedBins(ctx); err != nil {
			return err
		}
	}
	if _, err := c.resolveBin("colima"); err != nil {
		return errEngine("未找到 colima。可开启自动下载，或将二进制放入引擎 bin 目录。")
	}
	if _, err := c.resolveBin("docker"); err != nil {
		return errEngine("未找到 docker CLI。可开启自动下载，或将二进制放入引擎 bin 目录。")
	}
	if _, err := c.resolveBin("limactl"); err != nil {
		return errEngine("未找到 limactl（Colima 依赖）。可开启自动下载，或将二进制放入引擎 bin 目录。")
	}
	c.report(75, "引擎组件已就绪")
	return nil
}

func (c *Colima) Start(ctx context.Context) error {
	if err := c.Ensure(ctx); err != nil {
		return err
	}
	st, _ := c.Health(ctx)
	if st.Ready {
		c.report(100, "Harbor Colima 已就绪")
		return c.ApplyDockerHost()
	}
	c.report(78, "正在启动 Colima…")
	colima, err := c.resolveBin("colima")
	if err != nil {
		return err
	}
	startCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	// Light defaults: 2 CPU / 2 GiB / 20 GiB disk; docker runtime; dedicated profile.
	args := []string{
		"start", ProfileName,
		"--runtime", "docker",
		"--cpu", "2",
		"--memory", "2",
		"--disk", "20",
		"--activate=false", // do not steal user's global docker context
	}
	if goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64" {
		args = append(args, "--vm-type", "vz", "--mount-type", "virtiofs")
	}
	if _, err := c.run(startCtx, colima, args, c.baseEnv()); err != nil {
		return errEngine("无法启动 Harbor Colima 引擎。")
	}
	c.report(88, "等待 Docker daemon…")
	return c.waitDocker(ctx)
}

func (c *Colima) waitDocker(ctx context.Context) error {
	docker, err := c.resolveBin("docker")
	if err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := c.run(probeCtx, docker, []string{"info", "--format", "{{.ServerVersion}}"}, c.dockerEnv())
		cancel()
		if err == nil {
			c.report(92, "Docker daemon 已就绪")
			return c.ApplyDockerHost()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return errEngine("Colima 已启动但 Docker daemon 未就绪。")
}

func (c *Colima) Stop(ctx context.Context) error {
	colima, err := c.resolveBin("colima")
	if err != nil {
		return nil // nothing to stop
	}
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_, _ = c.run(stopCtx, colima, []string{"stop", ProfileName}, c.baseEnv())
	return nil
}

func (c *Colima) Health(ctx context.Context) (Status, error) {
	// Health 只探测，不下载/启停（下载留给 Ensure；避免 20s 探测超时误报「下载失败」）。
	for _, name := range []string{"colima", "docker", "limactl"} {
		if _, err := c.resolveBin(name); err != nil {
			return Status{
				Ready:   false,
				Source:  SourceHarborColima,
				Message: "引擎组件未就绪（缺 " + name + "）。请点「启用容器引擎」下载并启动。",
			}, nil
		}
	}
	docker, err := c.resolveBin("docker")
	if err != nil {
		return Status{Ready: false, Source: SourceHarborColima, Message: err.Error()}, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := c.run(probeCtx, docker, []string{"info", "--format", "{{.ServerVersion}}"}, c.dockerEnv()); err != nil {
		return Status{
			Ready:      false,
			Source:     SourceHarborColima,
			Message:    "Harbor Colima 未运行或 daemon 不可用。请点「启用容器引擎」。",
			DockerHost: c.DockerHost(),
		}, nil
	}
	return Status{
		Ready:      true,
		Source:     SourceHarborColima,
		Message:    "Harbor Colima 可用",
		DockerHost: c.DockerHost(),
	}, nil
}

// ApplyDockerHost sets process DOCKER_HOST so subsequent docker CLI (Runtime) hits Colima.
// Also prepends Harbor engine/bin to PATH so LookPath("docker") finds the pinned CLI
// (Runtime / Kit 探测不认 Application Support 绝对路径，只查 PATH)。
func (c *Colima) ApplyDockerHost() error {
	host := c.DockerHost()
	if err := os.Setenv("DOCKER_HOST", host); err != nil {
		return errEngine("无法设置 DOCKER_HOST。")
	}
	_ = os.Setenv("COLIMA_HOME", c.colimaHomeDir())
	_ = os.Setenv("COLIMA_PROFILE", ProfileName)
	c.prependBinToPATH()
	return nil
}

func (c *Colima) prependBinToPATH() {
	bin := c.binDir()
	if bin == "" {
		return
	}
	cur := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	for _, p := range strings.Split(cur, sep) {
		if p == bin {
			return
		}
	}
	if cur == "" {
		_ = os.Setenv("PATH", bin)
		return
	}
	_ = os.Setenv("PATH", bin+sep+cur)
}

var _ Engine = (*Colima)(nil)
