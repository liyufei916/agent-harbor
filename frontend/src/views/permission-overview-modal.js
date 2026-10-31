/**
 * PAGE-DESIGN §5.5 权限一览（弹窗）
 */
import * as api from "../api.js";
import { showToast } from "../toast.js";
import {
  projectWorkspaceOf,
  revokeRememberedHostUnified,
} from "../remembered-hosts.js";

let escHandler = null;

function escapeAttr(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;");
}

/**
 * @param {HTMLElement | null} root
 * @param {{
 *   sessionId?: string,
 *   mountMode?: string,
 *   snapshot?: object,
 *   ctx?: object,
 *   onChanged?: () => void,
 * }} opts
 */
export async function mountPermissionOverviewModal(root, opts = {}) {
  if (!root) return;
  hidePermissionOverviewModal(root);

  const sid = opts.sessionId || "";
  const mountMode = opts.mountMode || "";
  const projectWs = projectWorkspaceOf(opts.snapshot, opts.ctx);
  root.classList.remove("hidden");
  root.innerHTML = `
    <div class="modal-card permission-overview-card" role="dialog" aria-modal="true" aria-labelledby="perm-overview-title">
      <div class="row" style="justify-content:space-between;align-items:flex-start;gap:12px">
        <h2 id="perm-overview-title" style="margin:0">当前边界</h2>
        <button type="button" class="btn" id="perm-overview-close">关闭</button>
      </div>
      <p class="meta">加载中…</p>
    </div>`;

  const closeBtn = root.querySelector("#perm-overview-close");
  if (closeBtn) closeBtn.onclick = () => hidePermissionOverviewModal(root);
  bindEsc(root);

  const [res, hostsRes] = await Promise.all([
    api.getPermissionOverview(sid),
    api.listRememberedHosts(projectWs),
  ]);
  // 关闭后不再刷新内容
  if (!root.querySelector("#perm-overview-title")) return;

  const o = res.overview;
  let remembered = hostsRes.hosts || [];
  if (!remembered.length && projectWs) {
    const all = await api.listRememberedHosts("");
    remembered = (all.hosts || []).filter(
      (h) =>
        !h.workspace ||
        h.workspace === projectWs ||
        h.workspace === opts.snapshot?.workspace ||
        h.workspace === opts.snapshot?.sourceWorkspace,
    );
  }
  const card = root.querySelector(".permission-overview-card");
  if (!card) return;
  const displayWs = projectWs || o?.workspace || "—";

  card.innerHTML = `
    <div class="row" style="justify-content:space-between;align-items:flex-start;gap:12px">
      <h2 id="perm-overview-title" style="margin:0">当前边界</h2>
      <button type="button" class="btn" id="perm-overview-close">关闭</button>
    </div>
    ${
      res.error
        ? `<p class="bad">${res.error.code}: ${res.error.message}</p>`
        : `
      <p class="meta">会话 ${sid || "—"} · 工作区：${escapeAttr(displayWs)}</p>
      <h4>文件</h4>
      <ul class="audit-list">
        <li>可读写：${escapeAttr(o?.workspace || displayWs)}（${mountMode === "clone" ? "安全副本" : "直接编辑"}）</li>
        ${(o?.deniedPathCategories || []).map((m) => `<li>默认不可见：${m}</li>`).join("")}
      </ul>
      <h4>网络（白名单）</h4>
      <ul class="audit-list" id="perm-hosts">
        ${(o?.allowedHosts || [])
          .map((h) => `<li>${h} · 模型/系统</li>`)
          .join("") || "<li class='meta'>无</li>"}
        ${remembered
          .map(
            (h) =>
              `<li class="row" style="justify-content:space-between;gap:8px">
                <span>${escapeAttr(h.host)} · ${h.scope === "project" ? "本项目始终允许" : "本会话"}</span>
                <button type="button" class="btn" data-rev="${escapeAttr(h.host)}" data-ws="${escapeAttr(h.workspace || projectWs)}">撤销</button>
              </li>`,
          )
          .join("")}
      </ul>
      <h4>工具</h4>
      <ul class="audit-list">${(o?.mcpSummary || []).map((m) => `<li>${m}</li>`).join("") || "<li class='meta'>无挂载 MCP</li>"}</ul>
      <p class="meta">密钥与 Token 留在宿主机 Gate；沙箱仅见 Sentinel / Gateway。设置 · 网络安全与此处共用记住表；任一处撤销会同步。</p>
    `
    }
  `;

  card.querySelector("#perm-overview-close")?.addEventListener("click", () => {
    hidePermissionOverviewModal(root);
  });
  card.querySelectorAll("[data-rev]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const r = await revokeRememberedHostUnified(api, {
        host: btn.getAttribute("data-rev"),
        workspace: btn.getAttribute("data-ws") || projectWs,
      });
      if (r?.error) {
        showToast(`${r.error.code}: ${r.error.message}`, "bad");
        return;
      }
      showToast("已撤销", "ok");
      opts.onChanged?.();
      await mountPermissionOverviewModal(root, opts);
    });
  });
}

export function hidePermissionOverviewModal(root) {
  if (escHandler) {
    window.removeEventListener("keydown", escHandler);
    escHandler = null;
  }
  if (!root) return;
  root.classList.add("hidden");
  root.innerHTML = "";
}

function bindEsc(root) {
  if (escHandler) {
    window.removeEventListener("keydown", escHandler);
    escHandler = null;
  }
  escHandler = (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      hidePermissionOverviewModal(root);
    }
  };
  window.addEventListener("keydown", escHandler);
}
