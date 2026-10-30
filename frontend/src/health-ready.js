/**
 * 隔离健康是否允许开跑 / 进向导。
 * provisional=true（绑定占位）一律视为未验证，不得当「隔离就绪」。
 */

export function healthAllowsStart(health) {
  return !!health?.available && !health?.provisional;
}

export function healthBlockReason(health) {
  if (!health) return "ISOLATION_UNAVAILABLE";
  if (health.provisional) {
    return health.reason || "隔离未验证（桌面绑定占位），禁止开跑";
  }
  return health.reason || "ISOLATION_UNAVAILABLE";
}
