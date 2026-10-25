/**
 * PAGE-DESIGN §5.2 HOME — 会话中心 running x/2
 */
export function mountHome(root, ctx) {
  const list = ctx.state.sessionHistory || [];
  const running = list.filter((s) =>
    ["provisioning", "running", "stopping"].includes(s.state),
  ).length;
  const healthOk = ctx.state.health?.available !== false;
  const atLimit = running >= 2;

  root.innerHTML = `
    <div class="page-head row" style="justify-content:space-between">
      <div>
        <h2>我的会话</h2>
        <p class="sub">运行中 ${running}/2${atLimit ? " · 已达上限" : ""}</p>
      </div>
      <button type="button" class="btn primary" id="btn-new-session" ${healthOk ? "" : "disabled"}>
        + 新会话
      </button>
    </div>
    ${
      !healthOk
        ? `<p class="meta bad">隔离不可用，已禁用新会话。</p>`
        : ""
    }
    ${
      atLimit
        ? `<p class="meta warn">已有 2 个会话在运行。可进向导，开始时会提示先停止其中一个；同工作区不能双开。</p>`
        : ""
    }
    <div id="home-list" class="session-list"></div>
    ${list.length === 0 ? `<p class="meta">暂无会话。先在「模型」「工具」添加账号，再点「新会话」。</p>` : ""}
  `;

  const listEl = root.querySelector("#home-list");
  list.forEach((s) => {
    const terminal = ["stopped", "failed"].includes(s.state) && s.cleanupComplete;
    const card = document.createElement("article");
    card.className = "card session-card";
    card.innerHTML = `
      <div class="row" style="justify-content:space-between">
        <div>
          <strong>${s.agentId || "—"}</strong>
          <span class="status-pill ${s.state || ""}">${s.state || "unknown"}</span>
          <p class="meta">${s.workspace || ""} · 挂载:${s.mountMode || "direct"} · ${s.sessionId || ""}</p>
          <p class="meta">工具: ${(s.mcpIds || []).length ? (s.mcpIds || []).join(", ") : "无"}</p>
        </div>
        <div class="row">
          ${
            terminal
              ? `<button type="button" class="btn" data-again="${s.sessionId}">再次开始</button>
                 <button type="button" class="btn danger" data-destroy="${s.sessionId}">销毁</button>`
              : `<button type="button" class="btn" data-open="${s.sessionId}">打开工作台</button>
                 <button type="button" class="btn danger" data-stop="${s.sessionId}">停止</button>`
          }
        </div>
      </div>`;
    listEl.appendChild(card);
  });

  root.querySelector("#btn-new-session").onclick = () => ctx.navigate("wizard");
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
      if (!confirm("销毁隔离环境里的运行数据？本机保存的密钥会保留。")) return;
      btn.disabled = true;
      await ctx.destroySession?.(btn.getAttribute("data-destroy"));
      ctx.refreshHome?.();
    };
  });
  listEl.querySelectorAll("[data-again]").forEach((btn) => {
    btn.onclick = () => ctx.navigate("wizard");
  });
}
