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
  saveModelPrices,
  summarizeUsage,
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

  async function paint() {
    const s = summarizeUsage();
    const cost = estimateCostStatus();
    const prices = loadModelPrices();
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
        <h4 class="meta" style="margin-top:12px">参考单价（人工填写，CNY/USD 勿混加）</h4>
        <div class="row" style="gap:8px;flex-wrap:wrap;align-items:flex-end">
          <label class="field">模型 <input id="price-model" placeholder="如 deepseek-chat" /></label>
          <label class="field">输入/1M <input id="price-in" type="number" min="0" step="0.01" placeholder="0" /></label>
          <label class="field">输出/1M <input id="price-out" type="number" min="0" step="0.01" placeholder="0" /></label>
          <label class="field">币种
            <select id="price-ccy"><option value="CNY">CNY</option><option value="USD">USD</option></select>
          </label>
          <button type="button" class="btn" id="price-add">保存单价</button>
        </div>
        <ul class="audit-list" style="margin-top:8px">
          ${
            prices.length === 0
              ? "<li class='meta'>尚无单价快照</li>"
              : prices
                  .map(
                    (p) =>
                      `<li>${p.model} · in ${p.inputPer1M} / out ${p.outputPer1M} ${p.currency} · ${p.asOf}</li>`,
                  )
                  .join("")
          }
        </ul>
        <h4 class="meta" style="margin-top:12px">近期会话终态</h4>
        <ul class="audit-list">
          ${
            sessions.length === 0
              ? "<li class='meta'>暂无终态记录。</li>"
              : sessions.map((e) => `<li>${formatEventRow(e)}</li>`).join("")
          }
        </ul>
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
        paint();
      });
    });
    root.querySelector("#rec-refresh")?.addEventListener("click", () => {
      paint();
      showToast("已刷新", "ok");
    });
    root.querySelector("#rec-clear")?.addEventListener("click", () => {
      clearUsageAnalytics();
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
      const currency = root.querySelector("#price-ccy")?.value || "CNY";
      if (!model) {
        showToast("请填写模型名", "bad");
        return;
      }
      const next = [
        {
          model,
          inputPer1M,
          outputPer1M,
          currency,
          asOf: new Date().toISOString().slice(0, 10),
        },
        ...prices.filter((p) => p.model !== model),
      ];
      saveModelPrices(next);
      paint();
      showToast("已保存单价快照", "ok");
    });
  }

  paint();
}
