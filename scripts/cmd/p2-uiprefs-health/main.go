// 一次性证据：无 HARBOR_RUNTIME 时，经 UIPrefs isolationBackend=docker 走 GetRuntimeHealth。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/liyufei916/agent-harbor/internal/desktop"
)

func main() {
	if v := os.Getenv("HARBOR_RUNTIME"); v != "" {
		fmt.Fprintf(os.Stderr, "FAIL: HARBOR_RUNTIME unexpectedly set to %q (want unset for UIPrefs path)\n", v)
		os.Exit(2)
	}
	fmt.Printf("HARBOR_RUNTIME=<unset> DOCKER_HOST=%q\n", os.Getenv("DOCKER_HOST"))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	host := desktop.NewDesktopApp()
	api := desktop.NewDesktopAPI(host)
	before := api.GetUIPrefs()
	b, _ := json.MarshalIndent(before.Prefs, "", "  ")
	fmt.Printf("GetUIPrefs:\n%s\n", b)
	if err := host.AttachOrchestrator(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "AttachOrchestrator: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		sh, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = host.Shutdown(sh)
	}()
	h := api.GetRuntimeHealth()
	out, _ := json.MarshalIndent(h, "", "  ")
	fmt.Printf("GetRuntimeHealth:\n%s\n", out)
	eng := api.GetContainerEngineHealth()
	eout, _ := json.MarshalIndent(eng, "", "  ")
	fmt.Printf("GetContainerEngineHealth:\n%s\n", eout)
	if !h.Available || h.IsolationLevel != "docker" {
		fmt.Fprintf(os.Stderr, "FAIL: want available docker, got available=%v level=%q reason=%q\n", h.Available, h.IsolationLevel, h.Reason)
		os.Exit(1)
	}
	fmt.Println("PASS: UIPrefs path → isolationLevel=docker (no HARBOR_RUNTIME)")
}
