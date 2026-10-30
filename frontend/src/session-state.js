/** 会话 state → 中文展示（DOM class 仍用英文 state） */
export const SESSION_STATE_LABELS = {
  provisioning: "准备中",
  running: "运行中",
  stopping: "停止中",
  stopped: "已停止",
  failed: "失败",
};

export function sessionStateLabel(state) {
  if (!state) return "未知";
  return SESSION_STATE_LABELS[state] || state;
}

/** 路径缩短展示；完整路径放 title */
export function shortPath(path, max = 40) {
  const s = String(path || "");
  if (s.length <= max) return s;
  const parts = s.split("/").filter(Boolean);
  if (parts.length >= 3) {
    const tail = parts.slice(-2).join("/");
    const head = s.startsWith("/") ? "/" : "";
    const candidate = `${head}${parts[0]}/…/${tail}`;
    if (candidate.length <= max + 8) return candidate;
  }
  return `…${s.slice(-(max - 1))}`;
}
