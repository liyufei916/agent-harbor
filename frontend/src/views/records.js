/**
 * 侧栏「记录」— 用量 / 阻拦 / 扩权 / 诊断
 */
import * as api from "../api.js";
import { showToast } from "../toast.js";
import {
  clearUsageAnalytics,
  estimateCostStatus,
  exportUsageBundle,
  formatEventRow,
  listUsageEvents,
  loadModelPrices,
  lockedPriceCurrency,
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
 * @param {{ tab?: string }} [opts]
 */
export function mountRecords(root, ctx, opts = {}) {
  const allowed = ["usage", "blocks", "permissions", "diagnose"];
  let tab = allowed.includes(opts.tab) ? opts.tab : "usage";
  /** @type {string} 正在编辑的模型名；空=新建 */
  let editingModel = "";
  /** 近期会话终态分页（每页 10 行） */
  const SESSION_PAGE_SIZE = 10;
  let sessionPage = 0;

  async function paint() {
    const s = summarizeUsage();
    const cost = estimateCostStatus();
    const prices = loadModelPrices();
    const lockedCcy = lockedPriceCurrency(prices);
    const top =
      s.topFailures.length === 0
        ? "<li class='meta'>暂无</li>"
        : s.topFailures.map((f) => `<li>${f.label} · ${f.count}</li>`).join("");

    const blocks = listUsageEvents("blocks", { limit: 120 });
    const perms = listUsageEvents("permissions", { limit: 120 });
    const sessions = listUsageEvents("sessions", { limit: 80 });

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
    // 已有单价但账号列表里没有的，仍保留可选以便修改
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

    root.innerHTML = `
      <h2>记录</h2>
      <p class="sub">仅存本机，不外传。导出为脱敏 JSON（会话 ID 截断）；桌面会弹出「另存为」并在访达中显示文件。</p>

      <div class="row records-tabs" role="tablist" style="gap:8px;margin:12px 0;flex-wrap:wrap">
        <button type="button" class="btn ${tab === "usage" ? "primary" : ""}" data-tab="usage" role="tab">用量</button>
        <button type="button" class="btn ${tab === "blocks" ? "primary" : ""}" data-tab="blocks" role="tab">阻拦</button>
        <button type="button" class="btn ${tab === "permissions" ? "primary" : ""}" data-tab="permissions" role="tab">扩权</button>
        <button type="button" class="btn ${tab === "diagnose" ? "primary" : ""}" data-tab="diagnose" role="tab">诊断</button>
      </div>

      <section class="card ${tab === "usage" ? "" : "hidden"}" data-panel="usage">
        <h3>本机会话用量</h3>
        <p class="meta">按会话首次终态近似「首任务」；阻拦率来自审计 denied。</p>
        <div class="meta">
          <p>会话样本：${s.sessions} · 审计：${s.auditTotal} · 扩权：${s.permissionCount}</p>
          <p>首任务成功率：${s.firstTaskSuccessRate}</p>
          <p>阻拦占比：${s.denyRate} · 工具失败：${s.mcpFailRate}</p>
          <p>费用估算：<strong>${cost.status}</strong> — ${cost.detail}</p>
          <p>典型失败 Top：</p>
          <ul class="audit-list">${top}</ul>
        </div>
        <h4 class="meta" style="margin-top:12px">参考单价（人工填写；同一列表内 CNY/USD 勿混加）</h4>
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
        <h4 class="meta" style="margin-top:12px">近期会话终态</h4>
        ${(() => {
          if (sessions.length === 0) {
            return `<ul class="audit-list"><li class='meta'>暂无终态记录。</li></ul>`;
          }
          const pageCount = Math.max(1, Math.ceil(sessions.length / SESSION_PAGE_SIZE));
          if (sessionPage >= pageCount) sessionPage = pageCount - 1;
          if (sessionPage < 0) sessionPage = 0;
          const start = sessionPage * SESSION_PAGE_SIZE;
          const slice = sessions.slice(start, start + SESSION_PAGE_SIZE);
          const from = start + 1;
          const to = start + slice.length;
          return `
        <ul class="audit-list" id="rec-session-list">
          ${slice.map((e) => `<li>${formatEventRow(e)}</li>`).join("")}
        </ul>
        <div class="row" style="justify-content:space-between;align-items:center;margin-top:8px;gap:8px;flex-wrap:wrap">
          <span class="meta">第 ${from}–${to} 条 · 共 ${sessions.length} 条 · 每页 ${SESSION_PAGE_SIZE}</span>
          <div class="actions" style="margin:0">
            <button type="button" class="btn" id="rec-session-prev" ${sessionPage <= 0 ? "disabled" : ""}>上一页</button>
            <span class="meta">${sessionPage + 1}/${pageCount}</span>
            <button type="button" class="btn" id="rec-session-next" ${sessionPage >= pageCount - 1 ? "disabled" : ""}>下一页</button>
          </div>
        </div>`;
        })()}
      </section>

      <section class="card ${tab === "blocks" ? "" : "hidden"}">
        <h3>阻拦明细</h3>
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
        sessionPage = 0;
        paint();
      });
    });
    root.querySelector("#rec-session-prev")?.addEventListener("click", () => {
      if (sessionPage <= 0) return;
      sessionPage -= 1;
      paint();
    });
    root.querySelector("#rec-session-next")?.addEventListener("click", () => {
      sessionPage += 1;
      paint();
    });
    root.querySelector("#rec-refresh")?.addEventListener("click", () => {
      sessionPage = 0;
      paint();
      showToast("已刷新", "ok");
    });
    root.querySelector("#rec-clear")?.addEventListener("click", () => {
      clearUsageAnalytics();
      sessionPage = 0;
      paint();
      showToast("已清除本机记录", "ok");
    });
    root.querySelector("#rec-export")?.addEventListener("click", async () => {
      const bundle = exportUsageBundle();
      const name = `harbor-records-${Date.now()}.json`;
      const text = JSON.stringify(bundle, null, 2);
      // 优先 SaveTextFile：live=系统另存为+访达显示；mock=下载并提示「下载」文件夹
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
