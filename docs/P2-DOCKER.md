# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker**（非 microVM）。microVM 仍为后续预研。分支：`feat/p2-docker-runtime`。

## 目标

在不动 Gate / Policy / 页面主契约的前提下，将 `Runtime` 从 Seatbelt（P1）可替换为「一会话一容器」。

| 原则 | 约定 |
|---|---|
| 默认 | 仍为 P1（`NewDefault` / Seatbelt）；`HARBOR_RUNTIME=docker\|p2\|container` 才启用 P2 |
| 失败关闭 | 无 `docker` CLI 或 daemon 不可达 → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | **不**挂载 `docker.sock`；`--cap-drop ALL`；`no-new-privileges`；默认 `--network none` |
| 密钥 | 仍不进容器；出站经宿主 Gate（网络打通为后续切片） |

## 切片

| # | 内容 | 状态 |
|---|---|---|
| 0 | 选型文档、`NewSelected`、Health / Create / Destroy 骨架；Exec 待 Agent 镜像 | **本分支起步** |
| 1 | Harbor Agent 镜像（固定 Node/CLI）+ `Exec` / Interactive | 待做 |
| 2 | 自定义网络：仅通宿主 Gate `AllowedTCP` | 待做 |
| 3 | 设置/部署开关与 UI 能力差提示；P1 回落 | 待做 |
| 4 | 逃逸回归脚本、CPU/内存配额、并发压测 | 待做 |

## 本机启用（开发）

```bash
# 需本机 Docker Desktop / OrbStack / 等同 daemon
export HARBOR_RUNTIME=docker
# 可选：export HARBOR_DOCKER_IMAGE=alpine:3.20
./scripts/dev-desktop.sh
```

当前 Create 使用占位镜像 `sleep infinity` 证明容器生命周期；**开会话跑 Agent 仍会 `AGENT_UNSUPPORTED`**，直至切片 1。

## 代码入口

| 符号 | 说明 |
|---|---|
| `runtime.NewSelected()` | 桌面 `wire` 装配 |
| `runtime.NewDocker()` | P2 实现 |
| `HARBOR_RUNTIME` | `docker` / `p2` / `container` |
| `HARBOR_DOCKER_IMAGE` | 覆盖默认 `alpine:3.20` |

对照：[IMPLEMENTATION §7](IMPLEMENTATION.md) · [DESIGN §7.4/§7.6](DESIGN.md) · [ENG-STATUS](ENG-STATUS.md)。
