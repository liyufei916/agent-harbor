/**
 * PAGE-DESIGN 全量壳：顶栏会话切换 + 侧栏 HOME/模型/工具/记录/设置
 */
import "./xterm-boot.js";
import * as api from "./api.js";
import { initializeTransport } from "./startup.js";
import { mountHome } from "./views/home.js";
import { mountWizard } from "./views/wizard.js";
import { mountModels } from "./views/models.js";
import { mountTools } from "./views/tools.js";
import { mountRecords } from "./views/records.js";
import { mountSettings } from "./views/settings.js";
import { mountSession } from "./views/session.js";
import { mountOnboarding } from "./views/onboarding.js";
import { mountBlock } from "./views/block.js";
import { showToast } from "./toast.js";
import { showUsageGuide } from "./usage-guide.js";
import {
  mergeHistoryRow,
} from "./session-history.js";
import {
  recordAudit,
  recordSessionOutcome,
  seedShotUsageForScreenshots,
} from "./usage-analytics.js";
import {
  loadTutorialProgress,
  markTutorialStep,
  resetTutorial,
  skipOptionalTutorialStep,
  skipTutorial,
  tutorialSteps,
} from "./tutorial-progress.js";
import { normalizeTier } from "./security-tier.js";
import { healthAllowsStart, healthBlockReason } from "./health-ready.js";
import { requestAppRestart } from "./restart-app.js";
import {
  computeIsolationBadge,
  normalizeEngineSource,
  normalizeIsolationBackend,
} from "./isolation-badge.js";

const badge = document.getElementById("mode-badge");
const isolationBadge = document.getElementById("isolation-badge");
const restartAppBtn = document.getElementById("btn-restart-app");
const sidebar = document.getElementById("sidebar");
const healthDot = document.getElementById("health-dot");
const healthText = document.getElementById("health-text");
const onboardRoot = document.getElementById("onboarding-root");
const blockRoot = document.getElementById("block-root");
const shell = document.getElementById("app-shell");
const sessionSwitchWrap = document.getElementById("session-switch-wrap");
const sessionSwitch = document.getElementById("session-switch");

const views = {
  home: document.getElementById("view-home"),
  wizard: document.getElementById("view-wizard"),
  models: document.getElementById("view-models"),
  tools: document.getElementById("view-tools"),
  records: document.getElementById("view-records"),
  settings: document.getElementById("view-settings"),
  session: document.getElementById("view-session"),
};

let sessionView;
let wizardView;
let currentRoute = "home";
/** 本次停留工作台期间用户是否手动展开过侧栏（离开工作台后清零，下次进入可再自动收缩） */
let workbenchStayExpanded = false;

const ctx = {
  state: {
    agentId: "claude",
    agentAvailable: false,
    agents: [],
    modelAccount: null,
    mcpAccount: null,
    accounts: [],
    workspace: "",
    sessionId: "",
    lastSnapshot: null,
    sessionHistory: [],
    health: null,
    uiPrefs: {
      sidebarCollapsed: false,
      autoCollapseWorkbench: true,
      askClearOnStop: true,
      clearOnStopDefault: "keep",
      theme: "system",
      securityTier: "standard",
      isolationBackend: "seatbelt",
      engineSource: "harbor-colima",
    },
    /** 本进程启动时锁定的引擎来源；换引擎后与偏好不同则待重启 */
    engineSourceAtLaunch: "",
    permissionPending: false,
  },
  setPermissionPending(pending) {
    ctx.state.permissionPending = !!pending;
    updatePermissionBadge();
  },
  navigate(name, opts) {
    navigate(name, opts);
  },
  showBlock(reason) {
    showBlock(reason || ctx.state.health?.reason);
  },
  applyUIPrefs(prefs) {
    applyUIPrefs(prefs);
  },
  async stopSession(id) {
    const res = await api.stopSession(id);
    if (res.session) upsertHistory(res.session);
    syncSessionSwitch();
    return res;
  },
  async destroySession(id) {
    const res = await api.destroySession(id);
    if (!res.error) {
      ctx.state.sessionHistory = (ctx.state.sessionHistory || []).filter(
        (s) => s.sessionId !== id,
      );
      if (ctx.state.sessionId === id) {
        ctx.state.sessionId = "";
        ctx.state.lastSnapshot = null;
      }
    }
    syncSessionSwitch();
    return res;
  },
  onAccountsChanged() {},
  async refreshHome() {
    try {
      const res = await api.listSessions();
      if (!res.error && Array.isArray(res.sessions)) {
        for (const s of res.sessions) upsertHistory(s);
      }
    } catch (_) {
      /* mock/live 暂缺时用本地历史 */
    }
    if (currentRoute === "home") mountHome(views.home, ctx);
    syncSessionSwitch();
  },
  refreshHealth() {
    return refreshHealth();
  },
  onSessionCreated(snapshot, createSpec) {
    api.activateSession?.(snapshot);
    markTutorialStep(3);
    ctx.state.sessionId = snapshot.sessionId;
    ctx.state.lastSnapshot = snapshot;
    if (createSpec?.task) ctx.state.lastTask = createSpec.task;
    upsertHistory(snapshot, createSpec);
    try {
      sessionStorage.setItem("harbor-session-id", snapshot.sessionId);
    } catch {}
    sessionView.resetForNewSession(snapshot);
    syncSessionSwitch();
    navigate("session");
  },
  /** 在已停止的原会话上重新拉起，不新建会话。 */
  async restartSession(sessionId) {
    const prev = (ctx.state.sessionHistory || []).find(
      (s) => s.sessionId === sessionId,
    );
    if (!prev) {
      showToast("找不到会话记录", "bad");
      return { error: { code: "SESSION_NOT_FOUND", message: "无历史" } };
    }
    if (!healthAllowsStart(ctx.state.health)) {
      showBlock(healthBlockReason(ctx.state.health));
      return { error: { code: "ISOLATION_UNAVAILABLE", message: "隔离不可用" } };
    }
    const res = await api.startSession(sessionId);
    if (res.error || !res.session) {
      const message = res.error
        ? `${res.error.code}: ${res.error.message}`
        : "无法重新开启";
      showToast(message, "bad");
      return res;
    }
    if (res.session.sessionId !== sessionId) {
      showToast("重新开启返回了另一条会话", "bad");
      return res;
    }
    ctx.state.sessionId = res.session.sessionId;
    ctx.state.lastSnapshot = res.session;
    upsertHistory(res.session, {
      task: prev.task || "",
      workspace: prev.projectWorkspace || prev.workspace || "",
      agentId: prev.agentId,
      modelAccountId: prev.modelAccountId,
      mcpIds: prev.mcpIds || [],
      mountMode: prev.mountMode,
    });
    try {
      sessionStorage.setItem("harbor-session-id", res.session.sessionId);
    } catch {}
    sessionView?.showSnapshot?.(res.session, { freshStream: true });
    syncSessionSwitch();
    navigate("session");
    showToast("已重新开启原会话", "ok");
    return res;
  },
  async openSession(sessionId) {
    const res = await api.getSession(sessionId);
    if (res.error || !res.session) return;
    upsertHistory(res.session);
    ctx.state.sessionId = res.session.sessionId;
    ctx.state.lastSnapshot = res.session;
    try {
      sessionStorage.setItem("harbor-session-id", res.session.sessionId);
    } catch {}
    sessionView?.showSnapshot?.(res.session);
    syncSessionSwitch();
    navigate("session");
  },
  showTab(name) {
    if (name === "session") navigate("session");
    else if (name === "setup") navigate("wizard");
  },
};

function upsertHistory(snapshot, createSpec) {
  const row = mergeHistoryRow(
    (ctx.state.sessionHistory || []).find(
      (s) => s.sessionId === snapshot?.sessionId,
    ),
    snapshot,
    createSpec,
    ctx.state.lastTask || "",
  );
  if (!row) return;
  const list = ctx.state.sessionHistory || [];
  const i = list.findIndex((s) => s.sessionId === row.sessionId);
  if (i >= 0) list[i] = row;
  else list.unshift(row);
  ctx.state.sessionHistory = list.slice(0, 20);
  recordSessionOutcome(snapshot);
}

function syncSessionSwitch() {
  const running = (ctx.state.sessionHistory || []).filter((s) =>
    ["provisioning", "running", "stopping"].includes(s.state),
  );
  if (!running.length) {
    sessionSwitchWrap.classList.add("hidden");
    return;
  }
  sessionSwitchWrap.classList.remove("hidden");
  sessionSwitch.replaceChildren(
    ...running.map((s) => {
      const o = document.createElement("option");
      o.value = s.sessionId;
      o.textContent = `${s.agentId} · ${s.workspace}`;
      if (s.sessionId === ctx.state.sessionId) o.selected = true;
      return o;
    }),
  );
}

function syncBadge() {
  const banner = document.getElementById("mode-banner");
  if (!badge) {
    syncIsolationBadge();
    return;
  }
  if (api.isMock()) {
    // 仅 mock 顶栏提示；桌面真连不展示「真实绑定」（对用户无信息量）
    badge.classList.remove("hidden");
    badge.removeAttribute("hidden");
    badge.textContent = "模拟模式";
    badge.className = "badge mock";
    if (banner) {
      banner.classList.remove("hidden");
      banner.textContent = "模拟模式：仅看 UI，不能记入 I1–I11 真实验收。";
    }
  } else if (api.isHttpLive()) {
    // 双终端亮点：仅顶栏区分表面，不挂验收恐吓横幅
    badge.classList.remove("hidden");
    badge.removeAttribute("hidden");
    badge.textContent = "本机 live（浏览器）";
    badge.className = "badge live";
    banner?.classList.add("hidden");
  } else {
    badge.classList.add("hidden");
    badge.textContent = "";
    badge.className = "badge live hidden";
    banner?.classList.add("hidden");
  }
  syncIsolationBadge();
}

/** 标题栏标记隔离后端 + Docker 引擎底座；后端或引擎与本进程不一致时出重启键。 */
function syncIsolationBadge() {
  if (!isolationBadge) return;
  const preferred = preferredIsolationLevel();
  const preferredEngine = normalizeEngineSource(ctx.state.uiPrefs?.engineSource);
  const atLaunch =
    ctx.state.engineSourceAtLaunch || preferredEngine;
  const view = computeIsolationBadge({
    health: ctx.state.health,
    preferredBackend: preferred,
    preferredEngine,
    engineSourceAtLaunch: atLaunch,
    allowsStart: healthAllowsStart(ctx.state.health),
  });
  isolationBadge.textContent = view.text;
  isolationBadge.className = view.className;
  isolationBadge.title = view.title;
  if (restartAppBtn) {
    restartAppBtn.classList.toggle("hidden", !view.pendingRestart);
  }
}

if (restartAppBtn) {
  restartAppBtn.addEventListener("click", () => {
    void requestAppRestart({
      reason:
        "重启后将按已保存的隔离后端与容器引擎偏好重新装配 Runtime。「本项目」记住的主机会保留，再次开始或新建会话时回灌。",
    });
  });
}

function updatePermissionBadge() {
  const el = document.getElementById("perm-pending-badge");
  if (!el) return;
  const on = !!ctx.state.permissionPending;
  el.classList.toggle("hidden", !on);
  el.textContent = on ? "待扩权" : "";
}

function showTutorialPanel() {
  const modal = document.getElementById("modal-root");
  if (!modal) return;
  const prog = loadTutorialProgress();
  const steps = tutorialSteps();
  const current = !prog.completed && !prog.skipped ? steps[prog.step] || null : null;
  const lis = steps
    .map((s, i) => {
      const done = prog.completed || prog.step > i;
      const on = !prog.completed && !prog.skipped && prog.step === i;
      const mark = done ? "✓" : on ? "●" : "○";
      const cls = done ? "tut-step done" : on ? "tut-step on" : "tut-step";
      return `<li class="${cls}"><span class="tut-mark">${mark}</span><span>${s.label}${s.optional ? " <span class=\"meta\">可选</span>" : ""}</span></li>`;
    })
    .join("");
  const status = prog.completed
    ? "已完成上手步骤。细节可随时打开「使用说明」。"
    : prog.skipped
      ? "已跳过；可重新开始。"
      : `进度 ${Math.min(prog.step, steps.length)}/${steps.length}`;
  const focus = current
    ? `<div class="tut-focus">
        <h3>${current.label}</h3>
        <div class="meta tut-hint">${current.hint}</div>
        <div class="actions">
          <button type="button" class="btn primary" id="tut-go-step">${current.goLabel || "前往"}</button>
          ${
            current.secondaryRoute
              ? `<button type="button" class="btn" id="tut-go-secondary">${current.secondaryLabel || "前往"}</button>`
              : ""
          }
          ${current.optional ? `<button type="button" class="btn" id="tut-skip-optional">跳过本步</button>` : ""}
        </div>
      </div>`
    : prog.completed
      ? `<p class="meta">主路径已走通。扩权弹窗、Docker 引擎等见「使用说明」。</p>`
      : "";
  modal.classList.remove("hidden");
  modal.innerHTML = `
    <div class="modal-card tutorial-card" role="dialog" aria-modal="true" aria-labelledby="tut-title">
      <div class="row page-head-spread">
        <h2 id="tut-title" style="margin:0">使用教程</h2>
        <button type="button" class="btn" id="tut-close">关闭</button>
      </div>
      <p class="meta">${status}</p>
      <ol class="tut-list">${lis}</ol>
      ${focus}
      <div class="actions tut-footer">
        <button type="button" class="btn" id="tut-guide">使用说明</button>
        <button type="button" class="btn" id="tut-skip">稍后体验</button>
        <button type="button" class="btn" id="tut-reset">重新学习</button>
        <button type="button" class="btn" id="tut-home">HOME</button>
      </div>
    </div>`;
  const close = () => {
    modal.classList.add("hidden");
    modal.innerHTML = "";
  };
  modal.querySelector("#tut-close")?.addEventListener("click", close);
  modal.querySelector("#tut-skip")?.addEventListener("click", () => {
    skipTutorial();
    close();
    showToast("已跳过教程", "ok");
  });
  modal.querySelector("#tut-reset")?.addEventListener("click", () => {
    resetTutorial();
    close();
    showTutorialPanel();
  });
  modal.querySelector("#tut-home")?.addEventListener("click", () => {
    close();
    navigate("home");
  });
  modal.querySelector("#tut-guide")?.addEventListener("click", () => {
    close();
    showUsageGuide();
  });
  const goRoute = (route) => {
    close();
    if (route === "wizard") navigate("wizard", { fresh: true });
    else if (route === "session") {
      if (ctx.state.sessionId) navigate("session");
      else {
        showToast("请先开一条运行中的会话，再在工作台右侧点「请求扩权」", "ok", {
          durationMs: 5000,
        });
        navigate("home");
      }
    } else navigate(route || "home");
  };
  modal.querySelector("#tut-go-step")?.addEventListener("click", () => {
    if (current?.action === "complete") {
      markTutorialStep(prog.step);
      close();
      showTutorialPanel();
      return;
    }
    goRoute(current?.route);
  });
  modal.querySelector("#tut-go-secondary")?.addEventListener("click", () => {
    goRoute(current?.secondaryRoute);
  });
  modal.querySelector("#tut-skip-optional")?.addEventListener("click", () => {
    skipOptionalTutorialStep();
    close();
    showTutorialPanel();
  });
}

document.getElementById("tutorial-btn")?.addEventListener("click", () => {
  showTutorialPanel();
});

function isWailsShell() {
  if (
    location.protocol === "wails:" ||
    location.hostname === "wails.localhost"
  ) {
    return true;
  }
  return typeof window !== "undefined" && !!window._wails;
}

function setSidebarCollapsed(collapsed, { persist = true } = {}) {
  const on = !!collapsed;
  sidebar.classList.toggle("collapsed", on);
  shell?.classList.toggle("sidebar-collapsed", on);
  const btn = document.getElementById("sidebar-toggle");
  if (btn) {
    btn.textContent = on ? "»" : "«";
    btn.title = on ? "展开侧栏" : "收缩侧栏";
  }
  try {
    localStorage.setItem("harbor-sidebar-collapsed", on ? "1" : "0");
  } catch {}
  if (persist) {
    ctx.state.uiPrefs = {
      ...(ctx.state.uiPrefs || {}),
      sidebarCollapsed: on,
    };
    api.setUIPrefs(ctx.state.uiPrefs).catch(() => {});
  }
}

/** @type {MediaQueryList | null} */
let systemThemeMql = null;

function normalizeThemePref(theme) {
  if (theme === "light" || theme === "dark" || theme === "system") return theme;
  return "system";
}

function resolveTheme(themePref) {
  const pref = normalizeThemePref(themePref);
  if (pref === "light" || pref === "dark") return pref;
  try {
    if (typeof matchMedia === "function") {
      return matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    }
  } catch (_) {
    /* ignore */
  }
  return "dark";
}

function applyDocumentTheme(themePref) {
  const pref = normalizeThemePref(themePref);
  const resolved = resolveTheme(pref);
  const root = document.documentElement;
  root.dataset.theme = resolved;
  root.classList.toggle("theme-light", resolved === "light");
  root.classList.toggle("theme-dark", resolved === "dark");
  if (systemThemeMql) {
    systemThemeMql.removeEventListener?.("change", onSystemThemeChange);
    systemThemeMql.removeListener?.(onSystemThemeChange);
    systemThemeMql = null;
  }
  if (pref === "system" && typeof matchMedia === "function") {
    systemThemeMql = matchMedia("(prefers-color-scheme: light)");
    if (systemThemeMql.addEventListener) {
      systemThemeMql.addEventListener("change", onSystemThemeChange);
    } else if (systemThemeMql.addListener) {
      systemThemeMql.addListener(onSystemThemeChange);
    }
  }
}

function onSystemThemeChange() {
  if (normalizeThemePref(ctx.state.uiPrefs?.theme) === "system") {
    applyDocumentTheme("system");
  }
}

function applyUIPrefs(prefs) {
  if (!prefs) return;
  const engine = normalizeEngineSource(prefs.engineSource);
  // 首次读入偏好时锁定本进程引擎底座；之后仅换偏好，重启后才更新。
  if (!ctx.state.engineSourceAtLaunch) {
    ctx.state.engineSourceAtLaunch = engine;
  }
  ctx.state.uiPrefs = {
    ...(ctx.state.uiPrefs || {}),
    sidebarCollapsed: !!prefs.sidebarCollapsed,
    autoCollapseWorkbench: prefs.autoCollapseWorkbench !== false,
    askClearOnStop: prefs.askClearOnStop !== false,
    clearOnStopDefault:
      prefs.clearOnStopDefault === "clear" ? "clear" : "keep",
    theme: normalizeThemePref(prefs.theme),
    securityTier: normalizeTier(prefs.securityTier),
    isolationBackend: normalizeIsolationBackend(prefs.isolationBackend),
    engineSource: engine,
  };
  applyDocumentTheme(ctx.state.uiPrefs.theme);
  syncIsolationBadge();
  // 工作台内若开了自动收缩且用户未手动展开，保持收窄，不把「默认展开」盖掉临时态
  if (
    currentRoute === "session" &&
    ctx.state.uiPrefs.autoCollapseWorkbench &&
    !workbenchStayExpanded
  ) {
    setSidebarCollapsed(true, { persist: false });
    return;
  }
  setSidebarCollapsed(!!prefs.sidebarCollapsed, { persist: false });
}

function navigate(name, opts = {}) {
  const prev = currentRoute;
  const leavingSession = prev === "session" && name !== "session";
  const enteringSession = name === "session" && prev !== "session";
  currentRoute = name;
  Object.entries(views).forEach(([key, el]) => {
    el.classList.toggle("hidden", key !== name);
  });
  document.querySelectorAll(".nav-item").forEach((btn) => {
    btn.classList.toggle("on", btn.getAttribute("data-route") === name);
  });
  if (leavingSession) {
    workbenchStayExpanded = false;
    setSidebarCollapsed(!!ctx.state.uiPrefs?.sidebarCollapsed, { persist: false });
  }
  if (name === "home") {
    ctx.refreshHome();
  }
  if (name === "wizard") wizardView = mountWizard(views.wizard, ctx, opts);
  if (name === "models") mountModels(views.models, ctx, opts);
  if (name === "tools") mountTools(views.tools, ctx, opts);
  if (name === "records") mountRecords(views.records, ctx, opts);
  if (name === "settings") mountSettings(views.settings, ctx, opts);
  if (name === "session") {
    if (!ctx.state.sessionId) {
      sessionView?.showEmpty?.() || sessionView?.resetForNewSession?.(null);
    } else if (enteringSession && ctx.state.lastSnapshot) {
      sessionView?.showSnapshot?.(ctx.state.lastSnapshot);
    }
    // 进入工作台：按「进入工作台时自动收缩」临时收窄（不写死默认偏好）
    const auto =
      opts.autoCollapse !== false &&
      ctx.state.uiPrefs?.autoCollapseWorkbench !== false;
    if (enteringSession && auto && !workbenchStayExpanded) {
      setSidebarCollapsed(true, { persist: false });
    }
  }
  if (ctx.state.health && !healthAllowsStart(ctx.state.health) && name === "wizard") {
    showBlock(healthBlockReason(ctx.state.health));
  } else {
    hideBlock();
  }
}

function applyHealthToShell(health) {
  ctx.state.health = health;
  const ok = healthAllowsStart(health);
  healthDot.className = `health-dot ${ok ? "ok" : "bad"}`;
  healthText.textContent = ok
    ? "隔离就绪"
    : health?.provisional
      ? "隔离未验证（占位）"
      : "隔离不可用";
  if (!ok) healthText.title = healthBlockReason(health);
  else healthText.title = "";
  if (ok) markTutorialStep(0);
  const healthEl = document.getElementById("health-light");
  healthEl?.classList.toggle("clickable", !ok);
  syncIsolationBadge();
  if (currentRoute === "home") mountHome(views.home, ctx);
  return health;
}

async function refreshHealth() {
  const health = await api.getRuntimeHealth();
  return applyHealthToShell(health);
}

function preferredIsolationLevel() {
  return normalizeIsolationBackend(ctx.state.uiPrefs?.isolationBackend);
}

function showBlock(reason) {
  const health = ctx.state.health;
  blockRoot.classList.remove("hidden");
  mountBlock(blockRoot, {
    reason: reason || health?.reason || "ISOLATION_UNAVAILABLE",
    health: health || { available: false, reason },
    preferredLevel: preferredIsolationLevel(),
    getHealth: async () => {
      const h = await api.getRuntimeHealth();
      applyHealthToShell(h);
      return h;
    },
    onAvailable: () => {
      hideBlock();
      if (currentRoute === "wizard") navigate("wizard");
      else navigate(currentRoute || "home");
    },
    onSettings: (opts = {}) => {
      hideBlock();
      navigate("settings", { focusIsolation: opts.focusIsolation !== false });
    },
  });
}

function hideBlock() {
  blockRoot.classList.add("hidden");
  blockRoot.innerHTML = "";
}

function maybeOnboarding(_health) {
  let done = false;
  try {
    done = localStorage.getItem("harbor-onboarded") === "1";
  } catch {}
  if (done) return false;
  shell.classList.add("hidden");
  onboardRoot.classList.remove("hidden");
  mountOnboarding(onboardRoot, {
    preferredLevel: preferredIsolationLevel(),
    getHealth: async () => {
      const h = await api.getRuntimeHealth();
      applyHealthToShell(h);
      return h;
    },
    onHealth: (h) => applyHealthToShell(h),
    onEnter() {
      try {
        localStorage.setItem("harbor-onboarded", "1");
      } catch {}
      onboardRoot.classList.add("hidden");
      shell.classList.remove("hidden");
      navigate("home");
    },
    onSettings(opts = {}) {
      // 失败也不能卡死在引导：进设置改引擎；标记已引导以免反复遮罩
      try {
        localStorage.setItem("harbor-onboarded", "1");
      } catch {}
      onboardRoot.classList.add("hidden");
      shell.classList.remove("hidden");
      navigate("settings", { focusIsolation: opts.focusIsolation !== false });
    },
  });
  return true;
}

async function boot() {
  await initializeTransport(api, {
    shell: isWailsShell(),
    search: location.search,
  });
  syncBadge();
  applyDocumentTheme(ctx.state.uiPrefs?.theme || "system");

  try {
    const prefsRes = await api.getUIPrefs();
    if (prefsRes?.prefs) applyUIPrefs(prefsRes.prefs);
    else if (localStorage.getItem("harbor-sidebar-collapsed") === "1") {
      setSidebarCollapsed(true, { persist: false });
    }
  } catch {
    try {
      if (localStorage.getItem("harbor-sidebar-collapsed") === "1") {
        setSidebarCollapsed(true, { persist: false });
      }
    } catch {}
  }

  document.getElementById("sidebar-toggle").onclick = () => {
    const next = !sidebar.classList.contains("collapsed");
    setSidebarCollapsed(next);
    if (currentRoute === "session") {
      // 工作台内手动展开：本次停留不再自动收；手动收窄可再被设置页偏好覆盖
      workbenchStayExpanded = !next;
    }
  };

  document.querySelectorAll(".nav-item").forEach((btn) => {
    btn.addEventListener("click", () =>
      navigate(btn.getAttribute("data-route")),
    );
  });

  document.getElementById("health-light")?.addEventListener("click", () => {
    if (!healthAllowsStart(ctx.state.health)) showBlock(healthBlockReason(ctx.state.health));
  });

  sessionSwitch.onchange = () => {
    if (sessionSwitch.value) ctx.openSession(sessionSwitch.value);
  };

  window.addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "b") {
      e.preventDefault();
      document.getElementById("sidebar-toggle").click();
    }
  });

  sessionView = mountSession(views.session, ctx);
  api.onEvent((ev) => {
    if (ev.type === "state_changed" && ev.state) {
      upsertHistory(ev.state);
      syncSessionSwitch();
    }
  });

  const health = await refreshHealth();
  const params = new URLSearchParams(location.search);
  const routeParam = params.get("route");
  const allowedRoutes = new Set([
    "home",
    "models",
    "tools",
    "records",
    "settings",
    "wizard",
  ]);
  // 样张：?shotSeed=1 写入本机用量/单价演示数据（覆盖 localStorage）
  if (params.get("shotSeed") === "1") {
    seedShotUsageForScreenshots();
  }

  // ?route= 深链（截图/复验）跳过引导，直接进目标页
  // records 可带 tab / usageTab / meterTab / meterRange（样张）
  if (routeParam && allowedRoutes.has(routeParam)) {
    shell.classList.remove("hidden");
    onboardRoot?.classList.add("hidden");
    if (routeParam === "records") {
      navigate("records", {
        tab: params.get("tab") || undefined,
        usageTab: params.get("usageTab") || undefined,
        meterTab: params.get("meterTab") || undefined,
        meterRange: params.get("meterRange") || undefined,
      });
    } else navigate(routeParam);
  } else if (!maybeOnboarding(health)) {
    navigate(ctx.state.sessionId ? "session" : "home");
  }

  let recovering = false;
  const recover = async () => {
    if (recovering) return;
    recovering = true;
    try {
      await refreshHealth();
      if (wizardView?.refreshAgents) await wizardView.refreshAgents();
      const id = ctx.state.sessionId;
      if (id) {
        const res = await api.getSession(id);
        if (res.session) upsertHistory(res.session);
      }
      syncSessionSwitch();
    } finally {
      recovering = false;
    }
  };
  window.addEventListener("focus", recover);
  window.addEventListener("pageshow", recover);
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) recover();
  });

  try {
    const id = sessionStorage.getItem("harbor-session-id");
    if (id) {
      const response = await api.getSession(id);
      if (response.session && !ctx.state.sessionId) {
        ctx.onSessionCreated(response.session);
      } else if (response.error?.code === "SESSION_NOT_FOUND") {
        sessionStorage.removeItem("harbor-session-id");
      }
    }
  } catch {}

  window.addEventListener("pagehide", () => sessionView.destroy(), {
    once: true,
  });
}

boot().catch(() => {
  badge.textContent = "连接失败";
  badge.className = "badge bad";
  shell.classList.remove("hidden");
  views.home.classList.remove("hidden");
  views.home.textContent = "桌面连接初始化失败，请重新启动应用。";
});
