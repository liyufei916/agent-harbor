/**
 * 常驻教程进度（本机 localStorage，不含 Key）。
 */

const KEY = "harbor-tutorial-progress";

/**
 * @typedef {{
 *   version: number,
 *   step: number,
 *   completed: boolean,
 *   skipped: boolean,
 *   updatedAt: number,
 * }} TutorialProgress
 */

const STEPS = [
  { id: "health", label: "确认隔离就绪（侧栏绿灯）" },
  { id: "model", label: "在「模型」添加并测试账号" },
  { id: "session", label: "新建会话进入工作台" },
  { id: "chat", label: "发送一轮任务并看到回复" },
];

export function tutorialSteps() {
  return STEPS;
}

/** @returns {TutorialProgress} */
export function loadTutorialProgress() {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) {
      return { version: 1, step: 0, completed: false, skipped: false, updatedAt: 0 };
    }
    const p = JSON.parse(raw);
    return {
      version: 1,
      step: Math.min(Math.max(Number(p.step) || 0, 0), STEPS.length),
      completed: !!p.completed,
      skipped: !!p.skipped,
      updatedAt: Number(p.updatedAt) || 0,
    };
  } catch {
    return { version: 1, step: 0, completed: false, skipped: false, updatedAt: 0 };
  }
}

/** @param {Partial<TutorialProgress>} patch */
export function saveTutorialProgress(patch) {
  const cur = loadTutorialProgress();
  const next = {
    ...cur,
    ...patch,
    version: 1,
    updatedAt: Date.now(),
  };
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    /* ignore */
  }
  return next;
}

export function markTutorialStep(stepIndex) {
  const cur = loadTutorialProgress();
  if (cur.completed || cur.skipped) return cur;
  const step = Math.max(cur.step, stepIndex);
  const completed = step >= STEPS.length;
  return saveTutorialProgress({ step, completed });
}

export function skipTutorial() {
  return saveTutorialProgress({ skipped: true });
}

export function resetTutorial() {
  try {
    localStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
  return loadTutorialProgress();
}
