/**
 * PAGE-DESIGN §5.8 设置（本机默认；无关闭隔离）
 */
import * as api from "../api.js";
import { AGENT_INSTALL, copyText } from "../agent-install.js";
import { allEnvHintsHTML } from "../agent-env-hints.js";
import { requestAppRestart } from "../restart-app.js";
import { showToast } from "../toast.js";
import {
  freshCheckItems,
  paintCheckList,
  runIsolationProbe,
} from "../isolation-checks.js";
export function mountSettings(root, ctx, opts = {}) {
  const focusIsolation = !!opts.focusIsolation;
  root.innerHTML = `
    <h2>设置</h2>
    <p class="sub">只改本机默认偏好。无「关闭隔离 / 本机直跑 Agent」。</p>
    <div id="settings-recovery-banner" class="banner warn-banner ${focusIsolation ? "" : "hidden"}" role="status">
      <strong>隔离未就绪 · 可在此改引擎</strong>
      <p class="meta">托管 Colima 检测失败时，若本机已有 Docker Desktop，请改选「本机 Docker」并连接；或把后端改回 Seatbelt。改完后<strong>重启应用</strong>再开跑。</p>
      <div class="actions" id="settings-recovery-actions"></div>
    </div>

    <section class="card">
      <h3>Agent 检测</h3>
      <p class="meta">Seatbelt：须本机预装 CLI（Darwin 25 · arm64 · 非 root）。Docker：检测/运行用镜像内 CLI。MCP：远程 HTTP（Token/OAuth）或宿主 stdio（进程在宿主机）。</p>
      <div class="meta" id="settings-agent">—</div>
      <button type="button" class="btn" id="btn-refresh-agents">重新检测</button>
      <details style="margin-top:12px">
        <summary class="meta">预置 profile 环境变量（Codex / dsh）</summary>
        ${allEnvHintsHTML()}
      </details>
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

    <section class="card ${focusIsolation ? "settings-focus-isolation" : ""}" style="margin-top:12px" id="settings-isolation">
      <h3>隔离后端</h3>
      <p class="meta">二选一：<strong>Seatbelt</strong>（默认，macOS 沙箱）或 <strong>Docker</strong>（一会话一容器）。Colima / 本机 Docker 只是 Docker 后端的<strong>引擎</strong>，选 Seatbelt 时不必选引擎。<strong>改后端后需重启</strong>；<code>HARBOR_RUNTIME</code> 优先。</p>
      <label class="field">后端
        <select id="set-isolation-backend">
          <option value="seatbelt">Seatbelt（默认）</option>
          <option value="docker">Docker</option>
        </select>
      </label>
      <div class="actions" id="isolation-restart-row" hidden>
        <button type="button" class="btn primary" id="btn-restart-for-isolation">重启应用使生效</button>
        <span class="meta">偏好已保存，本进程仍用旧隔离后端。</span>
      </div>
      <div id="engine-panel" class="engine-panel" hidden>
        <p class="meta"><strong>无 Docker Desktop</strong> → 用「Harbor 托管 Colima」并<strong>一键安装</strong>（才下载）。<strong>本机已有 Docker</strong> → 选「本机 Docker」，只检测连接、<strong>不下载托管组件</strong>。</p>
        <label class="field">容器引擎来源
          <select id="set-engine-source">
            <option value="harbor-colima">Harbor 托管 Colima（无 Desktop 时推荐）</option>
            <option value="system-docker">本机 Docker（高级）</option>
          </select>
        </label>
        <p class="meta" id="engine-desktop-status" role="status">Docker Desktop：检测中…</p>
        <p class="meta" id="engine-health-status" role="status">引擎状态：—</p>
        <div class="engine-progress" id="engine-progress" aria-live="polite">
          <div class="engine-progress-track">
            <div class="engine-progress-bar" id="engine-progress-bar"></div>
          </div>
          <p class="meta engine-progress-label" id="engine-progress-label">0%</p>
        </div>
        <div class="actions">
          <button type="button" class="btn" id="btn-engine-health">检测引擎</button>
          <button type="button" class="btn primary" id="btn-engine-ensure">一键安装并启用</button>
          <button type="button" class="btn" id="btn-engine-stop">停止 / 取消</button>
        </div>
        <p class="meta" id="engine-action-status">未装 Desktop 时选 Colima 再安装；有本机 Docker 时改选「本机 Docker」后点连接即可。</p>
      </div>
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

  function paintEngineProgress(percent, label, on) {
    const wrap = root.querySelector("#engine-progress");
    const bar = root.querySelector("#engine-progress-bar");
    const lab = root.querySelector("#engine-progress-label");
    if (!wrap || !bar || !lab) return;
    const pct = Math.max(0, Math.min(100, Number(percent) || 0));
    wrap.classList.toggle("on", !!on || pct > 0);
    bar.style.width = `${pct}%`;
    lab.textContent = label ? `${pct}% · ${label}` : `${pct}%`;
  }

  function currentEngineSource() {
    return normalizeEngineSource(root.querySelector("#set-engine-source")?.value);
  }

  function syncEnsureButton(h) {
    const btn = root.querySelector("#btn-engine-ensure");
    const stopBtn = root.querySelector("#btn-engine-stop");
    const hint = root.querySelector("#engine-action-status");
    if (!btn) return;
    const src = currentEngineSource();
    const system = src === "system-docker";
    if (system) {
      btn.textContent = h?.ready ? "重新检测本机 Docker" : "连接本机 Docker";
      if (stopBtn) stopBtn.textContent = "取消连接";
      if (hint && !h?.installing) {
        hint.textContent = "本机 Docker：不下载 Colima，只检测本机 docker CLI / daemon 是否可用。";
      }
    } else {
      if (h?.ready) btn.textContent = "重新启用 Colima";
      else if (h?.componentsInstalled) btn.textContent = "启动 Colima";
      else btn.textContent = "一键安装并启用";
      if (stopBtn) stopBtn.textContent = "停止 / 取消";
      if (hint && !h?.installing) {
        hint.textContent = h?.dockerDesktopInstalled
          ? "已有 Desktop 时可改选「本机 Docker」。无 Desktop 或不想用 Desktop 时，一键安装 Harbor Colima。"
          : "未检测到 Docker Desktop：请用 Harbor Colima 一键安装（才会下载）。";
      }
    }
  }

  function paintRecoveryActions(h) {
    const banner = root.querySelector("#settings-recovery-banner");
    const actions = root.querySelector("#settings-recovery-actions");
    if (!banner || !actions) return;
    const backend = normalizeIsolationBackend(
      root.querySelector("#set-isolation-backend")?.value ||
        ctx.state.uiPrefs?.isolationBackend,
    );
    const show =
      focusIsolation ||
      (backend === "docker" && h && !h.ready && !h.installing);
    banner.classList.toggle("hidden", !show);
    if (!show) {
      actions.innerHTML = "";
      return;
    }
    const bits = [];
    if (h?.dockerDesktopInstalled && currentEngineSource() !== "system-docker") {
      bits.push(
        `<button type="button" class="btn primary" id="btn-prefer-system-docker">改用本机 Docker</button>`,
      );
    }
    if (backend === "docker") {
      bits.push(
        `<button type="button" class="btn" id="btn-prefer-seatbelt">退回 Seatbelt</button>`,
      );
    }
    actions.innerHTML = bits.join("") || `<p class="meta">请在下方选择引擎并检测。</p>`;
    actions.querySelector("#btn-prefer-system-docker")?.addEventListener("click", async () => {
      const eng = root.querySelector("#set-engine-source");
      const iso = root.querySelector("#set-isolation-backend");
      if (iso) iso.value = "docker";
      if (eng) eng.value = "system-docker";
      syncEnginePanel("docker");
      await savePrefs();
      showToast("已改为本机 Docker，正在连接…", "ok");
      root.querySelector("#btn-engine-ensure")?.click();
    });
    actions.querySelector("#btn-prefer-seatbelt")?.addEventListener("click", async () => {
      const iso = root.querySelector("#set-isolation-backend");
      if (iso) iso.value = "seatbelt";
      syncEnginePanel("seatbelt");
      await savePrefs();
      showToast("已退回 Seatbelt，请重启应用后重新检测", "ok");
    });
  }

  function paintEngineHealth(h) {
    const el = root.querySelector("#engine-health-status");
    const desk = root.querySelector("#engine-desktop-status");
    if (desk) {
      if (!h) {
        desk.textContent = "Docker Desktop：—";
      } else if (h.dockerDesktopInstalled) {
        desk.innerHTML = `<span class="ok">已检测到 Docker Desktop · 可直接选「本机 Docker」，无需装托管 Colima</span>`;
      } else {
        desk.innerHTML = `<span class="warn">未检测到 Docker Desktop · 请用 Harbor Colima 一键安装</span>`;
      }
    }
    syncEnsureButton(h);
    paintRecoveryActions(h);
    if (!el) return;
    if (!h) {
      el.textContent = "引擎状态：—";
      return;
    }
    const src = h.source || "";
    if (h.installing || (h.progressPercent > 0 && h.progressPercent < 100 && !h.ready)) {
      paintEngineProgress(h.progressPercent, h.progressLabel || "进行中…", true);
    }
    if (h.ready) {
      el.innerHTML = `<span class="ok">引擎就绪 · ${src}${h.message ? " · " + h.message : ""}</span>`;
      paintEngineProgress(100, h.progressLabel || "完成", false);
    } else {
      el.innerHTML = `<span class="bad">引擎未就绪 · ${src}${h.message ? " · " + h.message : ""}</span>`;
    }
  }

  function syncEnginePanel(backend) {
    const panel = root.querySelector("#engine-panel");
    const docker = normalizeIsolationBackend(backend) === "docker";
    if (!panel) return;
    if (docker) panel.removeAttribute("hidden");
    else panel.setAttribute("hidden", "");
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
    const backend = normalizeIsolationBackend(prefs.isolationBackend);
    if (isoEl) isoEl.value = backend;
    const engEl = root.querySelector("#set-engine-source");
    if (engEl) engEl.value = normalizeEngineSource(prefs.engineSource);
    syncClearDefaultEnabled();
    syncEnginePanel(backend);
    if (backend === "docker") {
      refreshEngineHealth()
        .then(async (h) => {
          // 从阻断页进入且已装 Desktop：默认切到本机 Docker，避免卡在托管 Colima 探测失败
          if (
            focusIsolation &&
            h?.dockerDesktopInstalled &&
            normalizeEngineSource(prefs.engineSource) !== "system-docker"
          ) {
            const engEl = root.querySelector("#set-engine-source");
            if (engEl) engEl.value = "system-docker";
            await savePrefs();
            showToast("已检测到 Docker Desktop，已切换为「本机 Docker」并连接", "ok");
            // 直接探测本机 daemon，避免只改偏好仍卡在未就绪
            root.querySelector("#btn-engine-ensure")?.click();
            return;
          }
          return h;
        })
        .catch(() => paintEngineHealth(null));
    } else if (focusIsolation) {
      paintRecoveryActions(null);
    }
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
    syncEnginePanel(nextBackend);
    paintRunMode(nextBackend);

    if (prevBackend !== nextBackend) {
      if (nextBackend === "docker") {
        st.innerHTML = `<span class="warn">已保存 · 请选择引擎并启用，再点「重启应用使生效」</span>`;
        showToast("已切到 Docker：启用引擎后请重启", "ok");
        refreshEngineHealth().catch(() => paintEngineHealth(null));
      } else {
        st.innerHTML = `<span class="warn">已保存 · 请点「重启应用使生效」</span>`;
        showToast("隔离后端已保存，请重启应用", "ok");
      }
    } else {
      st.innerHTML = `<span class="ok">已保存</span>`;
    }
    syncIsolationRestartRow();
  }

  function syncIsolationRestartRow() {
    const row = root.querySelector("#isolation-restart-row");
    if (!row) return;
    const preferred = normalizeIsolationBackend(
      root.querySelector("#set-isolation-backend")?.value ||
        ctx.state.uiPrefs?.isolationBackend,
    );
    const level = String(ctx.state.health?.isolationLevel || "").toLowerCase();
    const fromHealth =
      level.includes("docker")
        ? "docker"
        : level.includes("seatbelt") || level.includes("mock-seatbelt")
          ? "seatbelt"
          : "";
    const need = !!fromHealth && fromHealth !== preferred;
    row.hidden = !need;
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

  root.querySelector("#btn-restart-for-isolation")?.addEventListener("click", () => {
    void requestAppRestart({
      reason: "重启后将按已保存的隔离后端偏好重新装配 Runtime。",
    });
  });
  root.querySelector("#set-theme").onchange = savePrefs;
  root.querySelector("#set-sidebar-collapsed").onchange = savePrefs;
  root.querySelector("#set-auto-collapse").onchange = savePrefs;
  root.querySelector("#set-ask-clear").onchange = savePrefs;
  root.querySelector("#set-security-tier")?.addEventListener("change", savePrefs);
  root.querySelector("#set-isolation-backend")?.addEventListener("change", savePrefs);
  root.querySelector("#set-engine-source")?.addEventListener("change", () => {
    syncEnsureButton(null);
    savePrefs();
    if (normalizeIsolationBackend(root.querySelector("#set-isolation-backend")?.value) === "docker") {
      refreshEngineHealth().catch(() => paintEngineHealth(null));
    }
  });
  root.querySelectorAll('input[name="clear-default"]').forEach((el) => {
    el.onchange = savePrefs;
  });
  let offEngineProgress = null;
  if (typeof api.onEngineProgress === "function") {
    offEngineProgress = api.onEngineProgress((p) => {
      if (!p) return;
      paintEngineProgress(p.percent, p.label || "", !p.done);
      const st = root.querySelector("#engine-action-status");
      if (st && !p.done) {
        st.textContent = p.label || `安装中 ${p.percent || 0}%`;
      }
      if (p.done && p.error && st) {
        st.innerHTML = `<span class="bad">${p.error}</span>`;
      }
    });
  }

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
    const healthBtn = root.querySelector("#btn-engine-health");
    const stopBtn = root.querySelector("#btn-engine-stop");
    btn.disabled = true;
    if (healthBtn) healthBtn.disabled = true;
    if (stopBtn) stopBtn.disabled = false;
    const system = currentEngineSource() === "system-docker";
    paintEngineProgress(1, system ? "连接本机 Docker…" : "开始安装…", true);
    if (st) {
      st.textContent = system
        ? "正在检测本机 Docker…"
        : "一键安装中（下载 / 启动 Colima）…可点「停止 / 取消」";
    }
    showToast(system ? "正在连接本机 Docker…" : "正在一键安装 Harbor Colima…", "ok");
    try {
      const h = await api.ensureContainerEngine();
      paintEngineHealth(h);
      if (h?.ready) {
        paintEngineProgress(100, system ? "本机 Docker 可用" : "安装完成", false);
        if (st)
          st.innerHTML = `<span class="ok">引擎已就绪 · 请点上方「重启应用使生效」</span>`;
        showToast("容器引擎已就绪，请重启应用", "ok");
        syncIsolationRestartRow();
      } else {
        if (st)
          st.innerHTML = `<span class="bad">${h?.message || h?.error?.message || "失败"}</span>`;
        showToast(h?.message || "容器引擎未就绪", "bad");
      }
    } catch (e) {
      if (st) st.innerHTML = `<span class="bad">${system ? "连接失败" : "安装失败"}</span>`;
      showToast(system ? "连接本机 Docker 失败" : "一键安装失败", "bad");
    } finally {
      btn.disabled = false;
      if (healthBtn) healthBtn.disabled = false;
    }
  });
  root.querySelector("#btn-engine-stop")?.addEventListener("click", async () => {
    const btn = root.querySelector("#btn-engine-stop");
    const st = root.querySelector("#engine-action-status");
    btn.disabled = true;
    if (st) st.textContent = "正在停止 / 取消…";
    showToast("正在停止容器引擎…", "ok");
    try {
      const h = await api.stopContainerEngine();
      paintEngineHealth(h);
      paintEngineProgress(0, h?.message || "已停止", false);
      if (st) st.innerHTML = `<span class="ok">${h?.message || "已停止"}</span>`;
      showToast(h?.message || "已停止", "ok");
    } catch (e) {
      if (st) st.innerHTML = `<span class="bad">停止失败</span>`;
      showToast("停止失败", "bad");
    } finally {
      btn.disabled = false;
    }
  });
  loadPrefs();
  paintRunMode(ctx.state.uiPrefs?.isolationBackend);
  syncIsolationRestartRow();
  // cleanup when leaving settings: views typically remount; best-effort
  root.addEventListener(
    "harbor:settings-dispose",
    () => {
      try {
        offEngineProgress?.();
      } catch {}
    },
    { once: true },
  );
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
  if (focusIsolation) {
    requestAnimationFrame(() => {
      root.querySelector("#settings-isolation")?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
  }
}
