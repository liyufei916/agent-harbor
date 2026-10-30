//go:build !darwin

package runtime

import (
	"context"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

// NewDefault 返回当前平台默认 Runtime。非 Darwin 无 Seatbelt 实现，一律不可用。
func NewDefault() Runtime {
	return unavailableRuntime{}
}

type unavailableRuntime struct{}

func (unavailableRuntime) Health(context.Context) error {
	return runtimeError("ISOLATION_UNAVAILABLE")
}

func (unavailableRuntime) Create(context.Context, RuntimeSpec) (Handle, error) {
	return Handle{}, runtimeError("ISOLATION_UNAVAILABLE")
}

func (unavailableRuntime) Exec(context.Context, Handle, LaunchSpec, func(contracts.OutputData)) (Process, error) {
	return nil, runtimeError("ISOLATION_UNAVAILABLE")
}

func (unavailableRuntime) Destroy(context.Context, Handle) error {
	return runtimeError("ISOLATION_UNAVAILABLE")
}
