/**
 * 账号添加/编辑弹层：挂到 #modal-root，与扩权/确认框同层。
 * @returns {{ card: HTMLElement, close: () => void } | null}
 */
export function openFormModal({
  title = "",
  cardClass = "account-form-card",
  onClose = null,
} = {}) {
  const layer =
    typeof document !== "undefined"
      ? document.getElementById("modal-root")
      : null;
  if (!layer) return null;

  const prev = layer.innerHTML;
  const wasHidden = layer.classList.contains("hidden");
  layer.classList.remove("hidden");
  layer.innerHTML = `
    <div class="modal-card ${cardClass}" role="dialog" aria-modal="true" aria-labelledby="form-modal-title">
      <h2 id="form-modal-title">${escapeHtml(title)}</h2>
      <div id="form-modal-body"></div>
    </div>`;

  let closed = false;
  const close = () => {
    if (closed) return;
    closed = true;
    window.removeEventListener("keydown", onKey);
    if (wasHidden) layer.classList.add("hidden");
    layer.innerHTML = prev;
    onClose?.();
  };

  const onKey = (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      close();
    }
  };
  window.addEventListener("keydown", onKey);

  return {
    layer,
    card: layer.querySelector(".modal-card"),
    body: layer.querySelector("#form-modal-body"),
    setTitle(next) {
      const el = layer.querySelector("#form-modal-title");
      if (el) el.textContent = next || "";
    },
    close,
  };
}

function escapeHtml(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
