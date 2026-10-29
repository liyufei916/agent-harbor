/**
 * PAGE-DESIGN：工具双轨 — OAuth/SSO 或 API Key/Token
 * 列表支持编辑/删除（编辑=删旧账号后重连，与模型页同构）。
 * OAuth：pending 轮询 ListAccounts + 可取消 / 「我已完成登录」
 */
import * as api from "../api.js";

export function mountTools(root, ctx, opts = {}) {
  let mode = "token"; // oauth | token
  const returning = ctx.state.returnAfterAccount?.kind === "mcp";
  let editing = opts.intent === "add" || returning ? "new" : null;
  let pendingDelete = "";
  let listStatus = "";
  let pendingId = "";
  let pendingTimer = null;

  function clearReturn() {
    if (ctx.state.returnAfterAccount?.kind === "mcp") {
      ctx.state.returnAfterAccount = null;
    }
  }

  function resumeWizard() {
    const ret = ctx.state.returnAfterAccount;
    if (!ret || ret.kind !== "mcp" || ret.route !== "wizard") return false;
    const prev = ctx.state.wizardResume || { step: 3, draft: {} };
    ctx.state.wizardResume = { step: 3, draft: { ...(prev.draft || {}) } };
    ctx.state.returnAfterAccount = null;
    ctx.navigate("wizard");
    return true;
  }

  function clearPendingWatch() {
    if (pendingTimer) {
      clearInterval(pendingTimer);
      pendingTimer = null;
    }
  }

  async function load() {
    const res = await api.listAccounts("mcp");
    ctx.state.mcpAccounts = res.accounts || [];
    if (ctx.state.mcpAccounts[0]) ctx.state.mcpAccount = ctx.state.mcpAccounts[0];
    paint();
  }

  function paint() {
    const list = ctx.state.mcpAccounts || [];
    root.innerHTML = `
      <div class="page-head row" style="justify-content:space-between">
        <div>
          <h2>工具</h2>
          <p class="sub">给 Agent 一双手（MCP）。可跨 Claude/Codex/dsh 挂载。模型可在「模型」页勾选多 Agent 共用。</p>
        </div>
        <button type="button" class="btn primary" id="btn-add-mcp">+ 添加</button>
      </div>
      ${
        returning
          ? `<p class="meta ok">添加成功后将回到「新会话」工具步骤。
             <button type="button" class="btn" id="t-back-wizard">直接返回</button></p>`
          : ""
      }
      <div id="mcp-list" class="session-list"></div>
      ${listStatus ? `<p class="meta" id="mcp-list-status">${listStatus}</p>` : ""}
      ${
        list.length === 0 && editing !== "new"
          ? `<div class="empty-state"><p>还没有工具。用浏览器登录 / SSO，或 API Key 添加 MCP。</p>
             <div class="actions"><button type="button" class="btn primary" id="empty-add-mcp">+ 添加第一个</button></div></div>`
          : ""
      }
      <div id="mcp-form" class="${editing ? "" : "hidden"}"></div>
    `;
    const listEl = root.querySelector("#mcp-list");
    list.forEach((a) => {
      const row = document.createElement("article");
      row.className = "card session-card";
      const confirming = pendingDelete === a.id;
      const toolsHint =
        Array.isArray(a.tools) && a.tools.length
          ? ` · ${a.tools.length} 个工具`
          : a.profileId === "custom"
            ? " · 无工具列表（请编辑重连）"
            : "";
      row.innerHTML = `
        <div class="row" style="justify-content:space-between">
          <div>
            <strong>${a.name}</strong>
            <span class="meta">${a.authMode || "token"} · ${a.host || a.profileId}${toolsHint} · ● 已连接</span>
          </div>
          <div class="row">
            <button type="button" class="btn" data-edit="${a.id}">编辑</button>
            ${
              confirming
                ? `<button type="button" class="btn danger" data-del-confirm="${a.id}">确认删除</button>
                   <button type="button" class="btn" data-del-cancel="${a.id}">取消</button>`
                : `<button type="button" class="btn danger" data-del="${a.id}">删除</button>`
            }
          </div>
        </div>`;
      listEl.appendChild(row);
    });
    const startAdd = () => {
      editing = "new";
      pendingDelete = "";
      listStatus = "";
      mode = "token";
      paint();
      paintForm();
    };
    root.querySelector("#btn-add-mcp").onclick = startAdd;
    root.querySelector("#empty-add-mcp")?.addEventListener("click", startAdd);
    root.querySelector("#t-back-wizard")?.addEventListener("click", () => {
      clearReturn();
      ctx.navigate("wizard");
    });
    listEl.querySelectorAll("[data-edit]").forEach((btn) => {
      btn.onclick = () => {
        editing = btn.getAttribute("data-edit");
        pendingDelete = "";
        listStatus = "";
        mode = "token";
        paint();
        paintForm();
      };
    });
    listEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = () => {
        pendingDelete = btn.getAttribute("data-del") || "";
        listStatus = "再点一次「确认删除」将移除本机 Token（桌面内不用系统确认框）。";
        paint();
      };
    });
    listEl.querySelectorAll("[data-del-cancel]").forEach((btn) => {
      btn.onclick = () => {
        pendingDelete = "";
        listStatus = "";
        paint();
      };
    });
    listEl.querySelectorAll("[data-del-confirm]").forEach((btn) => {
      btn.onclick = async () => {
        const id = btn.getAttribute("data-del-confirm");
        pendingDelete = "";
        listStatus = `<span class="meta">正在删除…</span>`;
        paint();
        const res = await api.removeAccount(id);
        if (res?.error) {
          listStatus = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
          paint();
          return;
        }
        listStatus = "";
        if (editing === id) editing = null;
        await load();
      };
    });
    if (editing) paintForm();
  }

  function startPendingWatch() {
    clearPendingWatch();
    const baseline = (ctx.state.mcpAccounts || []).length;
    pendingTimer = setInterval(async () => {
      if (!pendingId) {
        clearPendingWatch();
        return;
      }
      const res = await api.listAccounts("mcp");
      const now = res.accounts || [];
      const found =
        now.length > baseline || now.some((a) => a.authMode === "oauth");
      if (found && now.length) {
        clearPendingWatch();
        pendingId = "";
        editing = null;
        ctx.onAccountsChanged?.();
        if (resumeWizard()) return;
        await load();
      }
    }, 500);
  }

  function paintForm() {
    const form = root.querySelector("#mcp-form");
    const acc =
      editing === "new"
        ? { name: "演示 MCP", host: "", profileId: "custom", basePath: "" }
        : (ctx.state.mcpAccounts || []).find((a) => a.id === editing) || {};
    const originValue =
      acc.origin ||
      (acc.host
        ? "https://" + acc.host + (acc.basePath || "")
        : "");
    const isEdit = editing !== "new";
    form.classList.remove("hidden");
    form.innerHTML = `
      <section class="card">
        <h3>${isEdit ? "编辑 MCP" : "添加 MCP"}</h3>
        <label class="field">显示名<input id="t-name" value="${acc.name || ""}" maxlength="100"/></label>
        <label class="field">服务地址<input id="t-origin" value="${originValue}" placeholder="https://mcp.example.com 或 https://host/v1/mcp"/></label>
        <p class="meta">填公网 HTTPS 地址即按自定义 MCP 探测；仅用本机已登记 demo-mcp 时可留空地址。${
          isEdit ? " 保存将删除旧账号并重新连接（须重贴 Token）。" : ""
        }</p>
        <p class="meta">认证方式</p>
        <label class="radio"><input type="radio" name="auth" value="oauth" ${mode === "oauth" ? "checked" : ""} ${isEdit ? "disabled" : ""}/> 浏览器登录 / SSO${isEdit ? "（编辑请用 Token 重连）" : ""}</label>
        <label class="radio"><input type="radio" name="auth" value="token" ${mode === "token" ? "checked" : ""}/> API Key / Token</label>
        <div id="t-token-box" class="${mode === "token" ? "" : "hidden"}">
          <label class="field">Token<input id="t-key" type="password" autocomplete="off" placeholder="${isEdit ? "重贴新 Token（不回显旧值）" : ""}"/></label>
          <p class="meta" id="t-key-hint">${
            isEdit && acc.keyHint
              ? `已保存 · 末四位 ${acc.keyHint}`
              : isEdit
                ? "已保存 · Token 在本机（重存一次后可显示末四位）"
                : ""
          }</p>
          <label class="field">profileId（一般勿改）<input id="t-profile" value="${acc.profileId || "custom"}" placeholder="custom 或 demo-mcp"/></label>
        </div>
        <div id="t-oauth-box" class="${mode === "oauth" && !isEdit ? "" : "hidden"}">
          <p class="meta">将打开系统浏览器完成登录；成功后账号出现在列表（mock 约 1 秒）。</p>
          ${
            pendingId
              ? `<p class="meta ok">等待授权完成… pending <code>${pendingId}</code></p>
                 <div class="actions">
                   <button type="button" class="btn primary" id="t-oauth-done">我已完成登录</button>
                   <button type="button" class="btn" id="t-oauth-cancel">取消</button>
                 </div>`
              : `<button type="button" class="btn primary" id="t-oauth">打开浏览器登录</button>`
          }
          <span class="meta" id="t-oauth-status"></span>
        </div>
        <div class="actions">
          <button type="button" class="btn" id="t-cancel">取消</button>
          <button type="button" class="btn primary ${mode === "token" ? "" : "hidden"}" id="t-save">保存</button>
          <span class="meta" id="t-status"></span>
        </div>
      </section>`;
    form.querySelectorAll('input[name="auth"]').forEach((el) => {
      el.onchange = () => {
        if (el.disabled) return;
        mode = el.value;
        paintForm();
      };
    });
    form.querySelector("#t-cancel").onclick = async () => {
      if (pendingId) {
        await api.cancelMcpOAuth(pendingId);
        clearPendingWatch();
        pendingId = "";
      }
      editing = null;
      if (returning) {
        clearReturn();
        ctx.navigate("wizard");
        return;
      }
      paint();
    };
    const save = form.querySelector("#t-save");
    if (save) {
      save.onclick = async () => {
        const name = form.querySelector("#t-name").value.trim();
        const key = form.querySelector("#t-key").value;
        const origin = form.querySelector("#t-origin").value.trim();
        let profileId = form.querySelector("#t-profile").value.trim();
        if (origin) {
          profileId = profileId || "custom";
        } else {
          profileId = profileId || "demo-mcp";
        }
        const status = form.querySelector("#t-status");
        if (!name || !key) {
          status.innerHTML = `<span class="bad">请填写显示名与 Token</span>`;
          return;
        }
        if (!origin && profileId === "custom") {
          status.innerHTML = `<span class="bad">自定义 MCP 请填写服务地址</span>`;
          return;
        }
        status.textContent = "正在探测上游…";
        save.disabled = true;
        try {
          if (editing !== "new") {
            const rm = await api.removeAccount(editing);
            if (rm?.error) {
              status.innerHTML = `<span class="bad">更新前删除旧账号失败：${rm.error.code}: ${rm.error.message}</span>`;
              return;
            }
          }
          const res = await api.connectMcp({ profileId, name, apiKey: key, origin });
          if (res.error) {
            let extra = "";
            if (res.error.code === "SESSION_BUSY") {
              extra =
                `<p class="meta">另有连接探测或会话正在创建中，稍后再试。</p>`;
            } else if (res.error.code === "INVALID_ARGUMENT") {
              extra = `<p class="meta">检查：Token 非空；服务地址为 https://主机[/路径]。</p>`;
            } else if (res.error.code === "UPSTREAM_UNAVAILABLE" || res.error.code === "UPSTREAM_AUTH_FAILED") {
              extra = `<p class="meta">须填 <strong>MCP</strong> 端点（多为 …/mcp 或厂商给的 streamable URL），不要填模型 API 的 /v1。探测会 POST initialize + tools/list。</p>`;
            }
            status.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>${extra}`;
            return;
          }
          form.querySelector("#t-key").value = "";
          editing = null;
          ctx.onAccountsChanged?.();
          if (resumeWizard()) return;
          await load();
        } finally {
          save.disabled = false;
        }
      };
    }
    const oauth = form.querySelector("#t-oauth");
    if (oauth) {
      oauth.onclick = async () => {
        const name = form.querySelector("#t-name").value.trim() || "OAuth MCP";
        const origin = form.querySelector("#t-origin").value.trim();
        const st = form.querySelector("#t-oauth-status");
        const res = await api.startMcpOAuth({ name, profileId: "demo-mcp", origin });
        if (res.error) {
          st.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
          return;
        }
        pendingId = res.pendingId || "";
        st.innerHTML = `<span class="ok">已打开系统浏览器授权（可轮询或点「我已完成」）。回调地址已就绪。</span>`;
        startPendingWatch();
        paintForm();
      };
    }
    const done = form.querySelector("#t-oauth-done");
    if (done) {
      done.onclick = async () => {
        await load();
        const has = (ctx.state.mcpAccounts || []).length > 0;
        if (has) {
          clearPendingWatch();
          pendingId = "";
          editing = null;
          ctx.onAccountsChanged?.();
          if (resumeWizard()) return;
          paint();
        } else {
          const st = form.querySelector("#t-oauth-status");
          if (st) st.innerHTML = `<span class="meta">尚未出现账号，请稍候再点或继续等待轮询</span>`;
        }
      };
    }
    const cancelO = form.querySelector("#t-oauth-cancel");
    if (cancelO) {
      cancelO.onclick = async () => {
        await api.cancelMcpOAuth(pendingId);
        clearPendingWatch();
        pendingId = "";
        paintForm();
      };
    }
  }

  load();
  return {
    destroy() {
      clearPendingWatch();
    },
  };
}
