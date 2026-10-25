/**
 * PAGE-DESIGN §5.8 设置（本机默认；无关闭隔离）
 */
import * as api from "../api.js";

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
          <option value="direct">直接编辑</option>
          <option value="clone">安全副本</option>
        </select>
      </label>
      <p class="meta">用于下次新会话向导预填。</p>
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
      <p class="meta">分支 feat/page-design-full · 对照 PAGE-DESIGN 全量。</p>
      <p class="meta" id="about-health">—</p>
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

  root.querySelector("#btn-refresh-agents").onclick = refreshAgents;
  root.querySelector("#btn-reload-hosts").onclick = refreshHosts;
  root.querySelector("#set-mount").value =
    localStorage.getItem("harbor-default-mount") || "direct";
  root.querySelector("#set-mount").onchange = (e) => {
    localStorage.setItem("harbor-default-mount", e.target.value);
  };
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
    localStorage.removeItem("harbor-onboarded");
    alert("已清除。刷新页面将再次显示引导。");
  };
  root.querySelector("#btn-rehealth").onclick = async () => {
    const h = await ctx.refreshHealth?.();
    root.querySelector("#about-health").innerHTML = h?.available
      ? `<span class="ok">● 隔离可用 ${h.isolationLevel || ""}</span>`
      : `<span class="bad">○ ${h?.reason || "不可用"}</span>`;
  };

  refreshAgents();
  refreshHosts();
  root.querySelector("#btn-rehealth").click();
}
