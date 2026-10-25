/**
 * PAGE-DESIGN：工具双轨 — OAuth/SSO 或 API Key/Token
 * OAuth：pending 轮询 ListAccounts + 可取消 / 「我已完成登录」
 */
import * as api from "../api.js";

export function mountTools(root, ctx) {
  let mode = "token"; // oauth | token
  let showForm = false;
  let pendingId = "";
  let pendingTimer = null;

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
          <p class="sub">给 Agent 一双手（MCP）。认证二选一：浏览器登录 / SSO，或 API Key。</p>
        </div>
        <button type="button" class="btn primary" id="btn-add-mcp">+ 添加</button>
      </div>
      <div id="mcp-list" class="session-list"></div>
      ${list.length === 0 && !showForm ? `<p class="meta">列表为空。添加后只展示已配置的 MCP。</p>` : ""}
      <div id="mcp-form" class="${showForm ? "" : "hidden"}"></div>
    `;
    const listEl = root.querySelector("#mcp-list");
    list.forEach((a) => {
      const row = document.createElement("article");
      row.className = "card session-card";
      row.innerHTML = `
        <div class="row" style="justify-content:space-between">
          <div>
            <strong>${a.name}</strong>
            <span class="meta">${a.authMode || "token"} · ${a.host || a.profileId} · ● 已连接</span>
          </div>
          <div class="row">
            <button type="button" class="btn danger" data-del="${a.id}">删除</button>
          </div>
        </div>`;
      listEl.appendChild(row);
    });
    root.querySelector("#btn-add-mcp").onclick = () => {
      showForm = true;
      paint();
      paintForm();
    };
    listEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = async () => {
        if (!confirm("删除该 MCP 及其本机凭据？")) return;
        await api.removeAccount(btn.getAttribute("data-del"));
        await load();
      };
    });
    if (showForm) paintForm();
  }

  function startPendingWatch() {
    clearPendingWatch();
    pendingTimer = setInterval(async () => {
      if (!pendingId) {
        clearPendingWatch();
        return;
      }
      const res = await api.listAccounts("mcp");
      const found = (res.accounts || []).some((a) => a.authMode === "oauth" || a.id === "mock-mcp");
      if (found && (res.accounts || []).length) {
        clearPendingWatch();
        pendingId = "";
        showForm = false;
        ctx.onAccountsChanged?.();
        await load();
      }
    }, 500);
  }

  function paintForm() {
    const form = root.querySelector("#mcp-form");
    form.classList.remove("hidden");
    form.innerHTML = `
      <section class="card">
        <h3>添加 MCP</h3>
        <label class="field">显示名<input id="t-name" value="演示 MCP" maxlength="100"/></label>
        <label class="field">服务地址（可选）<input id="t-origin" placeholder="https://mcp.example.com"/></label>
        <p class="meta">认证方式</p>
        <label class="radio"><input type="radio" name="auth" value="oauth" ${mode === "oauth" ? "checked" : ""}/> 浏览器登录 / SSO</label>
        <label class="radio"><input type="radio" name="auth" value="token" ${mode === "token" ? "checked" : ""}/> API Key / Token</label>
        <div id="t-token-box" class="${mode === "token" ? "" : "hidden"}">
          <label class="field">Token<input id="t-key" type="password" autocomplete="off"/></label>
          <label class="field">profileId<input id="t-profile" value="demo-mcp"/></label>
        </div>
        <div id="t-oauth-box" class="${mode === "oauth" ? "" : "hidden"}">
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
      showForm = false;
      paint();
    };
    const save = form.querySelector("#t-save");
    if (save) {
      save.onclick = async () => {
        const name = form.querySelector("#t-name").value.trim();
        const key = form.querySelector("#t-key").value;
        const profileId = form.querySelector("#t-profile").value.trim() || "demo-mcp";
        const origin = form.querySelector("#t-origin").value.trim();
        const status = form.querySelector("#t-status");
        if (!name || !key) {
          status.innerHTML = `<span class="bad">请填写显示名与 Token</span>`;
          return;
        }
        const res = await api.connectMcp({ profileId, name, apiKey: key, origin });
        form.querySelector("#t-key").value = "";
        if (res.error) {
          status.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
          return;
        }
        showForm = false;
        ctx.onAccountsChanged?.();
        await load();
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
        st.innerHTML = `<span class="ok">已请求授权：${res.authUrl || ""}</span>`;
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
          showForm = false;
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
