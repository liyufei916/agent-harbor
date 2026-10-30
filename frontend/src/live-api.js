// Wails v3 live transport。生成绑定来自 frontend/bindings，不要手改生成文件。
// 不写扩展名，以同时兼容 wails3 的 JS 与 TS 绑定输出模式。
import * as api from "../bindings/github.com/liyufei916/agent-harbor/internal/desktop/desktopapi";
import { Events } from "@wailsio/runtime";

function unwrapEvent(ev) {
  if (ev && Object.prototype.hasOwnProperty.call(ev, "data")) {
    return ev.data;
  }
  return ev;
}

function missing(name) {
  return () =>
    Promise.reject(new Error(`Wails 绑定缺少 ${name}，请重新 generate`));
}

function bind(name) {
  const fn = api[name];
  if (typeof fn === "function") return (...args) => fn(...args);
  return missing(name);
}

export function createLiveTransport() {
  if (!api.ListAgents || !Events || typeof Events.On !== "function") {
    throw new Error("Wails 绑定不完整");
  }
  return {
    listAgents: () => api.ListAgents(),
    selectWorkspace: () => api.SelectWorkspace(),
    connectModel: (input) => api.ConnectModel(input),
    connectMcp: (input) => api.ConnectMcp(input),
    createSession: (spec) => api.CreateSession(spec),
    getSession: (id) => api.GetSession(id),
    stopSession: (id) => api.StopSession(id),
    startSession: bind("StartSession"),
    listAccounts: bind("ListAccounts"),
    removeAccount: bind("RemoveAccount"),
    listSessions: bind("ListSessions"),
    destroySession: bind("DestroySession"),
    syncBackWorkspace: bind("SyncBackWorkspace"),
    getRuntimeHealth: bind("GetRuntimeHealth"),
    getPermissionOverview: bind("GetPermissionOverview"),
    listRememberedHosts: bind("ListRememberedHosts"),
    revokeRememberedHost: bind("RevokeRememberedHost"),
    resolvePermission: bind("ResolvePermission"),
    promptPermission: bind("PromptPermission"),
    getUIPrefs: bind("GetUIPrefs"),
    setUIPrefs: bind("SetUIPrefs"),
    switchModel: bind("SwitchModel"),
    startMcpOAuth: bind("StartMcpOAuth"),
    cancelMcpOAuth: bind("CancelMcpOAuth"),
    writePty: bind("WritePty"),
    resizePty: bind("ResizePty"),
    saveTextFile: bind("SaveTextFile"),
    onEvent(callback) {
      return Events.On("harbor:event", (ev) => callback(unwrapEvent(ev)));
    },
  };
}
