/**
 * 本机使用侧聚合：复用审计事件 + 会话历史，不外传。
 * localStorage key: harbor-usage-analytics
 */

const STORAGE_KEY = "harbor-usage-analytics";
const MAX_EVENTS = 400;

/** @typedef {{ t: number, kind: string, result?: string, tool?: string, target?: string, sessionId?: string, state?: string, code?: string }} UsageEvent */

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

/** @param {UsageEvent} ev */
export function recordUsageEvent(ev) {
  if (!ev?.kind) return;
  const state = load();
  state.events.push({
    t: Date.now(),
    kind: ev.kind,
    result: ev.result || "",
    tool: ev.tool || "",
    target: ev.target || "",
    sessionId: ev.sessionId || "",
    state: ev.state || "",
    code: ev.code || "",
  });
  save(state);
}

/** 工作台 audit 事件 */
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
 *   topFailures: { label: string, count: number }[],
 *   auditTotal: number,
 * }}
 */
export function summarizeUsage() {
  const { events } = load();
  const outcomes = events.filter((e) => e.kind === "session_outcome");
  // 按 sessionId 取首次终态（近似「首任务」）
  const firstBySession = new Map();
  for (const e of outcomes) {
    if (!firstBySession.has(e.sessionId)) firstBySession.set(e.sessionId, e);
  }
  const firsts = [...firstBySession.values()];
  const firstOk = firsts.filter((e) => e.state === "stopped" && !e.code).length;
  const firstTaskSuccessRate =
    firsts.length === 0 ? "—" : `${Math.round((100 * firstOk) / firsts.length)}%（${firstOk}/${firsts.length}）`;

  const audits = events.filter((e) => e.kind === "audit");
  const denied = audits.filter((e) => e.result === "denied").length;
  const failed = audits.filter((e) => e.result === "failed").length;
  const denyRate =
    audits.length === 0 ? "—" : `${Math.round((100 * denied) / audits.length)}%（${denied}/${audits.length}）`;
  const mcpFailRate =
    audits.length === 0 ? "—" : `${Math.round((100 * failed) / audits.length)}%（${failed}/${audits.length}）`;

  const failCounts = new Map();
  for (const e of events) {
    let label = "";
    if (e.kind === "session_outcome" && (e.state === "failed" || e.code)) {
      label = e.code || "session_failed";
    } else if (e.kind === "audit" && (e.result === "denied" || e.result === "failed")) {
      label = `${e.result}:${e.tool || "mcp"}`;
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
    topFailures,
    auditTotal: audits.length,
  };
}

export function clearUsageAnalytics() {
  storageRemove();
}
