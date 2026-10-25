/**
 * PAGE-DESIGN §5.1 首次引导
 */
export function mountOnboarding(root, { onEnter, health }) {
  const ok = !!health?.available;
  root.innerHTML = `
    <div class="onboarding-card">
      <h1>Agent Harbor</h1>
      <ol class="onboarding-points">
        <li>Agent 在隔离环境中运行</li>
        <li>API Key / 密码只存在你这台电脑上</li>
        <li>工具在本机配置一次，沙箱里不用填密</li>
      </ol>
      <p class="meta" id="onboard-health">
        ${
          ok
            ? `<span class="ok">● 隔离能力可用</span>`
            : `<span class="bad">○ 无法安全启动 · ${health?.reason || "隔离不可用"}</span>`
        }
      </p>
      <div class="actions">
        <button type="button" class="btn primary" id="btn-enter-home" ${ok ? "" : "disabled"}>
          进入 HOME
        </button>
      </div>
      ${ok ? "" : `<p class="meta bad">检测失败时不能跳过。请修复隔离/Agent 后重开应用。</p>`}
    </div>
  `;
  root.querySelector("#btn-enter-home").onclick = () => {
    if (!ok) return;
    onEnter();
  };
}
