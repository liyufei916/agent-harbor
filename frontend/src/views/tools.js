/**
 * PAGE-DESIGN：工具双轨 — OAuth/SSO 或 API Key/Token
 * 列表支持编辑/删除（编辑=删旧账号后重连，与模型页同构）。
 * OAuth：pending 轮询 ListAccounts + 可取消 / 「我已完成登录」
 */
import * as api from "../api.js";

export function mountTools(root, ctx, opts = {}) {
  let mode = "token"; // oauth | token | stdio
  const returning = ctx.state.returnAfterAccount?.kind === "mcp";
  let editing = opts.intent === "add" || returning ? "new" : null;
  let pendingDelete = "";
  let listStatus = "";
  let pendingId = "";
  let pendingTimer = null;
  /** @type {Set<string>} */
  let oauthSeenIds = new Set();
  let oauthHint = "";

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
          <p class="meta warn">MCP：<strong>远程 HTTP</strong>（Token / OAuth）或<strong>宿主 stdio</strong>（本机进程，经回环桥；密钥不进沙箱）。须预装 Agent CLI；隔离仅 <strong>macOS Darwin 25 · arm64 · 非 root</strong>。</p>
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
          ? `<div class="empty-state"><p>还没有工具。可添加远程 HTTP MCP（Token/OAuth），或宿主 stdio（本机命令，Harbor 在宿主机拉起）。</p>
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
            <span class="meta">${a.transport === "stdio" ? "stdio" : a.authMode || "token"} · ${a.transport === "stdio" ? a.command || "host" : a.host || a.profileId}${toolsHint} · ${a.offlineHint ? "○ 桥未就绪" : "● 已连接"}</span>
            ${
              a.offlineHint
                ? `<p class="meta warn">${a.offlineHint}</p>`
                : a.transport === "stdio"
                  ? `<p class="meta">宿主桥：重启后自动重拉（端口可能变化）。密钥与进程留在宿主机。</p>`
                  : ""
            }
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

  function newOAuthAccounts(accounts) {
    const now = accounts || [];
    return now.filter((a) => a?.id && !oauthSeenIds.has(a.id));
  }

  async function finishOAuthSuccess(accounts) {
    const added = newOAuthAccounts(accounts);
    clearPendingWatch();
    pendingId = "";
    oauthHint = "";
    editing = null;
    ctx.state.mcpAccounts = accounts || [];
    if (added[0]) ctx.state.mcpAccount = added[0];
    else if (ctx.state.mcpAccounts[0]) ctx.state.mcpAccount = ctx.state.mcpAccounts[0];
    ctx.onAccountsChanged?.();
    if (resumeWizard()) return;
    await load();
  }

  function startPendingWatch() {
    clearPendingWatch();
    pendingTimer = setInterval(async () => {
      if (!pendingId) {
        clearPendingWatch();
        return;
      }
      const res = await api.listAccounts("mcp");
      if (newOAuthAccounts(res.accounts).length) {
        await finishOAuthSuccess(res.accounts);
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
        <p class="meta">接入方式</p>
        <label class="radio"><input type="radio" name="auth" value="oauth" ${mode === "oauth" ? "checked" : ""} ${isEdit ? "disabled" : ""}/> 远程 · 浏览器登录 / SSO${isEdit ? "（编辑请用 Token/stdio 重连）" : ""}</label>
        <label class="radio"><input type="radio" name="auth" value="token" ${mode === "token" ? "checked" : ""}/> 远程 · API Key / Token</label>
        <label class="radio"><input type="radio" name="auth" value="stdio" ${mode === "stdio" ? "checked" : ""}/> 宿主 · 本地 stdio 进程</label>
        <div id="t-http-origin" class="${mode === "stdio" ? "hidden" : ""}">
          <label class="field">服务地址<input id="t-origin" value="${originValue}" placeholder="https://mcp.example.com 或 https://host/v1/mcp"/></label>
          <p class="meta">填公网 HTTPS 地址即按自定义 MCP 探测；仅用本机已登记 demo-mcp 时可留空地址。${
            isEdit ? " 保存将删除旧账号并重新连接。" : ""
          }</p>
        </div>
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
        <div id="t-stdio-box" class="${mode === "stdio" ? "" : "hidden"}">
          <p class="meta warn">命令在<strong>宿主机</strong>运行（非沙箱）；Harbor 用回环 HTTP 桥接，Agent 仍只连 Gateway。请只添加你信任的 MCP 服务器。</p>
          <label class="field">可执行文件（绝对路径）<input id="t-command" value="${acc.command || ""}" placeholder="/opt/homebrew/bin/npx"/></label>
          <label class="field">参数（空格分隔）<input id="t-args" value="" placeholder="-y @modelcontextprotocol/server-filesystem /path"/></label>
          <label class="field">环境变量（每行 KEY=VALUE，可选）<textarea id="t-env" rows="3" placeholder="GITHUB_PERSONAL_ACCESS_TOKEN=…"></textarea></label>
        </div>
        <div id="t-oauth-box" class="${mode === "oauth" && !isEdit ? "" : "hidden"}">
          <p class="meta">须填写上方 HTTPS 服务地址。将打开系统浏览器；浏览器出现「登录完成」后，点下方按钮或等待自动刷新。</p>
          <details class="meta" style="margin:8px 0">
            <summary>IdP 自助登记（多数云厂商需要）</summary>
            <ol>
              <li>在授权服务器新建 <strong>Native / Public</strong> 客户端（或机密客户端并配置密钥）。</li>
              <li>Redirect URI 填本机回环：<code>http://127.0.0.1:&lt;端口&gt;/callback</code>。固定端口请设环境变量 <code>HARBOR_MCP_OAUTH_REDIRECT_PORT</code>（便于预先登记）。</li>
              <li>Client ID 默认尝试动态注册（DCR）；失败则用 <code>agent-harbor</code>。也可设 <code>HARBOR_MCP_OAUTH_CLIENT_ID</code> / <code>HARBOR_MCP_OAUTH_CLIENT_SECRET</code> / <code>HARBOR_MCP_OAUTH_TOKEN_PATH</code>。</li>
              <li>换票失败时优先改用本页 <strong>API Key / Token</strong> 轨，不必卡在 OAuth。</li>
            </ol>
          </details>
          ${
            pendingId
              ? `<p class="meta ok">等待授权完成… pending <code>${pendingId}</code></p>
                 <div class="actions">
                   <button type="button" class="btn primary" id="t-oauth-done">我已完成登录</button>
                   <button type="button" class="btn" id="t-oauth-cancel">取消</button>
                 </div>`
              : `<button type="button" class="btn primary" id="t-oauth">打开浏览器登录</button>`
          }
          <span class="meta" id="t-oauth-status">${oauthHint || ""}</span>
        </div>
        <div class="actions">
          <button type="button" class="btn" id="t-cancel">取消</button>
          <button type="button" class="btn primary ${mode === "oauth" ? "hidden" : ""}" id="t-save">保存</button>
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
        oauthHint = "";
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
        const status = form.querySelector("#t-status");
        if (!name) {
          status.innerHTML = `<span class="bad">请填写显示名</span>`;
          return;
        }
        status.textContent = "正在探测…";
        save.disabled = true;
        try {
          if (editing !== "new") {
            const rm = await api.removeAccount(editing);
            if (rm?.error) {
              status.innerHTML = `<span class="bad">更新前删除旧账号失败：${rm.error.code}: ${rm.error.message}</span>`;
              return;
            }
          }
          let res;
          if (mode === "stdio") {
            const command = form.querySelector("#t-command")?.value.trim() || "";
            const argsRaw = form.querySelector("#t-args")?.value.trim() || "";
            const envRaw = form.querySelector("#t-env")?.value || "";
            if (!command.startsWith("/")) {
              status.innerHTML = `<span class="bad">可执行文件须为绝对路径（如 /opt/homebrew/bin/npx）</span>`;
              return;
            }
            const args = argsRaw ? argsRaw.split(/\s+/).filter(Boolean) : [];
            const env = {};
            for (const line of envRaw.split("\n")) {
              const trimmed = line.trim();
              if (!trimmed || trimmed.startsWith("#")) continue;
              const i = trimmed.indexOf("=");
              if (i <= 0) {
                status.innerHTML = `<span class="bad">环境变量格式须为 KEY=VALUE</span>`;
                return;
              }
              env[trimmed.slice(0, i)] = trimmed.slice(i + 1);
            }
            res = await api.connectMcp({
              name,
              profileId: "custom",
              transport: "stdio",
              command,
              args,
              env,
              apiKey: "stdio",
            });
          } else {
            const key = form.querySelector("#t-key")?.value || "";
            const origin = form.querySelector("#t-origin")?.value.trim() || "";
            let profileId = form.querySelector("#t-profile")?.value.trim() || "";
            if (origin) profileId = profileId || "custom";
            else profileId = profileId || "demo-mcp";
            if (!key) {
              status.innerHTML = `<span class="bad">请填写 Token</span>`;
              return;
            }
            if (!origin && profileId === "custom") {
              status.innerHTML = `<span class="bad">自定义 MCP 请填写服务地址</span>`;
              return;
            }
            res = await api.connectMcp({ profileId, name, apiKey: key, origin });
          }
          if (res.error) {
            let extra = "";
            if (res.error.code === "SESSION_BUSY") {
              extra = `<p class="meta">另有连接探测或会话正在创建中，稍后再试。</p>`;
            } else if (res.error.code === "INVALID_ARGUMENT") {
              extra =
                mode === "stdio"
                  ? `<p class="meta">检查：绝对路径命令；参数与环境变量无控制字符。</p>`
                  : `<p class="meta">检查：Token 非空；服务地址为 https://主机[/路径]。</p>`;
            } else if (res.error.code === "UPSTREAM_UNAVAILABLE" || res.error.code === "UPSTREAM_AUTH_FAILED") {
              extra =
                mode === "stdio"
                  ? `<p class="meta">进程须讲 MCP stdio（Content-Length 分帧），并在 initialize 后返回 tools/list。</p>`
                  : `<p class="meta">须填 <strong>MCP</strong> 端点（多为 …/mcp），不要填模型 /v1。</p>`;
            }
            status.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>${extra}`;
            return;
          }
          const keyEl = form.querySelector("#t-key");
          if (keyEl) keyEl.value = "";
          const envEl = form.querySelector("#t-env");
          if (envEl) envEl.value = "";
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
        if (!origin) {
          oauthHint =
            `<span class="bad">请先填写 HTTPS 服务地址，再打开浏览器登录。空地址无法完成 OAuth 回调写号。</span>`;
          if (st) st.innerHTML = oauthHint;
          return;
        }
        oauthSeenIds = new Set(
          (ctx.state.mcpAccounts || []).map((a) => a.id).filter(Boolean),
        );
        const res = await api.startMcpOAuth({
          name,
          profileId: "custom",
          origin,
        });
        if (res.error) {
          oauthHint = `<span class="bad">${res.error.code}: ${res.error.message}</span>
            <p class="meta">若 IdP 拒绝未登记客户端：先登记 redirect / client，或改用 Token；也可设 <code>HARBOR_MCP_OAUTH_CLIENT_ID</code> 后重试。</p>`;
          if (st) st.innerHTML = oauthHint;
          return;
        }
        pendingId = res.pendingId || "";
        const srcLabel =
          res.clientSource === "dcr"
            ? "动态注册"
            : res.clientSource === "env"
              ? "环境变量"
              : "默认 client_id";
        oauthHint = `<span class="ok">已打开系统浏览器。请等到回调页显示「登录完成」，再点「我已完成登录」；也可稍候自动写入列表。</span>
          <p class="meta">回调 <code>${res.redirectUri || "http://127.0.0.1:…/callback"}</code>
          · client_id <code>${res.clientId || "agent-harbor"}</code>（${srcLabel}）</p>`;
        startPendingWatch();
        paintForm();
      };
    }
    const done = form.querySelector("#t-oauth-done");
    if (done) {
      done.onclick = async () => {
        done.disabled = true;
        done.textContent = "检查中…";
        try {
          const res = await api.listAccounts("mcp");
          if (newOAuthAccounts(res.accounts).length) {
            await finishOAuthSuccess(res.accounts);
            return;
          }
          oauthHint = `<span class="bad">仍未写入 MCP 账号。请确认：① 服务地址已填且为 HTTPS；② 浏览器已跳到本机回调并显示「登录完成」（不是仅关掉登录页）；③ IdP 已登记 redirect 为 Harbor 打开的 127.0.0.1 端口。也可取消后改用 API Key / Token。</span>`;
          paintForm();
        } finally {
          done.disabled = false;
          done.textContent = "我已完成登录";
        }
      };
    }
    const cancelO = form.querySelector("#t-oauth-cancel");
    if (cancelO) {
      cancelO.onclick = async () => {
        await api.cancelMcpOAuth(pendingId);
        clearPendingWatch();
        pendingId = "";
        oauthHint = "";
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
