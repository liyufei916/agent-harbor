/**
 * PAGE-DESIGN §5.2 HOME — 会话中心 running x/N
 */
import { askConfirm } from "../confirm-dialog.js";
import { healthAllowsStart, healthBlockReason } from "../health-ready.js";
import { sessionStateLabel, shortPath } from "../session-state.js";
import { MAX_ACTIVE_SESSIONS } from "../session-limits.js";

function modelLabelFor(ctx, modelAccountId) {
  if (!modelAccountId) return "";
  const accounts = [
    ...(ctx.state.accounts || []),
    ...(ctx.state.modelAccounts || []),
  ];
  const hit = accounts.find((a) => a?.id === modelAccountId);
  if (!hit) return "";
  return hit.name || hit.host || hit.profileId || "";
}

export function mountHome(root, ctx) {
  const list = ctx.state.sessionHistory || [];
  const running = list.filter((s) =>
    ["provisioning", "running", "stopping"].includes(s.state),
  ).length;
  const healthOk = healthAllowsStart(ctx.state.health);
  const atLimit = running >= MAX_ACTIVE_SESSIONS;

  root.innerHTML = `
    <div class="page-head row page-head-spread">
      <div>
        <h2>我的会话</h2>
        <p class="sub">运行中 ${running}/${MAX_ACTIVE_SESSIONS}${atLimit ? " · 已达上限" : ""}</p>
      </div>
      <button type="button" class="btn primary" id="btn-new-session" ${healthOk ? "" : "disabled"}>
        + 新会话
      </button>
    </div>
    ${
      !healthOk
        ? `<div class="banner warn-banner">
            <strong>隔离不可用</strong>
            <p class="meta">已禁用新会话；与侧栏红灯一致，没有「仍要继续」。</p>
            <div class="actions">
              <button type="button" class="btn" id="btn-home-block">查看阻断说明</button>
              <button type="button" class="btn" id="btn-home-rehealth">重新检测</button>
            </div>
          </div>`
        : ""
    }
    ${
      atLimit
        ? `<div class="banner warn-banner">
            <strong>已达上限（${running}/${MAX_ACTIVE_SESSIONS}）</strong>
            <p class="meta">请先停止一个会话再开新的。同工作区不能双开——若提示 WORKSPACE_BUSY，请打开已有会话或另选文件夹。</p>
          </div>`
        : ""
    }
    <div id="home-list" class="session-list"></div>
    ${
      list.length === 0
        ? `<div class="empty-state">
            <p>还没有会话。先配置模型与工具，再创建隔离会话。</p>
            <p class="meta">未安装 Agent 可到「设置」复制安装命令。</p>
            <div class="actions">
              <button type="button" class="btn" id="empty-models">去模型</button>
              <button type="button" class="btn" id="empty-tools">去工具</button>
              <button type="button" class="btn" id="empty-settings">去设置</button>
              <button type="button" class="btn primary" id="empty-wizard" ${healthOk ? "" : "disabled"}>新会话</button>
            </div>
          </div>`
        : ""
    }
  `;

  const listEl = root.querySelector("#home-list");
  root.querySelector("#empty-models")?.addEventListener("click", () => ctx.navigate("models"));
  root.querySelector("#empty-tools")?.addEventListener("click", () => ctx.navigate("tools"));
  root.querySelector("#empty-settings")?.addEventListener("click", () => ctx.navigate("settings"));
  root.querySelector("#empty-wizard")?.addEventListener("click", () => {
    if (!healthAllowsStart(ctx.state.health)) {
      ctx.showBlock?.(healthBlockReason(ctx.state.health));
      return;
    }
    ctx.navigate("wizard", { fresh: true });
  });
  list.forEach((s) => {
    const terminal = ["stopped", "failed"].includes(s.state) && s.cleanupComplete;
    const card = document.createElement("article");
    card.className = "list-row session-card";
    const path =
      s.projectWorkspace || s.sourceWorkspace || s.workspace || "";
    const modelName = modelLabelFor(ctx, s.modelAccountId);
    const mount =
      s.mountMode === "clone" ? "安全副本" : s.mountMode || "直接编辑";
    card.innerHTML = `
      <div class="row page-head-spread">
        <div>
          <strong>${s.agentId || "—"}</strong>
          <span class="status-pill ${s.state || ""}">${sessionStateLabel(s.state)}</span>
          <p class="meta">${modelName ? `${modelName} · ` : ""}${shortPath(path)} · ${mount}</p>
          <p class="meta">工具: ${(s.mcpIds || []).length ? (s.mcpIds || []).join(", ") : "无"}</p>
        </div>
        <div class="row">
          ${
            terminal
              ? `<button type="button" class="btn primary" data-again="${s.sessionId}">再次开始</button>
                 <button type="button" class="btn danger" data-destroy="${s.sessionId}">销毁</button>`
              : `<button type="button" class="btn primary" data-open="${s.sessionId}">打开工作台</button>
                 <button type="button" class="btn danger" data-stop="${s.sessionId}">停止</button>`
          }
        </div>
      </div>`;
    listEl.appendChild(card);
  });

  root.querySelector("#btn-new-session").onclick = () => {
    if (!healthAllowsStart(ctx.state.health)) {
      ctx.showBlock?.(healthBlockReason(ctx.state.health));
      return;
    }
    ctx.navigate("wizard", { fresh: true });
  };
  root.querySelector("#btn-home-block")?.addEventListener("click", () => {
    ctx.showBlock?.(ctx.state.health?.reason);
  });
  root.querySelector("#btn-home-rehealth")?.addEventListener("click", async () => {
    await ctx.refreshHealth?.();
  });
  listEl.querySelectorAll("[data-open]").forEach((btn) => {
    btn.onclick = () => ctx.openSession?.(btn.getAttribute("data-open"));
  });
  listEl.querySelectorAll("[data-stop]").forEach((btn) => {
    btn.onclick = async () => {
      btn.disabled = true;
      await ctx.stopSession(btn.getAttribute("data-stop"));
      ctx.refreshHome?.();
    };
  });
  listEl.querySelectorAll("[data-destroy]").forEach((btn) => {
    btn.onclick = async () => {
      const ok = await askConfirm({
        title: "销毁会话数据",
        body: "销毁隔离环境里的运行数据？本机保存的密钥会保留。",
        confirmLabel: "销毁",
        cancelLabel: "取消",
        danger: true,
      });
      if (!ok) return;
      btn.disabled = true;
      await ctx.destroySession?.(btn.getAttribute("data-destroy"));
      ctx.refreshHome?.();
    };
  });
  listEl.querySelectorAll("[data-again]").forEach((btn) => {
    btn.onclick = async () => {
      if (!healthAllowsStart(ctx.state.health)) {
        ctx.showBlock?.(healthBlockReason(ctx.state.health));
        return;
      }
      btn.disabled = true;
      try {
        await ctx.restartSession?.(btn.getAttribute("data-again"));
      } finally {
        btn.disabled = false;
        ctx.refreshHome?.();
      }
    };
  });
}
