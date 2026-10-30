# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker**（非 microVM）。microVM 仍为后续预研。分支：`feat/p2-docker-runtime`。

## 目标

在不动 Gate / Policy / 页面主契约的前提下，将 `Runtime` 从 Seatbelt（P1）可替换为「一会话一容器」。

| 原则 | 约定 |
|---|---|
| 默认 | 仍为 P1；`HARBOR_RUNTIME=docker\|p2\|container` 才启用 P2 |
| 失败关闭 | 无 CLI/daemon → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | **不**挂 `docker.sock`；`--cap-drop ALL`；`no-new-privileges`；uid 65534 |
| 网络 | **`--internal` 网** `agent-harbor-p2`（无公网伪装）+ `host.docker.internal:host-gateway`；Gate `127.0.0.1` 改写 |
| 配额 | 默认 `--memory 2g` `--cpus 2` `--pids-limit 256`（`HARBOR_DOCKER_MEMORY` / `_CPUS` / `_PIDS`） |
| 二进制 | Linux 镜像内 Claude / Codex / dsh |

## 切片

| # | 内容 | 状态 |
|---|---|---|
| 0 | 选型、`NewSelected`、Health / Create / Destroy | **完成** |
| 1 | `Exec` / 路径映射 / 开发镜像骨架 | **完成** |
| 2 | internal 网 + host-gateway；资源配额 | **完成** |
| 3 | Kit 镜像路径；Dockerfile 装 CLI+dsh | **完成** |
| 4 | 逃逸冒烟脚本；Health `isolationLevel=docker` | **本提交** |
| 5 | 逐端口 AllowedTCP 防火墙；设置页开关 UI | 待做 |

## 本机启用

```bash
docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
export HARBOR_RUNTIME=docker
./scripts/dev-desktop.sh
# 边界冒烟（可选）
bash scripts/docker-p2-smoke.sh
```

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `HARBOR_RUNTIME` | （空=Seatbelt） | `docker` / `p2` / `container` |
| `HARBOR_DOCKER_IMAGE` | `agent-harbor/runtime:dev` | 客户镜像 |
| `HARBOR_DOCKER_MEMORY` | `2g` | 容器内存 |
| `HARBOR_DOCKER_CPUS` | `2` | CPU |
| `HARBOR_DOCKER_PIDS` | `256` | 进程数上限 |

对照：[deploy/docker](../deploy/docker/README.md) · [scripts/docker-p2-smoke.sh](../scripts/docker-p2-smoke.sh) · [ENG-STATUS](ENG-STATUS.md)。
