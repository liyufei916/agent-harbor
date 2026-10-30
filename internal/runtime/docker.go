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

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// DefaultDockerImage is the placeholder guest for Create/Destroy until the Harbor
// agent image (Claude/Codex/dsh) lands. Override with HARBOR_DOCKER_IMAGE.
const DefaultDockerImage = "alpine:3.20"

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
	readOnly             map[string]bool
	allowedTCP           []string
	attempted, destroyed bool
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
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := r.runner.Run(probeCtx, "info", "--format", "{{.ServerVersion}}"); err != nil {
		return runtimeError("ISOLATION_UNAVAILABLE")
	}
	return nil
}

func (r *Docker) Create(ctx context.Context, spec RuntimeSpec) (Handle, error) {
	if err := r.Health(ctx); err != nil {
		return Handle{}, err
	}
	if spec.SessionID == "" || (spec.MountMode != "direct" && spec.MountMode != "clone") || len(spec.AllowedTCP) == 0 || len(spec.ReadOnlyPaths) == 0 {
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
	readOnly := make([]string, 0, len(spec.ReadOnlyPaths))
	for _, path := range spec.ReadOnlyPaths {
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
	args := r.createArgs(home, workspace, readOnly)
	createCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	containerID, err := r.runner.Run(createCtx, args...)
	if err != nil || containerID == "" {
		return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
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
		readOnly:    make(map[string]bool),
		allowedTCP:  append([]string(nil), spec.AllowedTCP...),
	}
	for _, path := range readOnly {
		e.readOnly[path] = true
	}
	r.mu.Lock()
	r.entries[handle.ID] = e
	r.mu.Unlock()
	owned = true
	return handle, nil
}

func (r *Docker) createArgs(home, workspace string, readOnly []string) []string {
	args := []string{
		"create",
		"--label", "agent-harbor=1",
		"--label", "agent-harbor.runtime=docker",
		"--user", "65534:65534",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"--network", "none",
		"--mount", "type=bind,src=" + home + ",dst=/home/harbor",
		"--mount", "type=bind,src=" + workspace + ",dst=/workspace",
		"--workdir", "/workspace",
		"--env", "HOME=/home/harbor",
	}
	for i, path := range readOnly {
		dst := "/harbor/ro/" + hex.EncodeToString([]byte{byte(i)}) + "/" + filepath.Base(path)
		args = append(args, "--mount", "type=bind,src="+path+",dst="+dst+",readonly")
	}
	// Placeholder guest: keeps container alive for Destroy proof. Agent Exec is next slice.
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

// Exec is intentionally unavailable until the Harbor agent image maps host Kit
// launch paths into the guest. Create/Destroy already prove the container shell.
func (r *Docker) Exec(ctx context.Context, handle Handle, launch LaunchSpec, _ func(contracts.OutputData)) (Process, error) {
	if ctx.Err() != nil {
		return nil, runtimeError("CANCELLED")
	}
	e, err := r.lookup(handle)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.destroyed {
		return nil, runtimeError("INVALID_ARGUMENT")
	}
	if launch.Command == "" {
		return nil, runtimeError("INVALID_ARGUMENT")
	}
	e.attempted = true
	return nil, runtimeError("AGENT_UNSUPPORTED")
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
	if e.containerID != "" {
		rmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if _, err := r.runner.Run(rmCtx, "rm", "-f", e.containerID); err != nil {
			if ctx.Err() != nil {
				return runtimeError("CLEANUP_FAILED")
			}
			return runtimeError("CLEANUP_FAILED")
		}
		e.containerID = ""
	}
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
	e.allowedTCP = nil
	return nil
}
