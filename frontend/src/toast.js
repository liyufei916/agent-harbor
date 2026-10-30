/**
 * 轻量 toast（产品 C · 桌面/mock 共用）
 * @param {string} message
 * @param {string} [kind]
 * @param {{ durationMs?: number, actionLabel?: string, onAction?: () => void }} [opts]
 */
export function showToast(message, kind = "", opts = {}) {
  if (typeof document === "undefined") return;
  document.querySelectorAll(".toast").forEach((el) => el.remove());
  const el = document.createElement("div");
  el.className = `toast ${kind}`.trim();
  el.setAttribute("role", "status");
  const text = document.createElement("span");
  text.className = "toast-text";
  text.textContent = message;
  el.appendChild(text);
  if (opts.actionLabel && typeof opts.onAction === "function") {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "toast-action";
    btn.textContent = opts.actionLabel;
    btn.addEventListener("click", () => {
      el.remove();
      opts.onAction();
    });
    el.appendChild(btn);
  }
  document.body.appendChild(el);
  const ms =
    opts.durationMs ??
    (kind === "bad" ? 5200 : opts.actionLabel ? 8000 : 2800);
  setTimeout(() => el.remove(), ms);
}
