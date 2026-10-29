/**
 * PAGE-DESIGN §5.1 首次引导 + 隔离逐条检测
 */
import {
  freshCheckItems,
  paintCheckList,
  runIsolationProbe,
} from "../isolation-checks.js";

export function mountOnboarding(root, { onEnter, getHealth, onHealth }) {
  root.innerHTML = `
    <div class="onboarding-card onboarding-card-wide">
      <h1>Agent Harbor</h1>
      <p class="sub">密钥留在本机 · Agent 隔离运行</p>
      <ol class="onboarding-points">
        <li>Agent 在隔离环境中运行</li>
        <li>API Key / 密码只存在你这台电脑上</li>
        <li>工具在本机配置一次，沙箱里不用填密</li>
      </ol>
      <p class="meta" style="margin-bottom:8px">隔离能力检测</p>
      <ul class="block-checks" id="onboard-checks"></ul>
      <p class="meta" id="onboard-health" role="status">正在检测…</p>
      <div class="actions">
        <button type="button" class="btn primary" id="btn-enter-home" disabled>
          进入 HOME
        </button>
      </div>
      <p class="meta bad hidden" id="onboard-fail-note">检测失败时不能跳过。请修复环境后点「重新检测」，或重开应用。</p>
      <div class="actions hidden" id="onboard-retry-wrap">
        <button type="button" class="btn" id="btn-onboard-recheck">重新检测</button>
      </div>
    </div>
  `;

  const listEl = root.querySelector("#onboard-checks");
  const statusEl = root.querySelector("#onboard-health");
  const enterBtn = root.querySelector("#btn-enter-home");
  const failNote = root.querySelector("#onboard-fail-note");
  const retryWrap = root.querySelector("#onboard-retry-wrap");
  const retryBtn = root.querySelector("#btn-onboard-recheck");

  paintCheckList(listEl, freshCheckItems());

  async function run() {
    enterBtn.disabled = true;
    failNote.classList.add("hidden");
    retryWrap.classList.add("hidden");
    statusEl.innerHTML = `<span class="meta">正在逐项检测…</span>`;
    retryBtn.disabled = true;

    const health = await runIsolationProbe({
      getHealth:
        getHealth ||
        (async () => ({ available: false, reason: "ISOLATION_UNAVAILABLE" })),
      onUpdate: (items) => paintCheckList(listEl, items),
    });
    onHealth?.(health);

    if (health?.available) {
      statusEl.innerHTML = `<span class="ok">● 隔离能力可用${
        health.isolationLevel ? ` · ${health.isolationLevel}` : ""
      }</span>`;
      enterBtn.disabled = false;
    } else {
      statusEl.innerHTML = `<span class="bad">○ 无法安全启动 · ${
        health?.reason || "隔离不可用"
      }</span>`;
      failNote.classList.remove("hidden");
      retryWrap.classList.remove("hidden");
    }
    retryBtn.disabled = false;
    return health;
  }

  enterBtn.onclick = () => {
    if (enterBtn.disabled) return;
    onEnter?.();
  };
  retryBtn.onclick = () => run();
  run();
}
