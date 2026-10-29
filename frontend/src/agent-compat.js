/**
 * Agent ↔ 模型账号：默认同 profile 单绑；账号可带 agents[] 供多 Agent 共用。
 * MCP 工具账号可跨 Agent 挂载。
 */

export function expectedModelProfile(agentId, agents = []) {
  const a = agents.find((x) => x.id === agentId);
  if (a?.modelProfileId) return a.modelProfileId;
  if (agentId === "dsh") return "dsh-model";
  if (agentId === "codex") return "codex-model";
  return "demo-model";
}

export function defaultAgentsForProfile(profileId) {
  if (profileId === "codex-model") return ["codex"];
  if (profileId === "dsh-model") return ["dsh"];
  if (profileId === "demo-model" || profileId === "custom") return ["claude"];
  return [];
}

/** 账号显式 agents，或按 profile 默认单绑。 */
export function accountAgents(account) {
  if (!account) return [];
  if (Array.isArray(account.agents) && account.agents.length) {
    return [...new Set(account.agents.filter(Boolean))];
  }
  return defaultAgentsForProfile(account.profileId);
}

export function modelFitsAgent(account, agentId, _agents = []) {
  if (!account || account.kind !== "model") return false;
  return accountAgents(account).includes(agentId);
}

export function filterModelsForAgent(accounts, agentId, agents = []) {
  return (accounts || []).filter((a) => modelFitsAgent(a, agentId, agents));
}

export function agentsForModelProfile(profileId) {
  return defaultAgentsForProfile(profileId);
}

export function modelCompatLabel(accountOrProfileId) {
  const agents =
    typeof accountOrProfileId === "string"
      ? agentsForModelProfile(accountOrProfileId)
      : accountAgents(accountOrProfileId);
  if (!agents.length) return "未知兼容";
  if (agents.length === 1) return `仅 ${agents[0]}`;
  return `可用于 ${agents.join(" / ")}`;
}
