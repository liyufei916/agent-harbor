# 工程完成度（eng）vs 仍阻塞项

| 日期 | 口径 |
|---|---|
| 2026-10-30 | **eng 代码面已齐**；正式关单仍靠 [ACCEPTANCE](ACCEPTANCE.md) live 证据。本页不把单测/mock 升级为 I1–I11 通过。 |
| 2026-10-30b | **可复验交付补丁**：修 `gofmt` / 非法 `release.yml`（HEREDOC 顶格）；CI Artifact 附 SHA256；`SaveTextFile`/`PromptPermission`/`SelectWorkspace` 断言；根目录 MIT LICENSE。绿 CI 与可下载产物以本提交后 Actions 为准。 |
| 2026-10-30c | **可落地优化**：MCP 兼容矩阵；向导第 1 步 CLI 前检；ports 声明 PermissionAsker；MCP 协议接受 2024-11-05/2025-06-18；Gate usage 旁路扫描；dsh Detect stub 单测；失败 Top→改进表。 |
| 2026-10-30d | **UX/Detect**：扩权 toast 带主机；SyncBack 成功访达显示源目录；用量空态缩短；dsh 默认安装路径 `ResolvePaths`（无 `HARBOR_DSH_*` 亦可 Detect）；工作台 `stream-display` 解析 dsh `ready`/`text-delta`/`final`；亮点 mock 截图 `demos/screenshots/`。 |
| 2026-10-30e | **P2 起步**：确认 Docker（非 microVM）；分支 `feat/p2-docker-runtime`；`NewSelected` + `Docker` Health/Create/Destroy；Exec 待 Agent 镜像。见 [P2-DOCKER](P2-DOCKER.md)。 |
| 2026-10-30f | **P2 切片 1**：`Exec`/Interactive、guest 路径与 Gate `host.docker.internal` 改写；`deploy/docker` 开发镜像（Node 20 alpine）。 |
| 2026-10-30g | **P2 切片 3**：Kit Docker Detect/Build 用镜像路径；Dockerfile 装 Claude/Codex/dsh + Node 26.12.1；guest-only ReadOnly 不 bind。 |
| 2026-10-30h | **P2 切片 2+4**：`agent-harbor-p2` internal 网；内存/CPU/PIDs 配额；`docker-p2-smoke.sh`；Health `isolationLevel=docker`。 |
| 2026-10-30i | **P2 切片 5**：AllowedTCP 本机代理改写；`UIPrefs.isolationBackend` + 设置页；`HARBOR_RUNTIME` 仍优先。 |
| 2026-10-30j | **P2 切片 6**：Health 校验镜像；Gate 仅 `harbor.gate`；拉黑 `host.docker.internal`；切换隔离后端 toast 提示重启。 |
| 2026-10-30k | **P2 切片 7**：Docker Health 分步失败文案与 25s 探测超时；隔离检查 Docker 文案；冒烟校验 `harbor.gate` / 拉黑别名。 |
| 2026-10-30l | **P2 切片 8**：容器 `--init`；陌生人首跑记录补 Docker 勾选路径。 |
| 2026-10-30m | **P2 切片 9**：容器 session label；网络不可用 Health 文案单测。 |
| 2026-10-30n | **P2 切片 10**：Docker Terminal（宿主 PTY + `docker exec -it`）与 Resize；Agent 仍走 Interactive JSONL。 |
| 2026-10-30o | **P2 切片 11**：冒烟脚本在 TTY 环境下校验 `docker exec -t`。 |
| 2026-10-30p | **P2 切片 12**：浏览器 mock Health 随 `isolationBackend=docker`；CONTRACT 注记 Docker Terminal。 |
| 2026-10-30q | **P2 切片 13**：`ProbeDockerGuest` 先 `image inspect`；镜像/daemon 失败文案可辨。 |
| 2026-10-30r | **P2 切片 14**：Health 清理 `label=agent-harbor=1` 且已退出的孤儿容器；设置「关于」显示隔离后端偏好。 |
| 2026-10-30s | **P2 切片 15**：`HARBOR_DOCKER_*` 配额格式校验；设置保存后刷新关于栏后端文案。 |
| 2026-10-30t | **P2 切片 16**：`scripts/docker-p2-up.sh` 一键构建+冒烟；目录结构注明 docker*.go。 |
| 2026-10-30u | **P2 引擎 B0**：确认主路径 **B→C**（托管 Colima，不逼装 Desktop）；DECISIONS + `internal/engine` 接口骨架。 |
| 2026-10-30v | **P2 引擎 M2a**：SystemDocker 适配器 + Colima 占位（未实现前 fail-closed）；默认选型 Colima。 |
| 2026-10-30w | **P2 引擎 M2b**：Colima profile `agent-harbor` 启停/Health/`DOCKER_HOST`；`EnsureImage`；`--activate=false`。 |
| 2026-10-30x | **P2 引擎 M2c**：钉扎下载 colima/lima/docker（校验 SHA256）；默认自动下载，可用 `HARBOR_ENGINE_AUTO_DOWNLOAD=0` 关闭。 |
| 2026-10-30y | **P2 引擎 M3**：`Get/EnsureContainerEngine` + 设置页来源/检测/启用；`DOCKER_HOST` 在 Attach/Health 接入 Runtime。 |
| 2026-10-30z | **P2 引擎 M4**：切容器隔离自动 Ensure；Attach 自动拉起；陌生机无 Desktop 勾选表。 |
| 2026-10-30aa | **P2 引擎安装 UX**：探测无 Docker Desktop；一键安装；`harbor:engine-progress` 百分比。 |

## eng-complete（代码已接线）

| 项 | 说明 |
|---|---|
| DesktopAPI / 契约面 | CONTRACT v1.2 方法齐全；mock + live 绑定 |
| Claude 多轮 | `--input-format stream-json` + EncodeInput |
| dsh 多轮（桌面） | `dsh.NewConversation` 固定受信会话驱动；默认路径自动 Detect；对话区解析 conversation JSONL |
| Codex Kit | 已注册；`app-server` + turn bridge 多轮 |
| clone / SyncBack | Runtime 副本 + 导回 + Destroy 清理；桌面导回成功后访达显示源目录 |
| PTY / Resize | WritePty；Terminal 模式 `pty.Setsize` |
| SwitchModel | 同 profileId + Host；工作台仅列兼容账号 |
| OAuth / stdio MCP | 回环 OAuth、宿主 stdio 桥 |
| Keychain | Darwin Keychain fail-closed；`vaultNotice` |
| 浅色主题 | `UIPrefs.theme` system/light/dark |
| 安全档位种子 | `UIPrefs.securityTier` → CreateSession ExtraHosts |
| 记录/诊断/扩权 UX | 导出 JSON、诊断 Tab、Modal 三问+超时拒绝、出舱审查、运行报告、教程进度 |
| Seatbelt（macOS） | P1 主路径；Health fail-closed |
| Gate 非 2xx 抹 Key 片段 | 错误响应额外抹前 8/末 4 位 |

## 仍 blocked / backlog（非「忘做 eng」）

| 项 | 类别 | 说明 |
|---|---|---|
| I1–I11 / B6 live | **通过（2026-10-30 全量重跑）** | harness + 平台探针 + `bin/AgentHarbor.app` 启停；见 ACCEPTANCE |
| 扩权 302 异常项（可选） | **backlog** | 主路径改为 `PromptPermission` ask + 白名单；302 不挡验收 |
| Apple 公证 / 分发签名 | **blocked（账号）** | 需 Apple Developer；清单见 [`scripts/notarize-prep.md`](../scripts/notarize-prep.md) |
| Docker Runtime（P2） | **Runtime + 引擎 B→C 代码面齐** | Runtime 0–16；引擎 **M2–M4**；正式关单仍靠陌生机 live 证据 |
| microVM | **backlog（后于 Docker）** | 不挡 P2 容器；预研另立 |
| 官方 MCP SDK | **backlog** | 现用 `net/http` 转发；待真实上游后再换 SDK |
| Linux Seatbelt | **N/A** | Runtime 非 Darwin → `ISOLATION_UNAVAILABLE`；无 Linux Seatbelt 目标 |

并行编码与文档纪律仍见 [GO-CHECKLIST](GO-CHECKLIST.md)。
