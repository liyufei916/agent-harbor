/**
 * 本机 http-live transport：浏览器 → loopback DesktopAPI（双终端之一）。
 * 非托管 SaaS；SelectWorkspace 为绝对路径；SaveTextFile 本地下载；RestartApp 不可用。
 */

function bridgeBase() {
  const params = new URLSearchParams(location.search);
  const fromQuery = (params.get("bridge") || "").replace(/\/+$/, "");
  if (fromQuery) return fromQuery;
  // 由 http-live-serve 同端口托管 UI 时，默认本源
  return location.origin.replace(/\/+$/, "");
}

function bridgeToken() {
  const params = new URLSearchParams(location.search);
  return params.get("token") || "";
}

async function rpc(method, params) {
  const token = bridgeToken();
  if (!token) {
    throw new Error("http-live 缺少 token（URL ?token=）");
  }
  const res = await fetch(`${bridgeBase()}/v1/rpc`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      method,
      params: params === undefined ? null : params,
    }),
  });
  if (respUnauthorized(res)) {
    throw new Error("http-live token 无效");
  }
  if (!res.ok) {
    throw new Error(`http-live RPC HTTP ${res.status}`);
  }
  return res.json();
}

function respUnauthorized(res) {
  return res.status === 401;
}

function openSSE(path, onData) {
  const token = encodeURIComponent(bridgeToken());
  const url = `${bridgeBase()}${path}?token=${token}`;
  const es = new EventSource(url);
  es.onmessage = (ev) => {
    try {
      onData(JSON.parse(ev.data));
    } catch {
      /* ignore */
    }
  };
  return () => {
    try {
      es.close();
    } catch {
      /* ignore */
    }
  };
}

function downloadTextFile(input = {}) {
  const name = (input.suggestedName || input.filename || "harbor-export.txt").replace(
    /[/\\]/g,
    "_",
  );
  const text = input.content ?? input.text ?? "";
  const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 2000);
  return { path: name, cancelled: false, error: null };
}

async function selectWorkspacePath() {
  const { pickHttpLiveWorkspace } = await import("./http-live-workspace.js");
  return pickHttpLiveWorkspace(rpc);
}

export function createHttpLiveTransport() {
  const call = (name, arg) => {
    if (arg === undefined) return rpc(name, null);
    return rpc(name, [arg]);
  };
  return {
    listAgents: () => call("ListAgents"),
    selectWorkspace: () => selectWorkspacePath(),
    connectModel: (input) => call("ConnectModel", input),
    connectMcp: (input) => call("ConnectMcp", input),
    listMcpPresets: () => call("ListMcpPresets"),
    addMcpPreset: (input) => call("AddMcpPreset", input),
    createSession: (spec) => call("CreateSession", spec),
    getSession: (id) => call("GetSession", id),
    stopSession: (id) => call("StopSession", id),
    startSession: (id) => call("StartSession", id),
    listAccounts: (kind) => call("ListAccounts", kind || ""),
    removeAccount: (id) => call("RemoveAccount", id),
    listSessions: () => call("ListSessions"),
    destroySession: (id) => call("DestroySession", id),
    syncBackWorkspace: (id) => call("SyncBackWorkspace", id),
    getRuntimeHealth: () => call("GetRuntimeHealth"),
    getContainerEngineHealth: () => call("GetContainerEngineHealth"),
    ensureContainerEngine: () => call("EnsureContainerEngine"),
    stopContainerEngine: () => call("StopContainerEngine"),
    getPermissionOverview: (id) => call("GetPermissionOverview", id || ""),
    listRememberedHosts: (ws) => call("ListRememberedHosts", ws || ""),
    revokeRememberedHost: (input) => call("RevokeRememberedHost", input),
    resolvePermission: (input) => call("ResolvePermission", input),
    promptPermission: (input) => call("PromptPermission", input),
    getUIPrefs: () => call("GetUIPrefs"),
    setUIPrefs: (prefs) => call("SetUIPrefs", prefs),
    switchModel: (input) => call("SwitchModel", input),
    startMcpOAuth: (input) => call("StartMcpOAuth", input),
    cancelMcpOAuth: (id) => call("CancelMcpOAuth", id),
    writePty: (input) => call("WritePty", input),
    resizePty: (input) => call("ResizePty", input),
    saveTextFile: (input) => Promise.resolve(downloadTextFile(input || {})),
    restartApp: (input) => call("RestartApp", input || {}),
    onEvent(callback) {
      return openSSE("/v1/events", callback);
    },
    onEngineProgress(callback) {
      return openSSE("/v1/engine-progress", callback);
    },
  };
}

export async function probeHttpLive() {
  try {
    const res = await fetch(`${bridgeBase()}/v1/health`, { method: "GET" });
    if (!res.ok) return false;
    const j = await res.json();
    return !!j?.ok;
  } catch {
    return false;
  }
}
