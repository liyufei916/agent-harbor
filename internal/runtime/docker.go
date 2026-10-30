package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultDockerImage is the Harbor P2 guest. Build with:
//
//	docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
//
// Override with HARBOR_DOCKER_IMAGE. Falls back to alpine only if unset and env empty —
// prefer the Harbor image so /usr/local/bin/node exists.
const DefaultDockerImage = "agent-harbor/runtime:dev"

// dockerRunner is injectable so unit tests can assert fail-closed without a daemon.
type dockerRunner interface {
	LookPath() (string, error)
	Run(ctx context.Context, args ...string) (string, error)
}

type execDockerRunner struct{}

func (execDockerRunner) LookPath() (string, error) {
	return exec.LookPath("docker")
}

func (execDockerRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
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
			return out, &dockerRunError{err: err, detail: msg}
		}
		return out, err
	}
	return out, nil
}

type dockerRunError struct {
	err    error
	detail string
}

func (e *dockerRunError) Error() string {
	if e.detail != "" {
		return e.detail
	}
	return e.err.Error()
}

func (e *dockerRunError) Unwrap() error { return e.err }

// Docker is the P2 container Runtime: one session → one container, no docker.sock.
type Docker struct {
	mu      sync.Mutex
	runner  dockerRunner
	image   string
	entries map[string]*dockerEntry
}

type dockerEntry struct {
	mu                   sync.Mutex
	handle               Handle
	root, workspace      string
	containerID          string
	mounts               map[string]string // host abs → guest abs
	readOnly             map[string]bool
	allowedTCP           []string
	gateToProxy          map[string]string // Gate port → host proxy listen port
	proxies              []*tcpProxy
	process              *dockerProcess
	attempted, destroyed bool
	started              bool
}

var _ Runtime = (*Docker)(nil)

func NewDocker() *Docker {
	image := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_IMAGE"))
	if image == "" {
		image = DefaultDockerImage
	}
	return &Docker{
		runner:  execDockerRunner{},
		image:   image,
		entries: make(map[string]*dockerEntry),
	}
}

func (r *Docker) Health(ctx context.Context) error {
	if ctx.Err() != nil {
		return runtimeError("CANCELLED")
	}
	if _, err := r.runner.LookPath(); err != nil {
		return isolationUnavailable("未找到 docker CLI，已拒绝启动。")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ver, err := r.runner.Run(probeCtx, "info", "--format", "{{.ServerVersion}}")
	// 部分钉扎 docker CLI 在 daemon 不可达时仍 exit 0 且 stdout 为空；必须校验版本非空。
	if err != nil || strings.TrimSpace(ver) == "" {
		return isolationUnavailable("Docker daemon 不可用，已拒绝启动。")
	}
	if err := r.ensureHarborNetwork(ctx); err != nil {
		return isolationUnavailable("Harbor Docker 网络不可用，已拒绝启动。")
	}
	imgCtx, imgCancel := context.WithTimeout(ctx, 15*time.Second)
	defer imgCancel()
	if _, err := r.runner.Run(imgCtx, "image", "inspect", "--format", "{{.Id}}", r.image); err != nil {
		return isolationUnavailable("Harbor 运行时镜像不存在，已拒绝启动。请先构建 agent-harbor/runtime:dev。")
	}
	// Crash leftovers: remove exited harbor containers; never fail Health.
	r.pruneExitedHarborContainers(ctx)
	return nil
}

// HealthFor satisfies HealthChecker; Docker isolation does not vary by child-process capability.
func (r *Docker) HealthFor(ctx context.Context, _ bool) error {
	return r.Health(ctx)
}

func (r *Docker) Create(ctx context.Context, spec RuntimeSpec) (Handle, error) {
	if err := r.Health(ctx); err != nil {
		return Handle{}, err
	}
	if spec.SessionID == "" || (spec.MountMode != "direct" && spec.MountMode != "clone") || len(spec.AllowedTCP) == 0 {
		return Handle{}, runtimeError("INVALID_ARGUMENT")
	}
	workspace, err := canonicalPath(spec.Workspace)
	if err != nil {
		return Handle{}, err
	}
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() || forbiddenWorkspace(workspace) {
		return Handle{}, runtimeError("WORKSPACE_DENIED")
	}
	// Guest image paths (e.g. /usr/local/bin/claude) need not exist on the host and are not bind-mounted.
	readOnly := make([]string, 0, len(spec.ReadOnlyPaths))
	guestOnly := make(map[string]string)
	for _, path := range spec.ReadOnlyPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			return Handle{}, runtimeError("INVALID_ARGUMENT")
		}
		if IsGuestImagePath(path) {
			clean := filepath.Clean(path)
			guestOnly[clean] = clean
			readOnly = append(readOnly, clean)
			continue
		}
		resolved, err := canonicalPath(path)
		if err != nil || within(workspace, resolved) {
			return Handle{}, runtimeError("INVALID_ARGUMENT")
		}
		st, statErr := os.Stat(resolved)
		if statErr != nil || (st.IsDir() && (!safeInstallationRoot(resolved) || within(resolved, workspace))) {
			return Handle{}, runtimeError("INVALID_ARGUMENT")
		}
		readOnly = append(readOnly, resolved)
	}
	root, err := sessionRoot(spec.MountMode, workspace)
	if err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	owned := false
	defer func() {
		if !owned {
			_ = os.RemoveAll(root)
		}
	}()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	if err := os.Mkdir(filepath.Join(home, "tmp"), 0700); err != nil {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	if spec.MountMode == "clone" {
		clonePath := filepath.Join(root, "workspace")
		if err := cloneWorkspace(ctx, workspace, clonePath); err != nil {
			return Handle{}, err
		}
		workspace = clonePath
	}
	if ctx.Err() != nil {
		return Handle{}, runtimeError("CANCELLED")
	}
	hostMounts := make([]string, 0, len(readOnly))
	for _, path := range readOnly {
		if _, ok := guestOnly[path]; !ok {
			hostMounts = append(hostMounts, path)
		}
	}
	mounts := make(map[string]string, len(hostMounts)+len(guestOnly))
	for g, dst := range guestOnly {
		mounts[g] = dst
	}
	gateToProxy, proxies, err := startAllowedProxies(spec.AllowedTCP)
	if err != nil {
		return Handle{}, err
	}
	proxyOK := false
	defer func() {
		if !proxyOK {
			closeProxies(proxies)
		}
	}()
	args := r.createArgs(spec.SessionID, home, workspace, hostMounts, mounts)
	createCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	containerID, err := r.runner.Run(createCtx, args...)
	if err != nil || containerID == "" {
		return Handle{}, isolationUnavailable("无法创建 Harbor Docker 容器，已拒绝启动。")
	}
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		_, _ = r.runner.Run(context.Background(), "rm", "-f", containerID)
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
	}
	handle := Handle{ID: hex.EncodeToString(id[:]), PrivateHome: home, Workspace: workspace}
	e := &dockerEntry{
		handle:      handle,
		root:        root,
		workspace:   workspace,
		containerID: containerID,
		mounts:      mounts,
		readOnly:    make(map[string]bool),
		allowedTCP:  append([]string(nil), spec.AllowedTCP...),
		gateToProxy: gateToProxy,
		proxies:     proxies,
	}
	for _, path := range readOnly {
		e.readOnly[path] = true
	}
	proxyOK = true
	r.mu.Lock()
	r.entries[handle.ID] = e
	r.mu.Unlock()
	owned = true
	return handle, nil
}

func (r *Docker) createArgs(sessionID, home, workspace string, readOnly []string, mounts map[string]string) []string {
	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		sid = "unknown"
	}
	args := []string{
		"create",
		"--label", "agent-harbor=1",
		"--label", "agent-harbor.runtime=docker",
		"--label", "agent-harbor.session=" + sid,
		"--user", "65534:65534",
		"--init", // tini: reap zombies from Agent child tools
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		// Internal net; Gate only via harbor.gate (host-gateway). Blackhole common SSRF alias.
		"--network", HarborDockerNetwork,
		"--add-host", dockerGateHost + ":host-gateway",
		"--add-host", "host.docker.internal:127.0.0.1",
		"--mount", "type=bind,src=" + home + ",dst=" + dockerGuestHome,
		"--mount", "type=bind,src=" + workspace + ",dst=" + dockerGuestWorkspace,
		"--workdir", dockerGuestWorkspace,
		"--env", "HOME=" + dockerGuestHome,
	}
	args = appendResourceLimits(args)
	for i, path := range readOnly {
		dst := dockerROMount(i, path)
		mounts[path] = dst
		args = append(args, "--mount", "type=bind,src="+path+",dst="+dst+",readonly")
	}
	args = append(args, r.image, "sleep", "infinity")
	return args
}

func (r *Docker) lookup(handle Handle) (*dockerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[handle.ID]
	if e == nil || e.handle.PrivateHome != handle.PrivateHome {
		return nil, runtimeError("INVALID_ARGUMENT")
	}
	return e, nil
}

func (r *Docker) Destroy(ctx context.Context, handle Handle) error {
	e, err := r.lookup(handle)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.destroyed {
		return nil
	}
	if e.process != nil {
		e.process.kill(true)
		select {
		case <-e.process.done:
		case <-ctx.Done():
			return runtimeError("CLEANUP_FAILED")
		}
	}
	if e.containerID != "" {
		rmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if _, err := r.runner.Run(rmCtx, "rm", "-f", e.containerID); err != nil {
			return runtimeError("CLEANUP_FAILED")
		}
		e.containerID = ""
	}
	closeProxies(e.proxies)
	e.proxies = nil
	e.gateToProxy = nil
	if e.root != "" {
		if err := os.RemoveAll(e.root); err != nil {
			return runtimeError("CLEANUP_FAILED")
		}
		if _, err := os.Lstat(e.root); !os.IsNotExist(err) {
			return runtimeError("CLEANUP_FAILED")
		}
	}
	e.destroyed = true
	e.root, e.workspace = "", ""
	e.readOnly = nil
	e.mounts = nil
	e.allowedTCP = nil
	e.process = nil
	return nil
}
