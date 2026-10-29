/**
 * PAGE-DESIGN 全量壳：顶栏会话切换 + 侧栏 HOME/模型/工具/设置
 */
import "./xterm-boot.js";
import * as api from "./api.js";
import { initializeTransport } from "./startup.js";
import { mountHome } from "./views/home.js";
import { mountWizard } from "./views/wizard.js";
import { mountModels } from "./views/models.js";
import { mountTools } from "./views/tools.js";
import { mountSettings } from "./views/settings.js";
import { mountSession } from "./views/session.js";
import { mountOnboarding } from "./views/onboarding.js";
import { mountBlock } from "./views/block.js";
import { showToast } from "./toast.js";
import { showUsageGuide } from "./usage-guide.js";
import {
  mergeHistoryRow,
  buildRestartCreateSpec,
} from "./session-history.js";

const badge = document.getElementById("mode-badge");
const usageGuideBtn = document.getElementById("usage-guide-btn");
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
    },
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
  /** 用已停止会话的配置再 Create 一次，直接进工作台（I10）。 */
  async restartSession(sessionId) {
    const prev = (ctx.state.sessionHistory || []).find(
      (s) => s.sessionId === sessionId,
    );
    if (!prev) {
      showToast("找不到会话记录", "bad");
      return { error: { code: "SESSION_NOT_FOUND", message: "无历史" } };
    }
    if (ctx.state.health?.available === false) {
      showBlock(ctx.state.health.reason);
      return { error: { code: "ISOLATION_UNAVAILABLE", message: "隔离不可用" } };
    }
    const spec = buildRestartCreateSpec(prev, {
      modelAccountId: ctx.state.modelAccount?.id || "",
      lastTask: ctx.state.lastTask || "",
    });
    if (!spec) {
      ctx.state.wizardResume = {
        step: 0,
        draft: {
          workspace: prev.projectWorkspace || prev.workspace || "",
          mountMode: prev.mountMode || "clone",
          agentId: prev.agentId || "claude",
          modelAccountId: prev.modelAccountId || ctx.state.modelAccount?.id || "",
          mcpIds: [...(prev.mcpIds || [])],
          task: prev.task || ctx.state.lastTask || "",
        },
      };
      showToast("缺少模型或工作区，请在向导中确认后开始", "bad");
      navigate("wizard");
      return { error: { code: "INVALID_ARGUMENT", message: "配置不完整" } };
    }
    const res = await api.createSession(spec);
    if (res.error) {
      showToast(`${res.error.code}: ${res.error.message}`, "bad");
      if (
        res.error.code === "WORKSPACE_BUSY" ||
        res.error.code === "SESSION_LIMIT"
      ) {
        navigate("home");
      } else {
        ctx.state.wizardResume = {
          step: 3,
          draft: {
            workspace: spec.workspace,
            mountMode: spec.mountMode,
            agentId: spec.agentId,
            modelAccountId: spec.modelAccountId,
            mcpIds: [...spec.mcpIds],
            task: spec.task,
          },
        };
        navigate("wizard");
      }
      return res;
    }
    ctx.onSessionCreated(res.session, spec);
    showToast("已用上次配置重新开始", "ok");
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
    sessionView.resetForNewSession(res.session);
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
  if (api.isMock()) {
    badge.textContent = "模拟模式";
    badge.className = "badge mock";
    if (banner) {
      banner.classList.remove("hidden");
      banner.textContent = "模拟模式：仅看 UI，不能记入 I1–I11 真实验收。";
    }
  } else {
    badge.textContent = "真实绑定";
    badge.className = "badge live";
    // 桌面真连不再展示开发期 UNIMPLEMENTED 横幅；说明见顶栏「使用说明」
    banner?.classList.add("hidden");
  }
}

if (usageGuideBtn) {
  usageGuideBtn.addEventListener("click", () => showUsageGuide());
}

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

function applyUIPrefs(prefs) {
  if (!prefs) return;
  ctx.state.uiPrefs = {
    ...(ctx.state.uiPrefs || {}),
    sidebarCollapsed: !!prefs.sidebarCollapsed,
    autoCollapseWorkbench: prefs.autoCollapseWorkbench !== false,
    askClearOnStop: prefs.askClearOnStop !== false,
    clearOnStopDefault:
      prefs.clearOnStopDefault === "clear" ? "clear" : "keep",
  };
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
  if (name === "settings") mountSettings(views.settings, ctx);
  if (name === "session") {
    if (!ctx.state.sessionId) {
      sessionView?.showEmpty?.() || sessionView?.resetForNewSession?.(null);
    }
    // 进入工作台：按「进入工作台时自动收缩」临时收窄（不写死默认偏好）
    const auto =
      opts.autoCollapse !== false &&
      ctx.state.uiPrefs?.autoCollapseWorkbench !== false;
    if (enteringSession && auto && !workbenchStayExpanded) {
      setSidebarCollapsed(true, { persist: false });
    }
  }
  if (ctx.state.health && ctx.state.health.available === false && name === "wizard") {
    showBlock(ctx.state.health.reason);
  } else {
    hideBlock();
  }
}

function applyHealthToShell(health) {
  ctx.state.health = health;
  const ok = !!health?.available;
  healthDot.className = `health-dot ${ok ? "ok" : "bad"}`;
  healthText.textContent = ok
    ? health.provisional
      ? "隔离就绪（占位）"
      : "隔离就绪"
    : "隔离不可用";
  if (!ok) healthText.title = health?.reason || "点击查看说明";
  else healthText.title = "";
  const healthEl = document.getElementById("health-light");
  healthEl?.classList.toggle("clickable", !ok);
  if (currentRoute === "home") mountHome(views.home, ctx);
  return health;
}

async function refreshHealth() {
  const health = await api.getRuntimeHealth();
  return applyHealthToShell(health);
}

function showBlock(reason) {
  const health = ctx.state.health;
  blockRoot.classList.remove("hidden");
  mountBlock(blockRoot, {
    reason: reason || health?.reason || "ISOLATION_UNAVAILABLE",
    health: health || { available: false, reason },
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
    onSettings: () => {
      hideBlock();
      navigate("settings");
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
  });
  return true;
}

async function boot() {
  await initializeTransport(api, {
    shell: isWailsShell(),
    search: location.search,
  });
  syncBadge();

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
    if (ctx.state.health?.available === false) showBlock(ctx.state.health.reason);
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
  if (!maybeOnboarding(health)) {
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
