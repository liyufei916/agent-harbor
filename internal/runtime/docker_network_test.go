package runtime

import "testing"

func TestDockerResourceLimitSanitization(t *testing.T) {
	t.Setenv("HARBOR_DOCKER_MEMORY", "2g --privileged")
	if got := dockerMemoryLimit(); got != "2g" {
		t.Fatalf("invalid memory must fall back, got %q", got)
	}
	t.Setenv("HARBOR_DOCKER_MEMORY", "512m")
	if got := dockerMemoryLimit(); got != "512m" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("HARBOR_DOCKER_CPUS", "-1")
	if got := dockerCPULimit(); got != "2" {
		t.Fatalf("invalid cpus must fall back, got %q", got)
	}
	t.Setenv("HARBOR_DOCKER_CPUS", "1.5")
	if got := dockerCPULimit(); got != "1.5" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("HARBOR_DOCKER_PIDS", "2")
	if got := dockerPidsLimit(); got != "256" {
		t.Fatalf("too-small pids must fall back, got %q", got)
	}
	t.Setenv("HARBOR_DOCKER_PIDS", "512")
	if got := dockerPidsLimit(); got != "512" {
		t.Fatalf("got %q", got)
	}
}
