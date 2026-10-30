# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker**（非 microVM）。分支：`feat/p2-docker-runtime`。

## 目标

Gate / Policy / 页面主契约不变，只换 `Runtime` 实现。

| 原则 | 约定 |
|---|---|
| 默认 | Seatbelt（P1）；设置「隔离后端」或 `HARBOR_RUNTIME=docker` 启用 P2 |
| 失败关闭 | 无 CLI/daemon → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | 无 `docker.sock`；`--cap-drop ALL`；uid 65534 |
| 网络 | `--internal` 网 `agent-harbor-p2` + `host.docker.internal` |
| Gate 端口 | 每会话为 AllowedTCP 建 **本机 TCP 代理**，环境变量改写到代理端口（见残留风险） |
| 配额 | 默认 2g / 2 CPU / 256 PIDs |
| 二进制 | 镜像内 Claude / Codex / dsh |

## 切片

| # | 内容 | 状态 |
|---|---|---|
| 0–4 | Runtime / Exec / 镜像 / Kit / internal 网 / 配额 / 冒烟 | **完成** |
| 5 | AllowedTCP 代理改写；设置页隔离后端（需重启） | **本提交** |

## 残留风险（Docker Desktop）

容器经 `host.docker.internal` 仍可能直连宿主机其它 loopback 端口。代理保证「走 Gate 的 URL」落到专用监听口；**不能**在 Desktop 上仅靠 iptables 封死直连。Linux 生产可另加网关防火墙（后续）。

## 启用

```bash
docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
# 设置 → 隔离后端 → Docker，重启应用
# 或：export HARBOR_RUNTIME=docker
bash scripts/docker-p2-smoke.sh
```

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `HARBOR_RUNTIME` | （空则看 UIPrefs） | `docker` / `seatbelt`；**优先于设置项** |
| `HARBOR_DOCKER_IMAGE` | `agent-harbor/runtime:dev` | |
| `HARBOR_DOCKER_MEMORY` / `_CPUS` / `_PIDS` | `2g` / `2` / `256` | |

对照：[deploy/docker](../deploy/docker/README.md) · [CONTRACT UIPrefs](CONTRACT.md) · [ENG-STATUS](ENG-STATUS.md)。
