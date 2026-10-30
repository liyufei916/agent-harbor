# 研发 A 开发记录

> **交付状态**：现行验收以 [ACCEPTANCE.md](ACCEPTANCE.md) 为准。2026-10-25 点状交付表已归档：[A-DELIVERY-STATUS](archive/A-DELIVERY-STATUS.md)。

## 运行中切换模型账号

Orchestrator.SwitchModel 校验会话仍 running、账号存在且为 model，并要求新旧账号 ProfileID/Host 一致。Gate.SwitchModel 只在宿主内存中原子更新模型账号引用，不调用上游或回调编排；每个请求先捕获自己的账号，因此在途请求继续使用旧账号，后续请求使用新账号。工作区、Agent、Sentinel、隔离进程和固定模型路由均不变。失败不会改写编排中的当前账号。桌面 SwitchModel 已转发编排。

目前是兼容 profile 内的账号切换，不支持跨协议、跨 profile 或修改 CLI 固定模型名。后续若支持不同模型名，需增加明确的 Kit/Gate 消息重写契约，不能仅替换 Key 冒充模型切换。

测试验证正常切换、错误账号/种类、非 running 拒绝、Gate 更新失败保留原账号、桌面转发；内存 HTTP transport 验证在途旧请求与后续新请求的凭据不混用、撤销后拒绝切换。此验证不调用真实模型服务。

## 扩权等待与决策生命周期

A 已实现 `RequestPermission`/`ResolvePermission`，等待期间保持 running；用 `permission_needed` 告知 UI，允许 deny/once/project，重复决策和无挂起请求返回 INVALID_ARGUMENT。按会话及精确主机隔离等待，最多 8 个，同一目标不重复挂起；请求取消、超时、Stop、Shutdown 均通过 context 解除等待。Sentinel、URL、通配符、本机地址不能进入扩权载荷。

回调通过 GateSpec 注入，桌面 ResolvePermission 仅做错误映射和转发。A 不在编排层存项目白名单，也不直接转发网络请求。B 尚须接入 Gate 真实触发、DNS/IP 校验、once 单次消费和按原工作区持久化 project 规则，再重新求值请求；当前原有拒绝逻辑保持有效，不能称为真实扩权闭环通过。

测试覆盖三种决策、running 状态不变、重复/过期决策、跨会话拒绝、Stop/请求取消解除等待、非法目标及桌面转发。

## 2026-10-24：共享契约与编排首批实现

工作树基于 `ce2d47b`，本轮尚未提交。下节记录首批成果；最新 Runtime/Kit 实现与限制见文末第二批记录。当前不是可运行桌面应用，也不是研发 A 全部交付完成。

按三个子任务顺序实施：契约类型与消费接口 → 编排及替身测试 → Seatbelt 初探与交接记录。

### 已落地

- `internal/contracts`：按 CONTRACT v1 定义 DTO、响应、事件、状态和事件名。所有公开错误使用固定模板，不复用提供方错误原文。
- `internal/orchestrator`：`New(appContext, Dependencies, sink, Timeouts)` 返回实现 SessionService 的服务；注入的 Catalog 仅有 Lookup，编排没有 Resolve 能力。
- 原子单会话占用；Create 快速返回 provisioning；请求 context 返回/取消不取消后台任务；Get 返回独立错误副本；Stop 快速返回 stopping。
- 准备失败回滚；停止接管晚返回 Gate/Runtime/Process；Gate 致命故障、非零退出、准备超时和会话总时限均清理。先取消并撤销 Gate，再 Destroy，再确认 Wait，最后发布终态。
- 清理使用独立 5 秒预算，为 Runtime 留出至少一半预算。清理未确认时 failed、cleanupComplete=false 并保留锁，Stop 可重试；清理完成才允许再次创建。
- 事件串行分配 seq，早到输出排在 running 后、最终输出排在终态前；输出块最多 16 KiB，输出缓冲限 1 MiB 并显示截断提示；慢 sink 不阻塞输出读取。sink 在独立分发协程执行，必须最终返回；Shutdown 等待事件排空，受调用 context 限制。
- stdout/stderr 分别跨块过滤已知 Sentinel，残留的令牌前缀也隐藏。编排不持有上游真实密钥，B 仍须在 Gate 边界过滤已知真实凭据；不能据此宣称能识别任意敏感内容。
- Runtime/Kit 接口、Gate/Policy 导出数据类型按契约拆包；测试替身仅在 `*_test.go`，没有生产替身或宿主机直跑降级。

### B 接入说明

module 使用仓库已有建议值 `github.com/liyufei916/agent-harbor`。因尚无 go.mod，为编译 A 的包加入最小 module；`go 1.23.0` 是当前源码的暂定最低语言基线，本机实际使用 Go 1.27.2，不代表已经验证 Go 1.23。Wails 初始化及依赖仍由 B 接续。

`internal/gate/types.go` 和 `internal/policy/types.go` 只落地共享边界类型，未实现 B 的业务。B 直接使用这些类型，不另复制。具体消费接口在 `internal/orchestrator/ports.go`，方法签名沿用 CONTRACT v1，无协议变更。

框架启动时创建具体组件、将应用级 context 注入 New；sink 把 Event 转发为 `contracts.EventName`。退出必须用新的、有清理预算的 context 调用 Shutdown 并处理错误。不要把 Runtime/Gate/Secrets 绑定到 Wails。

Gate、Policy、Catalog 的真实实现和 Runtime/Claude Kit 的具体实现尚未接入，因此本轮没有生产装配或桌面启动命令。60 秒上游无数据超时由流式 Gate 负责，当前编排测试不证明这一行为。

### 实际验证

环境：macOS 26.7（25G229）、arm64、非 root UID 503、Go 1.27.2 darwin/arm64。Claude Code `2.1.292`，原生安装路径 `/opt/homebrew/Caskroom/claude-code/2.1.292/claude`；只执行了 `--version` 与 `--help`，未读取登录态或调用模型。

```sh
go test -race ./...
go test -race -count=10 ./internal/orchestrator
go vet ./...
python3 security/seatbelt_probe.py
```

Go 编排测试通过，覆盖正常生命周期、并发拒绝、九个准备失败点、晚到资源取消竞争、Gate/Runtime 清理失败与重试、非零退出、Gate 致命故障、两个超时、应用取消、账号类型校验、快照防修改、审计脱敏、跨块令牌过滤、UTF-8 块边界及慢 UI 截断。

Seatbelt 诊断使用临时非敏感目录与两个本机受控监听器：工作区读写、指定回环端口访问成功；工作区外读写、越界符号链接、其他回环端口均拒绝，七项通过。路径含空格、双引号和中文。脚本不读取真实凭据，不向公网发送请求，所有临时文件和监听器在结束时清理。

诊断过程发现：最初极简限制规则运行 true 异常退出（134）；导入系统基础规则后可运行；本机 SBPL 的 `remote tcp` 拒绝字面量 IPv4，需使用 localhost；系统 curl 还需读取固定 OpenSSL 配置文件。编译失败或进程崩溃不能被脚本记为拒绝测试通过。

**诊断模板不是生产 profile**：它导入了较宽的 `system.sb`（包含 IPC、系统路径等授权），网络使用 `localhost:port`，尚未证明契约要求的精确 `127.0.0.1:port` 限制，也未证明宿主管控 IPC、提权与逃离进程组的后代清理。不得复制该模板用于真实 Agent。

### 后续实施顺序

1. 收敛 macOS 生产 profile；验证精确地址/端口、敏感路径、必要运行路径及 IPC/提权限制；实现可靠进程树确认，不能只向进程组发送信号就报告成功。
2. 实现 Runtime 的私有 HOME、净化环境、相对配置路径校验、配置权限、流式读取和幂等 Destroy；隔离不可用时 Health 拒绝启动。
3. 在受限环境验证固定 Claude 版本的非交互参数、显式模型 Base URL、strict MCP 配置和 Sentinel；完成 Kit 实现。CLI 参数存在不等于端到端兼容。
4. 接入 B 的真实 Gate/Policy/Catalog，完成真实模型和认证 MCP 联调，再执行 ACCEPTANCE 的 I1–I11。当前这些真实集成项全部待执行。

## 2026-10-24：严格 Runtime 与 Claude Kit 第二批实现

### 方案与实现

本批顺序为隔离规则和真实探针 → Runtime → Claude Kit → 真实 Runtime 编排集成测试与文档。未修改前后端 DTO 或 B 的业务实现。

- `runtime.NewDefault()` / `runtime.NewMacOS()`：装配用 `NewDefault`（build tag：Darwin→`NewMacOS`，其它平台→`ISOLATION_UNAVAILABLE` stub）。`NewMacOS` 仅接受 Darwin 25 / arm64、非 root、非 setuid/setgid 宿主进程；Health 实际编译应用 profile，不可用时拒绝启动。CI 另跑可移植 `TestSeatbeltProfileFailClosedShape`（deny-default / 禁 system.sb）。
- `sandbox.sb` 默认拒绝；仅导入 dyld 基础规则，不导入较宽的 system.sb。固定系统运行目录只读，工作区及私有 HOME 可写，路径经符号链接解析和 SBPL 转义；父目录只开放必要元数据。
- Gate 网络规则使用 SBPL 的 `remote tcp4 "localhost:port"`。在本机实际验证：127.0.0.1 指定端口可达；127.0.0.2 同端口、::1 同端口、其他回环端口、UDP、TEST-NET-1 直连与自建 Unix socket 均为权限拒绝。127.0.0.2 未配置本机地址，不能建立正向监听，但探针确认返回 EPERM/EACCES，而非不可达/拒绝连接。
- Create 创建源码树外的私有 HOME、tmp 和不可由 Agent 写入的 profile；执行路径必须在只读文件白名单中，拒绝目录和 setuid/setgid 程序。配置先完整校验相对路径、冲突和大小，再以目录 0700、文件 0600 写入，拒绝覆盖和符号链接。
- 子进程环境从零构造，禁止覆盖 HOME、PATH、TMPDIR、CLAUDE_CODE_TMPDIR 和注入 DYLD/NODE 等变量；两个临时目录变量都指向私有 HOME。Kit 只可增加受限模型/运行开关。
- Runtime 流式读 stdout/stderr，跨 Read 保留不完整 UTF-8，每块最多 16 KiB。取消杀死受限进程，Wait 可重复读取，返回前排空输出回调；Destroy 确认结束及删除私有目录后才成功。输出未排空导致清理超时会保留目录，并允许重试。

**AllowDescendants 按 Kit 声明**：`KitInfo.NeedsDescendants`（Claude/dsh/Codex Detect 成功时置 true）→ `RuntimeSpec.AllowDescendants`；不再编排层写死 `true`。Health 探测仍强制 `deny process-fork`。会话 profile 在只读白名单 `process-exec` 内允许 fork；Stop/Destroy 对 PPID 闭包反复 SIGKILL。极端 `SETPGROUP` 脱离仍可能漏杀，超时则 `cleanupComplete=false`。「不开 Bash」指 **工具面**（Claude `--tools`），不是执行面 deny-all fork。

### Claude Code

`claude.New(executable, model)` 接收宿主机固定模型名。Detect 在无网络私有 HOME 下执行 CLI --version / --bare --help，当前只接受 2.1.292，并检查所需参数。Build 生成 strict Streamable HTTP MCP 配置、模型 Gate 环境与 stdin 任务；不执行任务、不写盘、不读取宿主登录态。内置工具暂限 Read/Write/Edit，MCP 授权仍由 Gate 的固定工具名单执行。Detect 的 available 仅证明安装、版本与受限参数检测通过，真实模型/MCP 兼容仍未验收。

实测修复的启动依赖：ICU/locale 数据的两个固定系统只读目录；私有配置路径的父目录元数据；CLAUDE_CODE_TMPDIR 必须显式重定向，否则该版本尝试使用 /tmp/claude-UID 并被拒绝。没有开放用户 Library、钥匙串、其他项目或通用网络来绕过这些问题。

### 验证与命令

```sh
go test -race ./...
go vet ./...
HARBOR_TEST_CLAUDE=1 go test -race -v ./internal/kits/claude -run TestInstalledCLI
```

- Runtime 的真实系统探针：授权读写、越界/符号链接拒绝、精确网络边界、子进程拒绝、净化环境、UTF-8 排空、取消、重复 Wait/Destroy、清理超时重试、非法配置及运行规格均通过。
- 编排真实进程集成：真实 Runtime + 测试进程，Gate/Policy/Catalog 仍为测试替身；正常退出、非零退出、主动停止及输出先于终态均通过。
- 本机 Claude 2.1.292：严格环境的版本/帮助检测通过；使用测试 Sentinel 和未运行服务的 127.0.0.1:1，真实 CLI 发出结构化 init，模型名、版本和 APIKeySource 正确，随后因无上游在 3 秒取消并清理。未向真实模型发送请求，未使用真实凭据，MCP 状态为失败；该测试不能填 I2 成功。
- 独立编译的最小 C/Go 可执行文件在本机沙箱外也会 SIGKILL，原因尚未确定。测试改用实际 Go 测试二进制的 helper 模式，cgo 测试包额外调用原生 posix_spawn；未将 SIGKILL 记作权限拒绝。涉及原生 spawn 的系统测试要求 CGO_ENABLED=1，本机已启用。

### 接入与剩余事项

B 可将 `runtime.NewDefault()` 和 `claude.New(固定CLI路径, 固定模型名)` 注入既有 orchestrator.New。所有组件仍由宿主机装配，不绑定 Runtime/Kit 到 Wails。`runtime.ProbeExecutable` 仅供受信 Kit 做离线检测。

尚未完成：真实 Gate/Policy/Catalog 及 Wails 装配；模型协议、真实认证 MCP 发现/调用；真实 Agent 文件修改任务；完整 IPC/提权负例验收；允许子进程时的可靠清理。完整 I1–I11 不能因本批单元/系统探针通过而整体标为通过。

## 2026-10-24：边界回归与清理安全加固

- Health 不再只执行 `/usr/bin/true`：会实际验证授权文件可读、外部 fixture 读失败且退出码是普通权限拒绝；profile 被意外改成 allow default 时会 fail closed。
- 准备超时只作用于 `provisioning`，会话总时限从 Create 开始计时并覆盖准备阶段；计时器晚到不能取消已进入 running 的会话。
- 事件队列的截断标记由 `sessionId + seq` 组成，避免不同会话重复 seq 时进入截断死循环；新增回归测试。
- 验证了只禁止 `setsid` / `setpgid` 的候选进程组方案：原生 `posix_spawn(POSIX_SPAWN_SETPGROUP)` 仍能创建新组。因此生产 profile 继续拒绝 `process-fork`，没有以不充分的进程组假设开放子进程。

最新验证：`go test -race ./...`、`go vet ./...`、`git diff --check` 通过；真实 Claude 离线测试需 `HARBOR_TEST_CLAUDE=1`，本机 2.1.292 版本/初始化检查通过。真实模型、MCP、完整子进程能力和 Wails 仍待 B 联调。

## 2026-10-25：同会话多轮输入

Runtime 新增 `InteractiveProcess` 和 `LaunchSpec.Interactive`。交互 Kit 启动时保持受控 stdin，不再在首轮任务后关闭；Orchestrator 暴露 `WritePty`/`ResizePty`，校验会话仍处于 running 且确实由交互 Kit 启动，再把后续轮次写入同一进程。交互输出通过既有 `harbor:event` 以 `pty` 载荷回传，停止/销毁时由 Runtime 关闭 stdin 并清理进程。

此批曾将 Claude 与 dsh 都声明交互模式，但仅保持 stdin 打开不足以支持多轮；下节协议修复替代该声明。Codex 仍需按其 app-server 协议单独接入。当前 macOS `Resize` 只校验尺寸并返回 `UNIMPLEMENTED`，尚未接入真实 PTY/窗口大小控制。

### 多轮协议及输入取消修复

Claude 改为 `--input-format stream-json`，由 Kit 的 EncodeInput 将首轮和后续轮次编码为独立 user JSONL 消息，正文换行不会成为额外协议记录。同一进程保留对话上下文，但真实模型上下文连续性仍须联调验收。dsh headless `-` 是 EOF 单任务输入，已恢复关闭 stdin 并拒绝多轮，防止首轮等待输入而卡住；dsh 多轮仍待实现。

Runtime 输入串行化，阻塞写入响应 context 取消/超时；写入中断可能产生半条消息，编排会取消并清理会话。进程能力发布改为在编排锁内完成，避免输出/输入回调与初始化竞争。真实终端 resize 仍为 UNIMPLEMENTED，JSONL 通道不冒充真实 PTY。

验证：先用失败测试重现旧 text/headless 配置，再验证消息转义、并发写入不交错、阻塞写入取消；真实 macOS 受限进程两轮输入和停止后拒绝输入通过，本机 Claude 固定版本离线检测及无上游初始化通过。这些均不等于远程模型多轮验收。

## 2026-10-24：Runtime 资源墓碑

Destroy 成功后保留最小句柄墓碑，使重复 Destroy 继续幂等；同时清空 profile、工作区、私有目录、只读路径和进程引用，避免长时间运行的桌面应用累计敏感元数据。新增测试检查这些字段释放，并确认重复 Destroy 与已销毁句柄执行均符合契约。

同时加入 `internal/kits.Registry`：只接受宿主机显式装配的 Kit，按 Agent ID 稳定排序批量检测；不可用项清空只读路径，ID 不一致自动关闭，取消和检测内部错误分别保留明确语义。它可直接供 B 的 `ListAgents` 绑定使用，不会自动扫描或注册未知 CLI。

Health 现在还会检查生成的 SBPL 文本本身必须包含 `deny default`、`deny process-fork`，且不能包含对应 allow 规则；随后仍执行授权/拒绝 fixture，防止仅靠字符串检查或仅靠单个探测遗漏 profile 回归。

新增 `security/checks.py` 和 `test_checks.py`：对后续 I1–I11 的脱敏 JSON 证据执行格式、唯一编号、状态、环境、通过证据和明显凭据模式检查。该脚本只校验证据，不把 fake、文档或诊断结果升级为真实通过。

共享 `contracts.Event.Validate` 已加入：事件必须有非空 session/seq，且只包含与 type 对应的一个载荷；state 的水位必须匹配，输出流限定 stdout/stderr 并拒绝 NUL，审计结果和错误字段也经过形状检查。事件队列在入队前丢弃非法事件，避免结构错误进入 Wails/UI。

## 2026-10-24：覆盖率验证稳定性修复

覆盖率模式会让 Runtime 的探针进程收到 Go 的 `GOCOVERDIR` 提示；该提示不是探针业务输出。测试回调现在过滤固定的 `warning: GOCOVERDIR not set` 行，避免 `go test -cover` 因工具链诊断噪声误报，同时仍保留真实 stdout/stderr 和退出结果校验。

本轮独立验证：

```sh
go test -race ./...
go test -race -tags harbor_platform ./internal/runtime ./internal/orchestrator
go test -cover ./internal/contracts ./internal/kits ./internal/kits/claude ./internal/orchestrator ./internal/runtime
go vet ./...
git diff --check
```

以上检查均通过；其中 `harbor_platform` 系统测试要求支持的 Darwin 25/arm64 环境，普通 CI 的 macOS 14 不满足该安全基线，只运行可移植测试。A 当前仍可独立交付并验证的范围是契约/事件校验、会话生命周期与清理重试、严格单进程 macOS Runtime、Claude Kit 离线检测、Kit Registry 以及脱敏验收证据校验。真实 Gate/Policy/Secrets、模型与认证 MCP、Wails 装配、真实 Agent 文件任务和允许子进程后的可靠进程树清理仍需 B 或真实环境联调。

2026-10-25 最新接线与修复状态见 [ABC 架构及联调审查](archive/ABC-INTEGRATION-REVIEW.md#修复进展)。上文按日期保留的“待 Wails/真实组件装配”是历史记录；当前已完成装配，但真实上游任务与完整业务验收仍待执行。

## 2026-10-25：feat/page-design-full 编排首批实现

在 `feat/page-design-full` 分支按 `GO-CHECKLIST.md` 启动 FULL-ORCH。编排层已从全局单会话锁改为最多两个非终态会话：创建前规范化工作区路径并拒绝同目录并发，准备中、运行中和清理失败的会话都会占用槽位；不同工作区可并行运行。连接测试仍与任一活动会话互斥，避免账号目录与运行时资源并发变更。

新增 `List` 和 `Destroy` 服务方法。`List` 保留终态摘要，`Destroy` 只删除已确认 `cleanupComplete=true` 的 stopped/failed 记录；运行中、准备中或清理未完成的会话必须先 Stop/重试清理。Shutdown 会遍历并等待所有活动会话完成清理。

本批验证：`go test ./...`、`go vet ./...`、`git diff --check` 通过。clone、扩权和 PTY 仍按 FULL-ORCH 后续卡实现，当前不将占位绑定或 mock 视为真实能力通过。

## 2026-10-25：clone 工作区落地

Policy 将挂载模式传给 Runtime；macOS Runtime 在应用数据目录 `AgentHarbor/sessions` 创建会话副本，仅向 Agent 授权副本。Handle.Workspace 返回实际执行目录，快照 `workspace` 为副本、`sourceWorkspace` 为原目录；会话互斥仍用原目录。停止时将副本挪到 `AgentHarbor/retained/<id>` 供 `SyncBackWorkspace`；`DestroySession` 删除保留副本。

复制使用 `os.Root` 限定源目录范围；拒绝符号链接、特殊文件，单文件上限 64 MiB、总内容 1 GiB、节点数 10 万，逐块检查取消。复制得到独立文件并保留用户执行位；不继承 setuid/setgid 或组/其他用户权限。复制期间原目录仍可能被外部程序修改，副本不承诺文件系统原子快照。

验证覆盖副本修改不影响原文件、链接拒绝、取消、Policy 挂载模式校验、快照副本路径及原目录锁。真实 macOS Runtime 的副本创建与 Destroy 删除测试通过；远程 Agent/模型联调仍待执行。

## 2026-10-25：全量收尾批次（持续验证）

- 输出与 PTY 事件共同计入 1 MiB 队列上限；修复慢 UI 下交互输出绕过预算的问题。Shutdown 先停止全部活动会话，再分别等待清理，不让一个慢会话延迟另一个会话的取消。
- 会话工具选择支持 0–8 个不重复账号；Policy、Gate、Kit 按顺序交接独立挂载。多挂载分别使用 `/mcp/0` 等路径，持证访问各自固定 `/mcp` 上游，工具白名单互不混用。0 工具只开放模型端口；Claude 空 MCP 配置与 dsh 不装载 MCP 插件。
- Runtime 新增互斥的 `LaunchSpec.Terminal` 模式，可信宿主分配独立 PTY、绑定控制终端、支持窗口 ioctl 和可取消写入。仍拒绝 fork、未授权执行和提权；Claude 的 JSONL 消息流继续使用管道，不冒充真实终端。真实受限 helper 已验证回显、中文第二轮、resize、终态关闭；尚不能把 helper 证据视为真实 Agent TUI 验收。
- `dsh.NewConversation` 使用本仓库固定的 `conversation.mjs` 驱动，通过固定版本官方 Agent API 保留同一实例和 Session。驱动随启动配置写入私有 HOME，只导入受信安装目录的官方模块，不装载任务指定代码、不启用动态插件或 shell。桌面装配采用此模式；`dsh.New` 仍保留官方 headless 单任务路径用于回归。输入采用有界 JSONL，正文不进入参数或插件源代码，执行失败退出并由编排清理。
- 新增真实 dsh + Seatbelt + Gate + 受控 HTTP 上游的两轮验证：第二轮上游请求包含第一轮用户消息及 assistant 回复，且没有新建 Agent 进程；无 MCP 可正常运行。此测试证明受控协议和上下文传递，不代表远程模型回答质量或真实认证 MCP 已验收。

当前未关闭项：Codex Kit 在全量分支的安全移植及受控子进程方案、真实 Agent TUI 与 C 工作台联调、真实远程模型/认证 MCP、扩权 Gate/Policy 与页面完整联调、对外安全证据索引。microVM 仍为 backlog。上述缺口未完成前，不将 A 总体标为完成。

同步 B 最新提交后，删除了另一份未经 Gate 原子校验的 SwitchModel 实现，旧 `SetModelAccount` 入口也统一转交受校验方法。Gate 重定向触发的 `Action` 已改为 `connect`，真实 HTTP→Gate→Orchestrator 测试确认挂起、决策及拒绝恢复；重定向仍不可因批准而自动携带凭据。会话权限一览由编排中的实际 Policy 构建，clone 项目规则标识仍为原工作区。证据索引见 [研发 A 交付状态](DEV-A.md)。

再次同步 B 的重定向/DNS 与 Homebrew Claude 修复：多 MCP 跟随判断使用各自挂载的 Origin；跨域响应仍过滤原密钥与 Sentinel。新增失败测试复现“先校验 DNS、再按主机名拨号”的重绑定窗口，现改为仅连接本次校验的公网 IP，禁代理和额外端口，保留 TLS 主机名校验。Homebrew 检测所需配置只开放 OpenSSL/CA 目录，拒绝给 Agent 授权整个 `/opt/homebrew/etc` 或 `/usr/local/etc`。
