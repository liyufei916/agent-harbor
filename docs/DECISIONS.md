# 实施决策（环境 / 版本 / 上游登记）

> **分支 `feat/page-design-full`**：产品范围与接口以 [PAGE-DESIGN](PAGE-DESIGN.md)、[CONTRACT v1.2](CONTRACT.md)、[SYSTEM-DESIGN v2](SYSTEM-DESIGN.md) 为准（含 OAuth/clone/PTY/双会话）。下文「单会话 / direct / 内存凭据」是 **P0 两天联调基线**与当前 live 能力现状，**不是**本分支砍功能的理由。

已定技术栈：Go 主业务、简单 HTML/CSS/JavaScript、Python 验收辅助；单应用；Claude Code 优先 + dsh；一个已验证模型组合与一个远程 Streamable HTTP 认证 MCP 用于 I 证据。详细边界以 AGENTS.md + SYSTEM-DESIGN 为准。

2026-10-25：用户确认本轮整体采用 **Wails v3.0.0-beta.34**，替代此前 v2 决策。Go module、CLI 与 `@wailsio/runtime` 必须锁定同版，不使用 latest。桌面入口为 `main.go`，受限绑定在 `internal/desktop`，生成绑定在 `frontend/bindings`，配置在 `build/config.yml` 与 `Taskfile.yml`。版本验证见 [Wails v3 评估与验证](archive/WAILS-V3-ASSESSMENT.md)。接口以 [CONTRACT.md](CONTRACT.md) **v1.2** 为准，不因升级 Wails 改变 DTO、事件或安全边界。

| 开工首小时登记项 | 负责人 | 当前状态 / 证据要求 |
|---|---|---|
| 目标 macOS 版本、CPU、隔离后端可用性 | A | 待验证，记录真实 profile 正/负例；非 root 不等于已隔离 |
| Go / Wails / Node / 前端工具及 MCP SDK 的确切版本 | B | **已统一**：Go toolchain 1.26（`go.mod`）、Wails **v3.0.0-beta.34**、Vite **7.3.7**、Node 20.19+ 或 22.12+（CI 使用 Node 22）；MCP 仍用标准 `net/http` 转发，完整 SDK 待真实上游验证后补记。绑定生成和生产构建已通过，本机 WebView 证据见评估记录 |
| Claude Code 版本、安装方式及必要只读运行路径 | A | 待检测；不得复制完整用户 HOME/登录配置 |
| demo-model 的真实提供方、协议、HTTPS origin 与允许路径 | B | profileId 固定 `demo-model`；无默认供应商；必须登记 `HARBOR_DEMO_MODEL_ORIGIN`（HTTPS origin）和 `HARBOR_CLAUDE_MODEL`，使用 Anthropic `/v1/messages` 验证；真实账号待联调 |
| demo-mcp 的真实服务、认证方式、origin、路径、只读工具名 | B/C | profileId 固定 `demo-mcp`；移除占位 origin；必须登记 `HARBOR_DEMO_MCP_ORIGIN` 及 `HARBOR_DEMO_MCP_TOOLS`；仅 Bearer、2025-03-26、`/mcp`，真实服务与账号待选 |
| 专用非敏感工作区与一条稳定演示任务 | C/A | **已准备**：目录 `demos/workspace/`（见 [demos/README.md](../demos/README.md)、`./demos/verify-workspace.sh`）；任务文案见 demos 固定脚本。明确会直接改原文件，MCP 操作只读。mock 仍用 `/demo/project`，live/I2 选本目录绝对路径 |
| 依赖安装、绑定生成、Go/前端独立测试和 Wails 开发命令 | B | **已可执行**（见根 README）：`go test ./internal/...`、`wails3 generate bindings ./...`、`frontend` 下 `npm run build`、`wails3 dev` / `wails3 build`；另有 GitHub Actions CI |

这些未决项阻碍真实端到端验收，但不阻碍基于契约、固定 profileId 和替身进行独立模块开发。实际服务不兼容时保留失败证据，调整受支持组合，不偷偷更改“所有 Agent/MCP 已支持”的产品口径。

### MCP OAuth 回环（B，2026-10-25；2026-10-30 补强）

| 项 | 约定 |
|---|---|
| 回调 | 默认 `127.0.0.1:<ephemeral>/callback`；可设 `HARBOR_MCP_OAUTH_REDIRECT_PORT` 固定端口便于 IdP 预登记 |
| 授权 | `GET {origin}/authorize?response_type=code&redirect_uri=…&client_id=…&state=…` |
| 换票 | `POST {origin}{HARBOR_MCP_OAUTH_TOKEN_PATH||/oauth/token}`，`grant_type=authorization_code`；可选 `HARBOR_MCP_OAUTH_CLIENT_SECRET` |
| client_id | 显式 `HARBOR_MCP_OAUTH_CLIENT_ID` 优先；否则尝试 RFC 7591 DCR；再回退默认 `agent-harbor` |
| 响应 | `McpOAuthResponse` 含 `redirectUri` / `clientId` / `clientSource`（`env`\|`dcr`\|`default`）供 UI 自助 |
| 成功 | `access_token` 写入 Secrets；UI 轮询 `ListAccounts` |

### 自定义 MCP Token 轨（B，2026-10-26）

| 项 | 约定 |
|---|---|
| 入口 | 工具页填 HTTPS Origin + API Key；`profileId` 空/`custom`（`demo-mcp`+异主机升为 custom） |
| 探测 path | 用户 path → path+`/mcp` → `/mcp`；须 Streamable HTTP，勿填模型 `/v1` |
| 白名单 | Connect 成功写入 `AccountInfo.tools`；会话 Gate 只用该快照，不自动授权新工具 |
| 旧账号 | 无 `tools` 时开会话 `INVALID_ARGUMENT`；删除后重连 |

### 宿主 stdio MCP（B/A，2026-10-30）

| 项 | 约定 |
|---|---|
| 入口 | `ConnectInput.transport=stdio` + 绝对路径 `command` / `args` / `env` |
| 实现 | `internal/mcpstdio`：Content-Length stdio + `http://127.0.0.1:port/mcp` 回环桥；桌面 `AdoptStdioBridge` 保活 |
| Policy | `AccountInfo.Host` 形如 `127.0.0.1:port`；仅 loopback http；`AllowInsecureLoopback` |
| 安全 | 进程与 env 在宿主机；沙箱仍只连 Gateway；删除账号关闭桥 |
| 重启 | `App.RestoreStdioBridges`：启动时按 vault 中 command/args/env 重拉桥并 `UpdateEndpoint` 换端口；失败账号保留 |
| 可见性 | `AccountInfo.offlineHint`：桥未就绪时 ListAccounts / 工具页标「○ 桥未就绪」+ 原因；成功重拉后清空 |

### 发布与 CI 产物（B，2026-10-30）

| 项 | 约定 |
|---|---|
| CI | `darwin:package` 后上传 `AgentHarbor-macos.zip`（14 天）；可移植 Seatbelt profile 断言步骤 |
| Release | `.github/workflows/release.yml`：`v*` 标签或手动触发 → GitHub Releases 挂 zip + sha256 |

### 二期隔离后端（2026-10-30）

| 项 | 约定 |
|---|---|
| 选型 | **Docker 容器**为 P2 主路径；**microVM 延后预研**，不挡二期 |
| 默认 | 桌面仍 P1 Seatbelt；显式切 Docker / 容器隔离后才用 P2 |
| 分支 | `feat/p2-docker-runtime`；计划见 [P2-DOCKER.md](P2-DOCKER.md) |
| 硬约束 | 不挂 `docker.sock` 进 Agent 容器；无 daemon → fail-closed；Gate/UI 契约不变 |

### P2 容器引擎主路径 B→C（2026-10-30；用户确认）

**产品选择**：不要求用户安装 Docker Desktop。主路径为 **Harbor 托管轻量引擎**；系统已有 Docker 仅作高级兼容。

| 项 | 约定 |
|---|---|
| 主路径 | **B→C**：安装/首启部署轻量引擎 → 用户无感「没有 Docker」 |
| 引擎 | **Colima + docker CLI**（macOS arm64 优先）；版本钉扎，随 Harbor 升级策略另表 |
| 落盘 | `~/Library/Application Support/AgentHarbor/engine/`（cli、colima、数据目录）；不写入系统 `/usr/local` 强依赖 |
| 控制面 | 仅 Harbor 宿主机进程调用 `Ensure/Start/Stop/Health`；**不对 Agent 暴露**管理面与 sock |
| Runtime | 现有 P2 `internal/runtime` Docker 实现不变语义；经 `DOCKER_HOST`（或等价）指向托管 daemon |
| 镜像 | 仍构建/使用 `agent-harbor/runtime:dev`（三 CLI 预装，一会话一容器） |
| Desktop | **非主路径**；探测到本机可用 Docker 时可「使用已有 Docker」，文案降为高级选项 |
| 默认隔离 | 仍为 **Seatbelt**；启用容器隔离失败 → fail-closed，禁止无隔离直跑 |
| 不做 | 安装包内嵌 Docker Desktop；把 `docker.sock` 挂进 Guest；跳过引擎假装已隔离 |
| 包体 | 优先「首启按需下载钉扎产物 + 校验和」；随包附带为可选优化，公证体积另估 |
| 里程碑 | M2b 启停+建镜像（用 PATH/`engine/bin`）；M2c 钉扎下载；M3 设置接来源；M4 默认无感路径（见 P2-DOCKER） |
| Colima 配置 | profile=`agent-harbor`；`COLIMA_HOME=<engine>/colima`；启动 `--activate=false`（不抢用户全局 context）；轻量默认 2CPU/2GiB/20GiB |

## 2026-10-24 开发环境观测

A 在 macOS 26.7 arm64、Go 1.27.2 下完成契约与编排测试，检测到 Claude Code 2.1.292 原生安装。为独立编译加入最小 go.mod（module 沿用上文建议值，暂定 go 1.23.0，尚未在该最低版本运行验证）；B 已在本分支完成 Wails v3 初始化与版本登记。Seatbelt 初探有正/负例，但生产限制与进程树清理仍待实现。观测不等于团队环境冻结或真实 Agent 兼容性通过，详细证据见 [研发 A 开发记录](DEV-A.md)。
