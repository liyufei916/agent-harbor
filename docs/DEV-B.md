# 研发 B 开发记录

本人提交在分支 `dplB`，作者 liyufei，2026-10-24 经 PR #4、#8、#9 合入。范围是两天版 Harbor Gate、Policy、内存 Secrets，以及 Wails v3 桌面桥和构建环境。会话状态机仍由 A 的 Orchestrator 实现，配置页与工作台结构仍由 C 维护。

下文按提交记录当时交付的行为。2026-10-25 起，同一目录又有后续合入，见文末索引；那些修改不记入下面各节的已落地范围，也不能把单测或桌面壳冒烟写成 I1–I11 通过。

## 2026-10-24：Wails v3 壳与控制面首批

提交 `c5b5f2e`。在 A 已有契约类型之上补齐宿主机控制面，并初始化 Wails v3 工程。

### 已落地

| 路径 | 说明 |
|---|---|
| `internal/gate` | 回环监听、模型反向代理、MCP Gateway、Sentinel、连接探测 |
| `internal/policy` | `Build` 生成会话策略快照；固定 `demo-model` / `demo-mcp` |
| `internal/secrets` | 进程内存 `Put` / `Resolve` / `Lookup` / `Delete` / `Clear` |
| `main.go` | Wails v3 薄入口：注册 `harbor:event`、嵌入 `frontend/dist`、绑定业务服务 |
| `build/`、`Taskfile.yml`、`go.mod` | Wails v3.0.0-beta.34 脚手架与任务 |
| `frontend/package.json` | Vite `^4.5.5`，`@wailsio/runtime`，`engines.node` 为 `>=16` |

Gate 只在会话准备时由编排调用 `Start`，停止或失败清理时 `Revoke`。应用启动只构造对象，不另起网关进程，也不在空闲时监听端口。

- 凭据只留在本进程内存。`Resolve` 只注入 Gate；编排只拿 `Lookup` 元数据。退出时 `Clear`。重启需要重新连接。
- Sentinel 使用至少 32 字节安全随机值，前缀 `harbor_sentinel_`，停止时撤销。沙箱配置只含网关地址和该令牌。
- 模型走固定 HTTPS origin 的显式 Base URL 反向代理，不实现 CONNECT，也不做通用 HTTPS 中间人。
- MCP 入口收口到本机 Gateway，按策略里的路径和工具名单转发。本记录使用自写 JSON-RPC 转发，没有接入官方 MCP Go SDK。
- 首批 `go.mod` 的 module 曾写成 `github.com/Wintercom/agent-harbor`。与 A 的模块合并后，当前模块路径是 `github.com/liyufei916/agent-harbor`。

当时桌面绑定还放在仓库根的 `app.go`、`bindings.go`。下一节迁入 `internal/desktop`。

## 2026-10-24：桌面归位与可编译

| 提交 | 内容 |
|---|---|
| `11a78ac` | 桌面 API、错误映射、会话占位和 `AttachOrchestrator` 迁入 `internal/desktop` |
| `0cbf24f` | 删除根目录 `app.go`、`bindings.go`、`bindings_test.go`、`fake_session.go` |
| `1045084` | 提交 `frontend/dist/index.html` 占位，并在 `.gitignore` 保留该文件，供 `go:embed` 与 CI 编译 |
| `76437a5` | `gofmt`，使 CI 的 `gofmt -l` 检查通过 |

`AttachOrchestrator` 在桌面进程里装配 A 的 `runtime.NewDefault()`（Darwin→Seatbelt，非 Darwin→不可用 stub）、Claude Kit 和 `orchestrator.New`。sink 使用桌面事件转发，事件名是 `contracts.HarborEventName`（`harbor:event`）。Wails 只暴露 `DesktopAPI`，不把 Runtime、Gate 或 Secrets 直接绑给前端。

绑定生成目录是 `frontend/bindings/`，已忽略，不手改。裸 `go build` 嵌入的是占位页；要带上 C 的页面，需先构建前端，或走 `wails3 dev` / `wails3 build`。

## 2026-10-24：桌面连线、占用、代理与 MCP

提交 `26321a3`，由 PR #9 合入。在首批网关能编译的基础上，补上前端真实通道和两天契约里仍缺的网关行为。

### 已落地

- 前端 `createLiveTransport()` / `useLive()`：动态加载 `frontend/bindings/github.com/liyufei916/agent-harbor/internal/desktop/desktopapi.js`，用 `@wailsio/runtime` 的 `Events.On('harbor:event')` 收事件。桌面壳或 `?live=1` 走真实绑定；`?live=0` 与 `?scenario=` 保持 C 的 mock。绑定尚未生成时留在 mock。
- 桌面侧根据会话状态拒绝忙碌期间的 `ConnectModel` / `ConnectMcp`，错误码 `SESSION_BUSY`。这份忙碌标记当时放在 `internal/desktop`，后来已从桌面层移除，见文末。
- Policy：账号主机必须与固定 profile 的主机一致，不一致返回 `INVALID_ARGUMENT`。路径 `"/"` 只匹配根路径，不再当成放行全部路径。
- 模型代理：只允许 profile 中的路径；Sentinel 常量时间比较；代填真实密钥；跨主机重定向不跟随；审计工具名固定为 `model`。
- 响应脱敏：跨分块抹掉 Sentinel 和已知真实凭据，替换为 `[redacted]`；短于 8 字节的秘密值不参与替换。
- 流式超时：单次请求空闲 60 秒，有数据则重置；总时长 10 分钟。`http.Client.Timeout` 置 0，避免客户端总超时切断流。
- MCP：允许 GET / POST / DELETE；解析 JSON-RPC 2.0；`tools/call` 不在白名单则 403，空名单也拒绝；`tools/list` 过滤工具，含 SSE 的 `data:` 行。转发 `Content-Type`、`Accept`、`Mcp-Session-Id`、`Mcp-Protocol-Version`、`Last-Event-ID`，并强制 `Accept-Encoding: identity`。审计工具名在没有工具名时为 `mcp`。

当时固定上游：

| 项 | 当时默认 |
|---|---|
| 模型 origin | `HARBOR_DEMO_MODEL_ORIGIN`，缺省 `https://api.deepseek.com` |
| MCP origin | `HARBOR_DEMO_MCP_ORIGIN`，缺省占位 `https://mcp.example.test` |
| 模型路径 | `/v1/`、`/chat/` |
| MCP 路径 | `/mcp` |
| MCP 工具 | `list_resources`、`read_resource` |
| Claude 模型名 | `HARBOR_CLAUDE_MODEL`，缺省 `claude-sonnet-4-5` |
| Claude 可执行文件 | `HARBOR_CLAUDE_BIN`，空则由 Kit 使用 `claude` |

这些默认值是当时提交里的行为。当前仓库已改为未显式登记 origin、模型名和工具白名单时失败关闭，见文末。

### 实际验证

本记录对应提交时，在本机执行过相关包测试，通过：

```sh
go test ./internal/gate ./internal/policy ./internal/secrets ./internal/desktop
```

CI 侧要求 `gofmt` 干净，以及 `go test`。这些测试证明网关改写、工具过滤、超时、撤销、策略主机/路径和桌面错误映射在替身或本机回环上符合当时用例。它们不证明真实模型、真实认证 MCP 或打包应用里的核心闭环。ACCEPTANCE 的 D3、D5、I1、I2、I6、I11 在本记录结束时仍是待执行。

本记录没有启动并点击过 Wails 窗口，也没有用真实密钥打过上游。

### 本地命令

`go install` 把 `wails3` 装到 `$HOME/go/bin`。该目录不在 `PATH` 里时，zsh 会报 `command not found`。当前终端需要：

```sh
export PATH="$HOME/go/bin:$PATH"
```

仓库根目录：

```sh
wails3 generate bindings ./...
cd frontend && npm install && cd ..
wails3 dev          # 桌面调试，自动走真实绑定
wails3 build        # bin/AgentHarbor
wails3 task package # macOS 上得到 bin/AgentHarbor.app，可双击打开
```

只看 C 的页面时，在 `frontend` 执行 `npm run dev`，不要加 `?live=1`。浏览器 mock 不会创建沙箱，也不会启动 Gate。

### 明确未做（历史两天记录）

当时两天范围之外、本记录没有实现：

- OAuth / SSO、钥匙串或任何重启后仍在的凭据
- 多会话、`switchModel`、clone / 导回、完整 PTY、会话内追加提问
- CPU / 内存配额
- 官方 MCP Go SDK 客户端
- 窗口导航锁定与 CSP
- 真实上游协议组合的验收证据

**分支 `feat/page-design-full`（2026-10-25）**：上列 OAuth / 多会话 / `switchModel` / clone / PTY **改回范围内**（见 [SYSTEM-DESIGN](SYSTEM-DESIGN.md) v2、[CONTRACT](CONTRACT.md) v1.2、[TASK-SPLIT-ABC](archive/TASK-SPLIT-ABC.md) v2.0）。DesktopAPI 方法名已占位；真实现与证据仍待 B（Gate OAuth）与 A（PTY/clone 编排）接线。其余项（配额、官方 MCP SDK、CSP 等）仍不在本分支必做。

连接探测当时把 HTTP 404 和其他小于 400 的状态当成成功后再写入 Secrets。这在后续审查里被标为缺陷，并已由后续提交改掉，不属于本节的完成项。

## 分工备忘（B）

| 负责 | 目录 |
|---|---|
| 模型代理与 MCP Gateway | `internal/gate` |
| 固定 profile 与会话策略快照 | `internal/policy` |
| 内存凭据 | `internal/secrets` |
| Wails 入口、绑定、事件与装配 | `main.go`、`internal/desktop`、`build/`、`Taskfile.yml` |
| 前端构建与 live transport 接线 | `frontend/package.json`、`frontend/src/api.js` 的真实通道 |

不改 Orchestrator 的状态迁移和 Runtime profile。不手改 `frontend/bindings/`，也不把 `frontend/dist` 的 Vite 哈希产物提交上去；嵌入用的占位 `frontend/dist/index.html` 需要保留。

## 后续索引

下面这些改动落在 B 负责的目录里，但作者不是本记录的提交，行为以那些提交和审查文档为准：

| 日期 | 提交 | 和本记录的关系 |
|---|---|---|
| 2026-10-25 | `d175e9d` 及同日 Wails 检查 | 统一 Wails v3 基线，补生产构建、`.app` 与原生 WebView 冒烟 |
| 2026-10-25 | `16347f7` | 连接探测禁止把凭据跟到其他 origin；模型/MCP 改为协议探测成功才保存；撤销等待在途请求结束；桌面层忙碌副本移除，改由编排原子准入。审查与修复见 [ABC-INTEGRATION-REVIEW.md](archive/ABC-INTEGRATION-REVIEW.md) |
| 2026-10-25 | `5028331`、`19b428b`、`44b08e7` | dsh Kit 装配，以及 MCP `tools/list` 的 SSE 长连接与有限 JSON 响应 |

当前启动命令、环境变量和「未登记则拒绝连接」以根 [README.md](../README.md) 为准。真实模型与认证 MCP 的端到端验收仍待执行。

## 2026-10-25：DesktopAPI 去占位（FULL-BIND 子集）

本分支产品页已齐，桌面仍返回「健康检查尚未接线」会误伤联调。本轮 **B** 将下列方法接到现有控制面 / A 编排（**不改** Orchestrator 状态机）：

| 方法 | 接线 |
|---|---|
| `GetRuntimeHealth` | `AttachOrchestrator` 注入的 `runtime.MacOS.Health`；成功 `isolationLevel=seatbelt` |
| `ListSessions` / `DestroySession` | 转发 `orchestrator.List` / `Destroy` |
| `ListAccounts` / `RemoveAccount` | `secrets.Store.List` / `Delete`（仍仅内存） |
| `GetUIPrefs` / `SetUIPrefs` | 进程内偏好（重启丢失；钥匙串见 P1-B-01） |
| `GetPermissionOverview` | 会话工作区 + 账号主机 + 记住主机；禁止路径类别文案 |
| `ListRememberedHosts` / `RevokeRememberedHost` | 进程内表；`ResolvePermission(decision=project)` 成功后写入 |

验证：

```sh
go test ./internal/desktop ./internal/secrets
```

桌面需重新 `./scripts/dev-desktop.sh`（或 `wails3 dev`）。Darwin 25 / arm64 非 root 上 Health 应可通过；其它环境 `Available=false` 且 reason 为隔离错误文案，**不再**是「尚未接线」。

## 2026-10-25 续：SwitchModel / 自定义 Origin / OAuth 回环 / 扩权挂接

| 项 | 状态 |
|---|---|
| `SwitchModel` | 编排更新账号 + `Gate.SetModelAccount`；桌面转发 |
| 自定义模型 Origin | `profileId=custom` + `origin=https://host`；Policy/Connect 支持（仅 Claude） |
| `StartMcpOAuth` / `CancelMcpOAuth` | 本机 `127.0.0.1` 回调；`?access_token=`/`code=` 写入 MCP 账号；隐式 fragment 有落地页脚本 |
| 扩权 | 跨主机重定向时调 `RequestPermission`；`project`/`once` 写入 `AllowExtraHost` + 记住表 |

## 2026-10-26：自定义 MCP + 会话期可连账号

| 项 | 状态 |
|---|---|
| 自定义 MCP | `ConnectMcp` + Origin；path 候选用户 path → +`/mcp` → `/mcp`；`probeMCP` 返回并落盘 `tools` |
| 开会话 | Policy 用账号 `tools` 作白名单；空 tools → `INVALID_ARGUMENT`（提示删后重连） |
| `WithConnection` | running 会话**不**拦 Connect；仅与并发 Connect / provisioning Create 互斥（覆盖上文「忙碌拒连」旧记） |
| 错误映射 | Policy 的 profile/MCP/Agent 错误 → `INVALID_ARGUMENT`，勿打成 `WORKSPACE_DENIED` |

## 2026-10-25 再续：Prefs 落盘 + Keychain + once 消费

| 项 | 状态 |
|---|---|
| UIPrefs / 记住主机 | `~/Library/Application Support/AgentHarbor/ui-state.json`（`NewDesktopApp`） |
| 凭据持久 | Darwin：Keychain `com.liyufei916.agent-harbor`（`-i`/`-X`；探测失败 **fail-closed**，`vaultNotice` 提示，不回退明文文件）；非 Darwin：文件库占位 |
| 退出 | 持久化 Store **不清空**凭据（仅纯内存 `NewApp` 退出时 `Clear`） |
| `once` 扩权 | `Harbor.HasExtraHost` 命中后删除；跨主机重定向本请求授权后立即消费 |

## 2026-10-25 三续：OAuth 换票 + 安全跟随重定向 + DNS/IP

| 项 | 状态 |
|---|---|
| OAuth | 默认 `response_type=code`；回调用 `code` 向 `{origin}/oauth/token` 换票；显式 `HARBOR_MCP_OAUTH_CLIENT_ID`/`SECRET` 优先，否则尝试 RFC 7591 DCR，再回退 `agent-harbor`；`HARBOR_MCP_OAUTH_REDIRECT_PORT` 可固定回环端口；`HARBOR_MCP_OAUTH_TOKEN_PATH`；响应含 `redirectUri`/`clientId`/`clientSource`；仍兼容 `access_token` 查询/fragment |
| 跨主机重定向 | 用户 `once`/`project` 后 **跟随一跳**；不携带 Authorization/API Key；多跳拒绝 |
| DNS/IP | `ValidatePublicHostname`：拒绝回环/私网/链路本地/组播/CGNAT；解析结果任一违规即拒 |

## 2026-10-25 四续：浏览器打开 + MCP 跟随 + Connect DNS

| 项 | 状态 |
|---|---|
| OAuth | `StartMcpOAuth` 成功后 `open`/`xdg-open` 打开系统浏览器（失败不阻断 pending） |
| MCP 网关 | 与模型反代同一套 `followCrossHostRedirect`（一跳、无凭据、DNS/IP） |
| Connect | 非 `AllowInsecureLoopback` 的 profile 在探测前 `ValidatePublicHostname` |

## 2026-10-25：Claude Detect 联调修复（配合 A Kit）

本机常见：`claude --version` 为 `2.1.69` 而旧 Detect 硬钉 `2.1.292`，且帮助已无 `--permission-prompts`，导致「重新检测」一直不可用。现改为能力检测 + 明确 reason；`HARBOR_CLAUDE_BIN` / Homebrew 路径回退；可选 `HARBOR_CLAUDE_REQUIRE_VERSION` 钉版本。

## 2026-10-30：Release / stdio 桥 / OAuth 自助

| 项 | 状态 |
|---|---|
| CI 产物 | `ci.yml`：`darwin:package` → `AgentHarbor-macos.zip` artifact（14 天）；可移植 Seatbelt profile 断言 |
| Release | `release.yml`：`v*` 或 workflow_dispatch → GitHub Releases |
| 宿主 stdio | `internal/mcpstdio` + `ConnectMcp(transport=stdio)`；`App.AdoptStdioBridge`；Policy 仅 loopback |
| OAuth | DCR 尝试 + 固定 `HARBOR_MCP_OAUTH_REDIRECT_PORT`；UI 自助登记与失败建议 |
| 装配 | `runtime.NewDefault()`（build tag） |

**仍未完**：真实 IdP / 模型 / MCP 的 I1 live 证据（操作清单见 [demos/I1-I2-LIVE.md](../demos/I1-I2-LIVE.md)）。

### stdio 重启重拉（同日补）

`App.RestoreStdioBridges` 在 `NewAppWithDataDir` 末尾执行；`secrets.UpdateEndpoint` 刷新 Host/Tools；`Shutdown` 关闭全部桥。单测：`TestRestoreStdioBridgesReloadsHostPort`。

```sh
go test ./internal/desktop ./internal/gate ./internal/policy ./internal/orchestrator ./internal/secrets
```



