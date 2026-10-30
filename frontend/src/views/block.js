/**
 * PAGE-DESIGN §5.10：隔离不可用阻断层
 * 全页说明；无「仍要继续」；与侧栏红灯一致；重新检测走逐条动画。
 * 恢复路径：主按钮进入设置「隔离后端 / 引擎」，避免卡在托管检测失败无法改本机 Docker。
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
  const dockerPreferred = preferredLevel === "docker";
  const recoveryHint = dockerPreferred
    ? "Docker 隔离未就绪时，请到设置改「本机 Docker」或退回 Seatbelt，再重启应用。不会无隔离开跑。"
    : "可到设置检查隔离后端（Seatbelt / Docker）；修好后点「重新检测」。不会无隔离开跑。";

  root.innerHTML = `
    <div class="block-card block-card-wide" role="alertdialog" aria-labelledby="block-title" aria-describedby="block-desc">
      <h1 id="block-title">无法安全启动</h1>
      <p id="block-desc" class="sub">这台电脑当前无法建立隔离环境。Agent Harbor 不会在无隔离时启动 Agent，也没有「仍要继续」。</p>
      <ul class="block-checks" id="block-checks"></ul>
      <p class="meta bad" id="block-reason">${escapeHtml(
        reason || health?.reason || "ISOLATION_UNAVAILABLE",
      )}</p>
      <p class="meta" id="block-recovery">${escapeHtml(recoveryHint)}</p>
      <div class="actions">
        <button type="button" class="btn primary" id="btn-block-settings">改隔离 / 引擎设置</button>
        <button type="button" class="btn" id="btn-block-recheck">重新检测</button>
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
  root.querySelector("#btn-block-settings").onclick = () =>
    onSettings?.({ focusIsolation: true });
}

function escapeHtml(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
