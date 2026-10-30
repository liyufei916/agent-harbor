/**
 * 进舱预检（前端启发式 + 产品口径说明）。
 * 真隔离路径拒绝仍由 Runtime/Seatbelt 执行；此处帮助用户在开跑前看懂风险。
 */

const SECRET_NAME_RE =
  /(^|\/)(\.env|\.env\..+|\.npmrc|\.pypirc|credentials\.json|id_rsa|id_ed25519|\.aws\/credentials|secrets?\.(ya?ml|json|toml))$/i;

const INJECTION_HINT_RE =
  /(ignore previous|忽略以上|exfiltrat|curl .{0,40}http|wget .{0,40}http)/i;

/**
 * @param {string} workspacePath
 * @returns {{ level: "ok"|"warn"|"info", title: string, detail: string }[]}
 */
export function ingressFindings(workspacePath) {
  const path = String(workspacePath || "").trim();
  /** @type {{ level: "ok"|"warn"|"info", title: string, detail: string }[]} */
  const out = [];

  out.push({
    level: "info",
    title: "密钥与敏感路径",
    detail:
      "隔离默认不可见 SSH 密钥、云厂商凭证与系统目录；请勿把真实 Key 写进工作区明文文件。",
  });

  out.push({
    level: "info",
    title: "安全副本",
    detail:
      "默认 clone：Agent 改副本；导回前会做出舱审查。依赖目录建议在副本内重装，勿提交本机 node_modules。",
  });

  if (!path) {
    out.push({
      level: "warn",
      title: "未选工作区",
      detail: "请先选择项目目录。",
    });
    return out;
  }

  const base = path.split(/[/\\]/).filter(Boolean).pop() || path;
  if (SECRET_NAME_RE.test(path) || SECRET_NAME_RE.test(base)) {
    out.push({
      level: "warn",
      title: "路径名像凭据文件",
      detail: `「${base}」名称像密钥文件。请改选项目根目录，勿把单个 .env 当工作区。`,
    });
  }

  if (/\.git$/i.test(path)) {
    out.push({
      level: "warn",
      title: "工作区指向 .git",
      detail: "请选择仓库根目录，而不是 .git 本身。",
    });
  }

  out.push({
    level: "ok",
    title: "出站与扩权",
    detail: "陌生主机经 Gate 暂扣；三选一拒绝 / 仅本次 / 本项目。无全网放行。",
  });

  return out;
}

/**
 * 出舱高风险相对路径判定（导回审查用）。
 * @param {string} relPath
 * @returns {{ highRisk: boolean, reason: string }}
 */
export function classifySyncPath(relPath) {
  const p = String(relPath || "").replace(/\\/g, "/");
  if (/\.git\/hooks\//i.test(p) || /(^|\/)\.git\/hooks\//i.test(p)) {
    return { highRisk: true, reason: "Git hooks：提交时可能在宿主机执行" };
  }
  if (/(^|\/)\.github\/workflows\//i.test(p) || /(^|\/)\.gitlab-ci\.yml$/i.test(p)) {
    return { highRisk: true, reason: "CI 配置：可能影响流水线" };
  }
  if (/(^|\/)(pre|post)(install|publish)\.(js|sh|mjs)$/i.test(p)) {
    return { highRisk: true, reason: "包安装脚本：安装时可能执行" };
  }
  if (/\.(sh|bash|zsh|ps1|bat|cmd|exe|dylib|so)$/i.test(p)) {
    return { highRisk: true, reason: "可执行/脚本文件" };
  }
  if (SECRET_NAME_RE.test(p)) {
    return { highRisk: true, reason: "疑似凭据文件名" };
  }
  if (INJECTION_HINT_RE.test(p)) {
    return { highRisk: true, reason: "路径含可疑指令片段" };
  }
  return { highRisk: false, reason: "" };
}

/**
 * mock / 无 diff API 时的审查说明条目。
 * @returns {string[]}
 */
export function defaultExitReviewNotes() {
  return [
    "将合并写回原文件夹：同名覆盖，原目录多余文件不删。",
    "请特别检查：.git/hooks、CI 工作流、安装脚本、新增可执行文件。",
    "不确定的文件可先在副本目录核对内容，再导回。",
  ];
}
