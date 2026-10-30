/**
 * PAGE-DESIGN：侧栏「记录」— 本机用量 / 阻拦 / 扩权（不外传）
 */
import { showToast } from "../toast.js";
import {
  clearUsageAnalytics,
  formatEventRow,
  listUsageEvents,
  summarizeUsage,
} from "../usage-analytics.js";

/**
 * @param {HTMLElement} root
 * @param {*} ctx
 * @param {{ tab?: string }} [opts]
 */
export function mountRecords(root, ctx, opts = {}) {
  let tab = opts.tab === "blocks" || opts.tab === "permissions" ? opts.tab : "usage";

  function paint() {
    const s = summarizeUsage();
    const top =
      s.topFailures.length === 0
        ? "<li class='meta'>暂无</li>"
        : s.topFailures.map((f) => `<li>${f.label} · ${f.count}</li>`).join("");

    const blocks = listUsageEvents("blocks", { limit: 120 });
    const perms = listUsageEvents("permissions", { limit: 120 });
    const sessions = listUsageEvents("sessions", { limit: 80 });

    root.innerHTML = `
      <h2>记录</h2>
      <p class="sub">仅存本机浏览器/桌面 localStorage，不外传。密钥与 Sentinel 不会写入此处。</p>

      <div class="row records-tabs" role="tablist" style="gap:8px;margin:12px 0;flex-wrap:wrap">
        <button type="button" class="btn ${tab === "usage" ? "primary" : ""}" data-tab="usage" role="tab" aria-selected="${tab === "usage"}">用量</button>
        <button type="button" class="btn ${tab === "blocks" ? "primary" : ""}" data-tab="blocks" role="tab" aria-selected="${tab === "blocks"}">阻拦</button>
        <button type="button" class="btn ${tab === "permissions" ? "primary" : ""}" data-tab="permissions" role="tab" aria-selected="${tab === "permissions"}">扩权</button>
      </div>

      <section class="card ${tab === "usage" ? "" : "hidden"}" data-panel="usage">
        <h3>本机会话用量</h3>
        <p class="meta">按会话首次终态近似「首任务」；阻拦率来自审计 denied，工具失败来自 failed。</p>
        <div class="meta" id="rec-usage-stats">
          <p>会话样本：${s.sessions} · 审计条数：${s.auditTotal} · 扩权决策：${s.permissionCount}</p>
          <p>首任务成功率：${s.firstTaskSuccessRate}</p>
          <p>阻拦（denied）占比：${s.denyRate}</p>
          <p>工具调用失败占比：${s.mcpFailRate}</p>
          <p>典型失败 Top：</p>
          <ul class="audit-list">${top}</ul>
        </div>
        <h4 class="meta" style="margin-top:12px">近期会话终态</h4>
        <ul class="audit-list" id="rec-sessions">
          ${
            sessions.length === 0
              ? "<li class='meta'>暂无终态记录。跑完会话后会出现在这里。</li>"
              : sessions.map((e) => `<li>${formatEventRow(e)}</li>`).join("")
          }
        </ul>
      </section>

      <section class="card ${tab === "blocks" ? "" : "hidden"}" data-panel="blocks">
        <h3>阻拦明细</h3>
        <p class="meta">Gate / 策略直接拒绝（denied）或工具失败（failed），<strong>未</strong>进入扩权弹窗。与「扩权」页分开。</p>
        <ul class="audit-list">
          ${
            blocks.length === 0
              ? "<li class='meta'>暂无阻拦。未授权出站被直接拒绝时会记入。</li>"
              : blocks.map((e) => `<li>${formatEventRow(e)}</li>`).join("")
          }
        </ul>
      </section>

      <section class="card ${tab === "permissions" ? "" : "hidden"}" data-panel="permissions">
        <h3>扩权明细</h3>
        <p class="meta">用户对 <code>permission_needed</code> 的 deny / once / project。界面演示项会标注。</p>
        <ul class="audit-list">
          ${
            perms.length === 0
              ? "<li class='meta'>暂无扩权决策。跨主机重定向触发 Modal 并选择后会出现。</li>"
              : perms.map((e) => `<li>${formatEventRow(e)}</li>`).join("")
          }
        </ul>
      </section>

      <div class="actions" style="margin-top:12px">
        <button type="button" class="btn" id="rec-refresh">刷新</button>
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
  }

  paint();
}
