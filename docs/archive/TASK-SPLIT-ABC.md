# A/B/C 任务拆分（对照第一版设计 × 当前实现）

| 项 | 内容 |
|---|---|
| 文档版本 | v2.0（**分支 feat/page-design-full：全量 PAGE-DESIGN**） |
| 日期 | 2026-10-25 |
| 基线代码 | 分支 `feat/page-design-full`；拉取后以 `git rev-parse HEAD` 为准 |
| 对照 | [SYSTEM-DESIGN](../SYSTEM-DESIGN.md) v2、[CONTRACT](../CONTRACT.md) **v1.2**、[PAGE-API-MAP](../PAGE-DESIGN.md#9-与架构-api-的绑定)、[PAGE-DESIGN](../PAGE-DESIGN.md)、[MODULE-DESIGN](../MODULE-DESIGN.md)、[MVP-2D3P](MVP-2D3P.md)、[ACCEPTANCE](../ACCEPTANCE.md)、[DECISIONS](../DECISIONS.md)、[ARCHITECTURE](../ARCHITECTURE.md) |

**读法**：开工前先过 [GO-CHECKLIST](../GO-CHECKLIST.md)（遗漏/未拍板收口）。产品真源是 **PAGE-DESIGN 全文**。再读 SYSTEM-DESIGN v2 + CONTRACT v1.2 → §3 全量并行实现 → §4 任务卡（**含** OAuth/clone/PTY）。P0 I 证据可并行，但**不得**再以排期裁掉首版能力。mock ≠ I 通过。

**文档分工**

| 层级 | 文档 |
|---|---|
| 整体架构 / 模块拆分 / ABC 边界 | **SYSTEM-DESIGN v2** |
| 接口定义（方法/DTO/事件/错误码） | **CONTRACT v1.2** |
| 哪一屏调哪个 API | **PAGE-API-MAP** |
| 包替身与文件归属 | MODULE-DESIGN |
| 交互与文案 | **PAGE-DESIGN（本分支真源）** |
| 谁何时交付 | **本文** |

**证据模板**（写入 ACCEPTANCE / 联调记录，截图前去凭据）：

```text
编号 / 日期 / 操作人(A|B|C) / commit / OS·Go·Wails·Agent 版本
模式：真实绑定 | 模拟
前置 / 操作 / 预期 / 实际（sessionId、state、cleanup、错误码）
状态：待执行 | 通过 | 失败 | 阻断
证据路径（脱敏）/ 下一步
```

---

## 0. 两层目标

| 层 | 含义 | 完成判据 |
|---|---|---|
| **P0 两天核心** | 桌面真连 → 隔离 Agent → 模型+认证 MCP → 停止清理 | D 项齐 + **I1–I11 真实证据** |
| **全量 PAGE（本分支）** | 首版导航+各屏：**含** OAuth / clone / PTY / 四步向导 / 扩权 / 多会话 | SYSTEM-DESIGN §4 + CONTRACT v1.2 + PAGE-DESIGN |

现状（`feat/page-design-full`）：契约 **v1.2** 与 SYSTEM-DESIGN **v2** 已定；C 全量 UI stub/mock 已推；live 新方法可先 `UNIMPLEMENTED`。

**优先级（本分支）**：**PAGE-DESIGN 完整体验 > 排期**；P0 I 证据可并行，但不得砍首版屏/能力。

---

## 0.5 已定文档入口

| 交付物 | 路径 | 状态 |
|---|---|---|
| 架构 + 模块拆分 + 分工 | [SYSTEM-DESIGN.md](../SYSTEM-DESIGN.md) v2 | **全量已定** |
| 接口全文 | [CONTRACT.md](../CONTRACT.md) **v1.2** | **已定**（含 OAuth/PTY/SwitchModel/clone） |
| 屏 ↔ API | [PAGE-API-MAP.md](../PAGE-DESIGN.md#9-与架构-api-的绑定) | **已定** |
| 绑定占位 | `internal/desktop` 新方法可 `UNIMPLEMENTED` | **已落**（含 PTY/OAuth） |
| C UI stub | `frontend/` 四步向导/双轨工具/工作台 PTY 区等 | **已落 mock** |

字段争议以 CONTRACT 为准改文档后再改代码。

### 复核结论（2026-10-25 · 全量）

| 项 | 结论 |
|---|---|
| 架构 / 接口 / 屏映射 | SYSTEM-DESIGN v2 + CONTRACT v1.2 + PAGE-API-MAP **已对齐全量** |
| 本文 §3–§4 | **本版改回**：OAuth/clone/PTY **全部开工**，不再 backlog |
| ABC 包所有权 | A 编排/Runtime/Kit/PTY·clone 真实现；B Gate/OAuth/账号/Prefs/绑定；C 全屏 UI+mock |
| 实现缺口 | Orch 多会话/扩权/PTY 真实现；Gate OAuth 回调；`wails3 generate` |
| GetRuntimeHealth | A 语义 · B 绑定 · C 消费 |
| 扩权 | B 触发事件 · A Resolve · C Modal |

---

## 1. 已完成（勿重复立项）

| 角色 | 已交付 |
|---|---|
| A | D2；严格单进程 Runtime/Claude Kit（探针）；dsh Kit 代码；测试 Gate 编排集成 |
| B | Gate/Policy/Secrets；Wails v3；bindings；live/`harbor:event`；MCP SSE；v1.2 DesktopAPI **方法占位**（含 OAuth/PTY） |
| C | **全量 PAGE stub/mock**（四步向导、OpenAPI 模型、工具双轨、工作台边界/PTY/扩权）；demos；单测 |

---

## 2. P0 — 两天必过（优先）

### 2.1 环境与工具链（阻塞 I1+）

#### P0-ENV-1 · 登记真实模型上游 · B（A 核对）

| | |
|---|---|
| **目标** | 固定 Anthropic 兼容 `/v1/messages` 上游，无占位默认 |
| **步骤** | 1）选定 HTTPS origin（仅 scheme+host[:port]） 2）设定 `HARBOR_CLAUDE_MODEL` 3）准备可轮换测试 Key（不入库） 4）写入 [DECISIONS](../DECISIONS.md) 5）用受控探测或桌面「连接模型」验证 |
| **DoD** | DECISIONS 有 origin+模型名；未设 env 时连接失败；设齐后 I1 可开始 |
| **证据** | DECISIONS  diff；脱敏连接成功截图（仅账号 id/host） |
| **关联** | 手册 P01–P04、M06；ACCEPTANCE I1 |

#### P0-ENV-2 · 选定真实只读 MCP · B/C

| | |
|---|---|
| **目标** | 一个 Bearer + Streamable HTTP `/mcp`（2025-03-26）服务 |
| **步骤** | 1）C 协助选只读工具场景 2）登记 `HARBOR_DEMO_MCP_ORIGIN` 3）`HARBOR_DEMO_MCP_TOOLS` 逗号白名单（真实工具名，勿写死 search） 4）测试 Token 仅本机 5）更新 demos 任务文案中的工具名 |
| **DoD** | tools 非空且 ValidConnection；`tools/list`+一次 `tools/call` 可在真连中出现 |
| **证据** | DECISIONS；白名单列表；脱敏审计一行 |
| **关联** | I1/I2；[DSH-INTEGRATION](../DSH-INTEGRATION.md) 若走 dsh |

#### P0-ENV-3 · Agent 本机 Detect 冻结 · A

| | |
|---|---|
| **目标** | 目标机 Agent 版本/路径可复现 |
| **步骤** | 1）记录 OS/CPU 2）Claude：确认 `--version` 为 **2.1.292 (Claude Code)** 或记阻断 3）若用 dsh：按文档 Node+版本 4）`HARBOR_CLAUDE_BIN` / dsh 安装路径 5）桌面「重新检测」截图 Available=true/false+reason |
| **DoD** | DECISIONS/DEV-A 有版本表；Available=false 时有明确下一步（升级/换机），不假装通过 |
| **证据** | `claude --version` 输出；Detect 截图 |
| **关联** | 手册 §2 版本约束；I2 |

#### P0-ENV-4 · 桌面 live 可重复启动 · B（C 跟做）

| | |
|---|---|
| **目标** | 任意开发机按同一命令起「真实绑定」窗 |
| **步骤** | 1）`go install …/wails3@v3.0.0-beta.34` 2）导出 P0-ENV-1/2 变量 3）`wails3 generate bindings ./...` 4）`npm ci --prefix frontend` 5）`wails3 dev` 6）确认顶栏「真实绑定」非「模拟模式」 |
| **DoD** | 书面命令可复制；bindings 存在且未手改；C 能独立复现 |
| **证据** | 命令+commit；窗口顶栏截图 |
| **关联** | README 研发 B 命令；手册 P04 |

---

### 2.2 独立检查收口

#### P0-D3 · Wails 契约面证据 · B

| | |
|---|---|
| **目标** | ACCEPTANCE D3 正式「通过」 |
| **步骤** | 对照 CONTRACT **v1.2 全方法面**（含 OAuth/PTY/SwitchModel 等）、Response.error、事件名 `harbor:event`、无原始 Go error 进 UI；补/贴已有测试与 WebView 冒烟 |
| **DoD** | ACCEPTANCE 增加 D3 证据段；状态=通过 |
| **证据** | 测试名+日志摘要；字段对照表 |
| **依赖** | 无 |

#### P0-D5 · 受控上游流与故障 · B

| | |
|---|---|
| **目标** | D5：流保留；401/断流/撤销/未知目标正确；非幂等不自动重试 |
| **步骤** | 用 Postman/受控 mock 或真实上游逐条打；记录码与是否代填 |
| **DoD** | ACCEPTANCE D5 证据段；每类故障至少一条 |
| **证据** | 用例表；脱敏响应码 |
| **依赖** | 受控上游或 P0-ENV |
| **关联** | 手册 M17+、gate 仿真测试 |

---

### 2.3 真实闭环 I1–I11（任务卡）

#### P0-I1 · 桌面真连模型+MCP · B/C

| | |
|---|---|
| **目标** | 关 mock；成功只返回账号引用；输入清空；失败不成可用账号 |
| **步骤（C 操作 / B 排障）** | 1）P0-ENV-4 起桌面 2）选 Agent 3）填显示名+真 Key → 连接模型 → 确认输入框空、状态含 id/host 无 Key 4）同法 MCP 5）故意错 Key → `UPSTREAM_AUTH_FAILED`（或契约码）且无账号 6）记 ACCEPTANCE §I1 |
| **DoD** | 正例+反例皆有；徽章=真实绑定 |
| **证据** | 成功/失败各一帧脱敏图 |
| **依赖** | P0-ENV-1/2/4 |
| **关联** | 手册 M06–M07 |

#### P0-I2 · 真 Agent 任务 + MCP · A/B/C

| | |
|---|---|
| **目标** | 演示目录内真实改文件 + 发现并调用白名单工具；非 curl/fixture |
| **步骤** | 1）`./demos/verify-workspace.sh` 2）替换默认任务（去掉 mock/search 字样，改用真实工具名） 3）选工作区绝对路径 4）开始 → 工作台看输出与审计 `allowed` 5）核对磁盘 `hello.txt`（或任务约定文件） 6）A 确认进程在隔离内 |
| **DoD** | 文件变更+工具审计+Agent 输出三者一致；记 I2 通过或失败原因 |
| **证据** | 工作区 diff；审计行；session 终态 |
| **依赖** | I1、P0-ENV-3；注意 Claude 无 Bash |
| **关联** | 手册 M08–M12 |

#### P0-I3 · Sentinel / 无真 Key 进执行面 · A/B

| | |
|---|---|
| **目标** | Agent env 与私有配置无上游真 Key；事件/日志无令牌 |
| **步骤** | 1）使用可轮换测试 Key 2）会话中由 A 查私有 HOME/env（工程师辅助，非 UI） 3）扫公开事件与日志 4）受控上游可回显认证头则核对仅为 Sentinel |
| **DoD** | 无真实 Key 落盘/进沙箱配置；ACCEPTANCE I3 |
| **证据** | 脱敏 env 摘要；检索脚本输出 |
| **依赖** | I2 环境 |
| **关联** | 手册 E02 |

#### P0-I4 · 文件隔离 · A

| | |
|---|---|
| **目标** | 工作区可读写；区外自建文件、凭据路径、越界链接拒绝 |
| **步骤** | 按 security 负例 + 受限进程探针；禁止打印用户真密钥 |
| **DoD** | 正负例成对；非「仅诊断模板」 |
| **证据** | 命令与拒绝原因 |
| **依赖** | Runtime |
| **关联** | 手册 M17、E 系列 |

#### P0-I5 · 网络隔离 · A/B

| | |
|---|---|
| **目标** | 仅精确 Gate 数据端口可达；公网/IP/其它回环拒绝由 Runtime 强制 |
| **步骤** | 受限进程访问 Gate vs example.com vs 其它 localhost；拒绝不得只靠 Gate 业务层 |
| **DoD** | I5 证据段 |
| **证据** | 探测结果表 |
| **依赖** | Runtime+Gate |
| **关联** | 手册 E01 |

#### P0-I6 · Gate 权限与重定向 · B

| | |
|---|---|
| **目标** | 错/撤 Sentinel、未授权工具、恶意目标、重定向不向它处代填 |
| **步骤** | 逐项注入；确认会话权限不可被 Agent 改写 |
| **DoD** | 每类至少一失败且无凭据泄漏 |
| **证据** | 拒绝日志（脱敏） |
| **依赖** | Gate；I1 后更佳 |
| **关联** | 手册 M17–M22 |

#### P0-I7 · 失败关闭与回滚 · A/B

| | |
|---|---|
| **目标** | profile 不可用、Gate 部分失败、Create/Exec 失败、运行中 Gate 致命 → 不直跑宿主机 |
| **步骤** | 工程师受控制造故障；核对状态机与资源清理 |
| **DoD** | 无「降级宿主机 shell」；晚返回资源也被收 |
| **证据** | 状态序列；进程列表前后 |
| **依赖** | A/B 故障注入能力 |
| **关联** | 手册 E03 |

#### P0-I8 · 停止与幂等 · A/C

| | |
|---|---|
| **目标** | provisioning/running 中 Stop；重复 Stop 幂等；终态事件确认清理 |
| **步骤（C）** | 开始后立刻停；跑起来再停；连续点两次停 **（A）** 确认授权撤销与进程结束 |
| **DoD** | UI 与事件一致；无卡死 busy |
| **证据** | 状态时间线截图 |
| **依赖** | 可创建会话 |
| **关联** | 手册 M13–M16 |

#### P0-I9 · 清理失败占锁 · A/B

| | |
|---|---|
| **目标** | Destroy 失败/超时 → `failed`+`cleanupComplete=false` 占锁；重试成功后才允许新会话 |
| **步骤** | 受控注入清理失败；C 见提示再点停止；二次成功后开新会话 |
| **DoD** | 占锁期间开始被拒；解锁后可开 |
| **证据** | cleanup 字段变化 |
| **依赖** | 故障注入 |
| **关联** | mock cleanup 仅作 UI 预习，不计本项通过 |

#### P0-I10 · 两轮可重跑 · A/B/C

| | |
|---|---|
| **目标** | 正常退出/取消/认证超时后再跑；连续两轮无残留进程或旧令牌 |
| **步骤** | 完整 I2 一轮 → 停净 → 再连（或沿用账号）再跑一轮；抽查进程与 secrets |
| **DoD** | 两轮均有输出/审计；无僵尸 Agent |
| **证据** | 两轮 sessionId；进程快照 |
| **依赖** | I1–I9 至少主路径可用 |
| **关联** | 手册 M11–M12、M23 |

#### P0-I11 · 打包运行与退出 · B/C

| | |
|---|---|
| **目标** | 登记机 `wails3 build` / `darwin:package`；退出触发 Shutdown，无残留 Agent |
| **步骤** | 1）生产构建 2）C 打开 .app 走缩略 I1/I2 3）运行中与空闲各退一次 4）查进程 |
| **DoD** | 资源可加载；Shutdown 证据；ACCEPTANCE I11 |
| **证据** | 构建日志；退出前后 `ps` |
| **依赖** | 绑定稳定；P0-ENV |
| **关联** | 手册 M23–M25 |

---

### 2.4 角色执行顺序（P0）

```text
共同: ENV-1 → ENV-2 → ENV-3 → ENV-4
B:    D3 → D5（可与 ENV 并行）→ 带 I1 → I6 → I11
C:    跟 I1 → I2 操作与 demos 真机话术 → I8/I10/I11 点选记录
A:    ENV-3 → I2 隔离确认 → I3/I4/I5 → I7/I9 → 支援 I8/I10
```

**C 额外交付（P0，非验收编号但必做）**

| ID | 任务 | DoD |
|---|---|---|
| P0-C-DOC-1 | `demos/README` 增加 live 启动、env、工作区路径、真实任务模板 | **已做**（2026-10-25）：新人按文档能开到真实绑定 |
| P0-C-DOC-2 | 10 分钟评审话术：架构一句 + I2 指文件/审计 + 一句「mock≠I」 | **已做**：demos「十分钟 live 话术」 |
| P0-C-DOC-3 | 将 I1/I2/I8/I10/I11 操作结果补进 ACCEPTANCE 或联调 RESULTS（脱敏） | 待真机：状态非「待执行」或标明阻断原因 |

---

## 3. 全量契约落地与并行实现（CONTRACT v1.2）

方案见 [SYSTEM-DESIGN v2](../SYSTEM-DESIGN.md) + [CONTRACT v1.2](../CONTRACT.md) + [PAGE-DESIGN](../PAGE-DESIGN.md)。

### 3.1 裁定摘要（本分支全部做）

| ID | 能力 | 裁定 | 主责 |
|---|---|---|---|
| API-01 | GetRuntimeHealth | **做** | A + B 绑定 |
| API-02 | ListSessions / DestroySession；上限 2；WORKSPACE_BUSY | **做** | A |
| API-03 | GetPermissionOverview + 记住主机 | **做** | B |
| API-04 | permission_needed + ResolvePermission | **做** | B·A·C |
| API-05 | ListAccounts / RemoveAccount；ConnectModel.origin | **做** | B + C |
| API-06 | 多 MCP Token + **OAuth**（Start/CancelMcpOAuth） | **做** | B + C |
| API-07 | mountMode `direct` **与** `clone` | **做** | A + C 向导 |
| API-08 | Get/SetUIPrefs | **做** | B |
| API-09 | WritePty / ResizePty + 事件 `pty` | **做** | A + B 桥 + C UI |
| API-10 | 设置页接已有 API | **做** | C |
| API-11 | SwitchModel | **做** | A/B + C 工作台 |

### 3.2 并行开工

```text
CONTRACT v1.2（全量）
  ├─ A：多会话/Destroy/ResolvePermission；clone 挂载；PTY 真实现；Health
  ├─ B：generate；OAuth 回调；Overview/Accounts/origin/Prefs；PTY 事件桥；去 UNIMPLEMENTED
  └─ C：PAGE 全屏（已 stub）→ 打磨 + mock 并发/OAuth/PTY 流
```

### 3.3 任务卡

#### FULL-DTO · contracts v1.2 · A/C 核对 · **文档+DTO 已完成**

| | |
|---|---|
| **目标** | DTO/错误码/事件含 OAuth、PTY、SwitchModel、clone |
| **DoD** | `internal/contracts` 与 CONTRACT v1.2 一致 |
| **状态** | 2026-10-25 已写入（本分支） |

#### FULL-BIND · 绑定生成与去占位 · B · 估 1–2d

| | |
|---|---|
| **目标** | `wails3 generate`；v1.2 方法从 UNIMPLEMENTED 接到实现 |
| **DoD** | C live 不因缺方法崩 |
| **依赖** | FULL-DTO |

#### FULL-MOCK · mock 全量 · C · 估 1d · **大部分已做**

| | |
|---|---|
| **目标** | mock 覆盖 v1.2 DesktopAPI（含 OAuth/PTY/SwitchModel） |
| **DoD** | `npm test` 过；浏览器可走通向导→工作台→扩权/PTY |
| **状态** | stub 已推；继续补并发与空态 |

#### FULL-ORCH · 多会话 / clone / 扩权 / PTY · A · 估 5–10d

| | |
|---|---|
| **目标** | 上限 2、同目录冲突、Destroy、ResolvePermission、clone 挂载、PTY 会话 |
| **DoD** | 单测 + 与 B 联调扩权/PTY 各一条 |
| **依赖** | FULL-DTO |

#### FULL-GATE · 一览/账号/origin/OAuth/Prefs · B · 估 5–8d

| | |
|---|---|
| **目标** | Overview、记住主机、Accounts、自定义 Origin、**OAuth 回调**、UIPrefs、PTY 事件转发 |
| **DoD** | 无 Key 进事件；OAuth 至少一条 mock→真通路径文档化 |
| **依赖** | FULL-BIND |

---

## 4. 页面与后端任务卡（全量 PAGE）

规则：接口以 **CONTRACT v1.2** 为准；**OAuth / clone / PTY 均在范围内**。估时仅供参考，本分支不因估时砍范围。

### 4.1 产品 C

#### P1-C-01 · 全局壳与导航 · 估 2–3d · **UI stub 已开工（2026-10-25）**

| | |
|---|---|
| **目标** | 侧栏 HOME / 模型 / 工具 / 设置；底栏隔离状态灯；收缩+记忆 |
| **步骤** | 1）路由/视图骨架 2）状态灯订阅健康 API（可先 stub） 3）`⌘B` 收缩 4）进工作台可自动缩一次 |
| **DoD** | 四入口可切换；收缩后内容区变宽；偏好重启仍在 |
| **依赖** | B：偏好存储位置约定；A：健康检查字段 |
| **参考** | PAGE §4 |
| **进度** | 壳+灯+收缩已落地；接 `getRuntimeHealth`（契约已定）；Prefs 可先 localStorage，后换 `Get/SetUIPrefs` |

#### P1-C-02 · Onboarding · 估 1–2d · **UI stub 已开工**

| | |
|---|---|
| **目标** | 一屏三句 + 隔离检测；失败主按钮不可用；**无跳过** |
| **步骤** | 调检测 API → ●/○ → 进入 HOME 或展示修复链 |
| **DoD** | 检测失败无法进入主路径 |
| **依赖** | A 健康/隔离探测 API |
| **参考** | PAGE §5.1 |
| **进度** | `views/onboarding.js`；失败禁用「进入 HOME」 |

#### P1-C-03 · HOME 会话中心 · 估 3–5d · **stub → 接 listSessions**

| | |
|---|---|
| **目标** | 列表字段、运行中 **x/2**、打开工作台/停止/再次开始/销毁 |
| **步骤** | 列表 UI → `listSessions`/`destroySession` → `SESSION_LIMIT`/`WORKSPACE_BUSY` 文案 |
| **DoD** | 达上限与同目录冲突有产品文案；销毁二次确认 |
| **依赖** | **FULL-ORCH / P1-A-01**（mock 可先演示） |
| **参考** | PAGE §5.2 |
| **进度** | stub：`sessionHistory` + mock `listSessions`；文案 x/2；销毁确认已有 |

#### P1-C-04 · 新会话 Wizard · 估 3–4d · **合并配置流 stub**

| | |
|---|---|
| **目标** | 工作区→Agent→模型→工具（可跳过）；**direct 与 clone** |
| **步骤** | 四步校验 → CreateSession（含 mountMode）；工具可跳过 |
| **DoD** | 可跳过工具；direct 强警告；clone 可选且文案正确 |
| **依赖** | Connect API；clone 真实现靠 A（UI 可先 mock） |
| **参考** | PAGE §5.3 |
| **进度** | **真四步 stub 已落**（本分支） |

#### P1-C-05 · 独立模型页 · 估 3–4d · **固定 profile stub**

| | |
|---|---|
| **目标** | 自由添加 OpenAPI（名+地址+Key）；列表无「未配置」占位 |
| **步骤** | CONTRACT 多账号 → UI CRUD → 提交清空 Key → ▲ 无效 |
| **DoD** | 不再写死唯一 demo-model（或并存迁移开关） |
| **依赖** | **P1-B-02** |
| **参考** | PAGE §2 |
| **进度** | `views/models.js` 自由 OpenAPI（名+地址+Key）stub 已落 |

#### P1-C-06 · 独立工具页 · 估 3–5d · **双轨（Token + OAuth）**

| | |
|---|---|
| **目标** | OAuth/SSO **或** Token；列表只展示已添加；删除 |
| **步骤** | 双轨表单 → `connectMcp` / `startMcpOAuth` → 列表 |
| **DoD** | 无密钥回显；两条轨 UI 齐全；OAuth 与 B 回调联调 |
| **依赖** | FULL-GATE（OAuth）；mock 可先行 |
| **参考** | PAGE §5.6 |
| **进度** | `views/tools.js` 双轨 stub 已落 |

#### P1-C-07 · 设置页 · 估 2–3d · **检测/场景 stub**

| | |
|---|---|
| **目标** | Agent 重检、会话默认、扩权策略入口、清理、关于 |
| **DoD** | 与扩权/清理后端字段对齐；无空壳开关 |
| **依赖** | A/B 设置相关 API |
| **进度** | Agent 重检 + mock 场景 + 重置引导；记住主机入口待接 API-03 |
| **参考** | PAGE 设置；API-10 |

#### P1-C-08 · 工作台增强 · 估 5–8d（**含 PTY**）

| | |
|---|---|
| **目标** | 边界条；当前模型切换；权限一览；**内嵌 PTY**；右侧摘要；顶栏/焦点切换 |
| **步骤** | 边界条 → `switchModel` → Overview → PTY 输入/事件 → 扩权 Modal |
| **DoD** | 对照 PAGE §5.4；PTY 区可演示（mock 或 live） |
| **依赖** | FULL-GATE / FULL-ORCH；API-09 |
| **参考** | PAGE §5.4 |
| **进度** | stub 已落（边界/PTY/扩权演示） |

#### P1-C-09 · 扩权 Modal · 估 2–3d

| | |
|---|---|
| **目标** | 一句话谁访什么；拒绝 / 仅本次 / 本项目始终；标会话；**无「全网」** |
| **DoD** | 三按钮都有后端回执；多会话标明 sessionId |
| **依赖** | **P1-B-04**、**P1-A-05** |
| **参考** | PAGE 系统打断 |

#### P1-C-10 · 隔离阻断页 · 估 1d

| | |
|---|---|
| **目标** | 隔离不可用全页说明；禁新会话 |
| **DoD** | 与状态灯红一致；无「仍要继续」 |
| **依赖** | A 探测 |
| **参考** | PAGE |

#### P1-C-11 · 视觉对齐原型 · 估 2d

| | |
|---|---|
| **目标** | 品牌/文案/密度贴近 `prototype/`，仍非 IDE |
| **DoD** | 对照 PAGE §3 清单自检通过 |
| **依赖** | 壳稳定（P1-C-01） |

---

### 4.2 研发 B

#### P1-B-01 · Keychain 持久化 · 估 3–5d

| | |
|---|---|
| **目标** | 密钥进系统钥匙串；重启可恢复引用；禁明文配置 |
| **DoD** | 进程外无 Key 文件；轮换/删除路径可用 |
| **参考** | DESIGN §11 |

#### P1-B-02 · 多模型账号 · 估 3–4d

| | |
|---|---|
| **目标** | 用户自填 Base URL+Key；仍只经 Gate；CONTRACT 扩展 |
| **DoD** | 前端不直连上游；策略校验 origin |
| **参考** | PAGE 模型 |

#### P1-B-03 · 多 MCP Token + OAuth · 估 5–8d · **全量**

| | |
|---|---|
| **目标** | 多 MCP；`mcpIds` 0..8；Token 轨 + **StartMcpOAuth/CancelMcpOAuth** 真通至少一条 |
| **DoD** | CreateSession 接受多 id；OAuth 回调写入账号目录；无 Key 进事件 |
| **参考** | CONTRACT v1.2；PAGE 工具双轨 |

#### P1-B-04 · 扩权策略引擎 · 估 4–6d

| | |
|---|---|
| **目标** | 挂起事件；仅本次/项目级持久；可撤销 |
| **DoD** | Agent 无法改写已持久策略；有审计 |
| **参考** | ARCHITECTURE |

#### P1-B-05 · 权限一览 API · 估 2–3d

| | |
|---|---|
| **目标** | 返回工作区、允许域名、MCP、禁止路径（只读） |
| **DoD** | C 可绑边界条；无密钥字段 |
| **参考** | DESIGN §4 |
| **可先** | mock 固定 JSON 解耦 C |

#### P1-B-06 · 安装与更新 · 估 3–5d（可后置）

| | |
|---|---|
| **目标** | 签名/公证/更新通道 |
| **DoD** | 内测机器可装；文档化限制 |
| **参考** | DESIGN M4 |

---

### 4.3 研发 A

#### P1-A-01 · 多会话编排 · 估 5–8d

| | |
|---|---|
| **目标** | running≤2；同工作区禁双开；列表/销毁语义 |
| **步骤** | CONTRACT 会话列表 API → 锁与冲突错误码 → 测试 |
| **DoD** | 单测覆盖上限与同路径；C 可接 HOME |
| **参考** | PAGE §5.2.1 |

#### P1-A-02 · clone 工作区 · 估 4–6d · **全量必做**

| | |
|---|---|
| **目标** | `mountMode=clone` 安全副本 + 可选导回 |
| **DoD** | direct/clone 切换有警告；与 C 向导联调 |
| **依赖** | 与 P1-C-04 同步 |
| **参考** | PAGE §5.3；CONTRACT v1.2 |

#### P1-A-03 · Codex + dsh 真机验收 · 估 3–5d（dsh）+ Codex 另估

| | |
|---|---|
| **目标** | dsh 按 [DSH-INTEGRATION](../DSH-INTEGRATION.md) 全手册过；Codex Kit 新増 |
| **DoD** | DECISIONS 登记版本；至少一条真任务证据 |
| **参考** | DESIGN Agent P0 |

#### P1-A-04 · 子进程方案 · 估 大（门禁）

| | |
|---|---|
| **目标** | 若核心任务需要 Bash/子进程：可靠追踪+清理后再放行 |
| **DoD** | 设计评审通过；全量 Runtime/安全重测；**禁止**偷偷开 fork |
| **参考** | DEV-A 约束 |

#### P1-A-05 · 扩权挂起状态机 · 估 3–4d

| | |
|---|---|
| **目标** | 会话保持 `running`；发 `permission_needed`；`ResolvePermission` 恢复/拒绝 |
| **DoD** | 与 P1-B-04/C-09 联调一条完整扩权；**不**新增 snapshot.state 枚举值 |
| **参考** | CONTRACT v1.2 事件节 |

#### P1-A-08 · 内嵌 PTY · 估 4–6d · **全量必做**（含于 FULL-ORCH）

| | |
|---|---|
| **目标** | 焦点会话可 `WritePty`/`ResizePty`；事件 `pty` 流式到 UI |
| **步骤** | Runtime PTY → Orch 绑定会话 → B desktop 转发 → 与 C 工作台联调 |
| **DoD** | mock 与 live 各至少一条可打字回显（live 可后于 stub）；终态关闭 PTY |
| **依赖** | FULL-BIND（B 桥）；P1-C-08 |
| **参考** | CONTRACT API-09；PAGE §5.4 |
| **边界** | **A 主实现**；B 只做绑定/事件桥；C 只做终端区 UI |

#### P1-A-06 · 一期隔离证据包 · 估 2–3d

| | |
|---|---|
| **目标** | 证明「工作区+降权」而非仅目录限制文案 |
| **DoD** | 对外一页证据索引；挂 ACCEPTANCE/security |
| **参考** | DESIGN §7.6 |

#### P1-A-07 · 二期容器/microVM · backlog

| | |
|---|---|
| **目标** | 不挡 P0/P1 桌面主路径；单独里程碑 |
| **DoD** | 仅预研/Spike 记录即可 |

---

## 5. 排期与依赖图

```text
P0-ENV-1/2/3/4
    └─► P0-I1 ─► P0-I2 ─► P0-I3
              │         ├─► P0-I4 / I5
              │         └─► P0-I10
              ├─► P0-I6
              ├─► P0-I8
              └─► P0-I7 / I9（故障注入可并行准备）
P0-D3 / D5 可与 ENV 并行
P0-I11 ← 绑定稳定 + I1 缩略路径

CONTRACT v1.2（全量，本分支）
  ├─► FULL-MOCK (C) ──► P1-C-* 全屏打磨
  ├─► FULL-BIND (B) ──► FULL-GATE（含 OAuth）
  └─► FULL-ORCH (A) ──► P1-A-01/02/05/08（多会话/clone/扩权/PTY）
P1-B-05 + FULL-GATE ──► P1-C-08
P1-A-05 + Gate 扩权 ──► P1-C-09
P1-A-02 clone ──► P1-C-04
P1-A-08 / API-09 PTY ──► P1-C-08
P1-B-03 OAuth ──► P1-C-06
```

**本分支不因时间删减**：clone / PTY / OAuth **均保留**。  
**不可删**：真 Agent、真认证 MCP、隔离强制、凭据外置、停止清理、CONTRACT 形状一致。

易混主责见 [SYSTEM-DESIGN §6.1](../SYSTEM-DESIGN.md)。

---

## 6. 分工边界

| 角色 | 可改 | 不可改 |
|---|---|---|
| A | `orchestrator` / `runtime` / `kits`、security、**CONTRACT 主笔**；**PTY·clone 真实现** | 手改 bindings；为赶演示放宽隔离；实现 MCP OAuth |
| B | `gate` / `policy` / `secrets`、desktop、构建、绑定生成；**OAuth 回调**；PTY **事件桥** | 第二套会话状态机；前端绕过 Gate；在 Runtime 里开伪终端 |
| C | `frontend/` 手写、`demos/`、验收记录、**字段需求提案** | `frontend/bindings/`、`internal/`；把 mock 记为 I 通过 |

---

## 7. 周会检查清单

**P0**
- [ ] ENV-1/2 写入 DECISIONS
- [ ] ENV-3 Detect 结果（通过或阻断原因）
- [ ] ENV-4 三人都能 `wails3 dev` 见真实绑定
- [ ] D3/D5 证据段
- [ ] I1 / I2
- [ ] I3–I7 各 ≥1 条真实负例
- [ ] I8 / I9 / I10 / I11
- [ ] P0-C-DOC-1/2/3

**全量契约落地（本分支）**
- [x] SYSTEM-DESIGN v2 / CONTRACT v1.2 / PAGE-API-MAP  
- [x] `internal/contracts` + desktop 占位（含 OAuth/PTY）  
- [x] FULL-MOCK 骨架（C UI stub）  
- [x] FULL-BIND 去占位子集（B：Health/会话/账号/Prefs/OAuth/SwitchModel；PTY 桥已绑，真终端仍靠 A）  
- [~] FULL-ORCH 多会话/clone/扩权（A 已合）；**真 PTY/Resize 仍 UNIMPLEMENTED**  
- [x] FULL-GATE 一览/账号/origin/OAuth/Prefs/Keychain/扩权跟随+DNS（B）  

**全量 PAGE 产品**
- [x] 侧栏 HOME/模型/工具/设置 + 顶栏会话切换（stub）  
- [x] 四步向导 / 模型 OpenAPI / 工具双轨 / 工作台 PTY·扩权（stub）  
- [x] HOME mock `listSessions` / 上限文案；live 待 I1  
- [ ] OAuth / clone / PTY **live** 各至少一条  
- [ ] 第二 Agent 真机一条路径  

---

## 8. 相关入口

| 文档 | 用途 |
|---|---|
| [SYSTEM-DESIGN.md](../SYSTEM-DESIGN.md) | **架构 + 模块拆分 + ABC（v2 全量）** |
| [CONTRACT.md](../CONTRACT.md) | **接口唯一来源（v1.2 全量）** |
| [PAGE-API-MAP.md](../PAGE-DESIGN.md#9-与架构-api-的绑定) | 屏 ↔ API |
| [PAGE-DESIGN.md](../PAGE-DESIGN.md) | **本分支产品真源** |
| [MODULE-DESIGN.md](../MODULE-DESIGN.md) | 替身与文件归属 |
| [ARCHITECTURE.md](../ARCHITECTURE.md) | 长期控制/执行面 |
| [ACCEPTANCE.md](../ACCEPTANCE.md) | 编号与证据格式 |
| [MANUAL-INTEGRATION-TEST.md](MANUAL-INTEGRATION-TEST.md) | 逐步人工用例 |
| [DECISIONS.md](../DECISIONS.md) | 上游与版本登记 |
| [GO-CHECKLIST.md](../GO-CHECKLIST.md) | **开工检查与未拍板收口** |
| [DEV-A.md](../DEV-A.md) / [DEV-B.md](../DEV-B.md) / [DEV-C.md](../DEV-C.md) | 角色笔记 |
| [demos/README.md](../demos/README.md) | 演示与工作区 |
