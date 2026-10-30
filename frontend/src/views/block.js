/**
 * PAGE-DESIGN §5.10：隔离不可用阻断层
 * 全页说明；无「仍要继续」；与侧栏红灯一致；重新检测走逐条动画。
 */
import {
  checksFromHealth,
  paintCheckList,
  runIsolationProbe,
} from "../isolation-checks.js";

export function mountBlock(
  root,
  { reason, health, getHealth, onAvailable, onSettings, preferredLevel },
) {
  const initial = checksFromHealth(
    health || { available: false, reason: reason || "ISOLATION_UNAVAILABLE" },
    preferredLevel,
  );

  root.innerHTML = `
    <div class="block-card block-card-wide" role="alertdialog" aria-labelledby="block-title" aria-describedby="block-desc">
      <h1 id="block-title">无法安全启动</h1>
      <p id="block-desc" class="sub">这台电脑当前无法建立隔离环境。Agent Harbor 不会在无隔离时启动 Agent，也没有「仍要继续」。</p>
      <ul class="block-checks" id="block-checks"></ul>
      <p class="meta bad" id="block-reason">${escapeHtml(
        reason || health?.reason || "ISOLATION_UNAVAILABLE",
      )}</p>
      <div class="actions">
        <button type="button" class="btn primary" id="btn-block-recheck">重新检测</button>
        <button type="button" class="btn" id="btn-block-settings">前往设置</button>
      </div>
    </div>
  `;

  const listEl = root.querySelector("#block-checks");
  const reasonEl = root.querySelector("#block-reason");
  paintCheckList(listEl, initial);

  const recheck = root.querySelector("#btn-block-recheck");
  recheck.onclick = async () => {
    recheck.disabled = true;
    recheck.textContent = "检测中…";
    try {
      const h = await runIsolationProbe({
        preferredLevel,
        getHealth:
          getHealth ||
          (async () => ({
            available: false,
            reason: "ISOLATION_UNAVAILABLE",
          })),
        onUpdate: (items) => paintCheckList(listEl, items),
      });
      reasonEl.textContent = h?.available
        ? ""
        : h?.reason || "ISOLATION_UNAVAILABLE";
      reasonEl.classList.toggle("bad", !h?.available);
      reasonEl.classList.toggle("ok", !!h?.available);
      if (h?.available) {
        reasonEl.textContent = "隔离能力已恢复";
        onAvailable?.(h);
      }
    } finally {
      recheck.disabled = false;
      recheck.textContent = "重新检测";
    }
  };
  root.querySelector("#btn-block-settings").onclick = () => onSettings?.();
}

function escapeHtml(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
