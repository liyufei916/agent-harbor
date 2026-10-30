//go:build darwin && harbor_platform

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/policy"
	"github.com/liyufei916/agent-harbor/internal/runtime"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--harbor-orchestrator-helper" {
		fmt.Print("真实隔离输出\n")
		switch os.Args[2] {
		case "wait":
			for {
				time.Sleep(time.Hour)
			}
		case "fail":
			os.Exit(7)
		default:
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

type realKit struct{ executable, mode string }

func (k realKit) Detect(context.Context) (kits.KitInfo, error) {
	return kits.KitInfo{Agent: contracts.AgentInfo{ID: "claude", Available: true}, ReadOnlyPaths: []string{k.executable}, NeedsDescendants: true}, nil
}
func (k realKit) Build(context.Context, kits.KitInput) (runtime.LaunchSpec, error) {
	return runtime.LaunchSpec{Command: k.executable, Args: []string{"--harbor-orchestrator-helper", k.mode}}, nil
}

type workspacePolicy struct{ workspace string }

func (p workspacePolicy) Build(_ context.Context, request policy.PolicyRequest) (policy.PolicySnapshot, error) {
	mounts := []policy.MCPMount{}
	for _, mcp := range request.MCPs {
		mounts = append(mounts, policy.MCPMount{AccountID: mcp.ID})
	}
	return policy.PolicySnapshot{Workspace: p.workspace, MountMode: "direct", MCPMounts: mounts}, nil
}

func TestOrchestratorWithRealIsolatedProcess(t *testing.T) {
	for _, mode := range []string{"success", "fail", "wait"} {
		t.Run(mode, func(t *testing.T) {
			workspace, err := os.MkdirTemp("/private/tmp", "harbor-orchestrator-integration-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(workspace)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			f := newFixture()
			o, err := New(context.Background(), Dependencies{Runtime: runtime.NewMacOS(), Kits: map[string]kits.Kit{"claude": realKit{executable: executable, mode: mode}}, Catalog: f, Policy: workspacePolicy{workspace}, Gate: f}, f.record, Timeouts{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := o.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			})
			s := create(t, o)
			if mode == "wait" {
				awaitState(t, o, s.SessionID, contracts.StateRunning)
				if _, err := o.Stop(context.Background(), s.SessionID); err != nil {
					t.Fatal(err)
				}
			}
			state := contracts.StateStopped
			if mode == "fail" {
				state = contracts.StateFailed
			}
			end := awaitState(t, o, s.SessionID, state)
			if !end.CleanupComplete {
				t.Fatal("真实 Runtime 未确认清理完成")
			}
			if mode == "fail" {
				assertCode(t, end.Error, "AGENT_EXIT_FAILED")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := o.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			var output strings.Builder
			terminal := uint64(0)
			for _, event := range f.events {
				if event.Output != nil {
					if terminal != 0 {
						t.Fatal("输出晚于终态")
					}
					output.WriteString(event.Output.Text)
				}
				if event.State != nil && event.State.CleanupComplete {
					terminal = event.Seq
				}
			}
			if mode != "wait" && !strings.Contains(output.String(), "真实隔离输出") {
				t.Fatal("真实进程最终输出未展示")
			}
		})
	}
}
