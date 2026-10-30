# A/B/C 架构遵循与联调准入审查

审查时间：2026-10-25。实现基线：`d175e9d`；已 fetch 源仓库，`origin/main=44491d1` 已合入 PR #10，与所审查代码树一致。本轮是审查，不修改业务实现、不使用真实凭据。

以下审查条目保留修复前的证据；最新结果见文末「修复进展」。

## 结论

整体分层、依赖方向和组件装配基本符合设计；**不能认定严格符合全部契约，也不能直接放行真实凭据全链路联调或核心闭环验收**。可以立即开始本机受控上游的接线排障，先修复下列 P1 项，再执行真实 I1/I2。

依据顺序：根 AGENTS.md 当前两天基线、CONTRACT.md、MODULE-DESIGN.md、DECISIONS.md、ACCEPTANCE.md。长期架构里的多会话、clone/导回、OAuth、完整 PTY 等不作为本次缺陷。Wails v3 已经用户确认，不再按旧 v2 决策判偏离。

## 模块遵循情况

| 归属 | 已符合的部分 | 缺口与边界 |
|---|---|---|
| A：contracts/orchestrator | 共享 DTO、窄消费接口、统一状态机、应用 context、异步 Stop、失败回滚和清理重试、事件 seq | 缺真实 Gate/Agent/认证上游整链路证据；不能将 fake 与 Runtime 探针认作 I2 |
| A：runtime/kits/security | 私有 HOME、净化环境、OS 强制文件/精确端口约束、失败关闭、Sentinel 配置、离线 CLI 检测 | 严格单进程子集，不能承诺 Bash/git 或允许后代时的进程树清理；只读/写文件与 MCP 的真实任务仍待验证 |
| B：desktop/main | 具体组件装配、唯一受限业务 Service、DTO 错误转换、Wails 绑定与事件、内存凭据 | 桌面又维护全局 busy 副本且会被旧快照改写；退出清理失败只记日志；多导出 AppInfo 未在契约登记 |
| B：gate/policy/secrets | 固定 profile、账号元信息/凭据分离、模型反代、MCP tools/list 过滤、tools/call 白名单、随机 Sentinel | 连接探测、重定向、在途撤销存在已复现缺陷；真实协议/工具组合未冻结 |
| C：frontend/demos | 两页、统一 api.js、独立 mock、真实 transport、先订阅、纯文本输出、清理重试按钮、非敏感工作区 | 先订阅之后未暂存早到事件；缺 seq 合并/去重/恢复；缓存保留头部而非尾部；mock 错误码偏离 |
| 共同：build/CI | 固定 v3 版本、绑定生成、生产构建、.app 校验、原生 WebView 冒烟已通过 | 普通 CI 不覆盖 Darwin 25 的真实隔离和真实模型/MCP；构建成功不等于核心闭环通过 |

归属按模块责任界定，不仅按最近一次修改者；升级、合并可能跨越多人提交。

## 必须优先修复的问题

### P1-1：连接测试可把真实凭据发送到未授权 origin（B）

位置：`internal/gate/connections.go:35`、`:98`。

`Connections` 的 HTTP Client 使用默认重定向策略，和会话 Gate 的拒绝重定向策略不同。同主机不同端口属于不同 origin，但 Go 默认可继续携带 Authorization。受控上游返回 307 到另一个本机端口，后者实际收到测试认证头。

违反固定上游/凭据代填边界及 I6。真实凭据联调前必须禁止探测重定向，或严格校验 scheme、host、port 与授权路径且不向新目标带凭据；补重定向回归测试。复现只用了自建 fixture Key，没有外发真实密钥。

### P1-2：仅凭 GET/404 就保存账号，不代表模型/MCP 认证与协议可用（B）

位置：`internal/gate/connections.go:88`、`:108`。

模型和 MCP 共用一次 GET，404 直接成功，其他小于 400 的状态也默认成功。不读取协议响应是否正确，未执行 MCP initialize/initialized/tools/list。受控服务器不校验认证、始终 404，ConnectModel 仍返回成功并在 Secrets 留下一个账号。

违反“连接验证成功才保存”的语义；GET /mcp 也不是完整 Streamable HTTP 客户端握手。应按冻结协议做最小只读验证，验证工具实际存在，失败不存账号。

### P1-3：Revoke 不取消在途上游，也不能证明关闭完成（B）

位置：`internal/gate/gate.go:187`、`internal/gate/model_proxy.go:32`、`:74`、`internal/gate/mcp_gateway.go:108`。

上游 context 从入站请求派生，没有关联 Gate 会话的取消 context。Revoke 删除会话后调用的 cancel 只用于监听器关闭；关闭使用固定两秒的 Server.Shutdown，忽略结果，也忽略 Revoke 传入的预算，未强制关闭活动连接或确认 handler 退出。

受控 Transport 阻塞响应体读取，在请求仍有效时调用 Revoke，返回 nil 后上游 context 仍未取消。新请求失效不等于在途调用结束，可能继续执行已发起的 MCP 操作。不能撤回已完成的外部副作用，但必须取消仍可取消的请求、等待资源关闭并如实报告失败，才能供编排判 cleanupComplete。

### P1-4：前端未实现契约要求的事件合并（C）

位置：`frontend/src/api.js:82`、`frontend/src/main.js:31`、`frontend/src/views/session.js:89`、`:132`。

- api.js 直接透传事件，没有按 sessionId 暂存创建响应前的事件；首次创建时早到输出会在 resetForNewSession 中清空，后续创建时还可能因为旧 sessionId 被直接丢弃。
- 没有 seq 水位合并和重复去重。受控调用先送 stopped/seq=10，再送 running/seq=2，页面实际退回 running；相同 seq 的 output 会展示两次。
- getSession 虽导出，但页面没有调用恢复路径；Stop 响应也直接 renderSnapshot，没有防止旧响应覆盖新终态。

违反 CONTRACT §2，直接影响任务状态、停止按钮与再次运行判断。应在统一事件层实现按会话暂存、状态水位及输出去重，避免仅凭快照水位丢弃未展示的输出，并补视图回归测试。

### P1-5：桌面 busy 副本可被历史快照改写（B，A 配合）

位置：`internal/desktop/app.go:95`、`internal/desktop/api.go:64`、`:109`。

全局 busyState 不含 sessionId/seq，每次 Create/Get/Stop 返回都写入。当前会话 running 时读取旧 stopped 会话，可解除 ConnectModel/ConnectMcp 禁用；异步事件与旧响应也会造成相反的永久 busy。补充测试直接把 running 后的历史终态交给 noteSnapshot，实际忙状态变为 false。

违反编排统一状态与会话期间禁止重连的契约。应由编排原子协调连接准入，避免桌面维护第二份权威状态；如需投影，至少按活动会话和水位更新，并处理 Connect 与 Create 的并发准入。

### P1-6：真实模型/MCP 组合尚未冻结（A/B/C）

位置：`internal/policy/profiles.go:28`、`internal/desktop/wire.go:23`、`internal/kits/claude/claude.go:140`。

Kit 配置的是 Anthropic Base URL，缺省模型名为 claude-sonnet-4-5；默认模型 profile 指向 api.deepseek.com，代码没有协议转换或可用组合的验证证据。默认 MCP 则仍是 mcp.example.test，占位工具白名单固定为 list_resources/read_resource，不能假定任何真实 MCP 都使用这些 tools 名称。

环境变量当前可覆盖 origin，但没有提供任意修改工具白名单的入口；须由 B 在固定后端配置中登记真实 HTTPS origin、路径、模型协议/名称、Bearer 认证 MCP 与人工选定的工具。A/C 共同确定只依赖当前受限工具的演示任务。这里是准入配置与证据缺口，不把未经真实调用的协议兼容性写成已通过。

## 其余契约偏差

- **P2 / C**：`frontend/src/views/session.js:74` 到上限后永久 return，保留最早 1 Mi 个 JS 字符而非最多 1 MiB UTF-8 输出尾部。已复现末尾 NEW-TAIL 被丢弃。应按字节滚动裁剪并持续展示最新输出。
- **P2 / B 与集成入口**：`main.go:50` Shutdown 失败只记日志并继续退出，无法保留窗口重试或给用户明确清理结果。无活动任务的原生冒烟未覆盖此场景，应补运行中退出及故障退出验收。
- **P2 / B**：`internal/desktop/api.go:50` 目录面板 error 与用户取消合并成 cancelled=true，应区分系统失败和正常取消。
- **P2 / B**：DesktopAPI 多导出了未登记的 AppInfo；契约要求只绑定七个业务方法，当前实际八个。未发现任意命令接口，但诊断能力应先登记契约或移出绑定对象。
- **P2 / C**：mock-api.js 认证失败使用 AUTH_FAILED/retryable=true，契约为 UPSTREAM_AUTH_FAILED/false；api.js 传输错误 INTERNAL_ERROR 也为 true，与契约 false 不一致。不能把 mock D4 通过视为所有错误语义都一致。
- **P2 / 文档**：MODULE-DESIGN §5 仍称尚无替身，与尾部状态相矛盾；DEV-A/DECISIONS/ACCEPTANCE 部分历史记录中的“待接线/待实现”应保持历史标记并给最新证据索引，避免团队重复等待已交付项。

## 本轮执行证据

| 验证 | 结果 | 证明范围 |
|---|---|---|
| go test -race -count=1 ./... | 通过 | 仓库原有 Go 测试；不是全部契约已覆盖 |
| go vet ./... | 通过 | 静态检查 |
| go test -race -count=1 -tags harbor_platform ./internal/runtime ./internal/orchestrator | 通过 | 本机真实隔离探针、真实 Runtime 与测试 Gate 的集成 |
| npm test --prefix frontend | 3 项通过 | 现有启动 transport 测试 |
| Python security unittest | 5 项通过 | 安全证据校验器，不是实际应用全项安全验收 |
| 临时 TestReview* 四个契约断言 | 4 项失败，符合缺陷预期 | 404 假成功、不同 origin 收认证头、在途未取消、旧快照清 busy |
| Node 加载实际 session.js，注入受控事件/最小 DOM | 4 项偏差复现 | 状态倒退、重复输出、超限丢尾、早到输出被清空；不宣称浏览器 E2E |

临时 Go 探针测试已从工作树移出，保存在本机 `/tmp/harbor-abc-review-evidence/`；日志为 `/tmp/harbor-abc-review-probes2.log`。Go 探针执行带 `-race -timeout 20s`，断言失败退出码为 1；其他既有测试通过。没有使用真实账号，没有修改生产代码。前一轮 CI 40150194952 的构建、.app 与原生 WebView 验证为已有证据，本轮没有重复把它当真实任务验收。

## 建议联调顺序与放行条件

1. **现在可以开始**：A/B/C 在既定 Darwin 25/arm64 机器上，用受控上游定位和修复 P1-1～P1-5，保留复现断言并加入正式回归；C 同步修正 mock 语义。不要用真实密钥排查重定向问题。
2. **真实凭据准入**：B 冻结并登记 P1-6 组合；先独立验证模型协议和 MCP 握手/白名单，再从桌面录入可轮换测试凭据，完成 I1。关键凭据边界与在途清理回归须先通过。
3. **ABC 核心链路**：真实 Agent 在专用工作区完成文件任务及已授权 MCP 调用，检查输出与审计，完成 I2；不得以 curl、mock 或离线初始化替代。
4. **正式验收**：执行 I3～I11，覆盖凭据隔离、越界/出站负例、provisioning/running Stop、清理失败重试、连续两轮、任务中关闭窗口及无残留确认。全部有证据后再宣布核心闭环可交付。

负责人下一步：A 负责真实 Agent 任务与跨模块安全闭环，协同解决准入接口；B 负责探测/协议/撤销/桌面 busy 和固定上游；C 负责事件归并、尾部缓存、恢复与真实状态展示。无需等待新增多会话或完整子进程能力才开始上述受控联调。

## 修复进展

2026-10-25，用户授权修复后完成以下代码调整。上文为审查时的历史缺陷，不代表修复后的当前行为。

| 原问题 | 当前实现与回归 |
|---|---|
| P1-1 重定向凭据越界 | 连接探测禁止重定向，生产仅 HTTPS，无凭据的受控 HTTP 例外仅由测试代码显式开启；不同 origin 的认证头回归通过 |
| P1-2 假连接成功 | 模型执行固定 Anthropic messages 探测并校验结构（最多 1 token）；MCP 执行 initialize/initialized、分页 tools/list，校验固定版本/会话头/工具；404、HTML、RPC 错误、错误 ID、错误认证及缺失工具均不保存 |
| P1-3 在途撤销 | 监听器 BaseContext 派生自会话，撤销在锁内标记失效、取消上下文、关闭连接、等待 Serve/handler 结束；预算过期报告失败，重试幂等，取消不再并发改写 Sentinel |
| P1-4 事件归并 | api.js 统一暂存早到事件并在工作台重置后激活；seq 去重，状态与输出独立水位；恢复/Stop 响应不覆盖新状态；按 seq 排列输出 |
| P1-5 busy 副本 | 已移除；编排 WithConnection 与 Create 原子互斥，Shutdown 取消并等待连接，读取历史快照无权改变准入 |
| P1-6 接入组合 | 已移除默认供应商及占位 MCP；显式登记 HTTPS origin、模型名、工具白名单，未配置失败关闭。**真实服务/账号/工具任务组合仍待团队登记并验收** |
| P2 输出与恢复 | 输出按 UTF-8 字节保留 1 MiB 最新尾部，审计列表有界；focus/pageshow/恢复时获取状态，sessionStorage 只记录非敏感 sessionId |
| P2 退出与目录面板 | 关闭钩子/ShouldQuit 异步确认 Shutdown，失败保留窗口并显示固定提示、允许重试；成功后才清空凭据。系统面板错误不再伪装用户取消 |
| P2 绑定及 mock | 移除未登记 AppInfo，仅七个业务方法；错误码与 retryable 对齐；mock 状态载荷与外层 seq 一致、ID 唯一、Stop 幂等 |
| P2 文档 | 当前基线和历史记录分开；MODULE-DESIGN 的替身状态、接入配置说明及 CLAUDE 入口已同步 |

验证：修复前的连接/撤销/Stop 回归断言确认失败后修复，Go 全量 race/vet、前端 11 项测试、Darwin 系统测试、Wails 生产构建、原生 WebView 关闭路径均通过。独立 Postman simulation 运行真实 Connections 客户端，模型与 MCP 接线及错误认证路径通过；6 条请求均出现在唯一 upstream 成员日志，没有请求真实服务。复现步骤见 postman/simulations/harbor-protocol.context.md。

前端工作台使用真实 api.js + mock transport 的测试额外验证延迟激活时早到输出不会消失；这不是浏览器/真实 Agent 验收。退出失败重试通过状态控制单测，无任务的实际 WebView 关闭通过；**运行中真实 Agent、真实模型/MCP、完整 I1–I11 仍未整体完成**。本轮没有放宽 Runtime、增加供应商协议转换或将模拟结果计入真实验收。
