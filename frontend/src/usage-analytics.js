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
 *   model?: string,
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
    model: String(ev.model || "").trim(),
    sessionId: ev.sessionId || "",
    state: ev.state || "",
    code: ev.code || "",
    decision: ev.decision || "",
    action: ev.action || "",
  };
  if (!row.model) delete row.model;
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
    model: audit.model,
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
 * 按会话聚合：审计/扩权/终态计数，便于本机迭代对照。
 * @param {{ limit?: number }} [opts]
 * @returns {{ sessionId: string, shortId: string, audits: number, denied: number, failed: number, permissions: number, outcome: string }[]}
 */
export function groupUsageBySession(opts = {}) {
  const limit = Math.min(Math.max(Number(opts.limit) || 20, 1), 80);
  const { events } = load();
  /** @type {Map<string, { sessionId: string, audits: number, denied: number, failed: number, permissions: number, outcome: string, lastT: number }>} */
  const by = new Map();
  const touch = (sid) => {
    const id = String(sid || "").trim() || "(无会话)";
    if (!by.has(id)) {
      by.set(id, {
        sessionId: id,
        audits: 0,
        denied: 0,
        failed: 0,
        permissions: 0,
        outcome: "",
        lastT: 0,
      });
    }
    return by.get(id);
  };
  for (const e of events) {
    const row = touch(e.sessionId);
    row.lastT = Math.max(row.lastT, Number(e.t) || 0);
    if (e.kind === "audit") {
      row.audits += 1;
      if (e.result === "denied") row.denied += 1;
      if (e.result === "failed") row.failed += 1;
    } else if (e.kind === "permission") {
      row.permissions += 1;
    } else if (e.kind === "session_outcome") {
      row.outcome = e.code ? `${e.state}:${e.code}` : e.state || row.outcome;
    }
  }
  return [...by.values()]
    .sort((a, b) => b.lastT - a.lastT)
    .slice(0, limit)
    .map((r) => ({
      sessionId: r.sessionId,
      shortId: shortSession(r.sessionId === "(无会话)" ? "" : r.sessionId),
      audits: r.audits,
      denied: r.denied,
      failed: r.failed,
      permissions: r.permissions,
      outcome: r.outcome || "—",
    }));
}

/**
 * 按工具聚合审计结果（含失败原因分布）。
 * @param {{ limit?: number }} [opts]
 * @returns {{ tool: string, total: number, allowed: number, denied: number, failed: number, other: number }[]}
 */
export function groupUsageByTool(opts = {}) {
  const limit = Math.min(Math.max(Number(opts.limit) || 15, 1), 60);
  const { events } = load();
  /** @type {Map<string, { tool: string, total: number, allowed: number, denied: number, failed: number, other: number }>} */
  const by = new Map();
  for (const e of events) {
    if (e.kind !== "audit") continue;
    const tool = String(e.tool || "mcp").trim() || "mcp";
    if (!by.has(tool)) {
      by.set(tool, {
        tool,
        total: 0,
        allowed: 0,
        denied: 0,
        failed: 0,
        other: 0,
      });
    }
    const row = by.get(tool);
    row.total += 1;
    if (e.result === "allowed") row.allowed += 1;
    else if (e.result === "denied") row.denied += 1;
    else if (e.result === "failed") row.failed += 1;
    else row.other += 1;
  }
  return [...by.values()]
    .sort((a, b) => b.total - a.total || b.denied + b.failed - (a.denied + a.failed))
    .slice(0, limit);
}

/**
 * 失败原因分布（会话错误码 + 审计 denied/failed + 扩权拒绝）。
 * @param {{ limit?: number }} [opts]
 * @returns {{ label: string, count: number }[]}
 */
export function failureReasonDistribution(opts = {}) {
  const limit = Math.min(Math.max(Number(opts.limit) || 12, 1), 40);
  return summarizeUsage().topFailures.slice(0, limit);
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
    const modelBit = ev.model ? ` · ${ev.model}` : "";
    const tok =
      Number(ev.inputTokens) > 0 || Number(ev.outputTokens) > 0
        ? ` · in ${ev.inputTokens || 0}/out ${ev.outputTokens || 0}`
        : "";
    return `${when} · ${sid} · ${ev.result} · ${ev.tool || "—"}${modelBit} @ ${ev.target || "—"}${tok}`;
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
      model: e.model || "",
      sessionId: e.sessionId ? shortSession(e.sessionId) : "",
      state: e.state || "",
      code: e.code || "",
      decision: e.decision || "",
      action: e.action || "",
      inputTokens: e.inputTokens || undefined,
      outputTokens: e.outputTokens || undefined,
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
 * 将审计 token 归到单价模型：优先 event.model；旧事件无 model 时用主机启发（如 deepseek 主机 ↔ 名称含 deepseek 的单价）。
 * @param {UsageEvent} ev
 * @param {ModelPrice[]} prices
 * @returns {string} 归集键；空表示未匹配
 */
export function priceKeyForEvent(ev, prices = loadModelPrices()) {
  const list = prices || [];
  const model = String(ev?.model || "").trim();
  if (model) {
    if (list.some((p) => p.model === model)) return model;
    return model; // 有模型名但尚未填单价 → 仍按模型归集，费用页提示无单价
  }
  const host = String(ev?.target || "").toLowerCase();
  if (!host) return "";
  /** @type {string[]} */
  const hits = [];
  for (const p of list) {
    const name = String(p.model || "").toLowerCase();
    if (!name) continue;
    if (host.includes("deepseek") && name.includes("deepseek")) hits.push(p.model);
    else if (
      host.includes("openai") &&
      (name.includes("gpt") || name.includes("o1") || name.includes("o3"))
    ) {
      hits.push(p.model);
    } else if (
      (host.includes("anthropic") || host.includes("claude")) &&
      name.includes("claude")
    ) {
      hits.push(p.model);
    }
  }
  // 旧事件无 model 字段时：主机启发仅在唯一命中时归集，避免多单价误绑
  return hits.length === 1 ? hits[0] : "";
}

/**
 * @returns {{ model: string, inputTokens: number, outputTokens: number, cost: number | null, currency: string, priced: boolean }[]}
 */
export function costByModel() {
  const prices = loadModelPrices();
  const { events } = load();
  /** @type {Map<string, { inputTokens: number, outputTokens: number }>} */
  const by = new Map();
  const solePrice = prices.length === 1 ? prices[0].model : "";
  for (const e of events) {
    const inTok = Number(e.inputTokens) || 0;
    const outTok = Number(e.outputTokens) || 0;
    if (!(inTok > 0 || outTok > 0)) continue;
    let key = priceKeyForEvent(e, prices);
    // 仅一条单价时：未标注/未匹配的 token 归到该单价（兼容旧事件）
    if (!key || (key && !prices.some((p) => p.model === key))) {
      if (solePrice) key = solePrice;
      else if (!key) key = "(未标注模型)";
    }
    const row = by.get(key) || { inputTokens: 0, outputTokens: 0 };
    row.inputTokens += inTok;
    row.outputTokens += outTok;
    by.set(key, row);
  }
  const priceMap = new Map(prices.map((p) => [p.model, p]));
  const out = [];
  for (const [model, tok] of by.entries()) {
    const p = priceMap.get(model);
    if (p) {
      const cost =
        (tok.inputTokens / 1e6) * Number(p.inputPer1M || 0) +
        (tok.outputTokens / 1e6) * Number(p.outputPer1M || 0);
      out.push({
        model,
        inputTokens: tok.inputTokens,
        outputTokens: tok.outputTokens,
        cost,
        currency: p.currency || "",
        priced: true,
      });
    } else {
      out.push({
        model,
        inputTokens: tok.inputTokens,
        outputTokens: tok.outputTokens,
        cost: null,
        currency: "",
        priced: false,
      });
    }
  }
  // 已填单价但尚无 token 的也列出，避免「加了价却像没记录」
  for (const p of prices) {
    if (!by.has(p.model)) {
      out.push({
        model: p.model,
        inputTokens: 0,
        outputTokens: 0,
        cost: 0,
        currency: p.currency || "",
        priced: true,
      });
    }
  }
  return out.sort((a, b) => b.inputTokens + b.outputTokens - (a.inputTokens + a.outputTokens));
}

/**
 * 计量时序：按模型 × 时间桶汇总入/出 token 与估价（人工单价）。
 * @param {{ range?: "1d"|"7d"|"all", now?: number }} [opts]
 */
export function meteringSeries(opts = {}) {
  const range = opts.range === "7d" || opts.range === "all" ? opts.range : "1d";
  const now = Number(opts.now) > 0 ? Number(opts.now) : Date.now();
  const prices = loadModelPrices();
  const priceMap = new Map(prices.map((p) => [p.model, p]));
  const solePrice = prices.length === 1 ? prices[0].model : "";
  const ccy = lockedPriceCurrency(prices);
  const currency = ccy === "MIXED" ? prices[0]?.currency || "" : ccy || prices[0]?.currency || "";

  let rangeMs = 24 * 3600 * 1000;
  let bucketMs = 3600 * 1000;
  let rangeLabel = "近1天";
  if (range === "7d") {
    rangeMs = 7 * 24 * 3600 * 1000;
    bucketMs = 24 * 3600 * 1000;
    rangeLabel = "近7天";
  } else if (range === "all") {
    rangeMs = 30 * 24 * 3600 * 1000;
    bucketMs = 24 * 3600 * 1000;
    rangeLabel = "近30天";
  }
  const since = now - rangeMs;
  const bucketCount = Math.max(1, Math.ceil(rangeMs / bucketMs));

  const bucketLabels = [];
  for (let i = 0; i < bucketCount; i++) {
    const t = since + i * bucketMs;
    const d = new Date(t);
    if (bucketMs >= 24 * 3600 * 1000) {
      bucketLabels.push(`${d.getMonth() + 1}/${d.getDate()}`);
    } else {
      const hh = String(d.getHours()).padStart(2, "0");
      bucketLabels.push(`${hh}:00`);
    }
  }

  /** @type {Map<string, { inputTokens: number, outputTokens: number, buckets: { inputTokens: number, outputTokens: number }[] }>} */
  const by = new Map();
  const ensure = (model) => {
    if (!by.has(model)) {
      by.set(model, {
        inputTokens: 0,
        outputTokens: 0,
        buckets: Array.from({ length: bucketCount }, () => ({
          inputTokens: 0,
          outputTokens: 0,
        })),
      });
    }
    return by.get(model);
  };

  const { events } = load();
  for (const e of events) {
    const t = Number(e.t) || 0;
    if (t < since || t > now) continue;
    const inTok = Number(e.inputTokens) || 0;
    const outTok = Number(e.outputTokens) || 0;
    if (!(inTok > 0 || outTok > 0)) continue;
    let key = priceKeyForEvent(e, prices);
    if (!key || (key && !priceMap.has(key) && key !== "(未标注模型)")) {
      if (solePrice) key = solePrice;
      else if (!key) key = "(未标注模型)";
    }
    const idx = Math.min(bucketCount - 1, Math.max(0, Math.floor((t - since) / bucketMs)));
    const row = ensure(key);
    row.inputTokens += inTok;
    row.outputTokens += outTok;
    row.buckets[idx].inputTokens += inTok;
    row.buckets[idx].outputTokens += outTok;
  }

  const costOf = (model, inTok, outTok) => {
    const p = priceMap.get(model);
    if (!p) return { inputCost: 0, outputCost: 0, priced: false };
    return {
      inputCost: (inTok / 1e6) * Number(p.inputPer1M || 0),
      outputCost: (outTok / 1e6) * Number(p.outputPer1M || 0),
      priced: true,
    };
  };

  let totalIn = 0;
  let totalOut = 0;
  let totalInCost = 0;
  let totalOutCost = 0;
  const models = [];
  for (const [model, row] of by.entries()) {
    const c = costOf(model, row.inputTokens, row.outputTokens);
    totalIn += row.inputTokens;
    totalOut += row.outputTokens;
    totalInCost += c.inputCost;
    totalOutCost += c.outputCost;
    models.push({
      model,
      priced: c.priced,
      currency: priceMap.get(model)?.currency || currency,
      inputTokens: row.inputTokens,
      outputTokens: row.outputTokens,
      inputCost: c.inputCost,
      outputCost: c.outputCost,
      buckets: row.buckets.map((b) => {
        const bc = costOf(model, b.inputTokens, b.outputTokens);
        return {
          inputTokens: b.inputTokens,
          outputTokens: b.outputTokens,
          inputCost: bc.inputCost,
          outputCost: bc.outputCost,
        };
      }),
    });
  }
  models.sort(
    (a, b) => b.inputTokens + b.outputTokens - (a.inputTokens + a.outputTokens),
  );

  return {
    range,
    rangeLabel,
    since,
    until: now,
    bucketMs,
    bucketLabels,
    totals: {
      inputTokens: totalIn,
      outputTokens: totalOut,
      inputCost: totalInCost,
      outputCost: totalOutCost,
      currency,
    },
    models,
  };
}

/**
 * 无真实 token 时只展示「未计价」；有人工单价时给出口径说明。
 * @returns {{ status: string, detail: string, byModel: ReturnType<typeof costByModel> }}
 */
export function estimateCostStatus() {
  const prices = loadModelPrices();
  const s = summarizeUsage();
  const byModel = costByModel();
  let inputTokens = 0;
  let outputTokens = 0;
  for (const row of byModel) {
    inputTokens += row.inputTokens;
    outputTokens += row.outputTokens;
  }
  if (prices.length === 0) {
    return {
      status: inputTokens || outputTokens ? "有 token · 先填单价" : "先填单价",
      detail: inputTokens || outputTokens
        ? `已采集 in ${inputTokens} / out ${outputTokens}；下方填参考单价即可粗算。`
        : "下方填模型参考单价（CNY/USD 勿混）；跑一轮经 Gate 出站后才会有 token。",
      byModel,
    };
  }
  if (!(inputTokens > 0 || outputTokens > 0)) {
    return {
      status: "有单价 · 先跑一轮",
      detail: `已存 ${prices.length} 条单价；尚无 token。新会话经 Gate 出站写入 usage 后即可按模型估算。会话样本 ${s.sessions}。`,
      byModel,
    };
  }
  const priced = byModel.filter((r) => r.priced && (r.inputTokens > 0 || r.outputTokens > 0));
  const unmatched = byModel.filter((r) => !r.priced && (r.inputTokens > 0 || r.outputTokens > 0));
  let totalCost = 0;
  const bits = [];
  for (const row of priced) {
    totalCost += Number(row.cost) || 0;
    bits.push(
      `${row.model} in ${row.inputTokens}/out ${row.outputTokens} ≈ ${(Number(row.cost) || 0).toFixed(4)} ${row.currency}`,
    );
  }
  for (const row of unmatched) {
    bits.push(`${row.model} in ${row.inputTokens}/out ${row.outputTokens}（无匹配单价）`);
  }
  const ccy = lockedPriceCurrency(prices);
  const ccyLabel = ccy === "MIXED" ? "" : ccy;
  return {
    status: "已估算（参考）",
    detail:
      bits.length === 1
        ? `${bits[0]}（非官方账单）`
        : `合计 ≈ ${totalCost.toFixed(4)} ${ccyLabel}；${bits.join("；")}（按模型匹配单价，非官方账单）`,
    byModel,
  };
}
