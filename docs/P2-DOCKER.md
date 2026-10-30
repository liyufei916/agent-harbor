# 二期容器 Runtime（Docker）

**选型（2026-10-30）**：二期隔离后端采用 **Docker API 兼容容器**（非 microVM）。**已合入 `main`**（源自 `feat/p2-docker-runtime` · merge `1a5db77`）。

**引擎主路径（2026-10-30 确认）**：**B→C** — Harbor **托管 Colima + docker CLI**，用户无需安装 Docker Desktop；系统 Docker 仅高级兼容。见 [DECISIONS](DECISIONS.md)。

## 目标

Gate / Policy / 页面主契约不变，只换 `Runtime` 实现；引擎由 Harbor 或本机 Docker 提供 daemon。

| 原则 | 约定 |
|---|---|
| 默认 | Seatbelt（P1）；设置隔离后端选 **Docker** 才用 P2 |
| 引擎主路径 | Harbor 托管 Colima（B→C）；不逼装 Docker Desktop |
| 失败关闭 | 无引擎/daemon/镜像 → `ISOLATION_UNAVAILABLE` |
| 禁逃逸面 | 不挂 `docker.sock` 进 Guest；`--cap-drop ALL`；uid 65534 |
| 网络 | `agent-harbor-p2`（**不用** `--internal`：Desktop 会连带阻断 host-gateway）；Gate 仅 `harbor.gate` |
| Gate 端口 | 每会话 AllowedTCP **本机 TCP 代理**；Env **与 ConfigFiles** 中 `127.0.0.1:<gate>` → `harbor.gate:<proxy>` |
| 配额 | 默认 2g / 2 CPU / 256 PIDs（环境变量非法则回落） |
| 二进制 | 镜像内 Claude / Codex / dsh；**一会话一容器** |

## Runtime 切片（0–16 · 底座已完成）

| # | 内容 | 状态 |
|---|---|---|
| 0–16 | Runtime / Exec / Terminal / Gate 代理 / 冒烟 / 孤儿清理 / 配额校验 / `docker-p2-up` 等 | **完成** |

## 引擎里程碑（B→C · 代码完成）

| 里程碑 | 内容 | 状态 |
|---|---|---|
| **B0** | DECISIONS：Colima 主路径、目录、fail-closed、不做 Desktop 内嵌 | **完成** |
| **M2a** | `internal/engine`：SystemDocker 适配 + Colima 占位（fail-closed） | **完成** |
| **M2b** | Colima Start/Stop/Health（PATH 或 `engine/bin`）；`EnsureImage`；不抢全局 docker context | **完成** |
| **M2c** | 钉扎下载 colima + lima + docker CLI（SHA256）；`HARBOR_ENGINE_AUTO_DOWNLOAD` | **完成** |
| **M3** | 设置：引擎来源 / Health / 启用；`DOCKER_HOST` 接入 Runtime | **完成** |
| **M4** | 切 Docker 后端自动 Ensure；启动时拉起；陌生机无 Desktop 勾选表 | **完成** |

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

产品主路径：设置 → 隔离后端选 **Docker（P2）** → 选引擎（Colima / 本机 Docker）→ 点 **「一键安装并启用」**（才下载；可「停止 / 取消」）→ **重启应用** → Health `isolationLevel=docker`。勾选见 [stranger-first-run-record](../demos/stranger-first-run-record.md) §P2 B。

**网络注意**：会话网 `agent-harbor-p2` **不得**使用 Docker `--internal`（Desktop 上会阻断 `harbor.gate` / host-gateway）。遗留 internal 网会在 Health/Ensure 时尝试迁移。

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `HARBOR_RUNTIME` | （空则看 UIPrefs） | `docker` / `seatbelt`；**优先于设置项** |
| `HARBOR_DOCKER_IMAGE` | `agent-harbor/runtime:dev` | |
| `HARBOR_DOCKER_MEMORY` / `_CPUS` / `_PIDS` | `2g` / `2` / `256` | 非法回落默认 |
| `DOCKER_HOST` | （引擎子系统设置） | 指向 Harbor Colima 或本机 Docker |
| `HARBOR_ENGINE_AUTO_DOWNLOAD` | 默认开启 | `0`/`false`/`off` 关闭钉扎下载 |

对照：[deploy/docker](../deploy/docker/README.md) · [CONTRACT](CONTRACT.md) · [ENG-STATUS](ENG-STATUS.md) · [DECISIONS](DECISIONS.md)。
