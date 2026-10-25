/**
 * PAGE-DESIGN 全量壳：顶栏会话切换 + 侧栏 HOME/模型/工具/设置
 */
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

const badge = document.getElementById("mode-badge");
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
  },
  navigate(name) {
    navigate(name);
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
  onSessionCreated(snapshot) {
    api.activateSession?.(snapshot);
    ctx.state.sessionId = snapshot.sessionId;
    ctx.state.lastSnapshot = snapshot;
    upsertHistory(snapshot);
    try {
      sessionStorage.setItem("harbor-session-id", snapshot.sessionId);
    } catch {}
    sessionView.resetForNewSession(snapshot);
    syncSessionSwitch();
    navigate("session", { autoCollapse: true });
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

function upsertHistory(snapshot) {
  if (!snapshot?.sessionId) return;
  const list = ctx.state.sessionHistory || [];
  const i = list.findIndex((s) => s.sessionId === snapshot.sessionId);
  const row = {
    sessionId: snapshot.sessionId,
    state: snapshot.state,
    workspace: snapshot.workspace,
    agentId: snapshot.agentId,
    cleanupComplete: snapshot.cleanupComplete,
    mountMode: snapshot.mountMode,
    mcpIds: snapshot.mcpIds,
  };
  if (i >= 0) list[i] = { ...list[i], ...row };
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
      banner.textContent =
        "feat/page-design-full · PAGE-DESIGN 全量（含 OAuth/clone/PTY）。mock 不能记入 I1–I11。";
    }
  } else {
    badge.textContent = "真实绑定";
    badge.className = "badge live";
    if (banner) {
      banner.textContent =
        "真实绑定：按 PAGE-DESIGN 全量；未接线方法可能返回 UNIMPLEMENTED。";
    }
  }
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

function setSidebarCollapsed(collapsed) {
  sidebar.classList.toggle("collapsed", collapsed);
  const btn = document.getElementById("sidebar-toggle");
  if (btn) {
    btn.textContent = collapsed ? "»" : "«";
    btn.title = collapsed ? "展开侧栏" : "收缩侧栏";
  }
  try {
    localStorage.setItem("harbor-sidebar-collapsed", collapsed ? "1" : "0");
  } catch {}
}

function navigate(name, opts = {}) {
  currentRoute = name;
  Object.entries(views).forEach(([key, el]) => {
    el.classList.toggle("hidden", key !== name);
  });
  document.querySelectorAll(".nav-item").forEach((btn) => {
    btn.classList.toggle("on", btn.getAttribute("data-route") === name);
  });
  if (name === "home") {
    ctx.refreshHome();
  }
  if (name === "wizard") wizardView = mountWizard(views.wizard, ctx);
  if (name === "models") mountModels(views.models, ctx);
  if (name === "tools") mountTools(views.tools, ctx);
  if (name === "settings") mountSettings(views.settings, ctx);
  if (name === "session" && opts.autoCollapse) {
    const pref = localStorage.getItem("harbor-sidebar-user-expanded");
    if (pref !== "1") setSidebarCollapsed(true);
  }
  if (ctx.state.health && ctx.state.health.available === false && name === "wizard") {
    showBlock(ctx.state.health.reason);
  } else {
    hideBlock();
  }
}

async function refreshHealth() {
  const health = await api.getRuntimeHealth();
  ctx.state.health = health;
  const ok = !!health.available;
  healthDot.className = `health-dot ${ok ? "ok" : "bad"}`;
  healthText.textContent = ok
    ? health.provisional
      ? "隔离就绪（占位）"
      : "隔离就绪"
    : "隔离不可用";
  if (!ok) healthText.title = health.reason || "";
  return health;
}

function showBlock(reason) {
  blockRoot.classList.remove("hidden");
  mountBlock(blockRoot, {
    reason,
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

function maybeOnboarding(health) {
  let done = false;
  try {
    done = localStorage.getItem("harbor-onboarded") === "1";
  } catch {}
  if (done) return false;
  shell.classList.add("hidden");
  onboardRoot.classList.remove("hidden");
  mountOnboarding(onboardRoot, {
    health,
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
    if (localStorage.getItem("harbor-sidebar-collapsed") === "1") {
      setSidebarCollapsed(true);
    }
  } catch {}

  document.getElementById("sidebar-toggle").onclick = () => {
    const next = !sidebar.classList.contains("collapsed");
    setSidebarCollapsed(next);
    try {
      localStorage.setItem("harbor-sidebar-user-expanded", next ? "0" : "1");
    } catch {}
  };

  document.querySelectorAll(".nav-item").forEach((btn) => {
    btn.addEventListener("click", () =>
      navigate(btn.getAttribute("data-route")),
    );
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
