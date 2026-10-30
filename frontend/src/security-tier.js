/**
 * 安全档位：严格 / 标准 / 宽松 — 会话启动时种子 ExtraHosts（桌面）或 UI 说明（mock）。
 */

/** @typedef {"strict"|"standard"|"relaxed"} SecurityTier */

export const TIER_LABELS = {
  strict: "严格（仅模型上游）",
  standard: "标准（+常用包管理源）",
  relaxed: "宽松（+常用开发站）",
};

/** 标准档：包管理 / 镜像（不含模型 host，模型来自账号 Origin） */
export const TIER_STANDARD_HOSTS = [
  "registry.npmjs.org",
  "registry.yarnpkg.com",
  "pypi.org",
  "files.pythonhosted.org",
  "proxy.golang.org",
  "sum.golang.org",
  "crates.io",
  "static.crates.io",
  "rubygems.org",
];

/** 宽松档在标准之上追加 */
export const TIER_RELAXED_EXTRA = [
  "github.com",
  "api.github.com",
  "raw.githubusercontent.com",
  "gitlab.com",
  "bitbucket.org",
  "objects.githubusercontent.com",
];

/**
 * @param {string} tier
 * @returns {SecurityTier}
 */
export function normalizeTier(tier) {
  if (tier === "strict" || tier === "relaxed" || tier === "standard") return tier;
  return "standard";
}

/**
 * @param {string} tier
 * @returns {string[]}
 */
export function hostsForTier(tier) {
  const t = normalizeTier(tier);
  if (t === "strict") return [];
  if (t === "standard") return [...TIER_STANDARD_HOSTS];
  return [...TIER_STANDARD_HOSTS, ...TIER_RELAXED_EXTRA];
}

/**
 * @param {string} tier
 * @returns {string}
 */
export function tierDescription(tier) {
  const t = normalizeTier(tier);
  if (t === "strict") {
    return "只自动放行已配置模型/MCP 的 Origin；其它出站一律进扩权。";
  }
  if (t === "standard") {
    return `自动放行常用包源（${TIER_STANDARD_HOSTS.length} 个主机）+ 模型 Origin；其余扩权。`;
  }
  return `在标准之上再放行 GitHub 等开发站；仍无「全网」，陌生主机要扩权。`;
}
