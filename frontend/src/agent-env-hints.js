/**
 * 预置 profile / Agent 启动前宿主机环境变量说明（与 models FIXED_PRESETS、文档对齐）。
 */

export const AGENT_ENV_HINTS = {
  claude: {
    label: "Claude",
    lines: [
      "可选：HARBOR_CLAUDE_BIN（默认 PATH 中的 claude）",
      "可选：HARBOR_CLAUDE_MODEL（默认 claude-sonnet-4-5）",
    ],
  },
  codex: {
    label: "Codex",
    lines: [
      "必填（预置 profile）：HARBOR_CODEX_MODEL_ORIGIN（如 https://api.openai.com）",
      "可选：HARBOR_CODEX_MODEL、HARBOR_CODEX_BASE_PATH、HARBOR_CODEX_BIN",
    ],
  },
  dsh: {
    label: "dsh",
    lines: [
      "必填（预置 profile）：HARBOR_DSH_MODEL_ORIGIN",
      "可选：HARBOR_DSH_MODEL、HARBOR_DSH_BASE_PATH、HARBOR_DSH_NODE、HARBOR_DSH_INSTALLATION",
    ],
  },
};

/** 模型页固定 profile 短 hint */
export function fixedProfileHint(profileId) {
  if (profileId === "codex-model") {
    return "Responses API；须已 export HARBOR_CODEX_MODEL_ORIGIN（如 https://api.openai.com）";
  }
  if (profileId === "dsh-model") {
    return "须已 export HARBOR_DSH_MODEL_ORIGIN 等（见设置 / 向导 Agent 步）";
  }
  return "";
}

export function envHintHTML(agentId) {
  const h = AGENT_ENV_HINTS[agentId];
  if (!h) return "";
  const lis = h.lines.map((l) => `<li>${l}</li>`).join("");
  return `<div class="meta env-hint"><strong>${h.label} 环境变量</strong><ul class="audit-list">${lis}</ul></div>`;
}

export function allEnvHintsHTML() {
  return ["claude", "codex", "dsh"].map((id) => envHintHTML(id)).join("");
}
