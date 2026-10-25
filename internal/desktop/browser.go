package desktop

import (
	"os/exec"
	"runtime"
)

func openBrowser(raw string) {
	if runtime.GOOS != "darwin" || raw == "" {
		return
	}
	cmd := exec.Command("/usr/bin/open", raw)
	_ = cmd.Start()
}
