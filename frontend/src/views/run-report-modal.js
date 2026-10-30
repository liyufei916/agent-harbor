/**
 * 运行报告：停止后汇总本机会话记录（脱敏）。
 */
import { listUsageEvents, summarizeUsage } from "../usage-analytics.js";

function escapeHtml(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/"/g, "&quot;");
}

let escHandler = null;

/**
 * @param {HTMLElement | null} root
 * @param {{ sessionId?: string, workspace?: string, agentId?: string }} opts
 */
export function mountRunReportModal(root, opts = {}) {
  if (!root) return;
  hideRunReportModal(root);
  const sid = opts.sessionId || "";
  const allBlocks = listUsageEvents("blocks", { limit: 200 });
  const allPerms = listUsageEvents("permissions", { limit: 200 });
  const blocks = sid
    ? allBlocks.filter((e) => e.sessionId === sid)
    : allBlocks.slice(0, 20);
  const perms = sid
    ? allPerms.filter((e) => e.sessionId === sid)
    : allPerms.slice(0, 20);
  const s = summarizeUsage();

  root.classList.remove("hidden");
  root.innerHTML = `
    <div class="modal-card permission-overview-card" role="dialog" aria-modal="true" aria-labelledby="run-report-title">
      <div class="row" style="justify-content:space-between;gap:12px;align-items:flex-start">
        <h2 id="run-report-title" style="margin:0">运行报告</h2>
        <button type="button" class="btn" id="run-report-close">关闭</button>
      </div>
      <p class="meta">会话 ${escapeHtml(sid || "—")} · Agent ${escapeHtml(opts.agentId || "—")}</p>
      <p class="meta">工作区 ${escapeHtml(opts.workspace || "—")}</p>
      <h4>本机摘要（不外传）</h4>
      <ul class="audit-list">
        <li>累计会话样本：${s.sessions} · 审计 ${s.auditTotal} · 扩权决策 ${s.permissionCount}</li>
        <li>首任务成功率：${escapeHtml(s.firstTaskSuccessRate)}</li>
        <li>阻拦占比：${escapeHtml(s.denyRate)} · 工具失败：${escapeHtml(s.mcpFailRate)}</li>
      </ul>
      <h4>本会话阻拦（${blocks.length}）</h4>
      <ul class="audit-list">
        ${
          blocks.length === 0
            ? "<li class='meta'>无</li>"
            : blocks
                .slice(0, 30)
                .map(
                  (e) =>
                    `<li>${escapeHtml(e.result)} · ${escapeHtml(e.tool)} @ ${escapeHtml(e.target)}</li>`,
                )
                .join("")
        }
      </ul>
      <h4>本会话扩权（${perms.length}）</h4>
      <ul class="audit-list">
        ${
          perms.length === 0
            ? "<li class='meta'>无</li>"
            : perms
                .slice(0, 30)
                .map(
                  (e) =>
                    `<li>${escapeHtml(e.decision)} · ${escapeHtml(e.target)}</li>`,
                )
                .join("")
        }
      </ul>
      <p class="meta">完整明细见侧栏「记录」。密钥与 Sentinel 不会写入报告。</p>
    </div>`;

  root.querySelector("#run-report-close")?.addEventListener("click", () => {
    hideRunReportModal(root);
  });
  escHandler = (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      hideRunReportModal(root);
    }
  };
  window.addEventListener("keydown", escHandler);
}

export function hideRunReportModal(root) {
  if (escHandler) {
    window.removeEventListener("keydown", escHandler);
    escHandler = null;
  }
  if (!root) return;
  root.classList.add("hidden");
  root.innerHTML = "";
}
