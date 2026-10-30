package runtime

import (
	"os"
	"strings"
)

// NewSelected 按 HARBOR_RUNTIME 选择隔离后端。
// 空 / seatbelt / p1 → NewDefault()（macOS Seatbelt）；docker / p2 / container → NewDocker()。
// 默认仍为 P1，避免未装 Docker 时桌面全红。
func NewSelected() Runtime {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HARBOR_RUNTIME"))) {
	case "docker", "p2", "container":
		return NewDocker()
	default:
		return NewDefault()
	}
}
