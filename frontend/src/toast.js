/** 轻量 toast（产品 C · 桌面/mock 共用） */
export function showToast(message, kind = "") {
  if (typeof document === "undefined") return;
  document.querySelectorAll(".toast").forEach((el) => el.remove());
  const el = document.createElement("div");
  el.className = `toast ${kind}`.trim();
  el.setAttribute("role", "status");
  el.textContent = message;
  document.body.appendChild(el);
  const ms = kind === "bad" ? 5200 : 2800;
  setTimeout(() => el.remove(), ms);
}
