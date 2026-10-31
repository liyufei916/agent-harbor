# 工程完成度（eng）vs 仍阻塞项

| 日期 | 口径 |
|---|---|
| 2026-10-30 | **eng 代码面已齐**；正式关单仍靠 [ACCEPTANCE](ACCEPTANCE.md) live 证据。本页不把单测/mock 升级为 I1–I11 通过。 |
| 2026-10-30b | **可复验交付补丁**：修 `gofmt` / 非法 `release.yml`（HEREDOC 顶格）；CI Artifact 附 SHA256；`SaveTextFile`/`PromptPermission`/`SelectWorkspace` 断言；根目录 MIT LICENSE。绿 CI 与可下载产物以本提交后 Actions 为准。 |
| 2026-10-30c | **可落地优化**：MCP 兼容矩阵；向导第 1 步 CLI 前检；ports 声明 PermissionAsker；MCP 协议接受 2024-11-05/2025-06-18；Gate usage 旁路扫描；dsh Detect stub 单测。 |
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
| 2026-10-30z | **P2 引擎 M4**：切 Docker 后端自动 Ensure；Attach 自动拉起；陌生机无 Desktop 勾选表。 |
| 2026-10-30aa | **P2 引擎安装 UX**：探测无 Docker Desktop；一键安装；`harbor:engine-progress` 百分比。 |
| 2026-10-30ab | **预置 MCP 首批**：本机时间 / 演示 filesystem / Context7；`ListMcpPresets`+`AddMcpPreset`；工具页一键、向导勾选已连账号。 |
| 2026-10-30ac | **一键重启**：`RestartApp`；设置/标题栏在隔离偏好待生效时显示「重启应用」。 |
| 2026-10-30ad | **P2 Gate ConfigFiles**：Docker Exec 写 Kit 配置前改写 `127.0.0.1`→`harbor.gate`（修 Codex MCP `Transport channel closed`）。 |
| 2026-10-30ae | **P2 网关可达**：去掉 `--internal`（Desktop 阻断 harbor.gate）；Claude CLI 用 wireModel，Gate 改写账号模型。 |
| 2026-10-30af | **文档/文案对齐**：使用说明、README 亮点、PAGE-DESIGN、FIRST-RUN/stranger、阻断与引导「Docker」用语；演示路径 B 标明仅 mock。 |
| 2026-10-30ag | **P2 资产落地（不合 main）**：mock 截图 13–16；`docker-p2-smoke` 证据入 `demos/evidence/`；stranger §P2 A 冒烟勾选。 |
| 2026-10-30ah | **P2 live 证据**：`HARBOR_RUNTIME=docker` 下 claude/codex/dsh marker 全绿（`p2-docker-agents-*.txt`）；Codex 关 plugins（防 Gate 外 git fetch）；harness 任务去掉 demos/workspace 相对路径歧义。 |
| 2026-10-30ai | **P2 §B Colima**：`COLIMA_HOME` 改 `~/Library/Caches/AgentHarbor/colima`（UNIX_PATH_MAX）；`engine/share`→`lima/share` guestagent；Ensure/Health 证据 `p2-colima-ensure-*.txt`（Desktop 已安装但未用）。 |
| 2026-10-30aj | **扩权演示收敛**：Gate `tools/list` 注入 `harbor_demo_ask`；工作区 `EXPAND-TASK`/`BLOCK-TASK`；删不可用 shell 触发脚本与冗余 demos md。 |
| 2026-10-30ak | **P2 桌面 GUI Docker live**：`bin/AgentHarbor.app` 无 `HARBOR_RUNTIME`；顶栏 Docker + 侧栏隔离就绪截图 `17-desktop-docker-health.png`；UIPrefs `GetRuntimeHealth` 证据 `p2-docker-gui-*.txt`；stranger §P2 A GUI 勾选。 |
| 2026-10-30al | **陌生人首跑表**：步骤 1–9 + Agent 矩阵 + §P2 A/B（除「陌生机无 Desktop GUI」）按 Release/live 证据勾选；公证/302/MCP SDK/microVM 本轮不推进。 |
| 2026-10-30am | **代填自检收紧**：步骤 1–2/Gatekeeper/扩权 GUI/§B「无 Desktop」标明代填或 API 口径，避免冒充陌生机重下或 GUI 补镜。 |
| 2026-10-30an | **`feat/p2-docker-runtime` → `main`**：`1a5db77`；文档去掉「不合 main」口径。 |
| 2026-10-31 | **评审 A 包**：gofmt + RestartApp race；`harbor_demo_ask` 默认关（UIPrefs/`HARBOR_DEMO_ASK`）；持久化口径注释；MCP「机制+清单」文案。 |
| 2026-10-31b | **评审 B 包**：非 context7 远程 MCP 证据 + 矩阵；模型页协议→Agent 显式勾选；`gatekeeper-verify.sh`；SyncBack sha256 旁证。 |
| 2026-10-31c | **双终端 + http-live**：浏览器与桌面同前端；loopback 桥默认同端口挂 dist；UI 不挂旁证恐吓横幅。 |
| 2026-10-31d | **http-live 复验闭环**：`harbor_demo_blocked` 始终注入；撤销清 Gate extraHosts + 三处记住表同步；向导 `WORKSPACE_BUSY` 早检；live-browser 样张 + EXPAND/BLOCK GUI 通过（permission-live-test 用例 4–5）。 |
| 2026-10-31e | **口径+记录**：CONTRACT/注释厘清「本项目」= 落盘跨会话 vs Gate `extraHosts` 会话热路径；记录页补按会话/工具分组与失败原因分布；前端测对齐 Docker 落点与 SESSION_LIMIT=5。 |
| 2026-10-31f | **评测 P0**：文档/README 会话上限统一 5；mock 用 `MAX_ACTIVE_SESSIONS`；`harbor_demo_blocked` 并入演示开关；http-live 仅 Authorization + `/v1/dev-token`；`connectStdioMCP` 单测。 |
| 2026-10-31g | **评测 P1 材料**：main 绿 CI `41187174465`（`3d0ded4`）`go-unit-coverage` 旁证入 demos/evidence；UIPrefs 引擎/演示开关测；工具页预置渲染测；http-live token 剥离单测。 |
| 2026-10-31h | **CI 拆分**：`Frontend tests` 独立 job（npm test 失败直达）；`go` needs frontend 再打包；format 比对 Go/JS 会话上限；coverage-badge 强制存在 `coverage-source-sha.txt`。 |
| 2026-10-31i | **首跑口径**：`stranger-first-run-record` 勾选保留为**本机/harness**；显式「独立陌生机未做·不声称」；FIRST-RUN/ACCEPTANCE/README 同步；dsh Seatbelt 短任务证据入库。 |
| 2026-10-31j | **扩权默认保留**：StartSession 与 Create 同源回灌落盘 project+档位；Resolve 写当前 Gate；换引擎/重启不清记住表（CONTRACT + 设置/指南文案）。 |
| 2026-10-31k | **评委 P0**：MCP 服务器名按 accountId 稳定派生；`No such tool available` 中文化+断言；README 标明 Release vs CI Artifact；设置页写明 Agent 能力边界；发 **v0.1.1**。 |
| 2026-10-31l | **MCP 长 path 转发**：Gate 按挂载 `Paths[0]` 上游（修 mcpcn 等 `…/mcp/…/uuid` 被错打成 `/mcp`，Claude/dsh 均挂不上工具）。 |
| 2026-10-31m | **通用远程 MCP 适配 live**：非 Context7 公网 mcpcn ×2（汇率 `exchange_rate_convert`、油价 `oil_price_query`）Codex `tools/call` 通过；证据 `demos/evidence/mcp-generic-external-2026-10-31.txt` + 矩阵。 |
| 2026-10-31n | **MCP OAuth 真 IdP**：回环 http 允许本机 IdP；`mcp-oauth-evidence`（authorize→code→token + DCR）；账号 `authMode=oauth`；证据 `demos/evidence/mcp-oauth-idp-2026-10-31.txt`；发 **v0.1.2**。 |
| 2026-10-31o | **Codex CLI 0.165**：去掉已移除的 `plugin_discovery_prefetch`（Unknown feature flag → AGENT_EXIT_FAILED）。 |
| 2026-10-31p | **记录·用量四子 Tab**：本机汇总（含单价）/ 按会话 / 按工具 / 近期会话终态；PAGE-DESIGN / README / 使用说明同步。 |
| 2026-10-31q | **用量按模型**：Audit 带 `model`；费用按单价名归集（deepseek-chat 等）；旧事件主机启发；汇总展示按模型 token/粗算。 |
| 2026-10-31r | **计量 Tab**：用量旁新增计量；数量/价格分 tab + 分模型时序（默认近 1 天）；参考单价迁入并可折叠；按会话每页 10 条。 |
| 2026-10-31s | **记录样张重截**：`06`/`06b`/`06c` + live `04`/`04b`；`records-sample-shots.sh`；`?shotSeed=1` + `tab`/`usageTab`/`meterTab` 深链。 |
| 2026-10-31t | **计量/单价 UX**：参考单价一模型一套（已配的不进下拉）；币种改按钮（修桌面 WebView select）；分模型时序悬停明细；无 token 不出该模型时序；发 **v0.1.3**。 |
| 2026-11-01 | **评审闭环**：`gofmt` 修 `events.go` Result 对齐；计量 UX 抽 `records-price.js` + 单测（隐藏已配模型/币种锁/悬停 tip）；`release.yml` 增 gofmt + `go test -race` + `npm test` 发版门禁；CI `go-unit` needs `format`。视频顶栏「…UNIMPLEMENTED」= api fail-closed 占位码拼文案（`frontend/src/api.js`），非字面隔离功能名。 |
| 2026-11-01b | **CI 分流**：format/frontend 改 ubuntu；Go race/覆盖率/打包仍 macOS（Darwin API）。ubuntu 全量 `go test` 会因 `NewMacOS`/`ProbeInstallation` 编不过，不可替代。 |

## eng-complete（代码已接线）

| 项 | 说明 |
|---|---|
| DesktopAPI / 契约面 | CONTRACT v1.2 方法齐全；mock + live 绑定 |
| Claude 多轮 | `--input-format stream-json` + EncodeInput |
| dsh 多轮（桌面） | `dsh.NewConversation` 固定受信会话驱动；默认路径自动 Detect；对话区解析 conversation JSONL |
| Codex Kit | 已注册；`app-server` + turn bridge 多轮 |
| clone / SyncBack | Runtime 副本 + 导回 + Destroy 清理；导回成功 toast 提示路径（不自动开访达） |
| PTY / Resize | WritePty；Terminal 模式 `pty.Setsize` |
| SwitchModel | 同 profileId + Host；工作台仅列兼容账号 |
| OAuth / stdio MCP | 回环 OAuth（含本机 http IdP + DCR 证据）、宿主 stdio 桥 |
| Keychain | Darwin Keychain fail-closed；`vaultNotice` |
| 浅色主题 | `UIPrefs.theme` system/light/dark |
| 安全档位种子 | `UIPrefs.securityTier` → CreateSession / StartSession ExtraHosts |
| 本项目扩权 | 落盘跨重启；Create/Start 回灌；换引擎不清；Resolve 写 Gate |
| 记录/诊断/扩权 UX | 导出 JSON、诊断 Tab、Modal 三问+超时拒绝、出舱审查、运行报告、教程进度 |
| Seatbelt（macOS） | P1 主路径；Health fail-closed |
| Gate 非 2xx 抹 Key 片段 | 错误响应额外抹前 8/末 4 位 |

## 仍 blocked / backlog（非「忘做 eng」）

| 项 | 类别 | 说明 |
|---|---|---|
| I1–I11 / B6 live | **通过（2026-10-30 全量重跑）** | harness + 平台探针 + `bin/AgentHarbor.app` 启停；见 ACCEPTANCE |
| 扩权 302 异常项（可选） | **backlog** | 主路径改为 `PromptPermission` ask + 白名单；302 不挡验收 |
| Apple 公证 / 分发签名 | **blocked（账号）** | 需 Apple Developer；清单见 [`scripts/notarize-prep.md`](../scripts/notarize-prep.md) |
| Docker Runtime（P2） | **已合 main；API + 桌面 GUI live 已绿；真无 Desktop 另机仍待** | 代码与证据已在 `main`（`1a5db77`）；冒烟/三 Agent/`p2-colima-ensure`/`p2-docker-gui` 见 `demos/evidence/`；§B 真无 Desktop 端到端 GUI 仍待另机 |
| microVM | **backlog（后于 Docker）** | 不挡 P2 容器；预研另立 |
| 官方 MCP SDK | **backlog** | 现用 `net/http` 转发；待真实上游后再换 SDK |
| Linux Seatbelt | **N/A** | Runtime 非 Darwin → `ISOLATION_UNAVAILABLE`；无 Linux Seatbelt 目标 |

并行编码与文档纪律仍见 [GO-CHECKLIST](GO-CHECKLIST.md)。
