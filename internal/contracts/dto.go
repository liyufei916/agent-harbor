// Package contracts 存放前后端共享的可序列化 DTO。
// 负责人：A 主笔，B 复核绑定兼容，C 核对页面字段。
// 字段与 docs/CONTRACT.md v1.2 一致，不包含业务实现。
package contracts

// AppError 是前端可见的业务错误。message 使用固定可读模板，不附加原始上游细节。
type AppError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// AgentInfo 描述本机可检测的 Agent 能力。
type AgentInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	Available      bool   `json:"available"`
	Reason         string `json:"reason"`
	ModelProfileID string `json:"modelProfileId,omitempty"`
}

// ConnectInput 为模型/MCP 连接的单向输入；APIKey 不得回显或写入事件。
// 自定义模型：Origin 为 https://host[:port][/basePath]（path 会收成 BasePath）；固定 profile 时 Origin 为空。
// Model 可选，探测/出站模型名；空则按主机推断（如 deepseek→deepseek-chat）。
// SkipProbe 仅自定义模型：跳过上游探测仍保存（出站主机校验照做）；会话时仍可能因 Key 无效失败。
type ConnectInput struct {
	ProfileID string `json:"profileId"`
	Name      string `json:"name"`
	APIKey    string `json:"apiKey"`
	Origin    string `json:"origin,omitempty"`
	Model     string `json:"model,omitempty"`
	SkipProbe bool   `json:"skipProbe,omitempty"`
	// Agents：该模型可供哪些 Agent 选用；空则按 profileId 默认单 Agent。
	Agents []string `json:"agents,omitempty"`
	// Transport：空/http=远程 HTTP；stdio=宿主侧本地进程（经回环桥，密钥不进沙箱）。
	Transport string `json:"transport,omitempty"`
	// Command / Args / Env：仅 transport=stdio；Command 须绝对路径；Env 只存本机凭据库。
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// AccountInfo 是脱敏后的账号元信息。
type AccountInfo struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"` // model 或 mcp
	Name      string   `json:"name"`
	ProfileID string   `json:"profileId"`
	Host      string   `json:"host"`
	BasePath  string   `json:"basePath,omitempty"` // 自定义模型 Anthropic 前缀，如 /anthropic
	Model     string   `json:"model,omitempty"`    // 探测成功时的模型名
	KeyHint   string   `json:"keyHint,omitempty"`  // 仅末四位提示，如 …a1b2；永不含完整 Key
	Tools     []string `json:"tools,omitempty"`    // 自定义 MCP：连接探测时 tools/list 快照，供会话白名单
	Agents    []string `json:"agents,omitempty"`   // 可选用的 Agent id；空=仅按 profileId 单绑
	// Protocols：连接时实际探测成功的协议（anthropic / openai-responses）；开会话按 Agent 校验。
	Protocols []string `json:"protocols,omitempty"`
	Transport string   `json:"transport,omitempty"` // stdio | 空(http)
	Command   string   `json:"command,omitempty"`   // stdio 可执行文件（绝对路径），无密钥
	AuthMode  string   `json:"authMode,omitempty"`  // token | oauth | stdio
	// OfflineHint：进程内状态提示（如 stdio 桥未就绪）；禁止含密钥。空=可用。
	OfflineHint string `json:"offlineHint,omitempty"`
}

// SessionSpec 是创建会话的前端请求。
type SessionSpec struct {
	Workspace      string   `json:"workspace"`
	AgentID        string   `json:"agentId"`
	ModelAccountID string   `json:"modelAccountId"`
	MCPIDs         []string `json:"mcpIds"`
	Task           string   `json:"task"`
	MountMode      string   `json:"mountMode,omitempty"` // direct 或 clone；缺省=direct
	// RememberedHosts 由桌面在 Create 前注入本项目记住主机；不入前端 JSON。
	RememberedHosts []RememberedHost `json:"-"`
}

// SessionSnapshot 是会话对外快照。
type SessionSnapshot struct {
	SessionID       string    `json:"sessionId"`
	State           string    `json:"state"`
	Seq             uint64    `json:"seq"`
	Workspace       string    `json:"workspace"`                 // 实际执行路径；clone 时为副本（或停止后保留的副本）
	SourceWorkspace string    `json:"sourceWorkspace,omitempty"` // 用户所选原目录；direct 时与 Workspace 相同
	MountMode       string    `json:"mountMode,omitempty"`       // direct | clone
	ModelAccountID  string    `json:"modelAccountId,omitempty"`
	MCPIDs          []string  `json:"mcpIds,omitempty"`
	AgentID         string    `json:"agentId"`
	CleanupComplete bool      `json:"cleanupComplete"`
	Error           *AppError `json:"error"`
}

// SessionSummary 是列表用摘要。
type SessionSummary struct {
	SessionID       string   `json:"sessionId"`
	State           string   `json:"state"`
	Workspace       string   `json:"workspace"`
	SourceWorkspace string   `json:"sourceWorkspace,omitempty"`
	MountMode       string   `json:"mountMode,omitempty"`
	MCPIDs          []string `json:"mcpIds,omitempty"`
	AgentID         string   `json:"agentId"`
	CleanupComplete bool     `json:"cleanupComplete"`
}

// AgentsResponse 是 ListAgents 的统一返回。
type AgentsResponse struct {
	Agents []AgentInfo `json:"agents"`
	Error  *AppError   `json:"error"`
}

// WorkspaceResponse 是 SelectWorkspace 的统一返回。
type WorkspaceResponse struct {
	Path      string    `json:"path"`
	Cancelled bool      `json:"cancelled"`
	Error     *AppError `json:"error"`
}

// AccountResponse 是 ConnectModel / ConnectMcp 的统一返回。
type AccountResponse struct {
	Account *AccountInfo `json:"account"`
	Error   *AppError    `json:"error"`
}

// AccountsResponse 是 ListAccounts 的统一返回。
type AccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
	Error    *AppError     `json:"error"`
}

// SessionResponse 是 Create/Get/Stop/Destroy Session 的统一返回。
type SessionResponse struct {
	Session *SessionSnapshot `json:"session"`
	Error   *AppError        `json:"error"`
}

// SessionsResponse 是 ListSessions 的统一返回。
type SessionsResponse struct {
	Sessions []SessionSummary `json:"sessions"`
	Error    *AppError        `json:"error"`
}

// RuntimeHealthResponse 是隔离/Runtime 探测结果（扁平字段）。
type RuntimeHealthResponse struct {
	Available      bool   `json:"available"`
	Reason         string `json:"reason"`
	IsolationLevel string `json:"isolationLevel,omitempty"`
	// VaultNotice：凭据介质告警（如 Keychain 不可用）；空表示正常。禁止含密钥。
	VaultNotice string    `json:"vaultNotice,omitempty"`
	Error       *AppError `json:"error"`
}

// PermissionOverview 是权限一览只读摘要；禁止含 Key/Sentinel。
type PermissionOverview struct {
	SessionID            string   `json:"sessionId,omitempty"`
	Workspace            string   `json:"workspace"`
	AllowedHosts         []string `json:"allowedHosts"`
	MCPSummary           []string `json:"mcpSummary"`
	DeniedPathCategories []string `json:"deniedPathCategories,omitempty"`
}

// PermissionOverviewResponse 包装权限一览。
type PermissionOverviewResponse struct {
	Overview *PermissionOverview `json:"overview"`
	Error    *AppError           `json:"error"`
}

// RememberedHost 是已记住的出站主机。
type RememberedHost struct {
	Host      string `json:"host"`
	Workspace string `json:"workspace"`
	Scope     string `json:"scope"` // project | session
}

// RememberedHostsResponse 是 ListRememberedHosts 返回。
type RememberedHostsResponse struct {
	Hosts []RememberedHost `json:"hosts"`
	Error *AppError        `json:"error"`
}

// RevokeHostInput 撤销记住主机。
type RevokeHostInput struct {
	Host      string `json:"host"`
	Workspace string `json:"workspace"`
}

// ResolvePermissionInput 是扩权决策。
type ResolvePermissionInput struct {
	SessionID string `json:"sessionId"`
	Target    string `json:"target"`
	Decision  string `json:"decision"` // deny | once | project
}

// UIPrefs 是本机 UI 偏好；不得含密钥。
type UIPrefs struct {
	SidebarCollapsed      bool `json:"sidebarCollapsed"`
	AutoCollapseWorkbench bool `json:"autoCollapseWorkbench"`
}

// UIPrefsResponse 包装偏好。
type UIPrefsResponse struct {
	Prefs *UIPrefs  `json:"prefs"`
	Error *AppError `json:"error"`
}

// EmptyResponse 无业务载荷的统一返回。
type EmptyResponse struct {
	Error *AppError `json:"error"`
}

// DesktopAPI 是绑定到桌面的业务方法集合。实现在 desktop 包，不在本包执行 IO。
type DesktopAPI interface {
	ListAgents() AgentsResponse
	SelectWorkspace() WorkspaceResponse
	ConnectModel(ConnectInput) AccountResponse
	ConnectMcp(ConnectInput) AccountResponse
	ListAccounts(kind string) AccountsResponse
	RemoveAccount(accountID string) EmptyResponse
	CreateSession(SessionSpec) SessionResponse
	GetSession(string) SessionResponse
	StopSession(string) SessionResponse
	ListSessions() SessionsResponse
	DestroySession(sessionID string) SessionResponse
	// SyncBackWorkspace 将 clone 副本合并写回 SourceWorkspace；direct 会话返回 INVALID_ARGUMENT。
	SyncBackWorkspace(sessionID string) SessionResponse
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

// SwitchModelInput 切换会话当前模型（影响之后出站）。
type SwitchModelInput struct {
	SessionID      string `json:"sessionId"`
	ModelAccountID string `json:"modelAccountId"`
}

// StartMcpOAuthInput 启动 MCP OAuth。
type StartMcpOAuthInput struct {
	Name      string `json:"name"`
	ProfileID string `json:"profileId"`
	Origin    string `json:"origin,omitempty"`
}

// McpOAuthResponse OAuth 启动结果。
type McpOAuthResponse struct {
	AuthURL      string    `json:"authUrl"`
	PendingID    string    `json:"pendingId"`
	RedirectURI  string    `json:"redirectUri,omitempty"`  // 须在 IdP 登记的回环回调
	ClientID     string    `json:"clientId,omitempty"`     // 实际使用的 client_id（勿当密钥）
	ClientSource string    `json:"clientSource,omitempty"` // env | dcr | default
	Error        *AppError `json:"error"`
}

// PtyInput PTY 写入或改尺寸。
type PtyInput struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data,omitempty"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}
