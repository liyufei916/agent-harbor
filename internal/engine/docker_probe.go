package engine

import (
	"context"
	"strings"
)

// dockerInfoVersion runs `docker info --format {{.ServerVersion}}` and requires a
// non-empty version. Some pinned docker CLI builds return exit 0 with empty stdout
// when the daemon socket is missing (error only on stderr) — treating that as healthy
// caused false "引擎就绪" and later "Harbor Docker 网络不可用".
func dockerInfoVersion(ctx context.Context, run func(context.Context, string, []string, []string) (string, error), dockerBin string, env []string) (string, error) {
	out, err := run(ctx, dockerBin, []string{"info", "--format", "{{.ServerVersion}}"}, env)
	ver := strings.TrimSpace(out)
	if err != nil {
		return "", err
	}
	if ver == "" {
		return "", errEngine("Docker daemon 不可用（无 ServerVersion）。")
	}
	return ver, nil
}
