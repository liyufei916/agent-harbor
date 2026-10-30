/**
 * PAGE-DESIGN §5.8 设置（本机默认；无关闭隔离）
 */
import * as api from "../api.js";
import { AGENT_INSTALL, copyText } from "../agent-install.js";
import { showToast } from "../toast.js";
import {
  freshCheckItems,
  paintCheckList,
  runIsolationProbe,
} from "../isolation-checks.js";
import {
  clearUsageAnalytics,
  summarizeUsage,
} from "../usage-analytics.js";

export function mountSettings(root, ctx) {
  root.innerHTML = `
    <h2>设置</h2>
    <p class="sub">只改本机默认偏好。无「关闭隔离 / 本机直跑 Agent」。</p>

    <section class="card">
      <h3>Agent 检测</h3>
      <p class="meta">须本机预装 CLI；隔离仅 <strong>macOS Darwin 25 · arm64 · 非 root</strong>。MCP：远程 HTTP（Token/OAuth）或宿主 stdio（进程在宿主机）。</p>
      <div class="meta" id="settings-agent">—</div>
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
      <h3>本机使用概况</h3>
      <p class="meta">仅存本机 localStorage，不外传。复用审计事件与会话终态。</p>
      <div class="meta" id="usage-stats">—</div>
      <div class="actions">
        <button type="button" class="btn" id="btn-refresh-usage">刷新</button>
        <button type="button" class="btn" id="btn-clear-usage">清除本地统计</button>
      </div>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>关于</h3>
      <p class="meta" id="about-run-mode">运行模式：检测中…</p>
      <p class="meta">对照 PAGE-DESIGN 全量（已合入 main）。可安装包见 GitHub Releases / CI Artifacts。</p>
      <div class="isolation-checks-head">
        <p class="meta" style="margin:0">隔离能力检测</p>
        <button type="button" class="btn" id="btn-toggle-health-checks" aria-expanded="false" aria-controls="about-health-checks-wrap">
          展开明细
        </button>
      </div>
      <div id="about-health-checks-wrap" class="isolation-checks-wrap collapsed" hidden>
        <ul class="block-checks" id="about-health-checks"></ul>
      </div>
      <p class="meta" id="about-health" role="status">—</p>
      <p class="meta warn hidden" id="about-vault" role="status"></p>
      <p class="meta hidden" id="about-onboard-note"></p>
      <div class="actions">
        <button type="button" class="btn" id="btn-rehealth">重新检测隔离</button>
        <button type="button" class="btn" id="btn-reset-onboard">下次启动重新显示引导</button>
      </div>
    </section>
  `;

  const aboutChecks = root.querySelector("#about-health-checks");
  const aboutChecksWrap = root.querySelector("#about-health-checks-wrap");
  const toggleChecksBtn = root.querySelector("#btn-toggle-health-checks");
  let aboutChecksExpanded = false;
  let aboutChecksBusy = false;
  let lastAboutCheckItems = freshCheckItems();
  paintCheckList(aboutChecks, lastAboutCheckItems);

  function setAboutChecksExpanded(expanded) {
    aboutChecksExpanded = !!expanded;
    if (!aboutChecksWrap) return;
    aboutChecksWrap.classList.toggle("collapsed", !aboutChecksExpanded);
    if (aboutChecksExpanded) {
      aboutChecksWrap.removeAttribute("hidden");
      paintCheckList(aboutChecks, lastAboutCheckItems);
    } else {
      aboutChecksWrap.setAttribute("hidden", "");
    }
    if (toggleChecksBtn) {
      toggleChecksBtn.setAttribute("aria-expanded", aboutChecksExpanded ? "true" : "false");
      toggleChecksBtn.textContent = aboutChecksExpanded ? "收起明细" : "展开明细";
    }
  }

  toggleChecksBtn.onclick = () => {
    if (aboutChecksBusy && aboutChecksExpanded) return; // 检测中勿收起，避免打断观感
    setAboutChecksExpanded(!aboutChecksExpanded);
  };

  async function refreshAgents() {
    const el = root.querySelector("#settings-agent");
    const res = await api.listAgents();
    if (res.error) {
      el.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
      return;
    }
    el.innerHTML = (res.agents || [])
      .map((a) => {
        const install = AGENT_INSTALL[a.id];
        const miss = !a.available;
        return `<div class="row" style="justify-content:space-between;align-items:flex-start;gap:8px;margin:6px 0">
          <div>
            ${a.available ? "●" : "○"} <strong>${a.name}</strong> ${a.version || ""}
            ${miss ? `<span class="meta"> · ${a.reason || "未安装"}</span>` : ""}
            ${miss && install ? `<p class="meta" style="margin:4px 0 0">${install.hint}</p>` : ""}
          </div>
          ${
            miss && install
              ? `<button type="button" class="btn" data-copy-install="${a.id}">复制安装命令</button>`
              : ""
          }
        </div>`;
      })
      .join("");
    el.querySelectorAll("[data-copy-install]").forEach((btn) => {
      btn.onclick = async () => {
        const id = btn.getAttribute("data-copy-install");
        const ok = await copyText(AGENT_INSTALL[id]?.cmd || "");
        showToast(ok ? "已复制安装命令" : "复制失败，请手动选中命令", ok ? "ok" : "bad");
      };
    });
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
  function paintUsage() {
    const el = root.querySelector("#usage-stats");
    if (!el) return;
    const s = summarizeUsage();
    const top =
      s.topFailures.length === 0
        ? "<li class='meta'>暂无</li>"
        : s.topFailures.map((f) => `<li>${f.label} · ${f.count}</li>`).join("");
    el.innerHTML = `
      <p>会话样本：${s.sessions} · 审计条数：${s.auditTotal}</p>
      <p>首任务成功率：${s.firstTaskSuccessRate}</p>
      <p>越权拒绝率：${s.denyRate}</p>
      <p>工具调用失败率：${s.mcpFailRate}</p>
      <p>典型失败 Top：</p>
      <ul class="audit-list">${top}</ul>`;
  }
  paintUsage();
  root.querySelector("#btn-refresh-usage")?.addEventListener("click", paintUsage);
  root.querySelector("#btn-clear-usage")?.addEventListener("click", () => {
    clearUsageAnalytics();
    paintUsage();
    showToast("已清除本机使用统计", "ok");
  });
  const runMode = root.querySelector("#about-run-mode");
  if (runMode) {
    runMode.textContent = api.isMock()
      ? "运行模式：模拟（浏览器 mock）— 不能当真实验收"
      : "运行模式：真实绑定（桌面后端）";
  }
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
    aboutChecksBusy = true;
    if (toggleChecksBtn) toggleChecksBtn.disabled = true;
    const el = root.querySelector("#about-health");
    el.textContent = "正在逐项检测…";
    lastAboutCheckItems = freshCheckItems();
    paintCheckList(aboutChecks, lastAboutCheckItems);
    setAboutChecksExpanded(true);
    try {
      const h = await runIsolationProbe({
        getHealth: async () => {
          if (typeof ctx.refreshHealth === "function") {
            return ctx.refreshHealth();
          }
          return api.getRuntimeHealth();
        },
        onUpdate: (items) => {
          lastAboutCheckItems = items;
          paintCheckList(aboutChecks, items);
        },
      });
      const vaultEl = root.querySelector("#about-vault");
      if (vaultEl) {
        // 凭据介质始终可见：正常也写一行，告警时用 warn 文案。
        vaultEl.textContent = h?.vaultNotice
          ? h.vaultNotice
          : "凭据介质：macOS Keychain（正常）。密钥不进沙箱。";
        vaultEl.classList.toggle("warn", Boolean(h?.vaultNotice));
        vaultEl.classList.remove("hidden");
      }
      if (h?.available) {
        el.innerHTML = `<span class="ok">● 隔离可用 ${h.isolationLevel || ""}</span>`;
        showToast(
          h?.vaultNotice ? "隔离通过；凭据库有告警，见关于" : "隔离检测通过",
          h?.vaultNotice ? "bad" : "ok",
        );
      } else {
        el.innerHTML = `<span class="bad">○ ${h?.reason || "不可用"}</span>`;
        showToast(h?.reason || "隔离不可用", "bad");
      }
    } finally {
      aboutChecksBusy = false;
      if (toggleChecksBtn) toggleChecksBtn.disabled = false;
      setAboutChecksExpanded(false);
      btn.disabled = false;
      btn.textContent = "重新检测隔离";
    }
  };

  refreshAgents();
  refreshHosts();
  root.querySelector("#btn-rehealth").click();
}
