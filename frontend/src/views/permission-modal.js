/**
 * PAGE-DESIGN §5.9 扩权模态 — 白话三问 + 可选超时默认拒绝
 */
import { showToast } from "../toast.js";

let escHandler = null;
let timeoutTimer = null;
let countdownTimer = null;

const DEFAULT_TIMEOUT_SEC = 90;

function escapeHtml(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/"/g, "&quot;");
}

/**
 * @param {HTMLElement | null} root
 * @param {{
 *   sessionLabel?: string,
 *   target?: string,
 *   action?: string,
 *   onDecide: (d: string) => Promise<void>,
 *   timeoutSec?: number,
 *   onPendingChange?: (pending: boolean) => void,
 * }} opts
 */
export function mountPermissionModal(root, opts) {
  const {
    sessionLabel,
    target,
    action,
    onDecide,
    timeoutSec = DEFAULT_TIMEOUT_SEC,
    onPendingChange,
  } = opts || {};
  if (!root) return;
  hidePermissionModal(root);
  clearTimers();
  onPendingChange?.(true);

  const host = String(target || "").trim() || "（未知主机）";
  const act = String(action || "connect").trim() || "connect";
  const sec = Math.max(15, Math.min(Number(timeoutSec) || DEFAULT_TIMEOUT_SEC, 600));

  root.classList.remove("hidden");
  root.innerHTML = `
    <div class="modal-card permission-decide-card" role="dialog" aria-modal="true" aria-labelledby="perm-title">
      <h2 id="perm-title">请求访问新地址</h2>
      <p class="meta">会话「${escapeHtml(sessionLabel || "—")}」</p>
      <p class="target-host">${escapeHtml(host)}</p>

      <ol class="perm-three-q">
        <li><strong>发生了什么</strong>：Agent 试图 <code>${escapeHtml(act)}</code> 访问尚未在白名单中的主机 <strong>${escapeHtml(host)}</strong>。请求已暂扣，未带你的真实 Key（沙箱只有 Sentinel）。</li>
        <li><strong>风险有多大</strong>：若允许，Agent 可向该主机发送副本内数据（不含钥匙串里的 Key）。恶意项目可能借此外传代码或探测结果。误点「本项目始终允许」可在权限一览撤销。</li>
        <li><strong>你可以怎么做</strong>：<em>拒绝</em>（推荐若陌生）；<em>仅本次</em>（本请求/本跳）；<em>本项目始终允许</em>（写入记住表，可撤销）。没有「全网允许」。</li>
      </ol>

      <p class="meta" id="perm-countdown">若 ${sec} 秒内未选择，将自动<strong>拒绝</strong>。</p>
      <div class="actions">
        <button type="button" class="btn danger" data-d="deny">拒绝</button>
        <button type="button" class="btn" data-d="once">仅本次</button>
        <button type="button" class="btn primary" data-d="project">本项目始终允许</button>
      </div>
    </div>`;

  const labels = {
    deny: "已拒绝",
    once: "仅本次允许",
    project: "已为本项目记住允许",
  };

  let finished = false;
  const finish = async (decision, fromTimeout = false) => {
    if (finished) return;
    finished = true;
    clearTimers();
    root.querySelectorAll("[data-d]").forEach((b) => {
      b.disabled = true;
    });
    try {
      await onDecide(decision);
      showToast(
        fromTimeout ? "超时未选择，已自动拒绝" : labels[decision] || "已处理",
        fromTimeout ? "bad" : "ok",
      );
      onPendingChange?.(false);
    } catch (e) {
      finished = false;
      const detail =
        e && typeof e.message === "string" && e.message
          ? e.message.slice(0, 120)
          : "";
      showToast(detail ? `处理失败：${detail}` : "处理失败，请重试", "bad");
      root.querySelectorAll("[data-d]").forEach((b) => {
        b.disabled = false;
      });
      armTimeout(sec, () => finish("deny", true));
    }
  };

  root.querySelectorAll("[data-d]").forEach((btn) => {
    btn.onclick = () => finish(btn.getAttribute("data-d") || "deny", false);
  });

  escHandler = (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
    }
  };
  window.addEventListener("keydown", escHandler);
  armTimeout(sec, () => finish("deny", true));
}

function armTimeout(sec, onFire) {
  clearTimers();
  let left = sec;
  const el = typeof document !== "undefined" ? document.getElementById("perm-countdown") : null;
  countdownTimer = setInterval(() => {
    left -= 1;
    if (el) {
      el.innerHTML =
        left > 0
          ? `若 ${left} 秒内未选择，将自动<strong>拒绝</strong>。`
          : `正在自动拒绝…`;
    }
    if (left <= 0 && countdownTimer) {
      clearInterval(countdownTimer);
      countdownTimer = null;
    }
  }, 1000);
  timeoutTimer = setTimeout(() => onFire(), sec * 1000);
}

function clearTimers() {
  if (timeoutTimer) {
    clearTimeout(timeoutTimer);
    timeoutTimer = null;
  }
  if (countdownTimer) {
    clearInterval(countdownTimer);
    countdownTimer = null;
  }
}

export function hidePermissionModal(root) {
  clearTimers();
  if (escHandler) {
    window.removeEventListener("keydown", escHandler);
    escHandler = null;
  }
  if (!root) return;
  root.classList.add("hidden");
  root.innerHTML = "";
}
