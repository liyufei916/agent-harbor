# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker**（非 microVM）。分支：`feat/p2-docker-runtime`。

## 目标

Gate / Policy / 页面主契约不变，只换 `Runtime` 实现。

| 原则 | 约定 |
|---|---|
| 默认 | Seatbelt（P1）；设置「隔离后端」或 `HARBOR_RUNTIME=docker` 启用 P2 |
| 失败关闭 | 无 CLI/daemon/镜像 → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | 无 `docker.sock`；`--cap-drop ALL`；uid 65534 |
| 网络 | `--internal` 网 `agent-harbor-p2`；Gate 仅 `harbor.gate`（host-gateway） |
| Gate 端口 | 每会话 AllowedTCP **本机 TCP 代理**；URL 改写到 `harbor.gate:<proxy>`；`host.docker.internal` 拉黑到 127.0.0.1 |
| 配额 | 默认 2g / 2 CPU / 256 PIDs |
| 二进制 | 镜像内 Claude / Codex / dsh |

## 切片

| # | 内容 | 状态 |
|---|---|---|
| 0–4 | Runtime / Exec / 镜像 / Kit / internal 网 / 配额 / 冒烟 | **完成** |
| 5 | AllowedTCP 代理改写；设置页隔离后端（需重启） | **完成** |
| 6 | Health 校验镜像；`harbor.gate` + 拉黑 `host.docker.internal`；切换后端 toast | **完成** |
| 7 | Health 分步文案 + 探测超时；隔离 UI Docker 文案；冒烟校验 Gate 别名 | **完成** |
| 8 | `--init` 收割僵尸；陌生人记录补 Docker 路径 | **完成** |
| 9 | 容器 `agent-harbor.session` label；网络失败 Health 文案 | **完成** |
| 10 | Docker Terminal：宿主 PTY + `docker exec -it`；Resize | **完成** |
| 11 | 冒烟可选校验 `docker exec -t`（非 TTY CI 跳过） | **本提交** |

## 残留风险（Docker Desktop）

Desktop 仍可能经其它路径触及宿主机 loopback；拉黑 `host.docker.internal` 降低常见 SSRF 别名面，**不**等价于宿主机防火墙。Linux 生产可另加网关规则（后续）。

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
