# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker API 兼容容器**（非 microVM）。分支：`feat/p2-docker-runtime`。

**引擎主路径（2026-10-30 确认）**：**B→C** — Harbor **托管 Colima + docker CLI**，用户无需安装 Docker Desktop；系统 Docker 仅高级兼容。见 [DECISIONS](DECISIONS.md)。

## 目标

Gate / Policy / 页面主契约不变，只换 `Runtime` 实现；引擎由 Harbor 或本机 Docker 提供 daemon。

| 原则 | 约定 |
|---|---|
| 默认 | Seatbelt（P1）；显式启用「容器隔离」才用 P2 |
| 引擎主路径 | Harbor 托管 Colima（B→C）；不逼装 Docker Desktop |
| 失败关闭 | 无引擎/daemon/镜像 → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | 不挂 `docker.sock` 进 Guest；`--cap-drop ALL`；uid 65534 |
| 网络 | `--internal` 网 `agent-harbor-p2`；Gate 仅 `harbor.gate`（host-gateway） |
| Gate 端口 | 每会话 AllowedTCP **本机 TCP 代理**；URL → `harbor.gate:<proxy>` |
| 配额 | 默认 2g / 2 CPU / 256 PIDs（环境变量非法则回落） |
| 二进制 | 镜像内 Claude / Codex / dsh；**一会话一容器** |

## Runtime 切片（0–16 · 底座已完成）

| # | 内容 | 状态 |
|---|---|---|
| 0–16 | Runtime / Exec / Terminal / Gate 代理 / 冒烟 / 孤儿清理 / 配额校验 / `docker-p2-up` 等 | **完成** |

## 引擎里程碑（B→C · 进行中）

| 里程碑 | 内容 | 状态 |
|---|---|---|
| **B0** | DECISIONS：Colima 主路径、目录、fail-closed、不做 Desktop 内嵌 | **本提交** |
| **M2** | 引擎 Ensure/Start/Stop/Health；钉扎下载或本地探测；构建 runtime 镜像 | 待做 |
| **M3** | 设置：引擎来源（Harbor / 本机 Docker）；`DOCKER_HOST` 接入 Runtime | 待做 |
| **M4** | 默认无感路径（C）；陌生机无 Desktop 证据 | 待做 |

开发期若本机已有 Docker，仍可用外置 daemon 验证 Runtime（`bash scripts/docker-p2-up.sh`），与 B→C 产品主路径不冲突。

## 残留风险

- **Docker Desktop / 任意 daemon**：Guest 仍可能经残留路径触及宿主机 loopback；`harbor.gate` + 拉黑别名降低面，不替代防火墙。  
- **Colima**：依赖 macOS 虚拟化；启停/升级/公证体积由引擎子系统负责。  
- Linux 生产网关规则后续另补。

## 启用（开发 · 外置 daemon）

```bash
bash scripts/docker-p2-up.sh   # 构建镜像 + 边界冒烟（需已有 docker）
# 设置 → 隔离后端 → Docker，重启应用
# 或：export HARBOR_RUNTIME=docker
```

产品主路径落地后：设置/向导「启用容器隔离」→ Harbor 拉起 Colima → 自动建镜像 → 切 P2（细节见 M3/M4）。

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `HARBOR_RUNTIME` | （空则看 UIPrefs） | `docker` / `seatbelt`；**优先于设置项** |
| `HARBOR_DOCKER_IMAGE` | `agent-harbor/runtime:dev` | |
| `HARBOR_DOCKER_MEMORY` / `_CPUS` / `_PIDS` | `2g` / `2` / `256` | 非法回落默认 |
| `DOCKER_HOST` | （引擎子系统设置） | 指向 Harbor Colima 或本机 Docker |

对照：[deploy/docker](../deploy/docker/README.md) · [CONTRACT](CONTRACT.md) · [ENG-STATUS](ENG-STATUS.md) · [DECISIONS](DECISIONS.md)。
