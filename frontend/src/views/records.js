/**
 * 侧栏「记录」— 用量 / 计量 / 阻拦 / 扩权 / 诊断
 */
import * as api from "../api.js";
import { showToast } from "../toast.js";
import {
  clearUsageAnalytics,
  costByModel,
  estimateCostStatus,
  exportUsageBundle,
  failureReasonDistribution,
  formatEventRow,
  groupUsageBySession,
  groupUsageByTool,
  listUsageEvents,
  loadModelPrices,
  lockedPriceCurrency,
  meteringSeries,
  removeModelPrice,
  summarizeUsage,
  upsertModelPrice,
} from "../usage-analytics.js";
import {
  TIER_LABELS,
  hostsForTier,
  normalizeTier,
  tierDescription,
} from "../security-tier.js";

/**
 * @param {HTMLElement} root
 * @param {*} ctx
 * @param {{ tab?: string, usageTab?: string }} [opts]
 */
export function mountRecords(root, ctx, opts = {}) {
  const allowed = ["usage", "metering", "blocks", "permissions", "diagnose"];
  const usageAllowed = ["summary", "by-session", "by-tool", "outcomes"];
  const meterAllowed = ["qty", "price"];
  const rangeAllowed = ["1d", "7d", "all"];
  let tab = allowed.includes(opts.tab) ? opts.tab : "usage";
  let usageTab = usageAllowed.includes(opts.usageTab) ? opts.usageTab : "summary";
  let meterTab = "qty";
  let meterRange = "1d";
  /** @type {string} 正在编辑的模型名；空=新建 */
  let editingModel = "";
  const PAGE_SIZE = 10;
  let outcomesPage = 0;
  let bySessionPage = 0;

  async function paint() {
    const s = summarizeUsage();
    const cost = estimateCostStatus();
    const modelCosts = cost.byModel || costByModel();
    const prices = loadModelPrices();
    const lockedCcy = lockedPriceCurrency(prices);
    const meter = meteringSeries({ range: meterRange });
    const modelCostList =
      modelCosts.length === 0
        ? `<p class="meta">尚无按模型 token。</p>`
        : `<ul class="rec-fail-list">${modelCosts
            .map((r) => {
              const tok = `in ${r.inputTokens} / out ${r.outputTokens}`;
              const money =
                r.priced && (r.inputTokens > 0 || r.outputTokens > 0)
                  ? ` ≈ ${(Number(r.cost) || 0).toFixed(4)} ${escapeHtml(r.currency)}`
                  : r.priced && r.inputTokens === 0 && r.outputTokens === 0
                    ? " · 已设单价，尚无出站 token"
                    : " · 无匹配单价";
              return `<li><span class="rec-fail-label">${escapeHtml(r.model)}</span><span class="rec-fail-count">${tok}${money}</span></li>`;
            })
            .join("")}</ul>`;

    const bySessionAll = groupUsageBySession({ limit: 80 });
    const byTool = groupUsageByTool({ limit: 40 });
    const failDist = failureReasonDistribution({ limit: 12 });
    const blocks = listUsageEvents("blocks", { limit: 120 });
    const perms = listUsageEvents("permissions", { limit: 120 });
    const sessions = listUsageEvents("sessions", { limit: 80 });

    const top =
      failDist.length === 0
        ? `<p class="meta">暂无失败样本。</p>`
        : `<ul class="rec-fail-list">${failDist
            .map(
              (f) =>
                `<li><span class="rec-fail-label">${escapeHtml(f.label)}</span><span class="rec-fail-count">${f.count}</span></li>`,
            )
            .join("")}</ul>`;

    const sessionGroups = renderPagedSessionTable(bySessionAll, bySessionPage, PAGE_SIZE);
    bySessionPage = sessionGroups.page;
    const toolGroups =
      byTool.length === 0
        ? "<li class='meta'>暂无按工具聚合。</li>"
        : byTool
            .map(
              (r) =>
                `<li>${escapeHtml(r.tool)} · ${r.total} 次（允 ${r.allowed}/拒 ${r.denied}/败 ${r.failed}${r.other ? `/其它 ${r.other}` : ""}）</li>`,
            )
            .join("");

    const outcomes = renderPagedEventList(sessions, outcomesPage, PAGE_SIZE, "rec-outcomes");
    outcomesPage = outcomes.page;

    const health = ctx.state.health;
    const tier = normalizeTier(ctx.state.uiPrefs?.securityTier || "standard");
    let hosts = [];
    try {
      const hr = await api.listRememberedHosts(ctx.state.workspace || "");
      hosts = hr.hosts || [];
    } catch {
      hosts = [];
    }

    /** @type {{ value: string, label: string }[]} */
    let modelChoices = [];
    try {
      const acc = await api.listAccounts("model");
      const seen = new Set();
      for (const a of acc.accounts || []) {
        if (a.kind && a.kind !== "model") continue;
        const value = String(a.model || a.name || "").trim();
        if (!value || seen.has(value)) continue;
        seen.add(value);
        const bits = [a.name || value];
        if (a.model && a.model !== a.name) bits.push(a.model);
        if (a.host) bits.push(a.host);
        modelChoices.push({ value, label: bits.join(" · ") });
      }
    } catch {
      modelChoices = [];
    }
    for (const p of prices) {
      if (p.model && !modelChoices.some((m) => m.value === p.model)) {
        modelChoices.push({ value: p.model, label: `${p.model}（仅单价）` });
      }
    }

    const editing = prices.find((p) => p.model === editingModel);
    const peersForCcy = editingModel
      ? prices.filter((p) => p.model !== editingModel)
      : prices;
    const peerLocked = lockedPriceCurrency(peersForCcy);
    const formCcy =
      editing?.currency ||
      (peerLocked === "CNY" || peerLocked === "USD"
        ? peerLocked
        : lockedCcy === "CNY" || lockedCcy === "USD"
          ? lockedCcy
          : "CNY");
    const ccySelectLocked = peerLocked === "CNY" || peerLocked === "USD";
    const modelOpts =
      modelChoices.length === 0
        ? `<option value="">（请先在「模型」页添加账号）</option>`
        : `${editingModel ? "" : `<option value="">选择已添加的模型</option>`}${modelChoices
            .map(
              (m) =>
                `<option value="${escapeAttr(m.value)}" ${
                  editingModel === m.value ? "selected" : ""
                }>${escapeHtml(m.label)}</option>`,
            )
            .join("")}`;

    const pricePanel = renderPricePanel({
      prices,
      modelOpts,
      editing,
      editingModel,
      lockedCcy,
      peerLocked,
      formCcy,
      ccySelectLocked,
    });
    const meteringPanel = renderMeteringPanel(meter, meterTab, meterRange, pricePanel);

    root.innerHTML = `
      <h2>记录</h2>
      <p class="sub">仅存本机，不外传。导出为脱敏 JSON（会话 ID 截断）；桌面会弹出「另存为」并在访达中显示文件。</p>

      <div class="row records-tabs records-tabs-primary" role="tablist" aria-label="记录分类">
        <button type="button" class="tab tab-usage ${tab === "usage" ? "on" : ""}" data-tab="usage" role="tab" aria-selected="${tab === "usage"}">用量</button>
        <button type="button" class="tab tab-metering ${tab === "metering" ? "on" : ""}" data-tab="metering" role="tab" aria-selected="${tab === "metering"}">计量</button>
        <button type="button" class="tab tab-blocks ${tab === "blocks" ? "on" : ""}" data-tab="blocks" role="tab" aria-selected="${tab === "blocks"}">阻拦</button>
        <button type="button" class="tab ${tab === "permissions" ? "on" : ""}" data-tab="permissions" role="tab" aria-selected="${tab === "permissions"}">扩权</button>
        <button type="button" class="tab ${tab === "diagnose" ? "on" : ""}" data-tab="diagnose" role="tab" aria-selected="${tab === "diagnose"}">诊断</button>
      </div>

      <section class="card records-panel-usage ${tab === "usage" ? "" : "hidden"}" data-panel="usage">
        <div class="row records-tabs records-tabs-secondary" role="tablist" aria-label="用量子页">
          <button type="button" class="tab ${usageTab === "summary" ? "on" : ""}" data-usage-tab="summary" role="tab" aria-selected="${usageTab === "summary"}">本机汇总</button>
          <button type="button" class="tab ${usageTab === "by-session" ? "on" : ""}" data-usage-tab="by-session" role="tab" aria-selected="${usageTab === "by-session"}">按会话</button>
          <button type="button" class="tab ${usageTab === "by-tool" ? "on" : ""}" data-usage-tab="by-tool" role="tab" aria-selected="${usageTab === "by-tool"}">按工具</button>
          <button type="button" class="tab ${usageTab === "outcomes" ? "on" : ""}" data-usage-tab="outcomes" role="tab" aria-selected="${usageTab === "outcomes"}">近期会话终态</button>
        </div>

        <div class="${usageTab === "summary" ? "" : "hidden"}" data-usage-panel="summary">
          <p class="meta">按会话首次终态近似「首任务」；阻拦率来自审计 denied。费用与参考单价见「计量」。</p>
          <div class="rec-metric-grid" aria-label="本机汇总指标">
            <div class="rec-metric"><span class="rec-metric-label">会话样本</span><span class="rec-metric-value">${s.sessions}</span></div>
            <div class="rec-metric"><span class="rec-metric-label">审计</span><span class="rec-metric-value">${s.auditTotal}</span></div>
            <div class="rec-metric"><span class="rec-metric-label">扩权</span><span class="rec-metric-value">${s.permissionCount}</span></div>
            <div class="rec-metric"><span class="rec-metric-label">首任务成功</span><span class="rec-metric-value">${escapeHtml(s.firstTaskSuccessRate)}</span></div>
            <div class="rec-metric"><span class="rec-metric-label">阻拦占比</span><span class="rec-metric-value">${escapeHtml(s.denyRate)}</span></div>
            <div class="rec-metric"><span class="rec-metric-label">工具失败</span><span class="rec-metric-value">${escapeHtml(s.mcpFailRate)}</span></div>
          </div>
          <div class="rec-cost-banner">
            <span class="rec-cost-status">${escapeHtml(cost.status)}</span>
            <span class="meta">${escapeHtml(cost.detail)}</span>
          </div>
          <h4 class="rec-section-title">按模型 token / 粗算</h4>
          ${modelCostList}
          <h4 class="rec-section-title">失败原因分布</h4>
          ${top}
        </div>

        <div class="${usageTab === "by-session" ? "" : "hidden"}" data-usage-panel="by-session">
          <p class="meta">按会话聚合审计 / 阻拦 / 失败 / 扩权（每页 ${PAGE_SIZE} 条）。</p>
          ${sessionGroups.html}
        </div>

        <div class="${usageTab === "by-tool" ? "" : "hidden"}" data-usage-panel="by-tool">
          <p class="meta">按工具名聚合允许 / 拒绝 / 失败次数。</p>
          <ul class="audit-list">${toolGroups}</ul>
        </div>

        <div class="${usageTab === "outcomes" ? "" : "hidden"}" data-usage-panel="outcomes">
          <p class="meta">会话进入 stopped / failed 的本机终态流水（每页 ${PAGE_SIZE} 条）。</p>
          ${outcomes.html}
        </div>
      </section>

      <section class="card records-panel-metering ${tab === "metering" ? "" : "hidden"}" data-panel="metering">
        ${meteringPanel}
      </section>

      <section class="card records-panel-blocks ${tab === "blocks" ? "" : "hidden"}">
        <h3 class="rec-panel-title rec-panel-title-blocks">阻拦明细</h3>
        <p class="meta">Gate / 策略直接拒绝的审计（未进扩权弹窗）。与「用量」分类独立。</p>
        <ul class="audit-list">
          ${
            blocks.length === 0
              ? "<li class='meta'>暂无阻拦。</li>"
              : blocks.map((e) => `<li>${formatEventRow(e)}</li>`).join("")
          }
        </ul>
      </section>

      <section class="card ${tab === "permissions" ? "" : "hidden"}">
        <h3>扩权明细</h3>
        <ul class="audit-list">
          ${
            perms.length === 0
              ? "<li class='meta'>暂无扩权决策。</li>"
              : perms.map((e) => `<li>${formatEventRow(e)}</li>`).join("")
          }
        </ul>
      </section>

      <section class="card ${tab === "diagnose" ? "" : "hidden"}">
        <h3>安全诊断（只读聚合）</h3>
        <p class="meta">不参与放行；汇总隔离、凭据库、档位与记住主机。</p>
        <ul class="audit-list">
          <li>隔离：${health?.available === false ? "不可用" : health?.available ? "就绪" : "未知"}${health?.reason ? ` · ${health.reason}` : ""}</li>
          <li>凭据库：${health?.vaultNotice || "无告警"}</li>
          <li>安全档位：${TIER_LABELS[tier] || tier}</li>
          <li>档位说明：${tierDescription(tier)}</li>
          <li>档位种子主机数：${hostsForTier(tier).length}（下次新会话由桌面注入 ExtraHosts）</li>
          <li>已记住主机：${hosts.length}</li>
          ${hosts
            .slice(0, 20)
            .map((h) => `<li class="meta">${h.host} · ${h.scope} · ${h.workspace || ""}</li>`)
            .join("")}
          <li>本机阻拦条数：${blocks.length} · 扩权条数：${perms.length}</li>
        </ul>
      </section>

      <div class="actions" style="margin-top:12px">
        <button type="button" class="btn" id="rec-refresh">刷新</button>
        <button type="button" class="btn primary" id="rec-export">导出脱敏 JSON</button>
        <button type="button" class="btn" id="rec-clear">清除本机记录</button>
      </div>
    `;

    root.querySelectorAll("[data-tab]").forEach((btn) => {
      btn.addEventListener("click", () => {
        tab = btn.getAttribute("data-tab") || "usage";
        if (!allowed.includes(tab)) tab = "usage";
        paint();
      });
    });
    root.querySelectorAll("[data-usage-tab]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const next = btn.getAttribute("data-usage-tab") || "summary";
        if (!usageAllowed.includes(next)) return;
        usageTab = next;
        paint();
      });
    });
    root.querySelectorAll("[data-meter-tab]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const next = btn.getAttribute("data-meter-tab") || "qty";
        if (!meterAllowed.includes(next)) return;
        meterTab = next;
        paint();
      });
    });
    root.querySelectorAll("[data-meter-range]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const next = btn.getAttribute("data-meter-range") || "1d";
        if (!rangeAllowed.includes(next)) return;
        meterRange = next;
        paint();
      });
    });
    root.querySelector("#rec-bysession-prev")?.addEventListener("click", () => {
      if (bySessionPage <= 0) return;
      bySessionPage -= 1;
      paint();
    });
    root.querySelector("#rec-bysession-next")?.addEventListener("click", () => {
      bySessionPage += 1;
      paint();
    });
    root.querySelector("#rec-outcomes-prev")?.addEventListener("click", () => {
      if (outcomesPage <= 0) return;
      outcomesPage -= 1;
      paint();
    });
    root.querySelector("#rec-outcomes-next")?.addEventListener("click", () => {
      outcomesPage += 1;
      paint();
    });
    root.querySelector("#rec-refresh")?.addEventListener("click", () => {
      paint();
      showToast("已刷新", "ok");
    });
    root.querySelector("#rec-clear")?.addEventListener("click", () => {
      clearUsageAnalytics();
      outcomesPage = 0;
      bySessionPage = 0;
      paint();
      showToast("已清除本机记录", "ok");
    });
    root.querySelector("#rec-export")?.addEventListener("click", async () => {
      const bundle = exportUsageBundle();
      const name = `harbor-records-${Date.now()}.json`;
      const text = JSON.stringify(bundle, null, 2);
      if (typeof api.saveTextFile === "function") {
        try {
          const res = await api.saveTextFile({
            suggestedName: name,
            content: text,
            reveal: !api.isMock(),
          });
          if (res?.cancelled) {
            showToast("已取消导出", "");
            return;
          }
          if (res?.error) {
            showToast(
              `${res.error.code || "错误"}: ${res.error.message || "导出失败"}`,
              "bad",
            );
            return;
          }
          if (api.isMock()) {
            showToast(
              `已下载 ${name}。请打开 Finder → 边栏「下载」（或 ~/Downloads）。`,
              "ok",
              { durationMs: 9000 },
            );
            return;
          }
          showToast(`已保存并在访达中显示：${res?.path || name}`, "ok", {
            durationMs: 9000,
          });
          return;
        } catch (e) {
          showToast(e?.message || "导出失败", "bad");
          return;
        }
      }
      showToast("当前通道未绑定 SaveTextFile，请重新生成绑定后再导出", "bad");
    });
    root.querySelector("#price-add")?.addEventListener("click", () => {
      const model = root.querySelector("#price-model")?.value?.trim();
      const inputPer1M = Number(root.querySelector("#price-in")?.value || 0);
      const outputPer1M = Number(root.querySelector("#price-out")?.value || 0);
      let currency = root.querySelector("#price-ccy")?.value || "CNY";
      if (ccySelectLocked) currency = peerLocked;
      const wasEdit = Boolean(editingModel);
      const result = upsertModelPrice(prices, {
        model: wasEdit ? editingModel : model,
        inputPer1M,
        outputPer1M,
        currency,
        asOf: new Date().toISOString().slice(0, 10),
      });
      if (!result.ok) {
        showToast(result.error, "bad");
        return;
      }
      editingModel = "";
      paint();
      showToast(wasEdit ? "已更新单价" : "已添加单价", "ok");
    });
    root.querySelector("#price-cancel-edit")?.addEventListener("click", () => {
      editingModel = "";
      paint();
    });
    root.querySelectorAll("[data-price-edit]").forEach((btn) => {
      btn.addEventListener("click", () => {
        editingModel = btn.getAttribute("data-price-edit") || "";
        paint();
      });
    });
    root.querySelectorAll("[data-price-del]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const model = btn.getAttribute("data-price-del") || "";
        removeModelPrice(prices, model);
        if (editingModel === model) editingModel = "";
        paint();
        showToast("已删除单价", "ok");
      });
    });
  }

  paint();
}

function renderPagedSessionTable(rows, page, pageSize) {
  if (!rows.length) {
    return { html: `<p class="meta">暂无按会话聚合。</p>`, page: 0 };
  }
  const pageCount = Math.max(1, Math.ceil(rows.length / pageSize));
  let p = page;
  if (p >= pageCount) p = pageCount - 1;
  if (p < 0) p = 0;
  const start = p * pageSize;
  const slice = rows.slice(start, start + pageSize);
  const from = start + 1;
  const to = start + slice.length;
  const table = `<div class="rec-session-table" role="table">
    <div class="rec-session-head" role="row">
      <span>会话</span><span>终态</span><span>审计</span><span>拒</span><span>败</span><span>扩权</span>
    </div>
    ${slice
      .map(
        (r) =>
          `<div class="rec-session-row" role="row">
            <span class="rec-mono">${escapeHtml(r.shortId)}</span>
            <span class="rec-outcome">${escapeHtml(r.outcome)}</span>
            <span>${r.audits}</span>
            <span class="${r.denied ? "rec-warn" : ""}">${r.denied}</span>
            <span class="${r.failed ? "rec-bad" : ""}">${r.failed}</span>
            <span>${r.permissions}</span>
          </div>`,
      )
      .join("")}
  </div>
  <div class="row" style="justify-content:space-between;align-items:center;margin-top:8px;gap:8px;flex-wrap:wrap">
    <span class="meta">第 ${from}–${to} 条 · 共 ${rows.length} 条 · 每页 ${pageSize}</span>
    <div class="actions" style="margin:0">
      <button type="button" class="btn" id="rec-bysession-prev" ${p <= 0 ? "disabled" : ""}>上一页</button>
      <span class="meta">${p + 1}/${pageCount}</span>
      <button type="button" class="btn" id="rec-bysession-next" ${p >= pageCount - 1 ? "disabled" : ""}>下一页</button>
    </div>
  </div>`;
  return { html: table, page: p };
}

function renderPagedEventList(events, page, pageSize, idPrefix) {
  if (!events.length) {
    return { html: `<ul class="audit-list"><li class='meta'>暂无终态记录。</li></ul>`, page: 0 };
  }
  const pageCount = Math.max(1, Math.ceil(events.length / pageSize));
  let p = page;
  if (p >= pageCount) p = pageCount - 1;
  if (p < 0) p = 0;
  const start = p * pageSize;
  const slice = events.slice(start, start + pageSize);
  const from = start + 1;
  const to = start + slice.length;
  return {
    page: p,
    html: `
      <ul class="audit-list" id="${idPrefix}-list">
        ${slice.map((e) => `<li>${formatEventRow(e)}</li>`).join("")}
      </ul>
      <div class="row" style="justify-content:space-between;align-items:center;margin-top:8px;gap:8px;flex-wrap:wrap">
        <span class="meta">第 ${from}–${to} 条 · 共 ${events.length} 条 · 每页 ${pageSize}</span>
        <div class="actions" style="margin:0">
          <button type="button" class="btn" id="rec-outcomes-prev" ${p <= 0 ? "disabled" : ""}>上一页</button>
          <span class="meta">${p + 1}/${pageCount}</span>
          <button type="button" class="btn" id="rec-outcomes-next" ${p >= pageCount - 1 ? "disabled" : ""}>下一页</button>
        </div>
      </div>`,
  };
}

function renderPricePanel({
  prices,
  modelOpts,
  editing,
  editingModel,
  lockedCcy,
  peerLocked,
  formCcy,
  ccySelectLocked,
}) {
  return `
    <details class="rec-price-details" id="rec-price-details" ${editingModel ? "open" : ""}>
      <summary class="rec-price-summary">参考单价（默认收起 · 点此展开配置）</summary>
      <div class="rec-price-body">
        <p class="meta">人工填写；同一列表内 CNY/USD 勿混加。单价名须与账号「模型名」一致。</p>
        <div class="row" style="gap:8px;flex-wrap:wrap;align-items:flex-end">
          <label class="field">模型
            <select id="price-model" ${editingModel ? "disabled" : ""}>${modelOpts}</select>
          </label>
          <label class="field">输入/1M <input id="price-in" type="number" min="0" step="0.01" placeholder="0" value="${editing ? editing.inputPer1M : ""}" /></label>
          <label class="field">输出/1M <input id="price-out" type="number" min="0" step="0.01" placeholder="0" value="${editing ? editing.outputPer1M : ""}" /></label>
          <label class="field">币种
            <select id="price-ccy" ${ccySelectLocked ? "disabled" : ""}>
              <option value="CNY" ${formCcy === "CNY" ? "selected" : ""}>CNY</option>
              <option value="USD" ${formCcy === "USD" ? "selected" : ""}>USD</option>
            </select>
          </label>
          <button type="button" class="btn primary" id="price-add">${editingModel ? "更新单价" : "添加单价"}</button>
          ${editingModel ? `<button type="button" class="btn" id="price-cancel-edit">取消编辑</button>` : ""}
        </div>
        <p class="meta">${
          lockedCcy === "MIXED"
            ? "检测到 CNY/USD 混用，请删除一侧后再保存。"
            : ccySelectLocked
              ? `当前列表币种已锁定为 ${peerLocked}（可删光后重选）。`
              : "尚未锁定币种；第一条单价将决定本列表币种。"
        }</p>
        <ul class="audit-list" style="margin-top:8px">
          ${
            prices.length === 0
              ? "<li class='meta'>尚无单价快照</li>"
              : prices
                  .map(
                    (p) =>
                      `<li class="row" style="justify-content:space-between;gap:8px;align-items:center;flex-wrap:wrap">
                        <span>${escapeHtml(p.model)} · in ${p.inputPer1M} / out ${p.outputPer1M} ${escapeHtml(p.currency)} · ${escapeHtml(p.asOf || "")}</span>
                        <span class="row" style="gap:6px">
                          <button type="button" class="btn" data-price-edit="${escapeAttr(p.model)}">修改</button>
                          <button type="button" class="btn" data-price-del="${escapeAttr(p.model)}">删除</button>
                        </span>
                      </li>`,
                  )
                  .join("")
          }
        </ul>
      </div>
    </details>`;
}

function renderMeteringPanel(meter, meterTab, meterRange, pricePanel) {
  const t = meter.totals;
  const ccy = escapeHtml(t.currency || "");
  const seriesHtml =
    meter.models.length === 0
      ? `<p class="meta">该时段暂无 token 事件。</p>`
      : meter.models
          .map((m) => {
            const maxTok = Math.max(
              1,
              ...m.buckets.map((b) => b.inputTokens + b.outputTokens),
              ...m.buckets.map((b) => b.inputCost + b.outputCost),
            );
            const maxCost = Math.max(
              1e-9,
              ...m.buckets.map((b) => b.inputCost + b.outputCost),
            );
            if (meterTab === "price") {
              return `<div class="rec-meter-model">
                <div class="row" style="justify-content:space-between;gap:8px;flex-wrap:wrap">
                  <strong class="rec-mono">${escapeHtml(m.model)}</strong>
                  <span class="meta">入估 ${(m.inputCost || 0).toFixed(4)} · 出估 ${(m.outputCost || 0).toFixed(4)} ${escapeHtml(m.currency || ccy)}${m.priced ? "" : "（无单价）"}</span>
                </div>
                <div class="rec-meter-bars" aria-hidden="true">
                  ${m.buckets
                    .map((b, i) => {
                      const v = b.inputCost + b.outputCost;
                      const h = Math.max(2, Math.round((100 * v) / maxCost));
                      return `<div class="rec-meter-col" title="${escapeAttr(meter.bucketLabels[i])}: 入 ${(b.inputCost || 0).toFixed(4)} / 出 ${(b.outputCost || 0).toFixed(4)}">
                        <div class="rec-meter-bar rec-meter-bar-price" style="height:${h}%"></div>
                        <span class="rec-meter-tick">${escapeHtml(meter.bucketLabels[i])}</span>
                      </div>`;
                    })
                    .join("")}
                </div>
              </div>`;
            }
            return `<div class="rec-meter-model">
              <div class="row" style="justify-content:space-between;gap:8px;flex-wrap:wrap">
                <strong class="rec-mono">${escapeHtml(m.model)}</strong>
                <span class="meta">入 ${m.inputTokens} · 出 ${m.outputTokens}</span>
              </div>
              <div class="rec-meter-bars" aria-hidden="true">
                ${m.buckets
                  .map((b, i) => {
                    const v = b.inputTokens + b.outputTokens;
                    const h = Math.max(2, Math.round((100 * v) / maxTok));
                    const inH = v ? Math.round((100 * b.inputTokens) / v) : 50;
                    return `<div class="rec-meter-col" title="${escapeAttr(meter.bucketLabels[i])}: 入 ${b.inputTokens} / 出 ${b.outputTokens}">
                      <div class="rec-meter-bar" style="height:${h}%">
                        <span class="rec-meter-bar-in" style="flex:${Math.max(inH, 1)}"></span>
                        <span class="rec-meter-bar-out" style="flex:${Math.max(100 - inH, 1)}"></span>
                      </div>
                      <span class="rec-meter-tick">${escapeHtml(meter.bucketLabels[i])}</span>
                    </div>`;
                  })
                  .join("")}
              </div>
            </div>`;
          })
          .join("");

  return `
    <div class="row records-tabs records-tabs-secondary" role="tablist" aria-label="计量子页">
      <button type="button" class="tab ${meterTab === "qty" ? "on" : ""}" data-meter-tab="qty" role="tab" aria-selected="${meterTab === "qty"}">数量</button>
      <button type="button" class="tab ${meterTab === "price" ? "on" : ""}" data-meter-tab="price" role="tab" aria-selected="${meterTab === "price"}">价格</button>
    </div>
    <div class="row records-range" role="group" aria-label="时间范围">
      <button type="button" class="btn ${meterRange === "1d" ? "primary" : ""}" data-meter-range="1d">近1天</button>
      <button type="button" class="btn ${meterRange === "7d" ? "primary" : ""}" data-meter-range="7d">近7天</button>
      <button type="button" class="btn ${meterRange === "all" ? "primary" : ""}" data-meter-range="all">近30天</button>
      <span class="meta">${escapeHtml(meter.rangeLabel)} · 分模型时序</span>
    </div>
    <div class="rec-metric-grid" style="margin-top:12px">
      ${
        meterTab === "qty"
          ? `<div class="rec-metric"><span class="rec-metric-label">入数量累计</span><span class="rec-metric-value">${t.inputTokens}</span></div>
             <div class="rec-metric"><span class="rec-metric-label">出数量累计</span><span class="rec-metric-value">${t.outputTokens}</span></div>`
          : `<div class="rec-metric"><span class="rec-metric-label">入价格评估</span><span class="rec-metric-value">${(t.inputCost || 0).toFixed(4)} <small>${ccy}</small></span></div>
             <div class="rec-metric"><span class="rec-metric-label">出价格评估</span><span class="rec-metric-value">${(t.outputCost || 0).toFixed(4)} <small>${ccy}</small></span></div>`
      }
    </div>
    <h4 class="rec-section-title">分模型时序</h4>
    <p class="meta">${meterTab === "qty" ? "柱高为该时段入+出 token；青绿=入、琥珀=出。" : "柱高为该时段入+出估价合计（须已配参考单价）。"}</p>
    <div class="rec-meter-series">${seriesHtml}</div>
    ${pricePanel}
  `;
}

function escapeHtml(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function escapeAttr(s) {
  return escapeHtml(s).replace(/'/g, "&#39;");
}
