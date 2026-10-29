/**
 * PAGE-DESIGN §5.8 设置（本机默认；无关闭隔离）
 */
import * as api from "../api.js";
import { showToast } from "../toast.js";

export function mountSettings(root, ctx) {
  root.innerHTML = `
    <h2>设置</h2>
    <p class="sub">只改本机默认偏好。无「关闭隔离 / 本机直跑 Agent」。</p>

    <section class="card">
      <h3>Agent 检测</h3>
      <p class="meta" id="settings-agent">—</p>
      <button type="button" class="btn" id="btn-refresh-agents">重新检测</button>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>会话默认</h3>
      <label class="field">默认挂载
        <select id="set-mount">
          <option value="clone">安全副本（推荐）</option>
          <option value="direct">直接编辑</option>
        </select>
      </label>
      <p class="meta">用于下次新会话向导预填。本分支默认安全副本。</p>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>结束与清理</h3>
      <label class="check-row">
        <input type="checkbox" id="set-ask-clear" />
        停止会话时询问是否清除隔离环境数据
      </label>
      <fieldset id="set-clear-default-wrap" class="radio-set">
        <legend class="meta">关闭询问后的默认动作</legend>
        <label class="radio"><input type="radio" name="clear-default" value="keep" /> 停止时总是保留（可稍后在 HOME 销毁）</label>
        <label class="radio"><input type="radio" name="clear-default" value="clear" /> 停止时总是清除</label>
      </fieldset>
      <p class="meta">「销毁」在 HOME 始终二次确认，不受此开关影响。</p>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>外观与侧栏</h3>
      <label class="check-row">
        <input type="checkbox" id="set-sidebar-collapsed" />
        默认收缩侧栏
      </label>
      <label class="check-row">
        <input type="checkbox" id="set-auto-collapse" />
        进入工作台时自动收缩侧栏
      </label>
      <p class="meta">写入 Get/SetUIPrefs（mock 落 localStorage）。快捷键 ⌘B / Ctrl+B。</p>
      <span class="meta" id="set-prefs-status"></span>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>网络安全 · 已记住的网站</h3>
      <div id="set-hosts" class="meta">加载中…</div>
      <button type="button" class="btn" id="btn-reload-hosts">刷新</button>
    </section>

    <section class="card ${api.isMock() ? "" : "hidden"}" style="margin-top:12px">
      <h3>模拟场景（仅 mock）</h3>
      <label class="field">场景
        <select id="scenario">
          <option value="normal">正常闭环</option>
          <option value="auth">认证失败</option>
          <option value="busy">SESSION_BUSY</option>
          <option value="isolation">ISOLATION_UNAVAILABLE</option>
          <option value="cleanup">cleanupComplete=false</option>
        </select>
      </label>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>关于</h3>
      <p class="meta">对照 PAGE-DESIGN 全量（已合入 main）。</p>
      <p class="meta" id="about-health">—</p>
      <p class="meta hidden" id="about-onboard-note"></p>
      <div class="actions">
        <button type="button" class="btn" id="btn-rehealth">重新检测隔离</button>
        <button type="button" class="btn" id="btn-reset-onboard">下次启动重新显示引导</button>
      </div>
    </section>
  `;

  async function refreshAgents() {
    const el = root.querySelector("#settings-agent");
    const res = await api.listAgents();
    if (res.error) {
      el.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
      return;
    }
    el.innerHTML = (res.agents || [])
      .map(
        (a) =>
          `${a.available ? "●" : "○"} ${a.name} ${a.version || ""} ${
            a.available ? "" : "· " + (a.reason || "")
          }`,
      )
      .join("<br/>");
    ctx.state.agents = res.agents || [];
  }

  async function refreshHosts() {
    const el = root.querySelector("#set-hosts");
    const res = await api.listRememberedHosts("");
    if (res.error) {
      el.innerHTML = `<span class="bad">${res.error.message}</span>`;
      return;
    }
    const hosts = res.hosts || [];
    if (!hosts.length) {
      el.textContent = "暂无记住项。";
      return;
    }
    el.innerHTML = hosts
      .map(
        (h) =>
          `<div class="row" style="justify-content:space-between"><span>${h.host} · ${h.workspace} · ${h.scope}</span>
           <button type="button" class="btn" data-rev="${h.host}" data-ws="${h.workspace}">撤销</button></div>`,
      )
      .join("");
    el.querySelectorAll("[data-rev]").forEach((btn) => {
      btn.onclick = async () => {
        await api.revokeRememberedHost({
          host: btn.getAttribute("data-rev"),
          workspace: btn.getAttribute("data-ws"),
        });
        refreshHosts();
      };
    });
  }

  root.querySelector("#btn-refresh-agents").onclick = async () => {
    const btn = root.querySelector("#btn-refresh-agents");
    btn.disabled = true;
    btn.textContent = "检测中…";
    try {
      await refreshAgents();
    } finally {
      btn.disabled = false;
      btn.textContent = "重新检测";
    }
  };
  root.querySelector("#btn-reload-hosts").onclick = refreshHosts;
  root.querySelector("#set-mount").value =
    localStorage.getItem("harbor-default-mount") || "clone";
  root.querySelector("#set-mount").onchange = (e) => {
    localStorage.setItem("harbor-default-mount", e.target.value);
  };

  function syncClearDefaultEnabled() {
    const ask = root.querySelector("#set-ask-clear").checked;
    const wrap = root.querySelector("#set-clear-default-wrap");
    wrap?.querySelectorAll("input").forEach((el) => {
      el.disabled = ask;
    });
    wrap?.classList.toggle("disabled", ask);
  }

  async function loadPrefs() {
    const res = await api.getUIPrefs();
    const prefs = res.prefs || ctx.state.uiPrefs || {};
    root.querySelector("#set-sidebar-collapsed").checked = !!prefs.sidebarCollapsed;
    root.querySelector("#set-auto-collapse").checked =
      prefs.autoCollapseWorkbench !== false;
    root.querySelector("#set-ask-clear").checked = prefs.askClearOnStop !== false;
    const clearDef = prefs.clearOnStopDefault === "clear" ? "clear" : "keep";
    root.querySelectorAll('input[name="clear-default"]').forEach((el) => {
      el.checked = el.value === clearDef;
    });
    syncClearDefaultEnabled();
  }

  async function savePrefs() {
    const clearRadio = root.querySelector('input[name="clear-default"]:checked');
    const prefs = {
      sidebarCollapsed: root.querySelector("#set-sidebar-collapsed").checked,
      autoCollapseWorkbench: root.querySelector("#set-auto-collapse").checked,
      askClearOnStop: root.querySelector("#set-ask-clear").checked,
      clearOnStopDefault: clearRadio?.value === "clear" ? "clear" : "keep",
    };
    const res = await api.setUIPrefs(prefs);
    const st = root.querySelector("#set-prefs-status");
    if (res.error) {
      st.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
      return;
    }
    ctx.state.uiPrefs = res.prefs || prefs;
    ctx.applyUIPrefs?.(ctx.state.uiPrefs);
    syncClearDefaultEnabled();
    st.innerHTML = `<span class="ok">已保存</span>`;
  }

  root.querySelector("#set-sidebar-collapsed").onchange = savePrefs;
  root.querySelector("#set-auto-collapse").onchange = savePrefs;
  root.querySelector("#set-ask-clear").onchange = savePrefs;
  root.querySelectorAll('input[name="clear-default"]').forEach((el) => {
    el.onchange = savePrefs;
  });
  loadPrefs();
  const sel = root.querySelector("#scenario");
  if (sel) {
    sel.value = api.getMockScenario();
    sel.onchange = () => {
      api.setMockScenario(sel.value);
      ctx.refreshHealth?.();
      refreshAgents();
    };
  }
  root.querySelector("#btn-reset-onboard").onclick = () => {
    try {
      localStorage.removeItem("harbor-onboarded");
    } catch (_) {
      showToast("无法写入本机偏好", "bad");
      return;
    }
    showToast("已清除引导标记；下次启动应用将再次显示引导", "ok");
    const note = root.querySelector("#about-onboard-note");
    if (note) {
      note.textContent = "引导将在下次启动时显示（关闭并重开桌面即可）。";
      note.classList.remove("hidden");
    }
  };
  root.querySelector("#btn-rehealth").onclick = async () => {
    const btn = root.querySelector("#btn-rehealth");
    btn.disabled = true;
    btn.textContent = "检测中…";
    try {
      const h = await ctx.refreshHealth?.();
      const el = root.querySelector("#about-health");
      if (h?.available) {
        el.innerHTML = `<span class="ok">● 隔离可用 ${h.isolationLevel || ""}</span>`;
        showToast("隔离检测通过", "ok");
      } else {
        el.innerHTML = `<span class="bad">○ ${h?.reason || "不可用"}</span>`;
        showToast(h?.reason || "隔离不可用", "bad");
      }
    } finally {
      btn.disabled = false;
      btn.textContent = "重新检测隔离";
    }
  };

  refreshAgents();
  refreshHosts();
  root.querySelector("#btn-rehealth").click();
}
