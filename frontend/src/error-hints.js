/**
 * 失败码 → 可操作下一步（不外传、不含 Key）。
 */

const HINTS = {
  UNAUTHENTICATED: "检查模型/MCP Key 是否过期，到「模型」或「工具」页重新连接并测试。",
  PERMISSION_DENIED: "本机钥匙串或文件权限异常；修复后重启应用。",
  INVALID_ARGUMENT: "核对会话仍在、目标主机合法，且未对已结束的扩权重复点选。",
  SESSION_NOT_FOUND: "会话已销毁或不存在；回 HOME 重新打开或新建。",
  SESSION_LIMIT: "同时运行中最多 5 个；先停止一条再开。",
  WORKSPACE_BUSY: "同工作区已有会话；打开已有会话或换目录。",
  WORKSPACE_DENIED: "工作区路径不可用或挂载被拒；改选目录或检查隔离。",
  ISOLATION_UNAVAILABLE: "隔离未就绪；到设置「重新检测隔离」，勿跳过。",
  UPSTREAM_TIMEOUT: "上游超时；稍后重试，或换网络/模型供应商。",
  RATE_LIMITED: "触发限流；等待几分钟或降低并发后再试。",
  NETWORK_ERROR: "网络不可达；检查本机联网与 Gate 白名单主机。",
  CANCELLED: "操作已取消。",
  INTERNAL_ERROR: "内部错误；查看最近活动后重试，仍失败则重启应用。",
  UNIMPLEMENTED: "当前运行模式未实现该方法（浏览器 mock 或旧后端）。",
};

/**
 * @param {string} code
 * @param {string} [message]
 * @returns {string}
 */
export function hintForError(code, message = "") {
  const c = String(code || "").trim().toUpperCase();
  const base = HINTS[c] || "对照错误码与设置/记录页排查；勿把 Key 贴到聊天。";
  const msg = String(message || "").trim();
  if (!msg) return base;
  return `${msg} — ${base}`;
}

/**
 * @param {{ code?: string, message?: string } | null | undefined} err
 * @returns {string}
 */
export function formatErrorWithHint(err) {
  if (!err) return "";
  const code = err.code || "";
  const message = err.message || "";
  const line = code ? `${code}: ${message}` : message;
  return hintForError(code, line);
}
