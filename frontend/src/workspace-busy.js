/**
 * 向导早检：同工作区已有未清理完的会话 → 对标 CreateSession WORKSPACE_BUSY。
 * 比较键优先 sourceWorkspace / projectWorkspace（用户所选源路径），不用 clone 副本。
 */

export function normalizeWorkspaceKey(path) {
  let s = String(path || "").trim();
  if (!s) return "";
  s = s.replace(/\\/g, "/").replace(/\/+/g, "/");
  if (s.length > 1 && s.endsWith("/")) s = s.slice(0, -1);
  return s;
}

/** @returns {object|null} 占用该工作区的会话摘要 */
export function findBusyWorkspaceSession(sessions, path) {
  const want = normalizeWorkspaceKey(path);
  if (!want) return null;
  for (const s of sessions || []) {
    if (!s || s.cleanupComplete) continue;
    const candidates = [s.projectWorkspace, s.sourceWorkspace];
    // direct：workspace ≈ 源；clone：workspace 是副本，仅当无 source/project 时才退回
    if (s.mountMode === "direct" || (!s.sourceWorkspace && !s.projectWorkspace)) {
      candidates.push(s.workspace);
    }
    for (const c of candidates) {
      if (c && normalizeWorkspaceKey(c) === want) return s;
    }
  }
  return null;
}
