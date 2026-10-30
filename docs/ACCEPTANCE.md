# 两天版开发与集成验收

执行依据：[接口契约](CONTRACT.md)、[模块设计](MODULE-DESIGN.md)、[安全验收说明](../security/README.md)。本表定义完整验收目标，逐项证据见下文。报告不得将文档检查或替身成功记作真实应用通过。

## 1. 独立开发检查

| 编号 | 负责人 | 操作 | 预期 |
|---|---|---|---|
| D1 | A/B | 将契约 Go 类型落地、编译各包 | 无循环导入；DTO/方法名与契约一致，能生成绑定 |
| D2 | A | 用 fake Gate/Runtime/Kit/Catalog 驱动编排 | 正常、准备失败、取消竞争、清理失败均有确定状态和锁行为 |
| D3 | B | 用 fake SessionService 接入 Wails 方法及事件 | 所有 Response.error、固定事件名和 JSON 字段符合契约，不把原始 error 交给 UI |
| D4 | C | 普通浏览器使用 mock transport | 两页、正常及四个失败场景可操作，显式模拟标记，无 wailsjs 导入依赖；实现与操作见 [DEV-C.md](DEV-C.md) |
| D5 | B | 对受控模型/MCP 上游进行流、认证、超时/取消检查 | 正常保留流；401、断流、撤销、未知目标正确处理；不自动重试非幂等工具 |

## 2. 真实核心闭环与安全检查

| 编号 | 负责人 | 操作与前提 | 必须结果 |
|---|---|---|---|
| I1 | B/C | 关闭 mock，桌面连接已登记真实模型和只读认证 MCP | 成功仅返回账号引用；输入字段清空；失败候选不成为可用账号 |
| I2 | A/B/C | 选择演示目录，真实 Agent 完成模型任务、发现并调用指定 MCP 工具 | 有真实输出、工具结果及脱敏审计；不能以 curl/普通 shell/fixture 替代 |
| I3 | A/B | 用可轮换测试凭据检查 Agent env 和私有配置，模拟可回显认证的受控上游 | 无真实上游凭据；仅有 Sentinel；公开事件和日志不含已知 Key/令牌 |
| I4 | A | 从实际受限进程读写工作区，读取工作区外自建文件、凭据路径和越界链接 | 仅授权文件成功，其他拒绝；不打印用户真实密钥 |
| I5 | A/B | 实际受限进程访问 Gate、普通公网/IP、其他回环端口和管控通道 | 仅精确 Gate 数据端口可达；拒绝不能仅由 Gate 业务层实现 |
| I6 | B | 错误/撤销 Sentinel、未授权工具、恶意目标和重定向 | 请求失败且不向其他目的地代填凭据；会话权限不可由 Agent 改写 |
| I7 | A/B | profile 不可用、Gate 部分启动失败、Create/Exec 失败、运行中 Gate 致命故障 | 不直跑宿主机；回滚已创建资源，状态准确，晚返回资源也被清理 |
| I8 | A/C | provisioning 中停止、running 中停止、重复 Stop | 先撤销授权后确认进程树结束；stop 幂等；清理结果由终态事件确认 |
| I9 | A/B | 模拟 Destroy 失败或超时 | failed、cleanupComplete=false，锁不释放；重试清理成功后才允许新会话 |
| I10 | A/B/C | 正常退出、取消、认证/超时故障后再次运行；连续两轮 | 无残留进程或旧有效令牌；失败但已清理的会话不永久占锁 |
| I11 | B/C | 构建并在已登记目标机器运行，退出应用 | 资源可加载、绑定/事件可用；退出触发 Shutdown，不留下 Agent |

禁止在真实宿主机运行 reboot/shutdown 验证拦截；使用无害权限探测。授权工作区仍可能被修改、内容仍可能发送给授权上游，不能把凭据代理描述成防止任意数据外泄。

## 3. 证据记录

每项记录编号、时间、负责人、Git 提交/工作树标识、OS/CPU/CLI/SDK 版本、真实或受控上游、具体操作、预期、实际结果、脱敏证据和失败后下一步。状态只能是待执行/通过/失败/阻断；真实服务项不能以模拟结果填“通过”。

D1–D5 支持并行开发与联调；I1–I11 支持本次真实闭环判断。首日结束前优先通过 I1/I2，第二日补齐安全与生命周期；尚未验证的项目应直接报告，不宣称“全部模块无阻塞”或“安全已保证”。

## 4. 2026-10-24 首批执行记录（历史）

> 当时 D1 绑定未生成、Runtime 未实现。**现行**：D1–D4（mock）与平台探针已前进；I1–I11 **live** 仍以 §「2026-10-25 P0」与 §「2026-10-30 I3/I4/I5」为准，勿用本节「全部待执行」覆盖现状。

环境与命令见 [DEV-A](DEV-A.md)；诊断/fake 不得计入真实闭环。

## 4.1 2026-10-24 D4 证据（产品 C / mock）

| 字段 | 内容 |
|---|---|
| 编号 / 状态 | **D4 / 通过（mock 独立检查）** |
| 时间 | 2026-10-24 |
| 负责人 | 产品 C |
| 工作树 | `frontend/` mock 两页 + `docs/DEV-C.md`（合入含 B Wails v3 壳的 main） |
| 环境 | 本机浏览器 + `python3 -m http.server 5173`；无 wailsjs |
| 上游 | 无真实模型/MCP（固定 mock 数据） |
| 操作 | 见下表五场景 |
| 预期 | 两页可操作；显式「模拟模式」；正常流与四失败可复现 |
| 实际 | 与预期一致（浏览器手工） |
| 证据 | [DEV-C.md](DEV-C.md)、[demos/README.md](../demos/README.md) 三分钟话术；页面顶栏 mock 徽章 |
| 下一步 | 等 B 生成绑定后切 live，再执行 I1；本项不得记入真实闭环通过 |

| 场景 | 操作摘要 | 实际结果 |
|---|---|---|
| normal | 连接模型+MCP → 选工作区 → 开始 | provisioning→running→输出/audit→stopped |
| auth | 场景=认证失败 → 连接模型 | 历史返回 `AUTH_FAILED`；2026-10-25 已修为 `UPSTREAM_AUTH_FAILED`；Key 清空、无会话 |
| busy | 场景=SESSION_BUSY → 已连接后开始 | `SESSION_BUSY`；不进入 running |
| isolation | 场景=隔离不可用 → 开始 | `ISOLATION_UNAVAILABLE`；Agent 显示不可用 |
| cleanup | 场景=cleanup → 开始后停止，再停一次 | 首次 failed 且 cleanupComplete=false + 提示重试；再次停止后 stopped |

**口径**：D4 仅为独立 UI/协议检查通过；**不是** I1–I11 真实闭环通过。

## 5. 2026-10-24 第二批执行记录

真实 Runtime 的文件/网络/禁止子进程/取消/输出/清理重试探针通过；真实 Runtime 加测试 Gate 的编排集成通过。Claude 2.1.292 在同一严格 Runtime 下版本、帮助、非交互初始化和无上游取消清理通过。I2 真实模型/认证 MCP 任务、完整 I4/I5/I8/I9 及其余正式验收仍待执行。当前禁止创建子进程，不能据此声称完整 Agent 工具能力或进程树追踪已实现。命令与证据见 [研发 A 第二批记录](DEV-A.md#2026-10-24严格-runtime-与-claude-kit-第二批实现)。

## 2026-10-25 Wails v3 升级检查

D1 的绑定生成已执行；原生 WebView 冒烟通过生成绑定校验 DTO、业务错误、Go→JS 中文事件及最后窗口关闭回调。Go/CLI/前端 runtime 均固定 3.0.0-beta.34。命令与证据范围见 [升级验证](archive/WAILS-V3-ASSESSMENT.md)。本项未使用真实凭据、未运行真实模型/MCP 任务，也未人工选择系统目录，不能据此将 I1–I11 整体改为通过。

## 2026-10-25 联调准入缺陷修复

连接重定向/假成功、在途撤销、桌面准入与前端事件问题已修复并加入回归；退出失败可保留窗口重试，受限绑定仅七个方法。证据与范围见 [ABC 修复进展](archive/ABC-INTEGRATION-REVIEW.md#修复进展)。独立 Postman 上游模拟已实际收到真实 Go HTTP 客户端请求；前端 11 项测试及实际 WebView 关闭通过。上述证据不改变真实 I1/I2 和完整 I3–I11 待执行的状态。

## 2026-10-25 P0-C-DOC-3 草稿状态（产品 C）

| 编号 | 状态 | 说明 |
|---|---|---|
| I1 | 见下节 live | 原「待重测」；以 2026-10-30 DesktopAPI harness 为准 |
| I2 | 见下节 live | 同上 |
| 扩权 live | **待补（GUI）** | Modal 三问 + project 记住；harness 未覆盖 UI 点选 |
| I8 / I10 | 见下节 live | |
| I11 | **阻断** | 本机无 `wails3` 打包二进制；Shutdown 单测通过≠打包退出 |
| D4 | 通过（mock） | 见 §4.1；不计 I |

**口径**：mock ≠ I 通过。工程已具备：AllowDescendants 杀树、Claude Terminal、xterm、Gate `authorizeOutboundHost` + 记住主机种子。

## 2026-10-30 live harness（DesktopAPI · 本机）

环境：Darwin arm64；工作树相对 `4e2b2ba` + SyncBack 跳过符号链接修复；命令：`go run ./scripts/cmd/live-acceptance`（须本机 Keychain 已存模型）。Agent=`claude`；模型账号 host=`api.deepseek.com`（Key 未回显）。工作区=`demos/workspace`；`mountMode=clone`。

| 编号 | 状态 | 操作与证据（脱敏） |
|---|---|---|
| I1 | **通过** | `ListAgents` claude/codex available；`ListAccounts` models=2 mcps=2；无 Key 字段回显；`GetRuntimeHealth` available |
| I2 | **通过** | session `ff09dab2…` → running；clone 内 `summary.txt` 含 `harbor-live-i2`；Stop + cleanupComplete |
| B6 | **通过** | 同会话 `SyncBackWorkspace` 后源目录 `summary.txt` 含标记（见下日志）；修复：导回跳过 `.claude/debug/latest` 等符号链接，不再整单 `WORKSPACE_DENIED` |
| I8 | **通过** | 已停会话上重复 `StopSession` 幂等 |
| I10 | **通过** | 销毁后第二轮 `476f9aef…` 再次 running 并停净 |

```text
$ grep -n harbor-live-i2 demos/workspace/summary.txt
1:harbor-live-i2: 只读检查完成，hello.txt 存在（内容为 "Hello from Agent Harbor mock"）。
```

说明：本 harness 走 DesktopAPI 编排面，**不是** WebView 口播录像；扩权 Modal / 打包退出仍须 GUI 或 `wails3` 包补证。

## 2026-10-30 I3–I7 / I9 工程证据（平台探针 · 非正式 Agent 负例关单）

下列为**受控单测 / 平台探针**；不等于「真实 Agent 诱导越界」关单。I1/I2/B6/I8/I10 已有上节 live。

| 编号 | 状态 | 时间 | 操作与证据（脱敏） | 下一步 |
|---|---|---|---|---|
| I3 | **平台探针通过 / live 待补** | 2026-10-30 | Keychain fail-closed + Gate Sentinel；`go test ./internal/secrets ./internal/gate -count=1` | 会话内核对 Agent env 仅 Sentinel |
| I4 | **平台探针通过 / live 待补** | 2026-10-30 | `TestMacOSFileBoundaryAndNoFork`（`harbor_platform`）区外/symlink/fork→13 | 真实 Claude 读区外 + 脱敏审计 |
| I5 | **平台探针通过 / live 待补** | 2026-10-30 | `TestMacOSExactGateNetwork` 仅 Gate 端口可达 | live 诱导未授权出站 |
| I6 | **受控回归通过 / live 待补** | 2026-10-30 | `TestReviewConnectDoesNotSendKeyToRedirectOrigin`、`TestRevokeInvalidatesSentinel`、`TestMCPFiltersToolsListAndCall`、扩权编排 `TestPermission*` | GUI 扩权 Modal 负例 |
| I7 | **受控回归通过 / live 待补** | 2026-10-30 | `TestProvisionFailuresRollback`（health/detect/policy/start/create/exec…） | 组合故障注入 live |
| I9 | **受控回归通过 / live 待补** | 2026-10-30 | `TestCleanupFailureRetainsLockAndCanRetry`；`WORKSPACE_BUSY` 保锁用例 | 真实 Destroy 超时注入 |
| I11 | **阻断** | 2026-10-30 | `TestShutdownCancelsEverySessionBeforeWaiting` 单测 OK；无 `wails3` 包 | 安装 Wails 后 `dev-desktop`/`package` 退出无残留 |

### 导回成功态 / B6

live harness 已关（上节）。口播脚本仍见 [B6-SUCCESS-DEMO](../demos/B6-SUCCESS-DEMO.md)；填空 [syncback-evidence](../demos/syncback-evidence.md)。工程旁证：`TestSyncWorkspaceOverwritesAndKeepsExtra`、`TestSyncWorkspaceSkipsSymlinks`。