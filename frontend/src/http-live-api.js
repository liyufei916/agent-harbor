/**
 * 本机 http-live transport：浏览器 → loopback DesktopAPI（双终端之一）。
 * 非托管 SaaS；SelectWorkspace 为绝对路径；SaveTextFile 本地下载；RestartApp 不可用。
 * Token 只走 Authorization；不在 EventSource URL 带 ?token=。
 */

export const HTTP_LIVE_TOKEN_KEY = "harbor-http-live-token";

function bridgeBase() {
  const params = new URLSearchParams(location.search);
  const fromQuery = (params.get("bridge") || "").replace(/\/+$/, "");
  if (fromQuery) return fromQuery;
  // 由 http-live-serve 同端口托管 UI 时，默认本源
  return location.origin.replace(/\/+$/, "");
}

/**
 * URL ?token= → 存储并剥离查询串（避免进 Referer/日志）。
 * @returns {{ token: string, search: string, stripped: boolean }}
 */
export function ingestHttpLiveTokenFromSearch(search, storedToken, setStored) {
  if (storedToken) {
    return { token: storedToken, search, stripped: false };
  }
  const raw = search.startsWith("?") ? search.slice(1) : search;
  const params = new URLSearchParams(raw);
  const fromQuery = params.get("token") || "";
  if (!fromQuery) return { token: "", search, stripped: false };
  if (typeof setStored === "function") setStored(fromQuery);
  params.delete("token");
  const q = params.toString();
  return {
    token: fromQuery,
    search: q ? `?${q}` : "",
    stripped: true,
  };
}

/** 读 token：sessionStorage 优先；兼容一次性 URL 手递后立刻剥掉查询串。 */
function bridgeToken() {
  let stored = "";
  try {
    stored = sessionStorage.getItem(HTTP_LIVE_TOKEN_KEY) || "";
  } catch {
    /* ignore */
  }
  const got = ingestHttpLiveTokenFromSearch(location.search, stored, (tok) => {
    try {
      sessionStorage.setItem(HTTP_LIVE_TOKEN_KEY, tok);
    } catch {
      /* ignore */
    }
  });
  if (got.stripped) {
    try {
      history.replaceState(null, "", `${location.pathname}${got.search}${location.hash}`);
    } catch {
      /* ignore */
    }
  }
  return got.token;
}

/** 同端口 UI：向 loopback 桥领取 token（仅 127.0.0.1）。 */
async function claimDevToken() {
  if (bridgeToken()) return true;
  try {
    const res = await fetch(`${bridgeBase()}/v1/dev-token`, { method: "GET" });
    if (!res.ok) return false;
    const j = await res.json();
    const tok = String(j?.token || "").trim();
    if (!tok) return false;
    try {
      sessionStorage.setItem(HTTP_LIVE_TOKEN_KEY, tok);
    } catch {
      /* ignore */
    }
    return true;
  } catch {
    return false;
  }
}

async function ensureToken() {
  if (bridgeToken()) return true;
  return claimDevToken();
}

async function rpc(method, params) {
  if (!(await ensureToken())) {
    throw new Error("http-live 缺少 token（请用同端口 UI 或终端说明写入 sessionStorage）");
  }
  const token = bridgeToken();
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

/** fetch + Authorization 订阅 SSE（避免 EventSource 只能走 ?token=）。 */
function openSSE(path, onData) {
  const ac = new AbortController();
  let buf = "";
  (async () => {
    if (!(await ensureToken())) return;
    const token = bridgeToken();
    let res;
    try {
      res = await fetch(`${bridgeBase()}${path}`, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: "text/event-stream",
        },
        signal: ac.signal,
      });
    } catch {
      return;
    }
    if (!res.ok || !res.body) return;
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    while (!ac.signal.aborted) {
      let chunk;
      try {
        chunk = await reader.read();
      } catch {
        break;
      }
      if (chunk.done) break;
      buf += dec.decode(chunk.value, { stream: true });
      let idx;
      while ((idx = buf.indexOf("\n\n")) >= 0) {
        const block = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        for (const line of block.split("\n")) {
          if (!line.startsWith("data:")) continue;
          const raw = line.slice(5).trim();
          if (!raw) continue;
          try {
            onData(JSON.parse(raw));
          } catch {
            /* ignore */
          }
        }
      }
    }
  })();
  return () => {
    ac.abort();
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
