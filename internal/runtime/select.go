package runtime

import (
	"os"
	"strings"
)

// NewSelected 按 HARBOR_RUNTIME 选择隔离后端。
// 空 / seatbelt / p1 → NewDefault()（macOS Seatbelt）；docker / p2 / container → NewDocker()。
// 默认仍为 P1，避免未装 Docker 时桌面全红。
func NewSelected() Runtime {
	return NewSelectedWithBackend(os.Getenv("HARBOR_RUNTIME"))
}

// NewSelectedWithBackend chooses Runtime. Env HARBOR_RUNTIME wins when non-empty;
// otherwise backend comes from UIPrefs (seatbelt|docker|p1|p2|container|"").
func NewSelectedWithBackend(backend string) Runtime {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("HARBOR_RUNTIME")))
	if env != "" {
		backend = env
	}
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "docker", "p2", "container":
		selectedDocker.Store(true)
		return NewDocker()
	default:
		selectedDocker.Store(false)
		return NewDefault()
	}
}
