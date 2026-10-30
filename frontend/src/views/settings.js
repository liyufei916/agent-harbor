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
export function mountSettings(root, ctx) {
  root.innerHTML = `
    <h2>设置</h2>
    <p class="sub">只改本机默认偏好。无「关闭隔离 / 本机直跑 Agent」。</p>

    <section class="card">
      <h3>Agent 检测</h3>
      <p class="meta">Seatbelt：须本机预装 CLI（Darwin 25 · arm64 · 非 root）。Docker：检测/运行用镜像内 CLI。MCP：远程 HTTP（Token/OAuth）或宿主 stdio（进程在宿主机）。</p>
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
      <label class="field">外观
        <select id="set-theme">
          <option value="system">跟随系统</option>
          <option value="light">浅色</option>
          <option value="dark">深色</option>
        </select>
      </label>
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
      <h3>安全档位（出站种子）</h3>
      <p class="meta">新建会话时把档位主机注入 Gate ExtraHosts（project 作用域）；仍无全网放行，陌生主机要扩权。</p>
      <label class="field">档位
        <select id="set-security-tier">
          <option value="strict">严格 — 仅模型/MCP Origin</option>
          <option value="standard">标准 — +常用包管理源（默认）</option>
          <option value="relaxed">宽松 — +GitHub 等开发站</option>
        </select>
      </label>
    </section>

    <section class="card" style="margin-top:12px">
      <h3>隔离后端（P2）</h3>
      <p class="meta">默认 Seatbelt。切到「容器隔离」时会<strong>自动启用</strong> Harbor Colima（无需 Docker Desktop；可改「本机 Docker」）。<strong>更改后端后需重启应用</strong>；<code>HARBOR_RUNTIME</code> 优先于本选项。</p>
      <label class="field">后端
        <select id="set-isolation-backend">
          <option value="seatbelt">Seatbelt（P1 · 默认）</option>
          <option value="docker">容器隔离（P2）</option>
        </select>
      </label>
      <label class="field">容器引擎来源
        <select id="set-engine-source">
          <option value="harbor-colima">Harbor 托管 Colima（推荐）</option>
          <option value="system-docker">本机已有 Docker（高级）</option>
        </select>
      </label>
      <p class="meta" id="engine-health-status" role="status">引擎状态：—</p>
      <div class="actions">
        <button type="button" class="btn" id="btn-engine-health">检测引擎</button>
        <button type="button" class="btn primary" id="btn-engine-ensure">启用容器引擎</button>
      </div>
      <p class="meta" id="engine-action-status">首次启用需从 GitHub 下载约数十 MB，网络慢时请耐心等待（勿反复点「检测」）。</p>
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

  function normalizeTheme(theme) {
    if (theme === "light" || theme === "dark" || theme === "system") return theme;
    return "system";
  }

  function normalizeTierPref(tier) {
    if (tier === "strict" || tier === "relaxed" || tier === "standard") return tier;
    return "standard";
  }

  function normalizeIsolationBackend(v) {
    if (v === "docker" || v === "p2" || v === "container") return "docker";
    return "seatbelt";
  }

  function normalizeEngineSource(v) {
    const s = String(v || "").toLowerCase();
    if (s === "system-docker" || s === "system" || s === "desktop") {
      return "system-docker";
    }
    return "harbor-colima";
  }

  function paintEngineHealth(h) {
    const el = root.querySelector("#engine-health-status");
    if (!el) return;
    if (!h) {
      el.textContent = "引擎状态：—";
      return;
    }
    const src = h.source || "";
    if (h.ready) {
      el.innerHTML = `<span class="ok">引擎就绪 · ${src}${h.message ? " · " + h.message : ""}</span>`;
    } else {
      el.innerHTML = `<span class="bad">引擎未就绪 · ${src}${h.message ? " · " + h.message : ""}</span>`;
    }
  }

  async function refreshEngineHealth() {
    const h = await api.getContainerEngineHealth();
    paintEngineHealth(h);
    return h;
  }

  async function loadPrefs() {
    const res = await api.getUIPrefs();
    const prefs = res.prefs || ctx.state.uiPrefs || {};
    root.querySelector("#set-theme").value = normalizeTheme(prefs.theme);
    root.querySelector("#set-sidebar-collapsed").checked = !!prefs.sidebarCollapsed;
    root.querySelector("#set-auto-collapse").checked =
      prefs.autoCollapseWorkbench !== false;
    root.querySelector("#set-ask-clear").checked = prefs.askClearOnStop !== false;
    const clearDef = prefs.clearOnStopDefault === "clear" ? "clear" : "keep";
    root.querySelectorAll('input[name="clear-default"]').forEach((el) => {
      el.checked = el.value === clearDef;
    });
    const tierEl = root.querySelector("#set-security-tier");
    if (tierEl) tierEl.value = normalizeTierPref(prefs.securityTier);
    const isoEl = root.querySelector("#set-isolation-backend");
    if (isoEl) isoEl.value = normalizeIsolationBackend(prefs.isolationBackend);
    const engEl = root.querySelector("#set-engine-source");
    if (engEl) engEl.value = normalizeEngineSource(prefs.engineSource);
    syncClearDefaultEnabled();
    refreshEngineHealth().catch(() => paintEngineHealth(null));
  }

  async function savePrefs() {
    const clearRadio = root.querySelector('input[name="clear-default"]:checked');
    const prevBackend = normalizeIsolationBackend(
      ctx.state.uiPrefs?.isolationBackend,
    );
    const nextBackend = normalizeIsolationBackend(
      root.querySelector("#set-isolation-backend")?.value,
    );
    const nextEngine = normalizeEngineSource(
      root.querySelector("#set-engine-source")?.value,
    );
    const prefs = {
      theme: normalizeTheme(root.querySelector("#set-theme").value),
      sidebarCollapsed: root.querySelector("#set-sidebar-collapsed").checked,
      autoCollapseWorkbench: root.querySelector("#set-auto-collapse").checked,
      askClearOnStop: root.querySelector("#set-ask-clear").checked,
      clearOnStopDefault: clearRadio?.value === "clear" ? "clear" : "keep",
      securityTier: normalizeTierPref(
        root.querySelector("#set-security-tier")?.value,
      ),
      isolationBackend: nextBackend,
      engineSource: nextEngine,
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
    paintRunMode(nextBackend);

    // M4：切到容器隔离时自动 Ensure（无感路径；失败仍 fail-closed，不静默放行）。
    if (nextBackend === "docker" && prevBackend !== "docker") {
      const engSt = root.querySelector("#engine-action-status");
      if (engSt) engSt.textContent = "正在自动启用容器引擎…";
      st.innerHTML = `<span class="warn">已保存 · 正在启用引擎…</span>`;
      showToast("已切到容器隔离，正在启用引擎…", "ok");
      try {
        const h = await api.ensureContainerEngine();
        paintEngineHealth(h);
        if (h?.ready) {
          if (engSt) engSt.innerHTML = `<span class="ok">引擎已就绪 · 请重启应用</span>`;
          st.innerHTML = `<span class="warn">已保存 · 引擎就绪 · 请重启应用后生效</span>`;
          showToast("容器引擎已就绪，请重启应用", "ok");
        } else {
          const msg = h?.message || h?.error?.message || "引擎未就绪";
          if (engSt) engSt.innerHTML = `<span class="bad">${msg}</span>`;
          st.innerHTML = `<span class="warn">已保存 · 引擎未就绪 · 可点「启用容器引擎」重试后重启</span>`;
          showToast(msg, "bad");
        }
      } catch {
        if (engSt) engSt.innerHTML = `<span class="bad">自动启用失败</span>`;
        st.innerHTML = `<span class="warn">已保存 · 自动启用失败 · 请手动启用后重启</span>`;
        showToast("自动启用容器引擎失败", "bad");
      }
      return;
    }

    if (prevBackend !== nextBackend) {
      st.innerHTML = `<span class="warn">已保存 · 隔离后端须重启应用后生效</span>`;
      showToast("隔离后端已保存，请重启应用后生效", "ok");
    } else {
      st.innerHTML = `<span class="ok">已保存</span>`;
    }
  }

  function paintRunMode(backend) {
    const el = root.querySelector("#about-run-mode");
    if (!el) return;
    const b = normalizeIsolationBackend(backend);
    const backendLabel =
      b === "docker"
        ? "隔离后端偏好 Docker（须重启生效）"
        : "隔离后端偏好 Seatbelt";
    el.textContent = api.isMock()
      ? `运行模式：模拟（浏览器 mock）— 不能当真实验收 · ${backendLabel}`
      : `运行模式：真实绑定（桌面后端）· ${backendLabel}`;
  }

  root.querySelector("#set-theme").onchange = savePrefs;
  root.querySelector("#set-sidebar-collapsed").onchange = savePrefs;
  root.querySelector("#set-auto-collapse").onchange = savePrefs;
  root.querySelector("#set-ask-clear").onchange = savePrefs;
  root.querySelector("#set-security-tier")?.addEventListener("change", savePrefs);
  root.querySelector("#set-isolation-backend")?.addEventListener("change", savePrefs);
  root.querySelector("#set-engine-source")?.addEventListener("change", savePrefs);
  root.querySelectorAll('input[name="clear-default"]').forEach((el) => {
    el.onchange = savePrefs;
  });
  root.querySelector("#btn-engine-health")?.addEventListener("click", async () => {
    const btn = root.querySelector("#btn-engine-health");
    const st = root.querySelector("#engine-action-status");
    btn.disabled = true;
    if (st) st.textContent = "检测中…";
    try {
      await refreshEngineHealth();
      if (st) st.innerHTML = `<span class="ok">已刷新</span>`;
    } catch (e) {
      if (st) st.innerHTML = `<span class="bad">检测失败</span>`;
    } finally {
      btn.disabled = false;
    }
  });
  root.querySelector("#btn-engine-ensure")?.addEventListener("click", async () => {
    const btn = root.querySelector("#btn-engine-ensure");
    const st = root.querySelector("#engine-action-status");
    btn.disabled = true;
    if (st) st.textContent = "启用中（可能下载组件 / 启动 Colima）…";
    showToast("正在启用容器引擎…", "ok");
    try {
      const h = await api.ensureContainerEngine();
      paintEngineHealth(h);
      if (h?.ready) {
        if (st) st.innerHTML = `<span class="ok">引擎已就绪</span>`;
        showToast("容器引擎已就绪", "ok");
      } else {
        if (st)
          st.innerHTML = `<span class="bad">${h?.message || h?.error?.message || "启用失败"}</span>`;
        showToast(h?.message || "容器引擎未就绪", "bad");
      }
    } catch (e) {
      if (st) st.innerHTML = `<span class="bad">启用失败</span>`;
      showToast("启用容器引擎失败", "bad");
    } finally {
      btn.disabled = false;
    }
  });
  loadPrefs();
  paintRunMode(ctx.state.uiPrefs?.isolationBackend);
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
    const preferredLevel = normalizeIsolationBackend(
      ctx.state.uiPrefs?.isolationBackend,
    );
    lastAboutCheckItems = freshCheckItems(preferredLevel);
    paintCheckList(aboutChecks, lastAboutCheckItems);
    setAboutChecksExpanded(true);
    try {
      const h = await runIsolationProbe({
        preferredLevel,
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
        const level = h.isolationLevel || "";
        const note = h.reason && level === "docker" ? ` · ${h.reason}` : "";
        el.innerHTML = `<span class="ok">● 隔离可用 ${level}${note}</span>`;
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
