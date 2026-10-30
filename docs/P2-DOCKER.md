# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker**（非 microVM）。microVM 仍为后续预研。分支：`feat/p2-docker-runtime`。

## 目标

在不动 Gate / Policy / 页面主契约的前提下，将 `Runtime` 从 Seatbelt（P1）可替换为「一会话一容器」。

| 原则 | 约定 |
|---|---|
| 默认 | 仍为 P1（`NewDefault` / Seatbelt）；`HARBOR_RUNTIME=docker\|p2\|container` 才启用 P2 |
| 失败关闭 | 无 `docker` CLI 或 daemon 不可达 → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | **不**挂载 `docker.sock`；`--cap-drop ALL`；`no-new-privileges` |
| 网络 | bridge + `host.docker.internal:host-gateway`；环境变量里 Gate 的 `127.0.0.1:<AllowedTCP>` 改写为该主机名 |
| 密钥 | 仍不进容器；出站经宿主 Gate |
| 二进制 | 容器是 **Linux**；宿主机 macOS CLI **不能**直接 bind 执行，须镜像内 Linux Agent |

## 切片

| # | 内容 | 状态 |
|---|---|---|
| 0 | 选型、`NewSelected`、Health / Create / Destroy | **完成** |
| 1 | `Exec` / Interactive、路径映射、开发镜像 `agent-harbor/runtime:dev` | **本提交** |
| 2 | 更紧网络（仅 AllowedTCP，非整网关）；可选自定义 bridge | 待做 |
| 3 | Kit Detect 在 docker 模式下返回镜像内路径；设置项 / UI 能力差 | 待做 |
| 4 | 逃逸回归、CPU/内存配额、并发压测 | 待做 |

## 本机启用（开发）

```bash
docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
export HARBOR_RUNTIME=docker
export HARBOR_DOCKER_IMAGE=agent-harbor/runtime:dev   # 默认即此名
./scripts/dev-desktop.sh
```

`Exec` 已支持镜像内 `/bin/*`、`/usr/bin/*`、`/usr/local/bin/*`（如 Node）及 bind-mount 的 **Linux** 工具。完整 Claude/Codex/dsh 会话仍待切片 3（镜像固化 CLI + Kit 路径）。

## 代码入口

| 符号 | 说明 |
|---|---|
| `runtime.NewSelected()` | 桌面 `wire` 装配 |
| `runtime.NewDocker()` | P2 实现 |
| `HARBOR_RUNTIME` | `docker` / `p2` / `container` |
| `HARBOR_DOCKER_IMAGE` | 默认 `agent-harbor/runtime:dev` |

对照：[IMPLEMENTATION §7](IMPLEMENTATION.md) · [DESIGN §7.4/§7.6](DESIGN.md) · [ENG-STATUS](ENG-STATUS.md) · [deploy/docker](../deploy/docker/README.md)。
