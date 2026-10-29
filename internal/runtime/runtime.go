// 本文件按 docs/CONTRACT.md v1 定义，不包含业务实现。
package runtime

import (
	"context"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

type RuntimeSpec struct {
	SessionID           string
	Workspace           string
	MountMode           string
	AllowedTCP          []string
	ReadOnlyPaths       []string
	AllowChildProcesses bool // Kit 显式声明：受控子进程白名单（如 Codex）
	AllowDescendants    bool // Agent 会话：白名单 process-exec 内允许 fork + PTY ioctl；Health 仍强制 deny
}

type Handle struct {
	ID          string
	PrivateHome string
	Workspace   string // Runtime 实际挂载路径；clone 时是会话副本
}

type Exit struct {
	Code      int
	Cancelled bool
	Err       error
}

type Process interface {
	Wait() Exit // 返回前完成 stdout/stderr 读取与最终输出回调
}

// InteractiveProcess 是可在同一会话内继续接收输入的受控进程。
// Runtime 实现必须把写入限制在该会话的 stdin，不得执行 shell 拼接。
type InteractiveProcess interface {
	Process
	Write(context.Context, []byte) error
	Resize(cols, rows int) error
}

type Runtime interface {
	Health(context.Context) error
	Create(context.Context, RuntimeSpec) (Handle, error)
	Exec(context.Context, Handle, LaunchSpec, func(contracts.OutputData)) (Process, error)
	Destroy(context.Context, Handle) error
}

// HealthChecker 是支持按 Kit 能力选择隔离 profile 的 Runtime 扩展接口。
// 不实现时，编排层回退到 Runtime.Health 的默认严格检查。
type HealthChecker interface {
	HealthFor(context.Context, bool) error
}

type ConfigFile struct {
	RelativePath string // 只能位于 Runtime 私有 HOME 下
	Content      []byte // 只允许 Gate URL、Sentinel 等无上游凭据内容
}

type LaunchSpec struct {
	Command     string
	Args        []string
	Env         map[string]string
	Stdin       []byte
	Terminal    bool                         // 分配独立 PTY；与结构化消息编码互斥
	Interactive bool                         // 保持 stdin 打开，允许同一会话多轮输入
	EncodeInput func([]byte) ([]byte, error) // Kit 提供协议编码；Runtime 只负责受控写入
	ConfigFiles []ConfigFile
	// ProjectHome：非空时作为进程 HOME（须等于会话 Workspace）。Claude Code 会 chdir 到 homedir，
	// 若 HOME 仍是私有 home，Read 会落到 AgentHarbor/sessions/.../home 并 InputValidationError。
	ProjectHome string
}
