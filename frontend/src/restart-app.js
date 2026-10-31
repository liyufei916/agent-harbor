/**
 * 一键重启应用（隔离后端切换等）。有进行中会话时先确认。
 */
import * as api from "./api.js";
import { askConfirm } from "./confirm-dialog.js";
import { showToast } from "./toast.js";

const BUSY = new Set(["running", "provisioning", "stopping"]);

export async function requestAppRestart({ reason = "" } = {}) {
  let busy = 0;
  try {
    const res = await api.listSessions();
    busy = (res?.sessions || []).filter((s) => BUSY.has(String(s.state || "").toLowerCase())).length;
  } catch {
    busy = 0;
  }
  const body = [
    reason ||
      "将退出并重新打开应用，使隔离后端等设置生效。「本项目」记住的主机会保留。",
    busy > 0 ? `\n有 ${busy} 个进行中的会话将被停止。` : "",
  ].join("");
  const ok = await askConfirm({
    title: "重启应用",
    body,
    confirmLabel: busy > 0 ? "停止并重启" : "重启应用",
    cancelLabel: "取消",
    danger: busy > 0,
  });
  if (!ok) return false;

  showToast("正在重启…", "ok");
  const res = await api.restartApp({ force: true });
  if (res?.error) {
    showToast(`${res.error.code}: ${res.error.message}`, "bad");
    return false;
  }
  if (api.isMock?.()) {
    showToast("模拟模式：请刷新页面", "ok");
    return true;
  }
  if (api.isHttpLive?.() && res?.restarting) {
    // 桥进程已拉起新实例；稍候刷新同 URL（token 已落盘，可复用）
    showToast("本机 live 正在重启，页面即将刷新…", "ok");
    setTimeout(() => {
      location.reload();
    }, 1200);
    return true;
  }
  return true;
}
