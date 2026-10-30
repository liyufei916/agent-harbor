/**
 * 本机记录：会话用量 + 阻拦/扩权明细。不外传。
 * localStorage key: harbor-usage-analytics
 */

const STORAGE_KEY = "harbor-usage-analytics";
const MAX_EVENTS = 800;

/**
 * @typedef {{
 *   t: number,
 *   kind: string,
 *   result?: string,
 *   tool?: string,
 *   target?: string,
 *   sessionId?: string,
 *   state?: string,
 *   code?: string,
 *   decision?: string,
 *   action?: string,
 * }} UsageEvent
 */

/** 测试可注入；浏览器用 localStorage，Node 单测回退内存。 */
let memoryStore = null;

function storageGet() {
  try {
    if (typeof localStorage !== "undefined" && localStorage) {
      return localStorage.getItem(STORAGE_KEY);
    }
  } catch {
    /* ignore */
  }
  return memoryStore;
}

function storageSet(value) {
  try {
    if (typeof localStorage !== "undefined" && localStorage) {
      localStorage.setItem(STORAGE_KEY, value);
      return;
    }
  } catch {
    /* ignore */
  }
  memoryStore = value;
}

function storageRemove() {
  try {
    if (typeof localStorage !== "undefined" && localStorage) {
      localStorage.removeItem(STORAGE_KEY);
      return;
    }
  } catch {
    /* ignore */
  }
  memoryStore = null;
}

function load() {
  try {
    const raw = storageGet();
    if (!raw) return { events: /** @type {UsageEvent[]} */ ([]) };
    const parsed = JSON.parse(raw);
    return { events: Array.isArray(parsed.events) ? parsed.events : [] };
  } catch {
    return { events: [] };
  }
}

function save(state) {
  try {
    storageSet(JSON.stringify({ events: state.events.slice(-MAX_EVENTS) }));
  } catch {
    /* quota / private mode */
  }
}

function shortSession(id) {
  const s = String(id || "").trim();
  if (!s) return "—";
  return s.length <= 12 ? s : `${s.slice(0, 8)}…`;
}

function shortTarget(target) {
  const t = String(target || "").trim();
  if (!t) return "—";
  if (t.length <= 64) return t;
  return `${t.slice(0, 61)}…`;
}

/** @param {UsageEvent} ev */
export function recordUsageEvent(ev) {
  if (!ev?.kind) return;
  const state = load();
  state.events.push({
    t: Date.now(),
    kind: ev.kind,
    result: ev.result || "",
    tool: ev.tool || "",
    target: shortTarget(ev.target),
    sessionId: ev.sessionId || "",
    state: ev.state || "",
    code: ev.code || "",
    decision: ev.decision || "",
    action: ev.action || "",
  });
  save(state);
}

/** 工作台 audit 事件（含 Gate 阻拦 denied/failed） */
export function recordAudit(audit, sessionId = "") {
  if (!audit?.result) return;
  recordUsageEvent({
    kind: "audit",
    result: audit.result,
    tool: audit.tool,
    target: audit.target,
    sessionId,
  });
}

/** 扩权决策结果（deny / once / project） */
export function recordPermissionDecision({
  sessionId = "",
  target = "",
  action = "",
  decision = "",
  uiDemo = false,
} = {}) {
  const d = String(decision || "").trim();
  if (!d) return;
  recordUsageEvent({
    kind: "permission",
    sessionId,
    target,
    action,
    decision: d,
    result: uiDemo ? "ui_demo" : "resolved",
  });
}

/** 会话进入终态 */
export function recordSessionOutcome(snapshot) {
  if (!snapshot?.sessionId || !snapshot.state) return;
  if (!["stopped", "failed"].includes(snapshot.state)) return;
  recordUsageEvent({
    kind: "session_outcome",
    sessionId: snapshot.sessionId,
    state: snapshot.state,
    code: snapshot.error?.code || "",
  });
}

/**
 * @returns {{
 *   sessions: number,
 *   firstTaskSuccessRate: string,
 *   denyRate: string,
 *   mcpFailRate: string,
 *   permissionCount: number,
 *   topFailures: { label: string, count: number }[],
 *   auditTotal: number,
 * }}
 */
export function summarizeUsage() {
  const { events } = load();
  const outcomes = events.filter((e) => e.kind === "session_outcome");
  const firstBySession = new Map();
  for (const e of outcomes) {
    if (!firstBySession.has(e.sessionId)) firstBySession.set(e.sessionId, e);
  }
  const firsts = [...firstBySession.values()];
  const firstOk = firsts.filter((e) => e.state === "stopped" && !e.code).length;
  const firstTaskSuccessRate =
    firsts.length === 0
      ? "—"
      : `${Math.round((100 * firstOk) / firsts.length)}%（${firstOk}/${firsts.length}）`;

  const audits = events.filter((e) => e.kind === "audit");
  const denied = audits.filter((e) => e.result === "denied").length;
  const failed = audits.filter((e) => e.result === "failed").length;
  const denyRate =
    audits.length === 0
      ? "—"
      : `${Math.round((100 * denied) / audits.length)}%（${denied}/${audits.length}）`;
  const mcpFailRate =
    audits.length === 0
      ? "—"
      : `${Math.round((100 * failed) / audits.length)}%（${failed}/${audits.length}）`;

  const failCounts = new Map();
  for (const e of events) {
    let label = "";
    if (e.kind === "session_outcome" && (e.state === "failed" || e.code)) {
      label = e.code || "session_failed";
    } else if (e.kind === "audit" && (e.result === "denied" || e.result === "failed")) {
      label = `${e.result}:${e.tool || "mcp"}`;
    } else if (e.kind === "permission" && e.decision === "deny") {
      label = "permission:deny";
    }
    if (!label) continue;
    failCounts.set(label, (failCounts.get(label) || 0) + 1);
  }
  const topFailures = [...failCounts.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([label, count]) => ({ label, count }));

  return {
    sessions: firstBySession.size,
    firstTaskSuccessRate,
    denyRate,
    mcpFailRate,
    permissionCount: events.filter((e) => e.kind === "permission").length,
    topFailures,
    auditTotal: audits.length,
  };
}

/**
 * @param {"blocks"|"permissions"|"sessions"} channel
 * @param {{ limit?: number }} [opts]
 * @returns {UsageEvent[]}
 */
export function listUsageEvents(channel, opts = {}) {
  const limit = Math.min(Math.max(Number(opts.limit) || 100, 1), 400);
  const { events } = load();
  let filtered = events;
  if (channel === "blocks") {
    filtered = events.filter(
      (e) =>
        e.kind === "audit" &&
        (e.result === "denied" || e.result === "failed"),
    );
  } else if (channel === "permissions") {
    filtered = events.filter((e) => e.kind === "permission");
  } else if (channel === "sessions") {
    filtered = events.filter((e) => e.kind === "session_outcome");
  }
  return filtered.slice(-limit).reverse();
}

export function formatEventTime(t) {
  try {
    return new Date(t).toLocaleString();
  } catch {
    return String(t);
  }
}

export function formatEventRow(ev) {
  const when = formatEventTime(ev.t);
  const sid = shortSession(ev.sessionId);
  if (ev.kind === "audit") {
    return `${when} · ${sid} · ${ev.result} · ${ev.tool || "—"} @ ${ev.target || "—"}`;
  }
  if (ev.kind === "permission") {
    const demo = ev.result === "ui_demo" ? "（界面演示）" : "";
    return `${when} · ${sid} · ${ev.decision}${demo} · ${ev.action || "connect"} @ ${ev.target || "—"}`;
  }
  if (ev.kind === "session_outcome") {
    return `${when} · ${sid} · ${ev.state}${ev.code ? ` · ${ev.code}` : ""}`;
  }
  return `${when} · ${sid} · ${ev.kind}`;
}

export function clearUsageAnalytics() {
  storageRemove();
}

/** @returns {{ exportedAt: string, version: number, events: UsageEvent[], summary: ReturnType<typeof summarizeUsage> }} */
export function exportUsageBundle() {
  const { events } = load();
  return {
    exportedAt: new Date().toISOString(),
    version: 1,
    summary: summarizeUsage(),
    events: events.map((e) => ({
      t: e.t,
      kind: e.kind,
      result: e.result || "",
      tool: e.tool || "",
      target: e.target || "",
      sessionId: e.sessionId ? shortSession(e.sessionId) : "",
      state: e.state || "",
      code: e.code || "",
      decision: e.decision || "",
      action: e.action || "",
    })),
  };
}

const PRICE_KEY = "harbor-usage-prices";

/**
 * @typedef {{ model: string, inputPer1M: number, outputPer1M: number, currency: string, asOf: string }} ModelPrice
 */

/** @returns {ModelPrice[]} */
export function loadModelPrices() {
  try {
    const raw =
      typeof localStorage !== "undefined"
        ? localStorage.getItem(PRICE_KEY)
        : null;
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

/** @param {ModelPrice[]} prices */
export function saveModelPrices(prices) {
  try {
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(PRICE_KEY, JSON.stringify(prices.slice(0, 40)));
    }
  } catch {
    /* ignore */
  }
}

/**
 * 无真实 token 时只展示「未计价」；有人工单价时给出口径说明。
 * @returns {{ status: string, detail: string }}
 */
export function estimateCostStatus() {
  const prices = loadModelPrices();
  const s = summarizeUsage();
  if (prices.length === 0) {
    return {
      status: "未配置单价",
      detail: "在下方填写模型参考单价后，仅作估算口径；当前事件无 token 字段，金额显示为未计价。",
    };
  }
  return {
    status: "未计价（缺 token）",
    detail: `已保存 ${prices.length} 条单价快照；本机记录未含 input/output token，故不编造金额。会话样本 ${s.sessions}。`,
  };
}
