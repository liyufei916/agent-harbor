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

> 当时 D1 绑定未生成、Runtime 未实现。**现行**：以 §「2026-10-30 全量重跑」为准；勿用本节历史「全部待执行」覆盖现状。

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

连接重定向/假成功、在途撤销、桌面准入与前端事件问题已修复并加入回归；退出失败可保留窗口重试。**历史笔记曾写「受限绑定仅七个方法」；现行 DesktopAPI 方法面以 `binding_surface_test.go` 为准**（含 `ListMcpPresets`/`AddMcpPreset`），勿用旧句覆盖。证据与范围见 [ABC 修复进展](archive/ABC-INTEGRATION-REVIEW.md#修复进展)。独立 Postman 上游模拟已实际收到真实 Go HTTP 客户端请求；前端测试及实际 WebView 关闭通过。现行 live 以 §「2026-10-30 全量重跑」与 `demos/evidence/` 命令级日志为准。

## 2026-10-25 P0-C-DOC-3 草稿状态（产品 C）

| 编号 | 状态 | 说明 |
|---|---|---|
| I1–I11 / B6 | 见下节 **2026-10-30 全量重跑** | |
| 扩权主路径 | 见下节 / [permission-live-test](../demos/permission-live-test.md) | 白名单 + `PromptPermission` ask；**302 仅为异常项** |
| Apple 公证 | **阻断（账号）** | 见 `scripts/notarize-prep.md` |
| D4 | 通过（mock） | 见 §4.1；不计 I |

**口径**：mock ≠ I 通过。

## 2026-10-30 全量重跑（DesktopAPI harness + 平台探针 + 打包 .app）

命令：`export PATH="$HOME/go/bin:$PATH" && go run ./scripts/cmd/live-acceptance`  
环境：Darwin arm64；`wails3`=`v3.0.0-beta.34`（`$HOME/go/bin`）；已有 `bin/AgentHarbor.app`；Keychain 模型 host=`api.deepseek.com`（Key 未回显）；Agent=`claude`；`demos/workspace` + `clone`。

| 编号 | 状态 | 操作与证据（脱敏） |
|---|---|---|
| I1 | **通过** | models=2 mcps=2；keyHint 仅末四；Health seatbelt |
| I2 | **通过** | `57ab44ca…` → running → marker → 停净 |
| I3 | **通过** | 会话私有 HOME 无上游 Origin+Key；Gate `127.0.0.1`+Sentinel；overview 无密钥 |
| I4 | **通过** | `harbor_platform` FileBoundary；另 I4live：区外文件内容未进 summary |
| I5 | **通过** | `harbor_platform` ExactGate |
| I6 | **通过（受控+API）** | Gate Redirect/Revoke/MCPFilters；`GetPermissionOverview`+无挂起 `ResolvePermission→INVALID_ARGUMENT` |
| I7 | **通过** | Create `/System` → 异步 `WORKSPACE_DENIED` 已清理；另 ProvisionFailuresRollback 单测 |
| 扩权主路径 | **通过（API）** | overview + 无挂起 Resolve→INVALID_ARGUMENT；`PromptPermission` 已接线（GUI 点「请求扩权」补镜可选） |
| 扩权 302 异常项 | **可选未跑** | 跨主机重定向跟随；不阻断主验收 |
| B6 | **通过** | SyncBack 后源目录 `summary.txt` 含 `harbor-live-i2` |
| I8 | **通过** | 重复 Stop 幂等 |
| I9 | **通过** | 运行中同工作区 `WORKSPACE_BUSY`；另 CleanupFailureRetainsLock 单测 |
| I10 | **通过** | 第二轮 `b31e6965…` |
| I11 | **通过（未公证）** | `open bin/AgentHarbor.app` 后退出，无 AgentHarbor 主进程残留 |

```text
$ grep -n harbor-live-i2 demos/workspace/summary.txt
1:harbor-live-i2: …
```

仍开放：Apple 公证；扩权 GUI **人工补镜**（toast 已带主机结果文案；须 live 录屏）；302 异常项按需。导回成功 toast 提示源路径（不自动开访达）。口播见 [B6-SUCCESS-DEMO](../demos/B6-SUCCESS-DEMO.md)。亮点 mock 截图：[screenshots](../demos/screenshots/)。

## Codex / dsh 真实短任务（固定提交 · 本机 harness）

命令：`go run ./scripts/cmd/agent-task-evidence -agents=codex,dsh`  
填空与陌生用户路径：[stranger-first-run-record.md](../demos/stranger-first-run-record.md)。**通过项须含本机跑出的报告行**（勿用 Claude 项顶替）。

| Agent | 状态 | 证据（脱敏） |
|---|---|---|
| codex | **通过（2026-10-30）** | `go run ./scripts/cmd/agent-task-evidence -agents=codex -timeout=120s` → `session=9921bdbe… model=model_67… marker=harbor-agent-codex-42901 srcSummary=true SyncBack=ok`（证据写入提交：以首次合入本段的 commit 为准；固定评审提交 `803f4a4` 之后的含本段合入请回填 SHA） |
| dsh | **Detect 绿；live 任务待补** | 2026-10-30 已 `bash scripts/install-dsh.sh`；默认路径下桌面可无 env 自动 Detect（`ResolvePaths`）。**缺口**：尚无 `agent-task-evidence -agents=dsh` 通过报告行。须 Keychain 中 **勾选 dsh** 的模型（或 `dsh-model` profile）后重跑，将报告行与 commit SHA 写回本表。 |

陌生用户获取→首配→首任务：预发布 [v0.0.0-ci.8efc0d9](https://github.com/liyufei916/agent-harbor/releases/tag/v0.0.0-ci.8efc0d9)；正式 [v0.1.0](https://github.com/liyufei916/agent-harbor/releases/tag/v0.1.0)（打标后）。人工勾选表：[stranger-first-run-record.md](../demos/stranger-first-run-record.md)。
