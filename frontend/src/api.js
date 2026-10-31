/**
 * 页面对外 API（CONTRACT.md camelCase）
 * 默认 mock transport；Wails v3 live transport 延迟加载，不影响浏览器 mock。
 */

import * as mock from "./mock-api.js";
import { SessionEvents } from "./session-events.js";
const events = new SessionEvents();
let stopEvents;
function subscribeTransport() {
  stopEvents?.();
  stopEvents = transport.onEvent((event) => events.receive(event));
}

let transport = mock;
let mode = "mock"; // mock | live | http-live

function wrapTransportError() {
  return {
    code: "INTERNAL_ERROR",
    message: "桌面调用失败，请重试。",
    retryable: false,
  };
}

async function call(method, ...args) {
  try {
    if (typeof transport[method] !== "function") {
      if (method === "getRuntimeHealth") {
        // 缺绑定不得伪装 available：provisional 占位一律 fail-closed。
        return {
          available: false,
          reason: "桌面未绑定 GetRuntimeHealth，隔离未验证（占位不可用）",
          isolationLevel: "",
          provisional: true,
          error: {
            code: "UNIMPLEMENTED",
            message: "当前桌面未绑定 GetRuntimeHealth，请重新生成 Wails 绑定后重启。",
            retryable: false,
          },
        };
      }
      if (
        method === "getContainerEngineHealth" ||
        method === "ensureContainerEngine" ||
        method === "stopContainerEngine"
      ) {
        return {
          ready: false,
          source: "harbor-colima",
          message: "当前桌面未绑定容器引擎 API，请重新生成 Wails 绑定后重启。",
          dockerHost: "",
          provisional: true,
          error: {
            code: "UNIMPLEMENTED",
            message: "当前桌面未绑定容器引擎 API，请重新生成绑定并重启应用。",
            retryable: false,
          },
        };
      }
      if (method === "listSessions") {
        return { sessions: [], provisional: true, error: null };
      }
      if (method === "listAccounts") {
        return { accounts: [], provisional: true, error: null };
      }
      if (method === "listMcpPresets") {
        return { presets: [], provisional: true, error: null };
      }
      if (method === "addMcpPreset") {
        return {
          account: null,
          error: {
            code: "UNIMPLEMENTED",
            message: "当前桌面未绑定预置 MCP，请重新生成 Wails 绑定后重启。",
            retryable: false,
          },
          provisional: true,
        };
      }
      if (method === "restartApp") {
        return {
          restarting: false,
          error: {
            code: "UNIMPLEMENTED",
            message: "当前桌面未绑定 RestartApp，请重新生成绑定并重启应用。",
            retryable: false,
          },
          provisional: true,
        };
      }
      if (
        method === "getUIPrefs" ||
        method === "setUIPrefs" ||
        method === "revokeRememberedHost" ||
        method === "resolvePermission" ||
        method === "destroySession" ||
        method === "getPermissionOverview" ||
        method === "listRememberedHosts"
      ) {
        return { error: null, provisional: true };
      }
      if (method === "startSession") {
        return {
          session: null,
          error: {
            code: "UNIMPLEMENTED",
            message: "当前桌面未绑定 StartSession，请重新生成绑定并重启应用后再开启。",
            retryable: false,
          },
          provisional: true,
        };
      }
      if (method === "syncBackWorkspace" || method === "stopSession") {
        return {
          session: null,
          error: {
            code: "UNIMPLEMENTED",
            message:
              method === "syncBackWorkspace"
                ? "当前桌面未绑定 SyncBackWorkspace，请重新生成 Wails 绑定后再导回。"
                : "当前桌面未绑定 StopSession。",
            retryable: false,
          },
          provisional: true,
        };
      }
      if (method === "removeAccount") {
        return {
          error: {
            code: "UNIMPLEMENTED",
            message: "当前通道未绑定 RemoveAccount，无法删除账号。",
            retryable: false,
          },
          provisional: true,
        };
      }
      throw new Error(`transport missing ${method}`);
    }
    const result = await transport[method](...args);
    return result;
  } catch (e) {
    if (method === "listAgents") {
      return { agents: [], error: wrapTransportError(e) };
    }
    if (method === "getRuntimeHealth") {
      return {
        available: false,
        reason: wrapTransportError(e).message,
        isolationLevel: "",
        error: wrapTransportError(e),
      };
    }
    if (
      method === "getContainerEngineHealth" ||
      method === "ensureContainerEngine" ||
      method === "stopContainerEngine"
    ) {
      return {
        ready: false,
        source: "",
        message: wrapTransportError(e).message,
        dockerHost: "",
        error: wrapTransportError(e),
      };
    }
    if (method === "listSessions") {
      return { sessions: [], error: wrapTransportError(e) };
    }
    if (method === "listAccounts") {
      return { accounts: [], error: wrapTransportError(e) };
    }
    if (method === "selectWorkspace") {
      return { path: "", cancelled: false, error: wrapTransportError(e) };
    }
    if (method === "connectModel" || method === "connectMcp") {
      return { account: null, error: wrapTransportError(e) };
    }
    if (
      method === "removeAccount" ||
      method === "revokeRememberedHost" ||
      method === "resolvePermission" ||
      method === "promptPermission" ||
      method === "saveTextFile" ||
      method === "getUIPrefs" ||
      method === "setUIPrefs"
    ) {
      if (method === "saveTextFile") {
        return { path: "", cancelled: false, error: wrapTransportError(e) };
      }
      return { error: wrapTransportError(e) };
    }
    return { session: null, error: wrapTransportError(e) };
  }
}

/**
 * 真实桌面通道。绑定由 wails3 generate 生成，不要手改。
 * 事件名固定为 harbor:event，回调收到契约 Event 本身。
 */
export async function createLiveTransport() {
  const { createLiveTransport: create } = await import("./live-api.js");
  return create();
}

export async function createHttpLiveTransport() {
  const { createHttpLiveTransport: create } = await import("./http-live-api.js");
  return create();
}

export function useMock() {
  transport = mock;
  mode = "mock";
  subscribeTransport();
}

export async function useLive() {
  transport = await createLiveTransport();
  mode = "live";
  subscribeTransport();
}

export async function useHttpLive() {
  transport = await createHttpLiveTransport();
  mode = "http-live";
  subscribeTransport();
}

export function getMode() {
  return mode;
}

export function isMock() {
  return mode === "mock";
}

export function isHttpLive() {
  return mode === "http-live";
}

export function setMockScenario(name) {
  if (typeof mock.setScenario === "function") mock.setScenario(name);
}

export function getMockScenario() {
  return typeof mock.getScenario === "function" ? mock.getScenario() : "normal";
}

export function listAgents() {
  return call("listAgents");
}
export function getRuntimeHealth() {
  return call("getRuntimeHealth");
}
export function getContainerEngineHealth() {
  return call("getContainerEngineHealth");
}
export function ensureContainerEngine() {
  return call("ensureContainerEngine");
}
export function stopContainerEngine() {
  return call("stopContainerEngine");
}
export function restartApp(input) {
  return call("restartApp", input || {});
}
export function onEngineProgress(callback) {
  if (typeof transport.onEngineProgress === "function") {
    return transport.onEngineProgress(callback);
  }
  return () => {};
}
export function listSessions() {
  return call("listSessions");
}
export function destroySession(sessionId) {
  return call("destroySession", sessionId);
}
export function syncBackWorkspace(sessionId) {
  return call("syncBackWorkspace", sessionId);
}
export function listAccounts(kind) {
  return call("listAccounts", kind || "");
}
export function removeAccount(accountId) {
  return call("removeAccount", accountId);
}
export function getPermissionOverview(sessionId) {
  return call("getPermissionOverview", sessionId || "");
}
export function listRememberedHosts(workspace) {
  return call("listRememberedHosts", workspace || "");
}
export function revokeRememberedHost(input) {
  return call("revokeRememberedHost", input);
}
export function resolvePermission(input) {
  return call("resolvePermission", input);
}
export function promptPermission(input) {
  return call("promptPermission", input);
}
export function saveTextFile(input) {
  return call("saveTextFile", input);
}
export function getUIPrefs() {
  return call("getUIPrefs");
}
export function setUIPrefs(prefs) {
  return call("setUIPrefs", prefs);
}
export function switchModel(input) {
  return call("switchModel", input);
}
export function startMcpOAuth(input) {
  return call("startMcpOAuth", input);
}
export function cancelMcpOAuth(pendingId) {
  return call("cancelMcpOAuth", pendingId);
}
export function writePty(input) {
  return call("writePty", input);
}
export function resizePty(input) {
  return call("resizePty", input);
}
export function selectWorkspace() {
  return call("selectWorkspace");
}
export function connectModel(input) {
  return call("connectModel", input);
}
export function connectMcp(input) {
  return call("connectMcp", input);
}
export function listMcpPresets() {
  return call("listMcpPresets");
}
export function addMcpPreset(input) {
  return call("addMcpPreset", input);
}
export async function createSession(spec) {
  if (!events.beginCreate())
    return {
      session: null,
      error: {
        code: "SESSION_BUSY",
        message: "会话创建正在进行中。",
        retryable: false,
      },
    };
  const response = await call("createSession", spec);
  if (response.error || !response.session) events.cancelCreate();
  return response;
}
export function activateSession(snapshot) {
  events.activate(snapshot);
}
export async function getSession(sessionId) {
  const response = await call("getSession", sessionId);
  if (response.session) events.applySnapshot(response.session);
  return response;
}
export async function startSession(sessionId) {
  if (!events.beginCreate())
    return {
      session: null,
      error: {
        code: "SESSION_BUSY",
        message: "会话创建正在进行中。",
        retryable: false,
      },
    };
  const response = await call("startSession", sessionId);
  if (response.error || !response.session) events.cancelCreate();
  return response;
}
export async function stopSession(sessionId) {
  const response = await call("stopSession", sessionId);
  if (response.session) events.applySnapshot(response.session);
  return response;
}

/** @returns {() => void} 取消订阅 */
export function onEvent(callback) {
  if (!stopEvents) subscribeTransport();
  return events.subscribe(callback);
}
