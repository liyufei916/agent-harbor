/**
 * 本项目记住主机：工作台 / 权限一览 / 设置须用同一源路径与撤销广播。
 */

export const HOSTS_CHANGED = "harbor:remembered-hosts-changed";

/** 记住表键是用户所选源目录，不是 clone 副本路径。 */
export function projectWorkspaceOf(snap, ctx = null) {
  const hist = (ctx?.state?.sessionHistory || []).find(
    (s) => s?.sessionId && s.sessionId === (snap?.sessionId || ctx?.state?.sessionId),
  );
  return (
    snap?.sourceWorkspace ||
    snap?.projectWorkspace ||
    hist?.projectWorkspace ||
    hist?.sourceWorkspace ||
    ctx?.state?.workspace ||
    snap?.workspace ||
    ""
  );
}

export function notifyRememberedHostsChanged(detail = {}) {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent(HOSTS_CHANGED, { detail }));
}

export function onRememberedHostsChanged(handler) {
  if (typeof window === "undefined") return () => {};
  const fn = (e) => handler(e?.detail || {});
  window.addEventListener(HOSTS_CHANGED, fn);
  return () => window.removeEventListener(HOSTS_CHANGED, fn);
}

/** 统一撤销：落盘 + Gate（后端）后广播，三处 UI 一起刷新。 */
export async function revokeRememberedHostUnified(api, { host, workspace }) {
  const res = await api.revokeRememberedHost({ host, workspace: workspace || "" });
  notifyRememberedHostsChanged({ host, workspace: workspace || "" });
  return res;
}
