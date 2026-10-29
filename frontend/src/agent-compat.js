/**
 * Agent ↔ 模型账号兼容：各 Agent 模型互不通用。
 * MCP 工具账号可跨 Agent 共用（Gate 白名单按会话挂载）。
 */

export function expectedModelProfile(agentId, agents = []) {
  const a = agents.find((x) => x.id === agentId);
  if (a?.modelProfileId) return a.modelProfileId;
  if (agentId === "dsh") return "dsh-model";
  if (agentId === "codex") return "codex-model";
  return "demo-model";
}

/** Claude 可用 custom；Codex/dsh 仅固定 profile。 */
export function modelFitsAgent(account, agentId, agents = []) {
  if (!account || account.kind !== "model") return false;
  const expected = expectedModelProfile(agentId, agents);
  if (account.profileId === expected) return true;
  return agentId === "claude" && account.profileId === "custom";
}

export function filterModelsForAgent(accounts, agentId, agents = []) {
  return (accounts || []).filter((a) => modelFitsAgent(a, agentId, agents));
}

/** 列表展示：该模型账号可用于哪些 Agent。 */
export function agentsForModelProfile(profileId) {
  if (profileId === "codex-model") return ["codex"];
  if (profileId === "dsh-model") return ["dsh"];
  if (profileId === "custom") return ["claude"];
  if (profileId === "demo-model") return ["claude"];
  return [];
}

export function modelCompatLabel(profileId) {
  const ids = agentsForModelProfile(profileId);
  if (!ids.length) return "未知兼容";
  return `仅 ${ids.join("/")}`;
}
