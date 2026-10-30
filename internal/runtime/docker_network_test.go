package runtime

import (
	"strings"
	"testing"
)

func TestAppendResourceLimitsDefaults(t *testing.T) {
	t.Setenv("HARBOR_DOCKER_MEMORY", "")
	t.Setenv("HARBOR_DOCKER_CPUS", "")
	t.Setenv("HARBOR_DOCKER_PIDS", "")
	args := appendResourceLimits(nil)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--memory 2g") || !strings.Contains(joined, "--cpus 2") || !strings.Contains(joined, "--pids-limit 256") {
		t.Fatal(joined)
	}
}

func TestAppendResourceLimitsOverride(t *testing.T) {
	t.Setenv("HARBOR_DOCKER_MEMORY", "512m")
	t.Setenv("HARBOR_DOCKER_CPUS", "1")
	t.Setenv("HARBOR_DOCKER_PIDS", "32")
	args := appendResourceLimits(nil)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--memory 512m") || !strings.Contains(joined, "--cpus 1") || !strings.Contains(joined, "--pids-limit 32") {
		t.Fatal(joined)
	}
}
