# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker**（非 microVM）。microVM 仍为后续预研。分支：`feat/p2-docker-runtime`。

## 目标

在不动 Gate / Policy / 页面主契约的前提下，将 `Runtime` 从 Seatbelt（P1）可替换为「一会话一容器」。

| 原则 | 约定 |
|---|---|
| 默认 | 仍为 P1（`NewDefault` / Seatbelt）；`HARBOR_RUNTIME=docker\|p2\|container` 才启用 P2 |
| 失败关闭 | 无 `docker` CLI 或 daemon 不可达 → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | **不**挂载 `docker.sock`；`--cap-drop ALL`；`no-new-privileges` |
| 网络 | bridge + `host.docker.internal:host-gateway`；Gate `127.0.0.1:<AllowedTCP>` 改写为该主机名（探测用 `--network none`） |
| 密钥 | 仍不进容器；出站经宿主 Gate |
| 二进制 | **Linux 镜像内** Claude / Codex / dsh；宿主机 macOS CLI 不 bind 执行 |

## 切片

| # | 内容 | 状态 |
|---|---|---|
| 0 | 选型、`NewSelected`、Health / Create / Destroy | **完成** |
| 1 | `Exec` / Interactive、路径映射、开发镜像骨架 | **完成** |
| 2 | 更紧网络（仅 AllowedTCP / 自定义 bridge） | 待做（当前：整桥 + host-gateway） |
| 3 | Kit Detect/Build 镜像内路径；Dockerfile 安装 CLI+dsh | **本提交** |
| 4 | 逃逸回归、CPU/内存配额、并发压测；设置项 UI | 待做 |

## 本机启用

```bash
docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
export HARBOR_RUNTIME=docker
./scripts/dev-desktop.sh
```

`HARBOR_RUNTIME=docker` 时：

- Detect 经 `docker run --rm` 探测镜像内 `/usr/local/bin/{claude,codex,node}` 与 `/opt/harbor/dsh-0.2.1-rc.2`
- Create **不**把镜像路径 bind 到宿主机；Exec 直接 `docker exec` 镜像内命令

## 代码入口

| 符号 | 说明 |
|---|---|
| `runtime.NewSelected()` / `DockerEnabled()` | 桌面装配 / Kit 分支 |
| `runtime.ProbeDockerGuest` | Kit Detect 用 |
| `kits.GuestClaude` 等 | 镜像内固定路径 |
| `HARBOR_DOCKER_IMAGE` | 默认 `agent-harbor/runtime:dev` |

对照：[deploy/docker](../deploy/docker/README.md) · [IMPLEMENTATION §7](IMPLEMENTATION.md) · [ENG-STATUS](ENG-STATUS.md)。
