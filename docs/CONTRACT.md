# 接口契约 v1.2（全量 PAGE-DESIGN · feat/page-design-full）

| 项 | 内容 |
|---|---|
| 状态 | **v1.2 本分支全量 PAGE**（`feat/page-design-full`）；实现以本文 + `internal/contracts` 为准 |
| 架构总纲 | [SYSTEM-DESIGN.md](SYSTEM-DESIGN.md) v2 |
| 屏级映射 | [PAGE-API-MAP.md](PAGE-API-MAP.md) |
| 进度 | 共享类型与编排已落地；真实隔离/上游兼容性仍见 [DEV-A](DEV-A.md) |

A/B 将声明落成 Go 类型与绑定，C 按本文 + [PAGE-DESIGN](PAGE-DESIGN.md) 开发页面与 mock。与旧「裁剪版」冲突时：**本分支以 PAGE-DESIGN 全量为准**。

**本分支包含（不再延后）**：MCP OAuth、工作区 `clone`、内嵌 PTY、`SwitchModel`。

### 1.0 全量能力最小约定（开工补注 · 2026-10-25）

与 [GO-CHECKLIST](GO-CHECKLIST.md) §3 一致；实现不得弱于下表。

| 能力 | 约定 |
|---|---|
| OAuth | `StartMcpOAuth` → 系统浏览器打开 `authUrl`；桌面在 **127.0.0.1** 收回环回调（具体端口写入 DECISIONS）；成功后账号出现在 `ListAccounts`，pending 清除；UI **轮询**列表或「我已完成登录」再刷；**暂不**新增 `account_changed` 事件；`CancelMcpOAuth` 清 pending |
| clone | `mountMode=clone` 时 Runtime 在应用数据目录建副本并挂载；快照 `workspace` 为副本路径，`sourceWorkspace` 为原目录；停止后保留副本直至 `DestroySession`；**`SyncBackWorkspace`** 将副本合并写回原目录（覆盖同名文件，不删原目录多余文件） |
| PTY | `WritePty`/`ResizePty` + 事件 `pty`；不支持时 `UNIMPLEMENTED` 或 `INVALID_ARGUMENT`，**不得缺方法名** |
| 默认挂载 | 向导默认推荐 **clone**；用户可选 direct（须强警告） |
| 扩权 | 快照 state **不**增加 `awaiting_permission`；仅 `permission_needed` + `ResolvePermission` |
| stdio MCP | **不在**本契约支持面；仅远程 HTTP MCP（Token 或 OAuth） |

## 1. 前端方法、返回与校验

Wails 绑定对象只导出下列业务方法；Go 为 PascalCase，`frontend/src/api.js` 为 camelCase。页面不得直接使用内部服务。下列为可独立编译的接口规格；类型按 §4 拆入实际包。

```go
package contractspec

import "context"

// 本组 DTO 落在 internal/contracts；APIKey 仅用于单向输入。
type AppError struct {
    Code string `json:"code"`
    Message string `json:"message"`
    Retryable bool `json:"retryable"`
}
type AgentInfo struct {
    ID string `json:"id"`
    Name string `json:"name"`
    Version string `json:"version"`
    Available bool `json:"available"`
    Reason string `json:"reason"`
    ModelProfileID string `json:"modelProfileId,omitempty"`
}
// ConnectInput：固定 profile 时 ProfileID 必填、Origin 空；
// 自定义模型：Origin=https://host[:port][/basePath]，ProfileID 空或 custom；
// 自定义 MCP：Origin=https://host[/path]，ProfileID 空/custom（或 demo-mcp+异主机时升为 custom）。
type ConnectInput struct {
    ProfileID string `json:"profileId"`
    Name string `json:"name"`
    APIKey string `json:"apiKey"`
    Origin string `json:"origin,omitempty"`
    Model string `json:"model,omitempty"`
    SkipProbe bool `json:"skipProbe,omitempty"`
}
type AccountInfo struct {
    ID string `json:"id"`
    Kind string `json:"kind"` // model 或 mcp
    Name string `json:"name"`
    ProfileID string `json:"profileId"`
    Host string `json:"host"`
    BasePath string `json:"basePath,omitempty"`
    Model string `json:"model,omitempty"`
    KeyHint string `json:"keyHint,omitempty"`
    Tools []string `json:"tools,omitempty"` // 自定义 MCP：Connect 时 tools/list 快照；开会话白名单
}
// SessionSpec.MountMode："direct"（缺省）| "clone"。
// MCPIDs 长度 0..8；0 表示本会话不挂 MCP。
type SessionSpec struct {
    Workspace string `json:"workspace"`
    AgentID string `json:"agentId"`
    ModelAccountID string `json:"modelAccountId"`
    MCPIDs []string `json:"mcpIds"`
    Task string `json:"task"`
    MountMode string `json:"mountMode,omitempty"`
}
type SwitchModelInput struct {
    SessionID string `json:"sessionId"`
    ModelAccountID string `json:"modelAccountId"`
}
type StartMcpOAuthInput struct {
    Name string `json:"name"`
    ProfileID string `json:"profileId"` // 可空；自定义时填 Origin
    Origin string `json:"origin,omitempty"`
}
type McpOAuthResponse struct {
    AuthURL string `json:"authUrl"` // 打开系统浏览器
    PendingID string `json:"pendingId"`
    Error *AppError `json:"error"`
}
type PtyInput struct {
    SessionID string `json:"sessionId"`
    Data string `json:"data,omitempty"` // 用户键盘输入（UTF-8）
    Cols int `json:"cols,omitempty"`
    Rows int `json:"rows,omitempty"`
}
type SessionSnapshot struct {
    SessionID string `json:"sessionId"`
    State string `json:"state"`
    Seq uint64 `json:"seq"`
    Workspace string `json:"workspace"` // clone 时为副本路径
    SourceWorkspace string `json:"sourceWorkspace,omitempty"` // 用户原目录
    MountMode string `json:"mountMode,omitempty"`
    ModelAccountID string `json:"modelAccountId,omitempty"`
    MCPIDs []string `json:"mcpIds,omitempty"`
    AgentID string `json:"agentId"`
    CleanupComplete bool `json:"cleanupComplete"`
    Error *AppError `json:"error"`
}
type SessionSummary struct {
    SessionID string `json:"sessionId"`
    State string `json:"state"`
    Workspace string `json:"workspace"`
    SourceWorkspace string `json:"sourceWorkspace,omitempty"`
    MountMode string `json:"mountMode,omitempty"`
    MCPIDs []string `json:"mcpIds,omitempty"`
    AgentID string `json:"agentId"`
    CleanupComplete bool `json:"cleanupComplete"`
}
type AgentsResponse struct {
    Agents []AgentInfo `json:"agents"`
    Error *AppError `json:"error"`
}
type WorkspaceResponse struct {
    Path string `json:"path"`
    Cancelled bool `json:"cancelled"`
    Error *AppError `json:"error"`
}
type AccountResponse struct {
    Account *AccountInfo `json:"account"`
    Error *AppError `json:"error"`
}
type AccountsResponse struct {
    Accounts []AccountInfo `json:"accounts"`
    Error *AppError `json:"error"`
}
type SessionResponse struct {
    Session *SessionSnapshot `json:"session"`
    Error *AppError `json:"error"`
}
type SessionsResponse struct {
    Sessions []SessionSummary `json:"sessions"`
    Error *AppError `json:"error"`
}
// RuntimeHealthResponse 字段扁平，便于壳层状态灯。
type RuntimeHealthResponse struct {
    Available bool `json:"available"`
    Reason string `json:"reason"`
    IsolationLevel string `json:"isolationLevel,omitempty"`
    Error *AppError `json:"error"`
}
type PermissionOverview struct {
    SessionID string `json:"sessionId,omitempty"`
    Workspace string `json:"workspace"`
    AllowedHosts []string `json:"allowedHosts"`
    MCPSummary []string `json:"mcpSummary"`
    DeniedPathCategories []string `json:"deniedPathCategories,omitempty"`
}
type PermissionOverviewResponse struct {
    Overview *PermissionOverview `json:"overview"`
    Error *AppError `json:"error"`
}
type RememberedHost struct {
    Host string `json:"host"`
    Workspace string `json:"workspace"`
    Scope string `json:"scope"` // project | session
}
type RememberedHostsResponse struct {
    Hosts []RememberedHost `json:"hosts"`
    Error *AppError `json:"error"`
}
type RevokeHostInput struct {
    Host string `json:"host"`
    Workspace string `json:"workspace"`
}
type ResolvePermissionInput struct {
    SessionID string `json:"sessionId"`
    Target string `json:"target"`
    Decision string `json:"decision"` // deny | once | project
}
type UIPrefs struct {
    SidebarCollapsed bool `json:"sidebarCollapsed"`
    AutoCollapseWorkbench bool `json:"autoCollapseWorkbench"`
}
type UIPrefsResponse struct {
    Prefs *UIPrefs `json:"prefs"`
    Error *AppError `json:"error"`
}
type EmptyResponse struct {
    Error *AppError `json:"error"`
}

// DesktopAPI：P0 七方法 + V1 扩展。未实现的新方法须返回 UNIMPLEMENTED，不得缺方法导致绑定崩溃。
type DesktopAPI interface {
    ListAgents() AgentsResponse
    SelectWorkspace() WorkspaceResponse
    ConnectModel(ConnectInput) AccountResponse
    ConnectMcp(ConnectInput) AccountResponse
    ListAccounts(kind string) AccountsResponse // kind: model | mcp | ""=全部
    RemoveAccount(accountID string) EmptyResponse
    CreateSession(SessionSpec) SessionResponse
    GetSession(string) SessionResponse
    StopSession(string) SessionResponse
    ListSessions() SessionsResponse
    DestroySession(sessionID string) SessionResponse
    SyncBackWorkspace(sessionID string) SessionResponse // clone → 原目录合并写回
    GetRuntimeHealth() RuntimeHealthResponse
    GetPermissionOverview(sessionID string) PermissionOverviewResponse
    ListRememberedHosts(workspace string) RememberedHostsResponse
    RevokeRememberedHost(RevokeHostInput) EmptyResponse
    ResolvePermission(ResolvePermissionInput) EmptyResponse
    GetUIPrefs() UIPrefsResponse
    SetUIPrefs(UIPrefs) UIPrefsResponse
    SwitchModel(SwitchModelInput) SessionResponse
    StartMcpOAuth(StartMcpOAuthInput) McpOAuthResponse
    CancelMcpOAuth(pendingID string) EmptyResponse
    WritePty(PtyInput) EmptyResponse
    ResizePty(PtyInput) EmptyResponse
}

// Event 经 harbor:event；下列载荷恰有一项非 nil。
type OutputData struct {
    Stream string `json:"stream"` // stdout 或 stderr
    Text string `json:"text"`
}
type AuditData struct {
    Target string `json:"target"`
    Tool string `json:"tool"`
    Result string `json:"result"` // allowed、denied 或 failed
}
type PermissionNeeded struct {
    Target string `json:"target"`
    Action string `json:"action"`
}
type Event struct {
    SessionID string `json:"sessionId"`
    Seq uint64 `json:"seq"`
    Type string `json:"type"` // state_changed|output|audit|error|permission_needed|pty
    State *SessionSnapshot `json:"state"`
    Output *OutputData `json:"output"`
    Audit *AuditData `json:"audit"`
    Error *AppError `json:"error"`
    Permission *PermissionNeeded `json:"permission"`
    Pty *OutputData `json:"pty"` // 与 output 同形；专供 PTY 流
}

// 下列为后端接口规格，不属于 UI DTO，也不得绑定到 Wails。
// 方法的 context 为调用控制；SessionService 内部用应用级 context 派生任务。
type SessionService interface {
    WithConnection(context.Context, func(context.Context) error) error
    Create(context.Context, SessionSpec) (SessionSnapshot, error)
    Get(context.Context, string) (SessionSnapshot, error)
    Stop(context.Context, string) (SessionSnapshot, error)
    List(context.Context) ([]SessionSummary, error)
    Destroy(context.Context, string) (SessionSnapshot, error)
    ResolvePermission(context.Context, ResolvePermissionInput) error
    SwitchModel(context.Context, SwitchModelInput) (SessionSnapshot, error)
    WritePty(context.Context, PtyInput) error
    ResizePty(context.Context, PtyInput) error
    Shutdown(context.Context) error
}
type AccountCatalog interface {
    Lookup(context.Context, string) (AccountInfo, error)
}
type EndpointRule struct {
    Origin string
    Paths []string
}
type PolicyRequest struct {
    Workspace string
    AgentID string
    MountMode string
    Model AccountInfo
    MCP AccountInfo
}
type PolicySnapshot struct {
    Workspace string
    MountMode string
    Model EndpointRule
    MCP EndpointRule
    MCPID string
    Tools []string
}
type PolicyBuilder interface {
    Build(context.Context, PolicyRequest) (PolicySnapshot, error)
}
type GateSpec struct {
    SessionID string
    Policy PolicySnapshot
    ModelAccountID string
    MCPAccountID string
    RequestPermission func(context.Context, PermissionNeeded) (string, error)
}
type GateBinding struct {
    ModelBaseURL string
    MCPURL string
    Sentinel string
    AllowedTCP []string // 精确的 127.0.0.1:port，供 Runtime 强制限制
}
type Gate interface {
    Start(context.Context, GateSpec, func(AuditData), func(error)) (GateBinding, error)
    Revoke(context.Context, string) error
}
type SecretStore interface {
    Put(context.Context, AccountInfo, []byte) (string, error)
    Resolve(context.Context, string) ([]byte, error)
    Delete(context.Context, string) error
}
type Connections interface {
    ConnectModel(context.Context, ConnectInput) (AccountInfo, error)
    ConnectMcp(context.Context, ConnectInput) (AccountInfo, error)
}
type KitInfo struct {
    Agent AgentInfo
    ReadOnlyPaths []string // 固定 CLI 文件及受信专用安装目录的真实路径；目录只读，不递归授权执行
}
type ConfigFile struct {
    RelativePath string // 只能位于 Runtime 私有 HOME 下
    Content []byte // 只允许 Gate URL、Sentinel 等无上游凭据内容
}
type LaunchSpec struct {
    Command string
    Args []string
    Env map[string]string
    Stdin []byte
    Interactive bool
    EncodeInput func([]byte) ([]byte, error) // Kit 负责协议编码，仅用于内部接口
    ConfigFiles []ConfigFile
}
type KitInput struct {
    Task string
    PrivateHome string
    ModelBaseURL string
    MCPURL string
    Sentinel string
}
type Kit interface {
    Detect(context.Context) (KitInfo, error)
    Build(context.Context, KitInput) (LaunchSpec, error)
}
type RuntimeSpec struct {
    SessionID string
    Workspace string
    MountMode string
    AllowedTCP []string
    ReadOnlyPaths []string
}
type Handle struct {
    ID string
    PrivateHome string
    Workspace string // 实际执行路径，clone 时为 Runtime 自有副本
}
type Exit struct {
    Code int
    Cancelled bool
    Err error
}
type Process interface {
    Wait() Exit // 返回前完成 stdout/stderr 读取与最终输出回调
}
type InteractiveProcess interface {
    Process
    Write(context.Context, []byte) error // 完整消息串行写入；支持取消
    Resize(cols, rows int) error
}
type Runtime interface {
    Health(context.Context) error
    Create(context.Context, RuntimeSpec) (Handle, error)
    Exec(context.Context, Handle, LaunchSpec, func(OutputData)) (Process, error)
    Destroy(context.Context, Handle) error
}
```

内部扩权回调 `GateSpec.RequestPermission` 由 A 按会话注入。仅接受 `Action=connect` 和小写精确主机名（不含 URL、端口、通配符或本机地址）；同一会话每个目标只允许一个挂起请求，最多 8 个。返回 `deny|once|project`，取消/停止返回 CANCELLED；无回调、回调失败或超时必须拒绝。批准只表示用户决策，Gate/Policy 仍须执行 DNS/IP 出站校验、按原工作区存 project 规则、限制 once 仅用于原请求并重新求值；不得直接转发任意目标或注入当前账号凭据。当前 Gate 的跨域重定向可触发回调，经批准后最多跟随一跳 HTTPS（默认/443），不复制认证头或原请求正文；拨号固定为本次已校验的公网 IP，TLS 仍验证原目标主机名。project 跨会话重新应用与撤销的完整闭环仍待联调，不代表通用出站扩权已验收。

### dsh 契约增量（2026-10-25）

`ListAgents` 返回已注册 Claude、Codex 与 dsh，未安装或隔离/版本检测失败时 `available=false`。`modelProfileId`：Claude→`demo-model`（另允许 `custom` Anthropic），Codex→`codex-model`（Responses，须宿主机 `HARBOR_CODEX_*`），dsh→`dsh-model`。页面切换 Agent 后清空当前模型账号引用和未提交 Key，保留工作区/MCP；Policy 仍在后端拒绝跨 Agent 模型账号，不能依赖页面校验。见 [DSH-INTEGRATION.md](DSH-INTEGRATION.md)、[CODEX-INTEGRATION.md](CODEX-INTEGRATION.md)。

`FixedProfile` / `EndpointRule` 新增宿主受信 `BasePath` 字段，仅用于模型路由前缀（如 `/anthropic`），不来自 DTO、任务或沙箱。Gate 先对执行面 `/v1/messages` 做原白名单和 Sentinel 检查，再拼接前缀访问固定 origin；Connections 探测使用同一前缀。前缀拒绝穿越、编码、查询串和片段，origin 继续仅允许主机级 URL。

`ReadOnlyPaths` 可以包含受信专用安装目录；Runtime 规范化并拒绝系统/敏感根、HOME、与工作区相交的目录。目录仅递归读取/映射，明确可执行文件仍需单独授权；不开放 fork。dsh 以私有 `DSH_HOME` 和 `DEEPSEEK_API_KEY=Sentinel` 启动，遥测禁用，任务只走 stdin，配置由 Runtime 在私有 HOME 落盘。

### 页面 API 行为

| 方法 | 正常行为 | 校验、故障与副作用 |
|---|---|---|
| ListAgents | 返回实际检测的列表；不依赖 Gate 可用 | CLI 未装属于 available=false；检测内部故障才返回 error |
| SelectWorkspace | 系统目录面板，绝对路径 | 取消：cancelled=true、path=""、error=null |
| ConnectModel | 10s 内测上游，成功才存账号 | **固定 profile**：Origin 空，ProfileID∈已登记；**自定义**：Origin 必填 HTTPS（可带 BasePath），ProfileID 空或 `custom`；禁止 query/fragment；失败清候选，account=null |
| ConnectMcp | Token 轨探测并保存；自定义 Origin 见下 | **固定**：Origin 空，ProfileID∈已登记；**自定义**：填 Origin（HTTPS，可带 path），ProfileID 空/`custom`（`demo-mcp`+异主机亦升为 custom）；探测候选 path→path+/mcp→/mcp；成功须 `tools/list` 非空并写入 `AccountInfo.tools`；空工具 → `UPSTREAM_UNAVAILABLE`；OAuth 轨走 StartMcpOAuth |
| ListAccounts | 返回已保存账号元信息（无 Key） | kind 过滤；空列表 `[]` |
| RemoveAccount | 删除账号与密钥 | 会话仍引用该账号 → `SESSION_BUSY`；未知 → `ACCOUNT_NOT_FOUND` |
| CreateSession | 占会话槽，返回 provisioning | running+preparing 总数 **≤2**，否则 `SESSION_LIMIT`；同规范化工作区已有非终态 → `WORKSPACE_BUSY`；mountMode∈direct\|clone；mcpIds 0..8 |
| SwitchModel | 仅影响之后模型出站 | 仅 running 可切换；未知会话/账号 → 对应错误；必须为相同 profile 与 Host 的模型账号，不兼容 → INVALID_ARGUMENT；不改工作区/Agent |
| StartMcpOAuth / CancelMcpOAuth | 打开浏览器完成 MCP 登录 | 回调成功后出现在 ListAccounts；取消清 pending |
| WritePty / ResizePty | 向焦点会话 PTY 写输入/改尺寸 | 无 PTY 或未 running → `INVALID_ARGUMENT` |
| GetSession | 最新快照与 seq | 未知 → `SESSION_NOT_FOUND`；无任务正文/凭据 |
| StopSession | 返回 stopping，事件报清理 | 终态幂等；未知 → `SESSION_NOT_FOUND` |
| ListSessions | 本应用生命周期内会话摘要 | 含终态直至 Destroy；空 `[]` |
| DestroySession | 移除终态会话记录 | 非 stopped/failed 或 cleanupComplete=false → `INVALID_ARGUMENT`；未知 → `SESSION_NOT_FOUND`；并删除停止后保留的 clone 副本 |
| SyncBackWorkspace | 将 clone 副本合并写回 `sourceWorkspace` | 仅 `mountMode=clone`；覆盖同名常规文件，不删原目录多余项；无副本/direct → `INVALID_ARGUMENT`；未知 → `SESSION_NOT_FOUND` |
| GetRuntimeHealth | 隔离/Runtime 探测 | available=false 时 reason 必填；不因此自动放行无隔离启动 |
| GetPermissionOverview | 只读边界摘要 | 无 Key/Sentinel/完整 Gate URL；sessionId 空则返回「默认/全局」允许摘要（无工作区时 workspace=""） |
| ListRememberedHosts | 已记住主机 | workspace 空=全部；与设置「管理网站」同源 |
| RevokeRememberedHost | 撤销 project/session 记住项 | 未知 host 成功幂等（error=null） |
| ResolvePermission | 处理扩权挂起 | decision∈deny\|once\|project；无挂起 → `INVALID_ARGUMENT`；**禁止**全网允许 |
| GetUIPrefs / SetUIPrefs | 读写本机 UI 偏好 | **不得**存 Key；缺文件返回默认值 |

AgentID 仅允许已验证项；Task 限 1–32768 UTF-8 字节。ConnectInput 的 Name/APIKey 必填；Name≤100 Unicode；APIKey≤16384 字节。固定 profile 路径下 ProfileID 必填。重连创建新账号 ID。活动会话仍引用某账号时 `RemoveAccount` → `SESSION_BUSY`。`ConnectModel`/`ConnectMcp` 经 `WithConnection`：与并发 Connect、以及 `provisioning` 中的 Create 互斥；**已有 running 会话不拦截新增账号**（新账号仅进本机列表，不自动挂到当前会话）。

工具白名单：**固定 MCP profile** 用登记 `Tools`；**自定义 MCP** 用连接探测写入的 `AccountInfo.tools`（会话期不再自动并入上游新工具）。自定义 MCP 账号缺 `tools`（旧版连接未落盘）时 `CreateSession` → `INVALID_ARGUMENT`（Policy `ErrMissingMCP`），须在「工具」删除后重新连接。Policy 失败勿一律映为 `WORKSPACE_DENIED`。

SessionSnapshot.state：`provisioning`|`running`|`stopping`|`stopped`|`failed`（扩权等待 **不** 新增 state；用 `permission_needed` 事件表示挂起，会话保持 `running`）。cleanupComplete 语义同 v1。AccountInfo.kind 仅 model/mcp。

Go 绑定统一 `error` 字段；未实现的新方法返回 `UNIMPLEMENTED`。框架失败由 api.js 映为 `INTERNAL_ERROR`。启动时调 ListAgents + GetRuntimeHealth；恢复会话用 GetSession / ListSessions。

## 2. 事件、快照与错误

- 全部会话事件只用 `harbor:event`。type 为 `state_changed`、`output`、`audit`、`error`、`permission_needed` 或 **`pty`**，且只有对应载荷非 null。`pty` 用 `Pty` 字段。error 为操作失败通知；需用户决策时发 `permission_needed` 并等待 `ResolvePermission`。
- sessionId 在当前应用中唯一；seq 从 1 开始，由 Orchestrator 串行分配，覆盖全部事件。状态载荷的 seq 与外层相同。GetSession 返回最新水位 seq；snapshot.state 只表示实际状态，不由页面推断。
- api.js 先订阅再创建，暂存尚未拿到 sessionId 的事件，响应后合并；晚到的旧快照不能覆盖更大 seq 的状态。重复事件去重，但输出先按 seq 处理再推进水位，不能仅因快照较新就丢弃未展示的输出。事件接入中断可 GetSession 恢复状态，历史输出不承诺重放。
- 本版只保证当前窗口连续连接的事件交付。状态和错误不可静默丢弃；前端文本使用 textContent。输出流按 UTF-8 解码并限每块 16 KiB；后端与前端各保留最多 1 MiB 输出尾部，截断时显示提示，不改变 Agent 实际 stdout 消费。Runtime 读取不得被慢 UI 无限阻塞。
- public error 的 message 为固定可读模板，不附加原始请求、认证头、环境变量、任务正文或上游异常。已知凭据与 Sentinel 必须在边界输出前过滤，流式过滤需覆盖跨块拆分；不能宣称可识别任意上游敏感内容。

事件示例（不包含真实秘密）：

```json
{"sessionId":"demo-session","seq":2,"type":"state_changed","state":{"sessionId":"demo-session","state":"running","seq":2,"workspace":"/demo/project","agentId":"claude","cleanupComplete":false,"error":null},"output":null,"audit":null,"error":null}
```

| 错误码 | 使用位置 | Retryable |
|---|---|---|
| INVALID_ARGUMENT | 输入/引用种类不合法 | false |
| ACCOUNT_NOT_FOUND | 未连接或不存在的账号引用 | false |
| SESSION_BUSY | 单会话锁被占用、会话中重连 | false |
| SESSION_NOT_FOUND | Get/Stop 未知 ID | false |
| AGENT_UNSUPPORTED | CLI/版本/配置不支持 | false |
| ISOLATION_UNAVAILABLE / WORKSPACE_DENIED | 隔离或工作区校验失败 | false |
| UPSTREAM_AUTH_FAILED | 模型/MCP 凭据或权限错误 | false |
| UPSTREAM_TIMEOUT / UPSTREAM_UNAVAILABLE | 手工重试连接/新任务可恢复的上游故障 | true |
| TARGET_DENIED / INVALID_SENTINEL | 目标/工具/会话能力验证失败 | false |
| AGENT_EXIT_FAILED | 非零退出，无已知更具体错误 | false |
| CANCELLED | 用户取消连接等操作；正常停止不算失败 | false |
| CLEANUP_FAILED / INTERNAL_ERROR | 清理或未知内部故障 | false |
| SESSION_LIMIT | Create 时活动会话数已达上限（2） | false |
| WORKSPACE_BUSY | 同工作区已有非终态会话 | false |
| UNIMPLEMENTED | DesktopAPI 新方法尚未接线 | false |

Retryable 只供人选择是否重新操作，不授权自动重发任务或有副作用的 MCP 调用。

## 3. 后端调用与资源语义

### 启动与关闭

1. internal/desktop 校验输入并调用 SessionService.Create；Orchestrator 持有单会话锁，用应用 context 派生 session context，而不是使用已返回的绑定请求 context。
2. Orchestrator 调用 Runtime.Health、Kit.Detect、AccountCatalog.Lookup；Policy.Build 校验真实工作区及固定上游，输出不可变快照。原账号元信息由 B 的连接目录提供，不从 Secrets 读密钥。
3. Gate.Start 创建随机 Sentinel，启动精确绑定 `127.0.0.1` 的模型及 MCP 数据端口并确认 ready，返回 AllowedTCP。失败必须关闭部分已建监听器及上游连接，不留半成品 binding。Start 不再做用户交互或无限期连接测试。
4. Runtime.Create 接收工作区、只读运行文件和 AllowedTCP，创建私有 HOME/隔离规则并确认限制。**本版工作区配置合并进 Create**，不另设 MountWorkspace，以避免调用顺序歧义；后续后端需保持同等授权语义。
5. Kit.Build 只生成无真实凭据的 LaunchSpec；Runtime.Exec 校验相对配置路径、写入私有 HOME（文件 0600、目录 0700），在 profile 内执行，并立即读取输出。写入/启动失败由 Runtime 清理部分子资源，Orchestrator 仍调用 Destroy。
6. Exec 返回进程已成功 spawn 后发布 running；监听进程 Wait 和 Gate 致命回调。Runtime.Health 或 Gate 失效禁止转为宿主机直跑。
7. 停止统一走一个串行清理器：先取消 session context并撤销 Gate → Runtime.Destroy（杀并等待后代、排空输出）→ 等待 Wait/回调结束 → 发布终态。每步即使失败仍尝试其余清理，不只返回第一个错误。

所有后端接口接收 context，普通内部调用返回 error，由 internal/desktop 统一转换成公开 AppError；不能让业务包依赖 Wails。Orchestrator 暂存资源 ID/handle；Gate 持有网络资源，Runtime 持有进程和磁盘资源，Secrets 持有账号生命周期。

Start/Exec 使用会话生命周期 context，准备超时由编排计时器触发会话取消；准备成功后停止该计时器，不能在成功返回时取消仍被监听器/进程使用的 context。Exec 早到输出由编排暂存，发布 running 后按序推送；进程极快退出也必须等最终输出完成再发布终态。Gate 的致命回调只表示监听器/网关自身失效，单次上游 401、超时或拒绝通过协议与审计报告，不等同 Gateway 整体崩溃。

### 超时与幂等

- 上游连接测试 10 秒，整个 provisioning 30 秒；超时触发回滚，不能再启动晚到 Agent。模型/MCP 正常任务采用 60 秒无数据超时与 10 分钟会话总时限，收到流数据重置空闲计时；主动 Stop 随时生效。
- 清理使用脱离已取消 session 的新 context，总预算 5 秒；单个清理步骤不得吞掉其他步骤的执行机会。Revoke/Destroy 必须幂等，即使已部分完成也能重试；Gate.Revoke 在 context 已超时仍先本地标记令牌失效，随后才处理网络关闭。
- Stop 在 provisioning 中与正在创建资源的代码同步；清理器要接管所有晚返回的资源，避免取消与 Start/Create/Exec 竞争泄露资源。
- 正常退出或用户停止后清理成功：stopped、cleanupComplete=true；故障且清理成功：failed、cleanupComplete=true，释放会话锁。清理未确认完成：failed、cleanupComplete=false，保留锁并禁新会话，Stop 可重试清理。
- Runtime.Destroy 返回 nil 表示进程树已确认结束、私有目录已清理；不能只发信号就报告成功。Process.Wait 可多次读取同一最终结果，不产生重复退出事件；没有后代运行且所有输出回调结束后才完成。
- 审计/输出回调可能并发，Orchestrator 串行化公共事件。Runtime/Gate 不能在持有关键资源锁时阻塞调用 UI；不回调进入自身 Stop/Destroy。

## 4. 类型归属与依赖约束

规格代码中的类型按下表拆包，**不是新增 internal/ports 或全局接口包**。只在消费边界声明所需小接口，构造函数注入实现。

| 实际归属 | 声明 | 消费方与依赖方向 |
|---|---|---|
| internal/contracts | AppError、AgentInfo、ConnectInput、AccountInfo、SessionSpec、SessionSnapshot、Responses、Event 及载荷 | 纯数据，所有层可用；不导入业务/Wails |
| internal/orchestrator | SessionService 的实现；其构造参数消费 Gate、Runtime、Kit、PolicyBuilder、AccountCatalog 所需方法 | A 定义消费接口，用各提供方的导出类型；不让提供方反向导入编排 |
| internal/policy | PolicyRequest、EndpointRule、PolicySnapshot、Build | 可用 contracts；路径/固定 profile 校验，无 Secrets/Runtime 依赖 |
| internal/gate | GateSpec、GateBinding、Gate 和 Connections 的实现 | 可用 policy、contracts、secrets，不导入 kits/runtime/orchestrator |
| internal/secrets | SecretStore 与内存存储实现、AccountCatalog 的只读元信息实现 | 只读目录不返回真实凭据；Resolve 只注入 Gate；Put/Delete 供受限连接流程 |
| internal/runtime | RuntimeSpec、Handle、LaunchSpec、ConfigFile、Process/Exit、Runtime | 可用 contracts 输出类型；不导入 kits/gate/policy/orchestrator |
| internal/kits | KitInfo、KitInput、Kit 接口及注册表 | 可用 contracts/runtime，返回 runtime.LaunchSpec；不依赖 gate，编排层把 binding 字段映射给 KitInput |
| internal/desktop | DesktopAPI 实现、错误转换、系统目录面板及 Wails 事件 | 只面向 SessionService、Connections 与只读检测能力，不绑定其他对象 |

SecretStore.Put 要求 AccountInfo 的 ID 为空，成功分配 ID 并关联元信息；失败不留记录，返回的 ID 由连接流程填入 AccountInfo。连接测试先使用仅内存候选凭据，验证成功后才 Put；失败不创建可用账号。Catalog 与 Store 可是同一个私有实现的不同窄视图，但 A 仅收到 Lookup 能力。

policy 的固定 profile 来自 B 管理的后端配置，不从前端或 Agent 任意 URL 构造。账户测试结果绑定该 profile，构造会话时重新核对种类与元信息；两天版没有公开编辑/删除账号接口。应用退出时 B 清空存储，任务停止不删除用户仍连接的账号。

## 5. 演进与版本

- **v1**：七方法 + 四类事件；单会话；mcpIds 长度 1；固定 profile。
- **v1.1**：健康/列表/扩权/多账号/Prefs 等。
- **v1.2（本分支）**：恢复并定义 OAuth、`clone`、`SwitchModel`、PTY（`WritePty`/`ResizePty` + 事件 `pty`）。形状未实现时可 `UNIMPLEMENTED`，**不得缺方法名**。

当前实现注记：macOS Runtime 仍为严格单进程子集；已支持 clone 安全副本（拒绝符号链接及特殊文件），原目录仍作为并发互斥键，副本随 Runtime 清理删除。真实 PTY 与 Agent 多轮协议仍须继续验证。证据见 [DEV-A](DEV-A.md)。

公开 Event 须满足 `contracts.Event.Validate`。本文是并行实现的唯一方法/字段来源；变更先改本文与 `internal/contracts`，再 sync mock/bindings。

既有约定仍有效：WithConnection 准入锁；Connect 探测语义。自定义 Origin 须 HTTPS（本机回环测试可 http+AllowInsecureLoopback）；OAuth 仅工具页。

### 多 MCP 的 Go 内部交接（2026-10-25；自定义 MCP 2026-10-26）

`SessionSpec.mcpIds` 为 0–8 个不重复的已连接账号 ID；空列表表示不挂载工具。PolicyRequest.MCPs 按该顺序构造 PolicySnapshot.MCPMounts（AccountID、Endpoint、Tools），GateBinding.MCPURLs 按同一顺序交给 Kit。每个挂载有独立 Gateway 路径，按挂载解析宿主凭据及工具白名单，不合并同名工具。Kit 将其映射为 harbor、harbor-2 等独立服务器；0 个挂载生成空 MCP 配置，不连接占位服务。

旧单 MCP 字段仅兼容内部单挂载调用：MCP/MCPID/Tools、MCPAccountID、MCPURL；新列表与旧字段同时提供且有歧义时拒绝。多挂载路径仅用于选定授权项。**固定 demo-mcp** 上游 path 仍为登记值（通常 `/mcp`）；**自定义 MCP** 以上游探测成功的 path 为准（用户 path 或自动补 `/mcp`），不得由执行面改写为任意目标。公开 Wails DTO 和绑定签名不变（`AccountInfo.tools` 为新增可选字段）。
