package runtime

import (
	"os"
	"strings"
)

// DockerEnabled reports whether the process selected the P2 Docker Runtime.
func DockerEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HARBOR_RUNTIME"))) {
	case "docker", "p2", "container":
		return true
	default:
		return false
	}
}

// DockerImage returns HARBOR_DOCKER_IMAGE or DefaultDockerImage.
func DockerImage() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_DOCKER_IMAGE")); v != "" {
		return v
	}
	return DefaultDockerImage
}

// IsGuestImagePath is true for paths that live inside the Harbor guest image
// (not bind-mounted from the host).
func IsGuestImagePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	prefixes := []string{
		"/opt/harbor/",
		"/usr/local/bin/",
		"/usr/bin/",
		"/bin/",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) || path == strings.TrimSuffix(p, "/") {
			return true
		}
	}
	return false
}
