/**
 * PAGE-DESIGN：隔离不可用阻断层
 */
export function mountBlock(root, { reason, onSettings }) {
  root.innerHTML = `
    <div class="block-card">
      <h1>无法安全启动</h1>
      <p class="sub">隔离能力不可用时不能创建新会话。</p>
      <p class="meta bad">${reason || "ISOLATION_UNAVAILABLE"}</p>
      <div class="actions">
        <button type="button" class="btn primary" id="btn-block-settings">前往设置</button>
      </div>
    </div>
  `;
  root.querySelector("#btn-block-settings").onclick = () => onSettings();
}
