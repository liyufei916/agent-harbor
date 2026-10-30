/**
 * 常驻教程进度（本机 localStorage，不含 Key）。
 * step = 已完成步数（下一待做下标）；completed 当 step >= STEPS.length。
 */

const KEY = "harbor-tutorial-progress";
const VERSION = 2;

/**
 * @typedef {{
 *   version: number,
 *   step: number,
 *   completed: boolean,
 *   skipped: boolean,
 *   updatedAt: number,
 * }} TutorialProgress
 */

/**
 * @typedef {{
 *   id: string,
 *   label: string,
 *   hint: string,
 *   route?: string,
 *   goLabel?: string,
 *   action?: "route" | "complete",
 *   secondaryRoute?: string,
 *   secondaryLabel?: string,
 *   optional?: boolean,
 * }} TutorialStep
 */

/** @type {TutorialStep[]} */
const STEPS = [
  {
    id: "health",
    label: "确认隔离就绪",
    hint: "侧栏底部应为绿灯「隔离就绪」。红灯时看阻断页，没有「仍要继续」。设置里可切换 Seatbelt / Docker（改后须重启）。",
    route: "settings",
    goLabel: "去设置",
    action: "route",
  },
  {
    id: "model",
    label: "添加模型账号",
    hint: "在「模型」填写 Origin + Key（可勾选 Claude / Codex / dsh）。提交后 Key 不会留在输入框。",
    route: "models",
    goLabel: "去模型",
    action: "route",
  },
  {
    id: "tools",
    label: "添加工具（可选）",
    hint: "「工具」可勾选预置 MCP，或添加远程 HTTP / 宿主 stdio。可跳过，稍后再加。",
    route: "tools",
    goLabel: "去工具",
    action: "route",
    optional: true,
  },
  {
    id: "session",
    label: "新建隔离会话",
    hint: "HOME → 新会话：选工作区 → Agent → 模型。默认「安全副本」，改动在副本里，稍后可导回。",
    route: "wizard",
    goLabel: "新会话",
    action: "route",
  },
  {
    id: "chat",
    label: "发送一轮任务",
    hint: "工作台下方输入框发下一轮（点终端无效）。应看到过滤后的可读回复。",
    route: "session",
    goLabel: "去工作台",
    action: "route",
  },
  {
    id: "syncback",
    label: "导回原文件夹",
    hint: "clone 会话结束后，点「更多 → 导回原文件夹」把副本写回源目录；也可停止时选择先导回再清除。",
    route: "session",
    goLabel: "去工作台",
    action: "route",
  },
  {
    id: "permission",
    label: "了解扩权",
    hint:
      "Agent 访问<strong>未在白名单</strong>的主机时会弹窗，三选一：<br/>" +
      "• <strong>拒绝</strong> — 不放行<br/>" +
      "• <strong>仅本次</strong> — 只放行这一次连接<br/>" +
      "• <strong>本项目始终允许</strong> — 记在当前工作区；可在工作台「更多 → 权限一览」或设置「已记住的网站」撤销<br/>" +
      "<strong>没有「全网放行」</strong>。<br/><br/>" +
      "要真机体验：先有一条<strong>运行中</strong>会话，打开工作台右侧点「请求扩权」，会弹出上述三问。",
    goLabel: "我知道了",
    action: "complete",
    secondaryRoute: "session",
    secondaryLabel: "去工作台演示",
  },
];

export function tutorialSteps() {
  return STEPS;
}

/** @returns {TutorialProgress} */
export function loadTutorialProgress() {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) {
      return emptyProgress();
    }
    const p = JSON.parse(raw);
    if (Number(p.version) !== VERSION) {
      return emptyProgress();
    }
    return {
      version: VERSION,
      step: Math.min(Math.max(Number(p.step) || 0, 0), STEPS.length),
      completed: !!p.completed,
      skipped: !!p.skipped,
      updatedAt: Number(p.updatedAt) || 0,
    };
  } catch {
    return emptyProgress();
  }
}

function emptyProgress() {
  return {
    version: VERSION,
    step: 0,
    completed: false,
    skipped: false,
    updatedAt: 0,
  };
}

/** @param {Partial<TutorialProgress>} patch */
export function saveTutorialProgress(patch) {
  const cur = loadTutorialProgress();
  const next = {
    ...cur,
    ...patch,
    version: VERSION,
    updatedAt: Date.now(),
  };
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    /* ignore */
  }
  return next;
}

/**
 * 标记某一步已完成（0-based）。progress.step 变为 completedIndex+1。
 * @param {number} completedIndex
 */
export function markTutorialStep(completedIndex) {
  const cur = loadTutorialProgress();
  if (cur.completed || cur.skipped) return cur;
  const idx = Math.max(0, Number(completedIndex) || 0);
  const step = Math.max(cur.step, idx + 1);
  const completed = step >= STEPS.length;
  return saveTutorialProgress({ step, completed });
}

/** 跳过当前可选步（仅当当前步 optional） */
export function skipOptionalTutorialStep() {
  const cur = loadTutorialProgress();
  if (cur.completed || cur.skipped) return cur;
  const current = STEPS[cur.step];
  if (!current?.optional) return cur;
  return markTutorialStep(cur.step);
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
