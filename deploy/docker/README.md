# Agent Harbor P2 guest image

```bash
docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
export HARBOR_RUNTIME=docker
export HARBOR_DOCKER_IMAGE=agent-harbor/runtime:dev
./scripts/dev-desktop.sh
```

## 镜像内容

| 路径 | 用途 |
|---|---|
| `/usr/local/bin/node` | Node **v26.12.1**（匹配 dsh） |
| `/usr/local/bin/claude` | `@anthropic-ai/claude-code`（npm -g） |
| `/usr/local/bin/codex` | `@openai/codex`（npm -g） |
| `/opt/harbor/dsh-0.2.1-rc.2/` | 固定版 `@deepseek-ai/dsh@0.2.1-rc.2` |

- `--user 65534`、无 `docker.sock`
- Kit 在 `HARBOR_RUNTIME=docker` 时 Detect/Build 使用上述**镜像内路径**，经 `docker run --rm` 探测
- Gate：`127.0.0.1:<port>` → 本机代理 → `harbor.gate:<proxyPort>`（`host.docker.internal` 拉黑到 127.0.0.1）
- Terminal：宿主 PTY + `docker exec -it`（与 Interactive JSONL 互斥）

构建需联网拉取 npm 包，体积较大；首次失败时检查 npm registry 与 Node 小版本 tag `26.12.1`。

运行时网络：`agent-harbor-p2`（`--internal`）+ `harbor.gate`（host-gateway）。边界冒烟：`bash scripts/docker-p2-smoke.sh`。
