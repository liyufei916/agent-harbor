/**
 * 浏览器独立 mock transport（CONTRACT.md v1.2）
 * 固定数据：claude / mock-model+demo-model / mock-mcp+demo-mcp / /demo/project
 * 场景：normal | auth | busy | isolation | cleanup
 * 多会话：running+preparing ≤2；同工作区禁双开；PTY/OAuth 可演示
 */

const FIXED = {
  agent: {
    id: "claude",
    name: "Claude Code",
    version: "2.1.292",
    available: true,
    reason: "",
    modelProfileId: "demo-model",
  },
  modelProfileId: "demo-model",
  mcpProfileId: "demo-mcp",
  workspace: "/demo/project",
};

let scenario = "normal";
let seq = 0;
let sessionNumber = 0;
let accountNumber = 0;
/** @type {Map<string, object>} */
const sessions = new Map();
/** @type {Map<string, number[]>} sessionId → timer ids */
const sessionTimers = new Map();
/** @type {Map<string, object>} id → model account */
const modelAccounts = new Map();
/** @type {Map<string, object>} id → mcp account */
const mcpAccounts = new Map();
/** @type {Map<string, object>} pending OAuth */
const oauthPending = new Map();
/** @type {Map<string, {host:string,workspace:string,scope:string}>} */
const rememberedHosts = new Map();
let uiPrefs = {
  sidebarCollapsed: false,
  autoCollapseWorkbench: true,
  askClearOnStop: true,
  clearOnStopDefault: "keep",
  theme: "system",
  securityTier: "standard",
  isolationBackend: "seatbelt",
  engineSource: "harbor-colima",
};

function hostKey(host, workspace) {
  return `${normPath(workspace || FIXED.workspace)}::${String(host || "").toLowerCase()}`;
}

/** 与 secrets.keyHint 对齐：仅末四位，短 Key 遮罩。 */
function keyHint(apiKey) {
  const s = String(apiKey || "").trim();
  if (!s) return "";
  if (s.length < 4) return "••••";
  return "…" + s.slice(-4);
}

function seedRememberedHosts() {
  rememberedHosts.clear();
  rememberedHosts.set(hostKey("api.example.com", FIXED.workspace), {
    host: "api.example.com",
    workspace: FIXED.workspace,
    scope: "project",
  });
}
const listeners = new Set();

function getModel(id) {
  return modelAccounts.get(id) || null;
}

function getMcp(id) {
  return mcpAccounts.get(id) || null;
}

function err(code, message, retryable = false) {
  return { code, message, retryable };
}

function nextSeq() {
  seq += 1;
  return seq;
}

function clearSessionTimers(sessionId) {
  const list = sessionTimers.get(sessionId) || [];
  list.forEach((t) => clearTimeout(t));
  sessionTimers.delete(sessionId);
}

function clearAllTimers() {
  for (const id of [...sessionTimers.keys()]) clearSessionTimers(id);
}

function scheduleFor(sessionId, ms, fn) {
  const tid = setTimeout(fn, ms);
  const list = sessionTimers.get(sessionId) || [];
  list.push(tid);
  sessionTimers.set(sessionId, list);
  return tid;
}

function isActive(s) {
  if (!s) return false;
  if (["provisioning", "running", "stopping"].includes(s.state)) return true;
  return s.state === "failed" && !s.cleanupComplete;
}

function activeSessions() {
  return [...sessions.values()].filter(isActive);
}

function normPath(p) {
  return String(p || "").replace(/\/+$/, "") || "/";
}

function emit(partial) {
  const sid = partial.sessionId || "";
  const event = {
    sessionId: sid,
    seq: nextSeq(),
    type: partial.type,
    state: partial.state || null,
    output: partial.output || null,
    audit: partial.audit || null,
    error: partial.error || null,
    permission: partial.permission || null,
    pty: partial.pty || null,
  };
  const s = sessions.get(sid);
  if (s) {
    s.seq = event.seq;
    sessions.set(sid, s);
  }
  if (event.state) event.state = { ...event.state, seq: event.seq };
  listeners.forEach((cb) => {
    try {
      cb(event);
    } catch (_) {
      /* UI 错误不打断 mock */
    }
  });
  return event;
}

function snapshotOf(sessionId, extra = {}) {
  const cur = sessions.get(sessionId);
  if (!cur) return null;
  const next = { ...cur, ...extra, seq };
  sessions.set(sessionId, next);
  return { ...next, error: next.error ? { ...next.error } : null };
}

export function setScenario(name) {
  scenario = name || "normal";
}

export function getScenario() {
  return scenario;
}

export function isMockTransport() {
  return true;
}

export async function listAgents() {
  const agents = [
    { ...FIXED.agent },
    {
      id: "dsh",
      name: "DeepSeek dsh",
      version: "0.2.1-rc.2",
      available: true,
      reason: "",
      modelProfileId: "dsh-model",
    },
  ];
  if (scenario === "isolation") {
    return {
      agents: agents.map((agent) => ({
        ...agent,
        available: false,
        reason: "隔离能力不可用（模拟）",
      })),
      error: null,
    };
  }
  return { agents, error: null };
}

export async function getRuntimeHealth() {
  if (scenario === "isolation") {
    return {
      available: false,
      reason: "隔离能力不可用（模拟）",
      isolationLevel: "",
      error: null,
    };
  }
  const backend = String(uiPrefs?.isolationBackend || "").toLowerCase();
  if (backend === "docker" || backend === "p2" || backend === "container") {
    return {
      available: true,
      reason: "P2 Docker（mock · harbor.gate · 镜像内 Agent）",
      isolationLevel: "docker",
      error: null,
    };
  }
  return {
    available: true,
    reason: "",
    isolationLevel: "mock-seatbelt",
    error: null,
  };
}

let mockEngineReady = false;
let mockEngineInstalling = false;
let mockEngineProgress = 0;
let mockEngineProgressLabel = "";
const engineProgressListeners = new Set();

function emitEngineProgress(payload) {
  for (const cb of engineProgressListeners) {
    try {
      cb(payload);
    } catch {}
  }
}

export function onEngineProgress(callback) {
  engineProgressListeners.add(callback);
  return () => engineProgressListeners.delete(callback);
}

export async function getContainerEngineHealth() {
  const source =
    String(uiPrefs?.engineSource || "").toLowerCase() === "system-docker"
      ? "system-docker"
      : "harbor-colima";
  const base = {
    source,
    dockerDesktopInstalled: false,
    componentsInstalled: mockEngineReady,
    installing: mockEngineInstalling,
    progressPercent: mockEngineProgress,
    progressLabel: mockEngineProgressLabel,
    dockerHost: mockEngineReady ? "unix:///tmp/mock-harbor.sock" : "",
    error: null,
  };
  if (mockEngineReady) {
    return {
      ...base,
      ready: true,
      message: source === "system-docker" ? "本机 Docker 可用（mock）" : "Harbor Colima 可用（mock）",
    };
  }
  return {
    ...base,
    ready: false,
    message: "未检测到 Docker Desktop（mock）。可一键安装 Harbor 托管引擎。",
  };
}

let mockEngineStopRequested = false;

export async function ensureContainerEngine() {
  mockEngineInstalling = true;
  mockEngineStopRequested = false;
  const steps = [
    [8, "下载 colima…"],
    [35, "下载 limactl…"],
    [60, "下载 docker…"],
    [78, "启动 Colima…"],
    [92, "等待 Docker daemon…"],
    [100, "安装完成"],
  ];
  for (const [pct, label] of steps) {
    if (mockEngineStopRequested) {
      mockEngineInstalling = false;
      mockEngineReady = false;
      mockEngineProgress = 0;
      mockEngineProgressLabel = "安装已取消";
      emitEngineProgress({
        percent: 0,
        label: "安装已取消",
        done: true,
        error: "安装已取消",
      });
      return getContainerEngineHealth();
    }
    mockEngineProgress = pct;
    mockEngineProgressLabel = label;
    emitEngineProgress({ percent: pct, label, done: false });
    await new Promise((r) => setTimeout(r, 120));
  }
  mockEngineReady = true;
  mockEngineInstalling = false;
  mockEngineProgress = 100;
  mockEngineProgressLabel = "安装完成";
  emitEngineProgress({ percent: 100, label: "安装完成", done: true });
  return getContainerEngineHealth();
}

export async function stopContainerEngine() {
  mockEngineStopRequested = true;
  mockEngineInstalling = false;
  mockEngineReady = false;
  mockEngineProgress = 0;
  mockEngineProgressLabel = "已停止";
  emitEngineProgress({
    percent: 0,
    label: "已取消安装并停止引擎（mock）",
    done: true,
  });
  return {
    ...(await getContainerEngineHealth()),
    ready: false,
    message: "已取消安装并停止引擎（mock）",
  };
}

export async function listSessions() {
  const list = [...sessions.values()].map((s) => ({
    sessionId: s.sessionId,
    state: s.state,
    workspace: s.workspace,
    sourceWorkspace: s.sourceWorkspace || s.workspace,
    agentId: s.agentId,
    cleanupComplete: s.cleanupComplete,
    mountMode: s.mountMode,
    mcpIds: [...(s.mcpIds || [])],
  }));
  return { sessions: list, error: null };
}

/** 将 clone 副本「导回」原工作区（模拟：仅标记成功，不写盘）。 */
export async function syncBackWorkspace(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) {
    return {
      session: null,
      error: err("SESSION_NOT_FOUND", "会话不存在（模拟）"),
    };
  }
  if (session.mountMode !== "clone") {
    return {
      session: snapshotOf(sessionId),
      error: err("INVALID_ARGUMENT", "仅安全副本会话可导回（模拟）"),
    };
  }
  if (session.cleanupComplete && !session.cloneRetained) {
    return {
      session: snapshotOf(sessionId),
      error: err("INVALID_ARGUMENT", "副本已清理，无法导回（模拟）"),
    };
  }
  snapshotOf(sessionId, { syncedBack: true });
  emit({
    type: "state_changed",
    sessionId,
    state: snapshotOf(sessionId),
  });
  return { session: snapshotOf(sessionId), error: null };
}

export async function destroySession(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) {
    return {
      session: null,
      error: err("SESSION_NOT_FOUND", "会话不存在（模拟）"),
    };
  }
  if (!["stopped", "failed"].includes(session.state) || !session.cleanupComplete) {
    return {
      session: snapshotOf(sessionId),
      error: err("INVALID_ARGUMENT", "仅终态且清理完成的会话可销毁（模拟）"),
    };
  }
  const last = snapshotOf(sessionId);
  clearSessionTimers(sessionId);
  sessions.delete(sessionId);
  return { session: last, error: null };
}

export async function listAccounts(kind) {
  const accounts = [];
  if (!kind || kind === "model") {
    for (const a of modelAccounts.values()) {
      const row = { ...a };
      // auth 场景：列表标 ▲ 无效，便于模型页演示（不删账号）
      if (scenario === "auth") row.invalid = true;
      accounts.push(row);
    }
  }
  if (!kind || kind === "mcp") {
    for (const a of mcpAccounts.values()) accounts.push({ ...a });
  }
  return { accounts, error: null };
}

export async function removeAccount(accountId) {
  if (modelAccounts.has(accountId)) {
    if (activeSessions().length) {
      return { error: err("SESSION_BUSY", "会话仍引用该账号（模拟）") };
    }
    modelAccounts.delete(accountId);
    return { error: null };
  }
  if (mcpAccounts.has(accountId)) {
    if (activeSessions().length) {
      return { error: err("SESSION_BUSY", "会话仍引用该账号（模拟）") };
    }
    mcpAccounts.delete(accountId);
    return { error: null };
  }
  return { error: err("ACCOUNT_NOT_FOUND", "账号不存在（模拟）") };
}

export async function getPermissionOverview(sessionId) {
  const s = sessionId ? sessions.get(sessionId) : null;
  return {
    overview: {
      sessionId: sessionId || s?.sessionId || "",
      workspace: s?.workspace || FIXED.workspace,
      allowedHosts: ["api.example.com"],
      mcpSummary: [...mcpAccounts.values()].map((a) => a.name),
      deniedPathCategories: ["系统目录"],
    },
    error: null,
  };
}

export async function listRememberedHosts(workspace) {
  const want = workspace ? normPath(workspace) : "";
  const hosts = [...rememberedHosts.values()].filter((h) =>
    want ? normPath(h.workspace) === want : true,
  );
  return { hosts, error: null };
}

export async function revokeRememberedHost(input) {
  const key = hostKey(input?.host, input?.workspace);
  if (!rememberedHosts.has(key)) {
    return { error: err("INVALID_ARGUMENT", "无此记住项（模拟）") };
  }
  rememberedHosts.delete(key);
  return { error: null };
}

export async function promptPermission(input) {
  const sessionId = input?.sessionId;
  const target = String(input?.target || "")
    .trim()
    .toLowerCase();
  if (!sessionId || !target || !target.includes(".")) {
    return { error: err("INVALID_ARGUMENT", "会话或目标主机不合法（模拟）") };
  }
  const session = sessions.get(sessionId);
  if (!session) {
    return { error: err("SESSION_NOT_FOUND", "会话不存在（模拟）") };
  }
  if (session.state !== "running") {
    return { error: err("INVALID_ARGUMENT", "仅 running 会话可请求扩权（模拟）") };
  }
  emit({
    type: "permission_needed",
    sessionId,
    permission: { target, action: "connect" },
  });
  return { error: null };
}

export async function saveTextFile(input) {
  const name = String(input?.suggestedName || "").trim();
  const content = String(input?.content || "");
  if (!name || name.includes("/") || name.includes("\\")) {
    return { path: "", cancelled: false, error: err("INVALID_ARGUMENT", "文件名不合法（模拟）") };
  }
  if (!content || content.length > 2 << 20) {
    return { path: "", cancelled: false, error: err("INVALID_ARGUMENT", "内容为空或过大（模拟）") };
  }
  // 浏览器 mock：走下载；提示用户打开「下载」文件夹
  const blob = new Blob([content], { type: "application/json" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
  return {
    path: `下载/${name}`,
    cancelled: false,
    error: null,
  };
}

export async function resolvePermission(input) {
  if (!input?.sessionId || !input?.target || !input?.decision) {
    return { error: err("INVALID_ARGUMENT", "扩权参数不完整（模拟）") };
  }
  if (!["deny", "once", "project"].includes(input.decision)) {
    return { error: err("INVALID_ARGUMENT", "非法决策（模拟）") };
  }
  const session = sessions.get(input.sessionId);
  if (!session) {
    return { error: err("SESSION_NOT_FOUND", "会话不存在（模拟）") };
  }
  // 与 A 一致：等待期保持 running；project 写入本工作区记住表（once 不持久）
  if (input.decision === "project") {
    const ws = session.workspace || FIXED.workspace;
    rememberedHosts.set(hostKey(input.target, ws), {
      host: input.target,
      workspace: ws,
      scope: "project",
    });
  }
  emit({
    type: "audit",
    sessionId: input.sessionId,
    audit: {
      target: input.target,
      tool: "permission",
      result:
        input.decision === "deny"
          ? "denied"
          : input.decision === "once"
            ? "allowed"
            : "allowed",
    },
  });
  return { error: null };
}

function loadPrefsFromStorage() {
  try {
    const raw = localStorage.getItem("harbor-ui-prefs");
    if (raw) uiPrefs = { ...uiPrefs, ...JSON.parse(raw) };
  } catch (_) {
    /* ignore */
  }
}

export async function getUIPrefs() {
  loadPrefsFromStorage();
  return { prefs: { ...uiPrefs }, error: null };
}

export async function setUIPrefs(prefs) {
  loadPrefsFromStorage();
  const next = { ...uiPrefs };
  if (prefs?.sidebarCollapsed !== undefined) {
    next.sidebarCollapsed = !!prefs.sidebarCollapsed;
  }
  if (prefs?.autoCollapseWorkbench !== undefined) {
    next.autoCollapseWorkbench = prefs.autoCollapseWorkbench !== false;
  }
  if (prefs?.askClearOnStop !== undefined) {
    next.askClearOnStop = !!prefs.askClearOnStop;
  }
  if (prefs?.clearOnStopDefault !== undefined) {
    next.clearOnStopDefault =
      prefs.clearOnStopDefault === "clear" ? "clear" : "keep";
  }
  if (prefs?.theme !== undefined) {
    const t = prefs.theme;
    next.theme =
      t === "light" || t === "dark" || t === "system" ? t : "system";
  }
  if (prefs?.isolationBackend !== undefined) {
    const b = prefs.isolationBackend;
    next.isolationBackend =
      b === "docker" || b === "p2" || b === "container" ? "docker" : "seatbelt";
  }
  if (prefs?.engineSource !== undefined) {
    const s = String(prefs.engineSource || "").toLowerCase();
    next.engineSource =
      s === "system-docker" || s === "system" || s === "desktop"
        ? "system-docker"
        : "harbor-colima";
  }
  if (prefs?.securityTier !== undefined) {
    const t = prefs.securityTier;
    next.securityTier =
      t === "strict" || t === "relaxed" || t === "standard" ? t : "standard";
  }
  uiPrefs = next;
  try {
    localStorage.setItem("harbor-ui-prefs", JSON.stringify(uiPrefs));
  } catch (_) {
    /* ignore */
  }
  return { prefs: { ...uiPrefs }, error: null };
}

export async function selectWorkspace() {
  // 浏览器无系统面板：给可区分的第二条路径，便于测同目录冲突时用 query
  return { path: FIXED.workspace, cancelled: false, error: null };
}

/** 测试/演示：指定工作区路径（浏览器 mock） */
export async function selectWorkspacePath(path) {
  return { path: path || FIXED.workspace, cancelled: false, error: null };
}

export async function connectModel(input) {
  if (scenario === "auth") {
    return {
      account: null,
      error: err("UPSTREAM_AUTH_FAILED", "模型认证失败（模拟）", false),
    };
  }
  if (!input?.name || !input?.apiKey) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "name、apiKey 均必填"),
    };
  }
  const profileId = input.profileId || "custom";
  const allowed = [FIXED.modelProfileId, "dsh-model", "codex-model", "custom"];
  if (!allowed.includes(profileId)) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "模型 profile 不受支持"),
    };
  }
  if (profileId === "custom" && !input.origin) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "自定义模型须提供 origin"),
    };
  }
  let host = "demo-model.local";
  if (profileId === "codex-model") host = "api.openai.com";
  if (profileId === "dsh-model") host = "api.deepseek.com";
  if (input.origin) {
    try {
      host = new URL(input.origin).host;
    } catch {
      return {
        account: null,
        error: err("INVALID_ARGUMENT", "origin 非法"),
      };
    }
  }
  // 首个账号固定 id，兼容既有测试与 setup 流
  const id =
    modelAccounts.size === 0 ? "mock-model" : `mock-model-${++accountNumber}`;
  let agents = Array.isArray(input.agents) ? input.agents.filter(Boolean) : [];
  if (!agents.length) {
    if (profileId === "codex-model") agents = ["codex"];
    else if (profileId === "dsh-model") agents = ["dsh"];
    else agents = ["claude"];
  }
  const protocols = [];
  if (agents.includes("claude") || agents.includes("dsh") || profileId === "custom" || profileId === "demo-model" || profileId === "dsh-model") {
    if (agents.includes("claude") || agents.includes("dsh") || (!agents.includes("codex") && profileId !== "codex-model")) {
      protocols.push("anthropic");
    }
  }
  if (agents.includes("codex") || profileId === "codex-model") {
    protocols.push("openai-responses");
  }
  // 模拟：DeepSeek 官方域名即使勾了 Codex 也不给 Responses（与真机探测一致）
  const protocolsFinal =
    host === "api.deepseek.com"
      ? protocols.filter((p) => p !== "openai-responses")
      : [...new Set(protocols)];
  if (host === "api.deepseek.com" && agents.includes("codex") && !protocolsFinal.includes("openai-responses")) {
    return {
      account: null,
      error: err(
        "UPSTREAM_UNAVAILABLE",
        "已勾选 Codex，但该地址未通过 Responses 探测。api.deepseek.com 不能配 Codex；请用「OpenAI Codex」。",
        true,
      ),
    };
  }
  const account = {
    id,
    kind: "model",
    name: input.name,
    profileId,
    host,
    origin: input.origin || "",
    model: input.model || "",
    keyHint: keyHint(input.apiKey),
    agents,
    protocols: protocolsFinal.length ? protocolsFinal : undefined,
  };
  modelAccounts.set(id, account);
  return { account: { ...account }, error: null };
}

export async function startMcpOAuth(input) {
  const pendingId = `oauth-${Date.now()}`;
  oauthPending.set(pendingId, {
    name: input?.name || "OAuth MCP",
    profileId: input?.profileId || FIXED.mcpProfileId,
    origin: input?.origin || "",
  });
  scheduleFor(
    pendingId,
    800,
    () => {
      const p = oauthPending.get(pendingId);
      if (!p) return;
      const id =
        mcpAccounts.size === 0 ? "mock-mcp" : `mock-mcp-${++accountNumber}`;
      mcpAccounts.set(id, {
        id,
        kind: "mcp",
        name: p.name,
        profileId: p.profileId,
        host: "oauth.mcp.local",
        authMode: "oauth",
      });
      oauthPending.delete(pendingId);
    },
  );
  return {
    authUrl: "https://example.com/oauth/mock",
    pendingId,
    redirectUri: "http://127.0.0.1:9/callback",
    clientId: "agent-harbor",
    clientSource: "default",
    error: null,
  };
}

export async function cancelMcpOAuth(pendingId) {
  if (pendingId) {
    clearSessionTimers(pendingId);
    oauthPending.delete(pendingId);
  } else {
    for (const id of [...oauthPending.keys()]) {
      clearSessionTimers(id);
      oauthPending.delete(id);
    }
  }
  return { error: null };
}

export async function switchModel(input) {
  const session = sessions.get(input?.sessionId);
  if (!session) {
    return {
      session: null,
      error: err("SESSION_NOT_FOUND", "会话不存在（模拟）"),
    };
  }
  if (!getModel(input.modelAccountId)) {
    return {
      session: null,
      error: err("ACCOUNT_NOT_FOUND", "模型账号不存在（模拟）"),
    };
  }
  return {
    session: snapshotOf(input.sessionId, {
      modelAccountId: input.modelAccountId,
    }),
    error: null,
  };
}

export async function writePty(input) {
  const session = sessions.get(input?.sessionId);
  if (!session || !isActive(session) || session.state === "stopping") {
    return { error: err("INVALID_ARGUMENT", "无可用 PTY（模拟）") };
  }
  emit({
    type: "pty",
    sessionId: session.sessionId,
    pty: { stream: "stdout", text: input.data || "" },
  });
  return { error: null };
}

export async function resizePty(input) {
  if (!sessions.has(input?.sessionId)) {
    return { error: err("INVALID_ARGUMENT", "无可用 PTY（模拟）") };
  }
  return { error: null };
}

const MCP_PRESETS = [
  {
    id: "harbor-time",
    name: "本机时间",
    description: "只读：返回当前本地时间（宿主 stdio，无密钥）。",
    kind: "stdio",
    needsApiKey: false,
  },
  {
    id: "harbor-filesystem",
    name: "演示工作区文件（只读）",
    description: "只读列举/读取 demos/workspace（路径锁死，宿主 stdio）。",
    kind: "stdio",
    needsApiKey: false,
  },
  {
    id: "context7",
    name: "Context7 文档",
    description: "远程只读文档检索；须自备 Token（HTTPS）。",
    kind: "http",
    needsApiKey: true,
    origin: "https://mcp.context7.com",
  },
];

export async function listMcpPresets() {
  return { presets: MCP_PRESETS.map((p) => ({ ...p })), error: null };
}

export async function addMcpPreset(input) {
  const preset = MCP_PRESETS.find((p) => p.id === input?.presetId);
  if (!preset) {
    return { account: null, error: err("INVALID_ARGUMENT", "请指定预置 MCP。") };
  }
  if (preset.needsApiKey && !input?.apiKey) {
    return { account: null, error: err("INVALID_ARGUMENT", "Context7 须填写 Token。") };
  }
  if (preset.kind === "stdio") {
    return connectMcp({
      name: input?.name || preset.name,
      profileId: "custom",
      transport: "stdio",
      command: "/usr/bin/true",
      args: [],
      apiKey: "stdio",
    });
  }
  return connectMcp({
    name: input?.name || preset.name,
    profileId: "custom",
    apiKey: input.apiKey,
    origin: preset.origin,
  });
}

export async function connectMcp(input) {
  if (scenario === "auth") {
    return {
      account: null,
      error: err("UPSTREAM_AUTH_FAILED", "MCP 认证失败（模拟）", false),
    };
  }
  if (!input?.name) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "name 必填"),
    };
  }
  if (input.transport === "stdio") {
    if (!input.command || !String(input.command).startsWith("/")) {
      return {
        account: null,
        error: err("INVALID_ARGUMENT", "stdio 须绝对路径 command"),
      };
    }
    const id =
      mcpAccounts.size === 0 ? "mock-mcp" : `mock-mcp-${++accountNumber}`;
    const account = {
      id,
      kind: "mcp",
      name: input.name,
      profileId: "custom",
      host: "127.0.0.1:9",
      transport: "stdio",
      command: input.command,
      authMode: "stdio",
      keyHint: "stdio",
      tools: ["echo"],
    };
    mcpAccounts.set(id, account);
    return { account: { ...account }, error: null };
  }
  if (!input?.profileId || !input?.apiKey) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "profileId、name、apiKey 均必填"),
    };
  }
  if (input.profileId !== FIXED.mcpProfileId && input.profileId !== "custom") {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "仅支持 demo-mcp 或 custom"),
    };
  }
  const id =
    mcpAccounts.size === 0 ? "mock-mcp" : `mock-mcp-${++accountNumber}`;
  const account = {
    id,
    kind: "mcp",
    name: input.name,
    profileId: input.profileId,
    host: input.origin ? "custom.mcp.local" : "demo-mcp.local",
    authMode: "token",
    keyHint: keyHint(input.apiKey),
    tools: input.profileId === "custom" ? ["list_resources"] : undefined,
  };
  mcpAccounts.set(id, account);
  return { account: { ...account }, error: null };
}

export async function createSession(spec) {
  if (scenario === "isolation") {
    return {
      session: null,
      error: err("ISOLATION_UNAVAILABLE", "当前无法建立隔离环境（模拟）"),
    };
  }
  if (scenario === "busy") {
    return {
      session: null,
      error: err("SESSION_BUSY", "已有会话占用（模拟）"),
    };
  }
  if (
    !spec?.workspace ||
    !spec?.agentId ||
    !spec?.modelAccountId ||
    !spec?.task
  ) {
    return {
      session: null,
      error: err(
        "INVALID_ARGUMENT",
        "workspace、agentId、modelAccountId、task 必填",
      ),
    };
  }
  if (!Array.isArray(spec.mcpIds) || spec.mcpIds.length > 8) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "mcpIds 长度须为 0..8"),
    };
  }
  const modelAcc = getModel(spec.modelAccountId);
  if (!modelAcc) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "请先连接模型账号"),
    };
  }
  const agents =
    Array.isArray(modelAcc.agents) && modelAcc.agents.length
      ? modelAcc.agents
      : modelAcc.profileId === "codex-model"
        ? ["codex"]
        : modelAcc.profileId === "dsh-model"
          ? ["dsh"]
          : ["claude"];
  if (!agents.includes(spec.agentId)) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "Agent 与模型不兼容（未勾选该 Agent）"),
    };
  }
  if (spec.mcpIds.length > 0) {
    if (!spec.mcpIds.every((id) => getMcp(id))) {
      return {
        session: null,
        error: err("INVALID_ARGUMENT", "请先连接 MCP 账号，且 mcpIds 须匹配"),
      };
    }
  }
  if (![FIXED.agent.id, "dsh", "codex"].includes(spec.agentId)) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "Agent 不受支持"),
    };
  }

  const active = activeSessions();
  if (active.length >= 2) {
    return {
      session: null,
      error: err(
        "SESSION_LIMIT",
        "已有 2 个会话在运行，请先停止其中一个（模拟）",
      ),
    };
  }
  const ws = normPath(spec.workspace);
  if (
    active.some(
      (s) => normPath(s.sourceWorkspace || s.workspace) === ws,
    )
  ) {
    return {
      session: null,
      error: err(
        "WORKSPACE_BUSY",
        "该工作区已有会话在运行，请打开已有会话或换目录（模拟）",
      ),
    };
  }

  const sessionId = `sess-mock-${++sessionNumber}`;
  const mountMode = spec.mountMode === "direct" ? "direct" : "clone";
  const session = {
    sessionId,
    state: "provisioning",
    mountMode,
    mcpIds: [...(spec.mcpIds || [])],
    modelAccountId: spec.modelAccountId,
    seq: 0,
    sourceWorkspace: spec.workspace,
    workspace:
      mountMode === "clone"
        ? `/tmp/harbor-mock-clone/${sessionId}`
        : spec.workspace,
    agentId: spec.agentId,
    cleanupComplete: false,
    cloneRetained: false,
    syncedBack: false,
    error: null,
  };
  sessions.set(sessionId, session);
  emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
  const first = snapshotOf(sessionId);

  scheduleFor(sessionId, 200, () => {
    const s = sessions.get(sessionId);
    if (!s || s.state === "stopping") return;
    snapshotOf(sessionId, { state: "running" });
    emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
    emit({
      type: "output",
      sessionId,
      output: { stream: "stdout", text: "（模拟）开始执行任务…\n" },
    });
  });
  scheduleFor(sessionId, 700, () => {
    const s = sessions.get(sessionId);
    if (!s || s.state !== "running") return;
    emit({
      type: "output",
      sessionId,
      output: { stream: "stdout", text: "（模拟）调用模型完成摘要。\n" },
    });
    if ((s.mcpIds || []).length) {
      emit({
        type: "audit",
        sessionId,
        audit: { target: "demo-mcp.local", tool: "search", result: "allowed" },
      });
    }
  });
  // 演示扩权：保持 running（契约不增加 awaiting_permission state）
  scheduleFor(sessionId, 1100, () => {
    const s = sessions.get(sessionId);
    if (!s || s.state !== "running" || scenario !== "normal") return;
    emit({
      type: "permission_needed",
      sessionId,
      permission: {
        target: "api.npmjs.org",
        action: "请求访问新地址",
      },
    });
  });
  // 不自动终态，便于演示双会话；用户点停止结束
  scheduleFor(sessionId, 1600, () => {
    const s = sessions.get(sessionId);
    if (!s || s.state !== "running") return;
    emit({
      type: "output",
      sessionId,
      output: {
        stream: "stdout",
        text: "（模拟）已写入工作区示例文件。\n同会话可在 PTY 继续输入；点停止结束本趟。\n",
      },
    });
  });

  return { session: first, error: null };
}

function beginStop(sessionId, fromUser) {
  const session = sessions.get(sessionId);
  if (!session) return;
  if (
    session.state === "stopped" ||
    (session.state === "failed" && session.cleanupComplete)
  ) {
    return;
  }
  clearSessionTimers(sessionId);
  snapshotOf(sessionId, { state: "stopping", cleanupComplete: false });
  emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
  scheduleFor(sessionId, 400, () => {
    if (!sessions.has(sessionId)) return;
    if (scenario === "cleanup" && fromUser) {
      snapshotOf(sessionId, {
        state: "failed",
        cleanupComplete: false,
        error: err("CLEANUP_FAILED", "清理未确认（模拟）", false),
      });
      emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
      emit({
        type: "error",
        sessionId,
        error: err("CLEANUP_FAILED", "清理未确认，锁仍占用（模拟）", false),
      });
      return;
    }
    const cur = sessions.get(sessionId);
    snapshotOf(sessionId, {
      state: "stopped",
      cleanupComplete: true,
      error: null,
      cloneRetained: cur?.mountMode === "clone",
    });
    emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
  });
}

export async function getSession(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) {
    return {
      session: null,
      error: err("SESSION_NOT_FOUND", "未知会话"),
    };
  }
  return { session: snapshotOf(sessionId), error: null };
}

export async function startSession(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) {
    return {
      session: null,
      error: err("SESSION_NOT_FOUND", "未知会话"),
    };
  }
  const terminal =
    (session.state === "stopped" || session.state === "failed") &&
    session.cleanupComplete;
  if (!terminal) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "只能重新开启已停止且清理完成的会话"),
    };
  }
  let active = 0;
  for (const other of sessions.values()) {
    if (other.sessionId === sessionId || other.cleanupComplete) continue;
    active += 1;
    if (
      other.workspace &&
      session.workspace &&
      other.workspace === session.workspace
    ) {
      return {
        session: null,
        error: err("WORKSPACE_BUSY", "该工作区已有会话正在使用"),
      };
    }
  }
  if (active >= 2) {
    return {
      session: null,
      error: err("SESSION_LIMIT", "已达到同时运行的会话上限"),
    };
  }
  snapshotOf(sessionId, {
    state: "provisioning",
    cleanupComplete: false,
    error: null,
  });
  emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
  const opened = snapshotOf(sessionId);
  scheduleFor(sessionId, 200, () => {
    const s = sessions.get(sessionId);
    if (!s || s.state !== "provisioning") return;
    snapshotOf(sessionId, { state: "running" });
    emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
  });
  return { session: opened, error: null };
}

export async function stopSession(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) {
    return {
      session: null,
      error: err("SESSION_NOT_FOUND", "未知会话"),
    };
  }
  if (["stopped", "stopping"].includes(session.state)) {
    return { session: snapshotOf(sessionId), error: null };
  }
  if (session.state === "failed" && session.cleanupComplete) {
    return { session: snapshotOf(sessionId), error: null };
  }
  if (
    session.state === "failed" &&
    !session.cleanupComplete &&
    scenario === "cleanup"
  ) {
    snapshotOf(sessionId, { state: "stopping", error: null, cleanupComplete: false });
    emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
    scheduleFor(sessionId, 300, () => {
      snapshotOf(sessionId, {
        state: "stopped",
        cleanupComplete: true,
        error: null,
      });
      emit({ type: "state_changed", sessionId, state: snapshotOf(sessionId) });
    });
    return { session: snapshotOf(sessionId), error: null };
  }
  beginStop(sessionId, true);
  return { session: snapshotOf(sessionId), error: null };
}

export function onEvent(callback) {
  listeners.add(callback);
  return () => listeners.delete(callback);
}

export function resetMock() {
  clearAllTimers();
  listeners.clear();
  seq = 0;
  sessionNumber = 0;
  accountNumber = 0;
  sessions.clear();
  oauthPending.clear();
  modelAccounts.clear();
  mcpAccounts.clear();
  seedRememberedHosts();
  scenario = "normal";
  mockEngineReady = false;
  mockEngineInstalling = false;
  mockEngineStopRequested = false;
  mockEngineProgress = 0;
  mockEngineProgressLabel = "";
  engineProgressListeners.clear();
  uiPrefs = {
    sidebarCollapsed: false,
    autoCollapseWorkbench: true,
    askClearOnStop: true,
    clearOnStopDefault: "keep",
    theme: "system",
    securityTier: "standard",
    isolationBackend: "seatbelt",
    engineSource: "harbor-colima",
  };
}

seedRememberedHosts();
