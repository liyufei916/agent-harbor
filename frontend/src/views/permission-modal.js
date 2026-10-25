/**
 * PAGE-DESIGN §5.9 扩权模态
 */
export function mountPermissionModal(root, { sessionLabel, target, action, onDecide }) {
  root.classList.remove("hidden");
  root.innerHTML = `
    <div class="modal-card">
      <h2>请求访问新地址</h2>
      <p class="meta">会话「${sessionLabel || "—"}」</p>
      <p class="sub">${action || "请求访问"}</p>
      <p><strong>${target || ""}</strong></p>
      <p class="meta">选错可在权限一览或设置里撤销始终允许。无「全部网站都允许」。</p>
      <div class="actions">
        <button type="button" class="btn danger" data-d="deny">拒绝</button>
        <button type="button" class="btn" data-d="once">仅本次</button>
        <button type="button" class="btn primary" data-d="project">本项目始终允许</button>
      </div>
    </div>`;
  root.querySelectorAll("[data-d]").forEach((btn) => {
    btn.onclick = () => onDecide(btn.getAttribute("data-d"));
  });
}

export function hidePermissionModal(root) {
  if (!root) return;
  root.classList.add("hidden");
  root.innerHTML = "";
}
