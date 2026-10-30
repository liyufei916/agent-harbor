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
| Docker Runtime（P2） | **进行中** | 切片 0–4 已在 `feat/p2-docker-runtime`；待逐端口防火墙与设置开关 UI |
| microVM | **backlog（后于 Docker）** | 不挡 P2 容器；预研另立 |
| 官方 MCP SDK | **backlog** | 现用 `net/http` 转发；待真实上游后再换 SDK |
| Linux Seatbelt | **N/A** | Runtime 非 Darwin → `ISOLATION_UNAVAILABLE`；无 Linux Seatbelt 目标 |

并行编码与文档纪律仍见 [GO-CHECKLIST](GO-CHECKLIST.md)。
