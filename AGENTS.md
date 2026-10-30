# AGENTS.md

本文件为 Codex、Claude Code、OpenCode、ZCode 等编码代理提供在本仓库中工作的项目级约束。

## 规范范围与继承

- 本文件适用于全仓库的服务、共享代码和工具目录。
- 处理服务目录下的文件时，同时遵守本文件和距离目标文件最近的服务级 `AGENTS.md`；未覆盖内容继承父级规范，冲突时以更近目录的规范为准。
- 跨代理通用的强制规则写入 `AGENTS.md`，不要只写入某一种代理的配置文件。
- 根目录和服务目录中的 `CLAUDE.md` 仅作为 Claude Code 入口，通过 `@AGENTS.md` 引入同目录规范，不重复维护通用规则。
- 修改规范文件时，保持上述入口和继承关系一致。

## 当前开发基线（三人两天版）

本节记录后续讨论形成的实施基线。技术栈、分工与两天范围如与现有文档或目录 README 冲突，以本节为准；下文完整 V1 的设计能力不自动纳入两天交付。

### 开发语言与技术栈

- **Go 为主语言**：实现 Runtime、Agent Kit、Orchestrator、Harbor Gate、Policy、Secrets 与桌面后端。业务模块在同一个 Go 应用中通过接口协作，不拆微服务。
- **桌面采用 Wails v3.0.0-beta.34**：前端使用简单 HTML/CSS/JavaScript 和团队熟悉的工具，不为两个页面额外引入复杂框架。Go 方法绑定与事件通道由研发实现，前端只调用受限的业务能力。
- **Python 用于安全验收与演示辅助**，不再并行建设一套 Python 业务后端，避免增加跨语言进程通信与打包成本。
- Go 后端统一使用 context 传播取消与超时；启动外部程序采用明确的可执行文件、参数与净化环境，不拼接 shell 命令。流式输出、进程树清理及失败关闭属于核心实现要求。
- 技术栈已替代此前 Electron + TypeScript 方案。目录骨架现为 `internal/{contracts,orchestrator,runtime,kits,gate,policy,secrets,desktop}/`、`frontend/`、`build/`，保留 `security/` 与 `demos/`；旧 src 骨架已移除。根 `main.go` 是薄入口，桌面绑定在 `internal/desktop`，Wails v3 配置在 `build/`。不再建立重复 cmd/harbor 入口。详细目标文件及历史映射见 [Go/Wails 目录设计](docs/DIRECTORY-STRUCTURE.md)。

### 团队分工

| 成员 | 主责 | 交付责任 |
|---|---|---|
| 研发 A：隔离、执行与技术集成 | Runtime、Agent Kit、Orchestrator、跨模块装配、安全负例 | 真实 Agent 受限运行；状态、回滚、停止和进程树清理完整；对集成与安全闭环负责 |
| 研发 B：网关、凭据与桌面后端 | Gate、Secrets、Policy、Wails 工程与 Go 绑定、构建环境 | 模型与真实认证 MCP 可用；凭据外置；桌面方法和事件可供前端调用；对协议与应用可运行负责 |
| 产品 C：产品与前端 | 范围收敛、配置页、工作台、前端联调、产品验收和演示 | 桌面操作可理解、状态准确、演示可重复；不承担隔离、MCP 协议、Go 编排或构建故障处理 |

准确接口以 [CONTRACT.md](docs/CONTRACT.md) 为准，模块边界、资源所有权、替身与文件归属以 [MODULE-DESIGN.md](docs/MODULE-DESIGN.md) 为准。A 首次落地共享 Go DTO 和消费边界，B 复核并生成 Wails 绑定，C 核对页面需求。Wails 绑定只校验并转交业务请求，会话状态机统一由 A 的 Orchestrator 实现，不在桌面绑定层重复维护。

### 当前范围增量（2026-10-25）

用户已授权实现 DeepSeek 官方 dsh 适配及 OpenAI Codex CLI 适配，覆盖下文历史两天范围中的 Agent 排除项。当前注册 Claude/Codex/dsh Kit；dsh 固定 npm `0.2.1-rc.2` 与 Node `v26.12.1`，Codex 使用 OpenAI Responses provider、私有 `CODEX_HOME` 与受控子进程 Runtime；受限文件工具、模型、认证 MCP、桌面选择和清理实现见 [dsh 适配](docs/DSH-INTEGRATION.md) 与 [Codex 适配](docs/CODEX-INTEGRATION.md)。Kit 经 `NeedsDescendants` / `NeedsChildProcesses` 显式声明执行面能力：Agent 会话可白名单内 fork（PTY/杀树），Codex 另开受控子进程路径；Health 仍 deny fork。工具面仍禁未授权 Bash/提权/动态插件（Claude `--tools Read,Write,Edit`）。真实 CLI 加受控服务通过不等于真实远程验收。

### 两天交付范围与检查点

- 核心闭环：宿主机界面连接模型与工具 → 选择工作区并提交任务 → 隔离启动真实 Agent → 经 Gate 调模型和认证 MCP → 展示结果 → 停止清理并可再次运行。
- 以单一 macOS/CPU 环境为目标，优先 Claude Code；一个已验证的模型协议组合、一个远程 Streamable HTTP 认证 MCP，同时仅一个会话，每会话一个非交互任务。
- 使用专用非敏感工作区，direct 模式明确提示会修改原文件；真实凭据只存宿主机内存，重启需重新输入。模型使用显式 Base URL 反向代理，不实现通用 HTTPS MITM。
- 历史「两天必交」曾裁剪多项；**当前 main 已合入全量 PAGE**：Codex/dsh、OAuth、宿主 stdio MCP、多会话≤2、clone/导回、Keychain、PTY/对话区等以 CONTRACT / PAGE-DESIGN 为准（勿再按旧两天裁剪砍功能）。
- 不因赶工删除真实 Agent、唯一认证 MCP、操作系统强制隔离、凭据外置或可靠清理；静态原型、curl 成功、模拟 MCP 不能冒充核心闭环验收。
- 首小时冻结目标环境、Agent/模型/MCP 组合、接口与演示任务；随后验证隔离、上游接入、Wails 构建/绑定/事件。首日结束前三人完成真实联调，第二天下午冻结功能并回归。
- 决策、接口和验收入口已建立：[DECISIONS.md](docs/DECISIONS.md)、[CONTRACT.md](docs/CONTRACT.md)、[ACCEPTANCE.md](docs/ACCEPTANCE.md)。版本、外部接入与隔离能力仍须实测，当前真实验证均待执行，不能凭设计推定通过。
- 并行开发采用同契约替身：C 使用浏览器独立 mock transport，A 注入 Gate/Policy/Catalog/Runtime/Kit fake，B 用 fake SessionService 测桌面桥。C 的浏览器 mock、A 的测试替身和 B 的桌面桥测试已经落地。替身不能进入真实验收或作为生产降级。接口变更先同步契约与所有受影响的真实/替身实现。
- Runtime.Create 在本版同时配置工作区、私有 HOME 与网络端口，不另设 MountWorkspace。Stop 异步确认清理；failed 且 cleanupComplete=true 释放锁，cleanupComplete=false 保留锁并允许重试清理。具体超时与幂等语义见接口契约，优先于下文长期 Runtime 摘要。

## 项目架构知识

### 定位与文档入口

Agent Harbor 是安全优先的桌面 Agent 沙箱：在隔离环境中运行 Claude Code、Codex、dsh，模型凭据与 MCP 密钥留在宿主机。当前仓库主要包含设计文档与 UI 原型；下述组件、目录与接口是设计契约或规划，不代表已经实现。

| 文档 | 阅读场景 |
|---|---|
| [模块设计与并行开发](docs/MODULE-DESIGN.md) | 当前模块边界、依赖、所有权、替身、共享文件和检查点 |
| [开工检查](docs/GO-CHECKLIST.md) | **本分支开工**：遗漏/未拍板收口、本周 ABC、禁止项 |
| [V1 总纲](docs/SYSTEM-DESIGN.md) | **本分支**：架构分层、模块所有权、ABC 分工；易混主责见 §6.1 |
| [接口契约](docs/CONTRACT.md) | **唯一方法/DTO 来源（v1.2 全量）**；含 OAuth/PTY/clone/SwitchModel |
| [页面设计](docs/PAGE-DESIGN.md) | **产品真源**：导航与各屏；屏↔API 见 §9 |
| [实施决策](docs/DECISIONS.md) / [验收清单](docs/ACCEPTANCE.md) | 环境/上游登记；独立开发与真实集成检查 |
| [Go/Wails 目录设计](docs/DIRECTORY-STRUCTURE.md) | 目录、模块职责、三人分工、依赖方向 |
| [产品总纲](docs/DESIGN.md) / [架构](docs/ARCHITECTURE.md) / [实现列表](docs/IMPLEMENTATION.md) | 威胁模型、控制/执行面、长期分期 |
| [文档归档](docs/archive/README.md) | Spike / 两天裁剪 / 旧任务卡 / 一次性联调；**不以归档为准开工** |

文档口径：隔离分期按 `IMPLEMENTATION.md` 与 `DESIGN.md` §7.4 / §7.6；**并发上限 2**、挂载含 `clone`、工具含 OAuth、工作台含对话区/WritePty，见 PAGE-DESIGN / CONTRACT v1.2。遇到未消解冲突先澄清，不自行把 MVP「延后」列当成已定砍法。

### 两平面与组件边界

- **控制面（宿主机，可信）**：Desktop App、Session Orchestrator、Policy Engine、Secrets Store、Harbor Gate，以及 Runtime 管控。真实密钥只留在宿主机钥匙串/密钥库及必要的持证进程内存。
- **执行面（沙箱，不可信）**：Agent Kit、Agent 及其子进程、授权工作区。配置只含 Gateway URL 与本会话 Sentinel，不得读取 Secrets。
- **分层**：L5 Desktop → L4 Orchestrator → L3 Policy / Secrets / Gate → L2 Runtime → L1 Agent / Kit。页面只调用编排层，不直接操作 Runtime 或沙箱；Runtime 执行隔离与网络强制，业务策略由 Policy 提供。

| 组件 | 职责与边界 |
|---|---|
| Session Orchestrator | 管理会话状态，校验启动条件，生成 Policy 快照，协调 Runtime、Gate 与 Kit；不长期存密钥 |
| Policy Engine | 管理授权目录、域名白名单、MCP 挂载与风险标记；不负责发 HTTP |
| Secrets Store | 封装宿主机钥匙串/密钥库；真实凭据仅供 Gate 使用，不向沙箱或 UI 回显 |
| Harbor Gate | Credential Proxy 与 MCP Gateway 的统称，是执行面的唯一出站闸口 |
| Credential Proxy | 强制代理 HTTP(S)，检查目标白名单，按会话将 Sentinel 替换为真实凭据；不解析 MCP 协议 |
| MCP Gateway | 沙箱唯一 MCP 入口，按会话暴露已授权工具，在宿主机持证访问上游 |
| Runtime | 创建隔离环境、挂载工作区、强制网络路径、执行与销毁；不解释业务策略 |
| Agent Kit | Agent 启动命令与无密钥配置模板，由编排层在启动时注入；默认不修改 Agent 源码 |

当前模块导航见 [Go 后端](internal/README.md) 和 [桌面前端](frontend/README.md)，各目录 README 已同步 Go/Wails 与三人分工。控制面 Gate、Policy、Secrets、桌面绑定，以及编排与 Runtime 已有实现；真实模型与 MCP 上游仍须联调，不能把单测当成验收通过。长期文档中的根级 `runtime/`、`host-services/`、`proxy/`、`app/` 等也是目录草案，不应建立重复组件。

### 完整 V1 会话与请求主路径

- **启动**：校验隔离能力、工作区、Agent、模型账号、并发与工作区冲突 → 生成 Policy 快照（workspace、mount_mode、allow_domains、mcp_ids、agent、model_account_id）→ 创建 Runtime → 启动 Gate → Kit 注入 Sentinel 与网关地址 → 执行 Agent。Runtime、Gate、注入全部就绪后才进入 `running`。
- **状态机**：`idle → provisioning → running → stopping → stopped → destroyed`；准备失败进入 `failed` 并回滚。扩权等待**不**增加快照态：会话保持 `running`，用 `permission_needed` 事件 + `ResolvePermission`（见 [CONTRACT](docs/CONTRACT.md)）。`stopped` 可保留运行数据并再次启动；销毁删除会话运行数据，保留宿主机密钥。
- **并发**：V1 同时运行最多 2 条（含扩权等待中的 `running`）；工作区解析为绝对路径并解析符号链接后互斥，同工作区禁止双开。同 Agent、不同工作区可并行，每会话独立 session_id、Sentinel、Policy、Runtime 与审计。
- **切换模型**：`switchModel(session_id, model_account_id)` 校验已连接账号，更新 Gate 绑定，仅影响后续出站；默认不重建沙箱、不改变 Agent 与工作区。
- **HTTP(S)**：执行面只能连接 Gate；Proxy 检查域名白名单，需要凭据时验证本会话 Sentinel，再由 Secrets 解析并代填真实密钥。不在白名单的访问触发扩权流程，未经批准不得转发。
- **MCP**：Agent 仅配置统一 Gateway URL 与 Sentinel；Gateway 校验会话和 MCP 挂载列表后持证访问上游。支持**远程 Streamable HTTP**（Token/OAuth）与**宿主侧 stdio**（本机进程 + 回环 HTTP 桥，`internal/mcpstdio`；密钥与进程留在宿主机，沙箱仍只见 Gateway）。禁止在沙箱内直接拉起 stdio 或写入真实密钥。
- **扩权**：只提供拒绝、仅本次、本项目始终允许；批准后更新相应作用域 Policy，再重新求值挂起请求。项目规则按工作区隔离，设置与权限一览共用存储，撤销即时生效；会话记忆在停止时清空。
- **事件与审计**：UI 以 Orchestrator 的状态、权限和审计事件为准，不自行推断隔离状态。审计记录 session_id、域名、工具名/计数和结果，不记录密钥、Authorization 原文或文件正文。

### Runtime 分期与安全不变量

- **P1**：工作区隔离 + 进程降权。Agent 不得以 root / Administrators 运行，不得借 sudo、docker 组等提权；Linux 使用 capability/seccomp 等约束，macOS 使用对应 sandbox profile。仅限制目录或拦截命令字符串不能作为隔离保证。
- **P2**：替换为容器 Runtime，强化 PID / mount / network 隔离；Gate、Policy、编排与页面主契约保持一致。microVM 属后续预研，不是当前一期必选后端。
- **可替换接口**：`Create(spec)`、`Exec(handle, cmd, env)`、`MountWorkspace(...)`、`Destroy(handle)`、`Health() → ok | fail_closed`；spec 包含 workspace、mount_mode、network_to_gate_only、privilege_profile。
- **失败关闭**：隔离或降权不可用必须拒绝启动，不得静默回落到宿主机直接执行。Agent 无法接入安全代理时，不得默认启用其安全模式。
- **密钥不入执行面**：禁止真实密钥进入沙箱文件、环境变量、Agent/MCP 配置、仓库及演示默认值。Sentinel 是可撤销的会话能力，使用至少 32 字节安全随机值，不能仅由 session_id 推导；只在本会话 Gate 有效，停止时撤销，不得通过日志或 UI 事件泄露。
- **文件与宿主管控隔离**：仅开放用户授权工作区及必要的只读运行时；默认屏蔽 `~/.ssh`、`~/.aws`、钥匙串与其他项目。禁止挂载宿主机 Docker socket 或暴露等价宿主管控通道。
- **挂载模式**：`clone` 为可写安全副本，结束后可导回；`direct` 直接改写授权目录。两者都不能保证授权工作区内容不被 Agent 修改或经已授权服务发送；P1 不承诺多租户硬隔离。
- **安全验收**：覆盖敏感路径不可读、环境与配置无真实密钥、未授权出站被拒、危险系统操作被阻止、隔离失败拒启、停止后无残留进程；完整安全验收参见产品总纲与实现列表。

### 完整 V1 页面与控制面的契约

- 新会话依次选择工作区、Agent、模型、工具（可跳过）；工作台以边界条、PTY 与活动摘要为主，不扩展为带文件树/编辑器的 IDE。
- Agent 为预置 Kit 的可用性检测；模型为自由添加的名称 + API 地址 + Key（模型页无浏览器 OAuth）；工具为 MCP 注册，支持 OAuth/SSO、API Key 或宿主 stdio。三者概念与数据源不可混用。
- 密钥只在宿主机「模型 / 工具」页录入，不在新会话向导或工作台录入，不回显完整密钥。默认设置通常只影响后续新会话，不隐式改写运行中会话。
- 编排层入口包括 `createSession`、`getSession` / `listSessions`、`switchModel`、`stopSession` / `destroySession`、`respondPermission`、`getBoundary`、`streamTerminal`、`onEvent`；具体签名以 [CONTRACT](docs/CONTRACT.md) 为准。
- **文档同步**：功能优化须同步更新受影响 md（至少 CONTRACT / ARCHITECTURE / PAGE-DESIGN / README / 角色 DEV；见 [GO-CHECKLIST §7](docs/GO-CHECKLIST.md)），避免代码与文档口径漂移。

## 语言要求

| 场景 | 语言 | 示例 |
|---|---|---|
| 回复、注释、commit、PR | 中文 | `// 检查域名是否存在` |
| 代码（函数名、变量名） | 英文 | `checkDomainExists` |
| 专有名词、缩写 | 英文 | HTTP、API |

## 开发原则

### 1. 先设计后编码
- 清晰描述实现方案后再编码
- 需求不明确时**先澄清**，不基于猜测编码

### 2. 任务分解
- 涉及 >3 个文件时，必须分解为子任务
- 按顺序逐个完成，避免大范围同时修改

### 3. 代码自审
- 检查逻辑、边界条件、错误处理
- 编写测试覆盖正常流程、边界、错误场景

### 4. Bug 修复（TDD）
1. 先写能重现 Bug 的测试
2. 确认测试失败
3. 修复代码
4. 确认测试通过
5. 确保不破坏其他测试

### 5. 持续学习规则

每次用户纠正 Claude 的错误后，需要：
- 在本章节下方的「经验教训」中添加新规则
- 规则应具体、可执行，防止类似问题再次发生

本次经验：上游 `BasePath` 与协议请求路径可能分别包含 `/v1`，连接探测和代理转发必须使用一次性路径拼接，避免生成 `/v1/v1/...`；配置文档中的 Origin 不应包含路径，前缀统一由 `BasePath` 表达。

CI 测试经验：新增真实 macOS 隔离测试时，必须兼容 CI runner 的 OS/CPU 前提；仅对明确的 `ISOLATION_UNAVAILABLE` 标记跳过，其他错误仍失败，不得放宽生产隔离校验。提交前运行 CI 同款 `go test -race ./...`，输出回调中的共享状态必须同步。

 ### 6. 自动更新文档
- 每次新开发的服务，代码，文档等需要及时总结更新CLAUDE.md

### 7. 文档精简高效
- 保持CLAUDE.md的行数在合理范围内，如果涉及更长篇幅的文档，需要作为子md文档，外链到CLAUDE.md中
