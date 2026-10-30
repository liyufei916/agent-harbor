/**
 * 出舱审查：导回前确认（高风险路径提示）。
 */
import { askConfirm } from "../confirm-dialog.js";
import {
  classifySyncPath,
  defaultExitReviewNotes,
} from "../ingress-check.js";

/**
 * @param {HTMLElement | null} modalRoot
 * @param {{
 *   sourceWorkspace?: string,
 *   paths?: string[],
 * }} opts
 * @returns {Promise<boolean>}
 */
export async function askExitReview(modalRoot, opts = {}) {
  const dest = opts.sourceWorkspace || "（未知）";
  const paths = Array.isArray(opts.paths) ? opts.paths : [];
  const notes = defaultExitReviewNotes();

  let riskBlock = "";
  if (paths.length) {
    const rows = paths.slice(0, 80).map((p) => {
      const { highRisk, reason } = classifySyncPath(p);
      return highRisk
        ? `• ⚠ ${p} — ${reason}`
        : `• ${p}`;
    });
    const high = paths.filter((p) => classifySyncPath(p).highRisk).length;
    riskBlock = `\n变更文件（最多 80 条，高风险 ${high}）：\n${rows.join("\n")}`;
  } else {
    riskBlock = `\n（无文件级 diff 列表时）请自行在副本目录核对后再导回。\n注意：\n${notes.map((n) => `• ${n}`).join("\n")}`;
  }

  return askConfirm(
    {
      title: "出舱审查 · 导回原文件夹",
      body: `目标：${dest}\n\n同名文件将被覆盖；原目录多余文件不会删除。${riskBlock}\n\n确认将安全副本合并写回？`,
      confirmLabel: "确认导回",
      cancelLabel: "取消",
      danger: true,
    },
    modalRoot,
  );
}
