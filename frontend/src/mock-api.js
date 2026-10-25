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
/** @type {Map<string, object>} */
const sessions = new Map();
/** @type {Map<string, number[]>} sessionId → timer ids */
const sessionTimers = new Map();
let modelAccount = null;
let mcpAccount = null;
/** @type {Map<string, object>} pending OAuth */
const oauthPending = new Map();
let uiPrefs = { sidebarCollapsed: false, autoCollapseWorkbench: true };
const listeners = new Set();

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
  return {
    available: true,
    reason: "",
    isolationLevel: "mock-seatbelt",
    error: null,
  };
}

export async function listSessions() {
  const list = [...sessions.values()].map((s) => ({
    sessionId: s.sessionId,
    state: s.state,
    workspace: s.workspace,
    agentId: s.agentId,
    cleanupComplete: s.cleanupComplete,
    mountMode: s.mountMode,
    mcpIds: [...(s.mcpIds || [])],
  }));
  return { sessions: list, error: null };
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
  if ((!kind || kind === "model") && modelAccount) accounts.push({ ...modelAccount });
  if ((!kind || kind === "mcp") && mcpAccount) accounts.push({ ...mcpAccount });
  return { accounts, error: null };
}

export async function removeAccount(accountId) {
  if (modelAccount?.id === accountId) {
    if (activeSessions().length) {
      return { error: err("SESSION_BUSY", "会话仍引用该账号（模拟）") };
    }
    modelAccount = null;
    return { error: null };
  }
  if (mcpAccount?.id === accountId) {
    if (activeSessions().length) {
      return { error: err("SESSION_BUSY", "会话仍引用该账号（模拟）") };
    }
    mcpAccount = null;
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
      mcpSummary: mcpAccount ? [mcpAccount.name] : [],
      deniedPathCategories: ["系统目录"],
    },
    error: null,
  };
}

export async function listRememberedHosts() {
  return {
    hosts: [
      {
        host: "api.example.com",
        workspace: FIXED.workspace,
        scope: "project",
      },
    ],
    error: null,
  };
}

export async function revokeRememberedHost() {
  return { error: null };
}

export async function resolvePermission(input) {
  if (!input?.sessionId || !input?.target || !input?.decision) {
    return { error: err("INVALID_ARGUMENT", "扩权参数不完整（模拟）") };
  }
  if (!["deny", "once", "project"].includes(input.decision)) {
    return { error: err("INVALID_ARGUMENT", "非法决策（模拟）") };
  }
  if (!sessions.has(input.sessionId)) {
    return { error: err("SESSION_NOT_FOUND", "会话不存在（模拟）") };
  }
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
  uiPrefs = {
    sidebarCollapsed: !!prefs?.sidebarCollapsed,
    autoCollapseWorkbench: prefs?.autoCollapseWorkbench !== false,
  };
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
  const allowed = [FIXED.modelProfileId, "dsh-model", "custom"];
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
  modelAccount = {
    id: "mock-model",
    kind: "model",
    name: input.name,
    profileId,
    host,
    origin: input.origin || "",
  };
  return { account: { ...modelAccount }, error: null };
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
      mcpAccount = {
        id: "mock-mcp",
        kind: "mcp",
        name: p.name,
        profileId: p.profileId,
        host: "oauth.mcp.local",
        authMode: "oauth",
      };
      oauthPending.delete(pendingId);
    },
  );
  return {
    authUrl: "https://example.com/oauth/mock",
    pendingId,
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
  if (!modelAccount || modelAccount.id !== input.modelAccountId) {
    return {
      session: null,
      error: err("ACCOUNT_NOT_FOUND", "模型账号不存在（模拟）"),
    };
  }
  return { session: snapshotOf(input.sessionId), error: null };
}

export async function writePty(input) {
  const session = sessions.get(input?.sessionId);
  if (!session || !isActive(session) || session.state === "stopping") {
    return { error: err("INVALID_ARGUMENT", "无可用 PTY（模拟）") };
  }
  emit({
    type: "pty",
    sessionId: session.sessionId,
    pty: { stream: "stdout", text: `[pty echo] ${input.data || ""}` },
  });
  return { error: null };
}

export async function resizePty(input) {
  if (!sessions.has(input?.sessionId)) {
    return { error: err("INVALID_ARGUMENT", "无可用 PTY（模拟）") };
  }
  return { error: null };
}

export async function connectMcp(input) {
  if (scenario === "auth") {
    return {
      account: null,
      error: err("UPSTREAM_AUTH_FAILED", "MCP 认证失败（模拟）", false),
    };
  }
  if (!input?.profileId || !input?.name || !input?.apiKey) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "profileId、name、apiKey 均必填"),
    };
  }
  if (input.profileId !== FIXED.mcpProfileId) {
    return {
      account: null,
      error: err("INVALID_ARGUMENT", "仅支持固定 profileId=demo-mcp"),
    };
  }
  mcpAccount = {
    id: "mock-mcp",
    kind: "mcp",
    name: input.name,
    profileId: input.profileId,
    host: "demo-mcp.local",
    authMode: "token",
  };
  return { account: { ...mcpAccount }, error: null };
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
  if (!modelAccount || spec.modelAccountId !== modelAccount.id) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "请先连接模型账号"),
    };
  }
  const expected = spec.agentId === "dsh" ? "dsh-model" : "demo-model";
  if (
    modelAccount.profileId !== "custom" &&
    modelAccount.profileId !== expected
  ) {
    return {
      session: null,
      error: err("INVALID_ARGUMENT", "Agent 与模型 profile 不匹配"),
    };
  }
  if (spec.mcpIds.length > 0) {
    if (!mcpAccount || !spec.mcpIds.every((id) => id === mcpAccount.id)) {
      return {
        session: null,
        error: err("INVALID_ARGUMENT", "请先连接 MCP 账号，且 mcpIds 须匹配"),
      };
    }
  }
  if (![FIXED.agent.id, "dsh"].includes(spec.agentId)) {
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
  if (active.some((s) => normPath(s.workspace) === ws)) {
    return {
      session: null,
      error: err(
        "WORKSPACE_BUSY",
        "该工作区已有会话在运行，请打开已有会话或换目录（模拟）",
      ),
    };
  }

  const sessionId = `sess-mock-${++sessionNumber}`;
  const session = {
    sessionId,
    state: "provisioning",
    mountMode: spec.mountMode === "direct" ? "direct" : "clone",
    mcpIds: [...(spec.mcpIds || [])],
    seq: 0,
    workspace: spec.workspace,
    agentId: spec.agentId,
    cleanupComplete: false,
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
  // 不自动终态，便于演示双会话；用户点停止结束
  scheduleFor(sessionId, 1400, () => {
    const s = sessions.get(sessionId);
    if (!s || s.state !== "running") return;
    emit({
      type: "output",
      sessionId,
      output: {
        stream: "stdout",
        text: "（模拟）已写入工作区示例文件。\n任务可继续；点停止结束本趟。\n",
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
    snapshotOf(sessionId, {
      state: "stopped",
      cleanupComplete: true,
      error: null,
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
  sessions.clear();
  oauthPending.clear();
  modelAccount = null;
  mcpAccount = null;
  scenario = "normal";
  uiPrefs = { sidebarCollapsed: false, autoCollapseWorkbench: true };
}
