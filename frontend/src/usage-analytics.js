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
  const row = {
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
  };
  if (Number(ev.inputTokens) > 0) row.inputTokens = Number(ev.inputTokens);
  if (Number(ev.outputTokens) > 0) row.outputTokens = Number(ev.outputTokens);
  state.events.push(row);
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
    inputTokens: Number(audit.inputTokens) > 0 ? Number(audit.inputTokens) : undefined,
    outputTokens: Number(audit.outputTokens) > 0 ? Number(audit.outputTokens) : undefined,
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

/** @type {string | null} Node 单测回退 */
let priceMemoryStore = null;

function priceStorageGet() {
  try {
    if (typeof localStorage !== "undefined" && localStorage) {
      return localStorage.getItem(PRICE_KEY);
    }
  } catch {
    /* ignore */
  }
  return priceMemoryStore;
}

function priceStorageSet(value) {
  try {
    if (typeof localStorage !== "undefined" && localStorage) {
      localStorage.setItem(PRICE_KEY, value);
      return;
    }
  } catch {
    /* ignore */
  }
  priceMemoryStore = value;
}

/** @returns {ModelPrice[]} */
export function loadModelPrices() {
  try {
    const raw = priceStorageGet();
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
    priceStorageSet(JSON.stringify((prices || []).slice(0, 40)));
  } catch {
    /* ignore */
  }
}

/** 已保存单价的统一币种；空表示尚无快照。CNY/USD 不得混加。 */
export function lockedPriceCurrency(prices = loadModelPrices()) {
  const set = new Set(
    (prices || [])
      .map((p) => String(p?.currency || "").toUpperCase())
      .filter((c) => c === "CNY" || c === "USD"),
  );
  if (set.size === 0) return "";
  if (set.size > 1) return "MIXED";
  return [...set][0];
}

/**
 * @param {ModelPrice[]} prices
 * @param {ModelPrice} entry
 * @returns {{ ok: true, prices: ModelPrice[] } | { ok: false, error: string }}
 */
export function upsertModelPrice(prices, entry) {
  const model = String(entry?.model || "").trim();
  const currency = String(entry?.currency || "").toUpperCase();
  if (!model) return { ok: false, error: "请选择模型" };
  if (currency !== "CNY" && currency !== "USD") {
    return { ok: false, error: "币种仅支持 CNY 或 USD" };
  }
  // 同模型覆盖时排除自身，便于唯一条目改币种；其余条目仍锁币种
  const others = (prices || []).filter((p) => p.model !== model);
  const locked = lockedPriceCurrency(others);
  if (locked === "MIXED") {
    return { ok: false, error: "已有 CNY/USD 混用快照，请先删除一侧后再保存" };
  }
  if (locked && locked !== currency) {
    return {
      ok: false,
      error: `已有 ${locked} 单价，勿与 ${currency} 混加；请改币种或删除现有快照`,
    };
  }
  const inputPer1M = Number(entry.inputPer1M);
  const outputPer1M = Number(entry.outputPer1M);
  if (!(inputPer1M >= 0) || !(outputPer1M >= 0)) {
    return { ok: false, error: "单价须为非负数" };
  }
  const nextEntry = {
    model,
    inputPer1M,
    outputPer1M,
    currency,
    asOf: entry.asOf || new Date().toISOString().slice(0, 10),
  };
  const next = [nextEntry, ...others];
  saveModelPrices(next);
  return { ok: true, prices: next };
}

/** @param {ModelPrice[]} prices @param {string} model */
export function removeModelPrice(prices, model) {
  const want = String(model || "").trim();
  const next = (prices || []).filter((p) => p.model !== want);
  saveModelPrices(next);
  return next;
}

/**
 * 无真实 token 时只展示「未计价」；有人工单价时给出口径说明。
 * @returns {{ status: string, detail: string }}
 */
export function estimateCostStatus() {
  const prices = loadModelPrices();
  const s = summarizeUsage();
  const { events } = load();
  let inputTokens = 0;
  let outputTokens = 0;
  for (const e of events) {
    if (Number(e.inputTokens) > 0) inputTokens += Number(e.inputTokens);
    if (Number(e.outputTokens) > 0) outputTokens += Number(e.outputTokens);
  }
  if (prices.length === 0) {
    return {
      status: inputTokens || outputTokens ? "有 token · 先填单价" : "先填单价",
      detail: inputTokens || outputTokens
        ? `已采集 in ${inputTokens} / out ${outputTokens}；下方填参考单价即可粗算。`
        : "下方填模型参考单价（CNY/USD 勿混）；跑一轮经 Gate 出站后才会有 token。",
    };
  }
  if (!(inputTokens > 0 || outputTokens > 0)) {
    return {
      status: "有单价 · 先跑一轮",
      detail: `已存 ${prices.length} 条单价；尚无 token。新会话经 Gate 出站写入 usage 后即可估算。会话样本 ${s.sessions}。`,
    };
  }
  const ccy = lockedPriceCurrency(prices);
  // 用列表首条单价做粗算（人工快照；非账单）
  const p0 = prices[0];
  const cost =
    (inputTokens / 1e6) * Number(p0.inputPer1M || 0) +
    (outputTokens / 1e6) * Number(p0.outputPer1M || 0);
  const ccyLabel = ccy === "MIXED" ? p0.currency || "" : ccy || p0.currency || "";
  return {
    status: "已估算（参考）",
    detail: `in ${inputTokens} / out ${outputTokens} token ≈ ${cost.toFixed(4)} ${ccyLabel}（按「${p0.model}」单价粗算，非官方账单）。`,
  };
}
