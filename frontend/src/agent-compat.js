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

/** 连接探测成功的协议；缺省按 profile（旧账号无 protocols）。 */
export function accountProtocols(account) {
  if (!account) return [];
  if (Array.isArray(account.protocols) && account.protocols.length) {
    return account.protocols;
  }
  if (account.profileId === "codex-model") return ["openai-responses"];
  if (
    account.profileId === "dsh-model" ||
    account.profileId === "demo-model" ||
    account.profileId === "custom"
  ) {
    return ["anthropic"];
  }
  return [];
}

export function agentNeedsProtocol(agentId) {
  if (agentId === "codex") return "openai-responses";
  if (agentId === "claude" || agentId === "dsh") return "anthropic";
  return "";
}

/** 协议勾选 → 默认可勾 Agent（Codex 仅 Responses；Claude/dsh 仅 Anthropic）。 */
export function agentsForProtocols(protocols = []) {
  const set = new Set(protocols);
  const out = [];
  if (set.has("anthropic")) out.push("claude", "dsh");
  if (set.has("openai-responses")) out.push("codex");
  return [...new Set(out)];
}

export function protocolLabel(id) {
  if (id === "anthropic") return "Anthropic Messages（/v1/messages）";
  if (id === "openai-responses") return "OpenAI Responses（/v1/responses）";
  return id || "";
}

export function modelFitsAgent(account, agentId, _agents = []) {
  if (!account || account.kind !== "model") return false;
  if (!accountAgents(account).includes(agentId)) return false;
  const need = agentNeedsProtocol(agentId);
  if (!need) return false;
  return accountProtocols(account).includes(need);
}

export function filterModelsForAgent(accounts, agentId, agents = []) {
  return (accounts || []).filter((a) => modelFitsAgent(a, agentId, agents));
}

/**
 * 运行中 SwitchModel：与 CONTRACT / Gate 一致，仅同 profileId + 同 host。
 * current 缺 host/profile 时返回空（不可静默放宽）。
 */
export function filterModelsForSwitch(accounts, current) {
  if (!current || current.kind !== "model") return [];
  const profileId = current.profileId || "";
  const host = (current.host || "").toLowerCase();
  if (!profileId || !host) return [];
  return (accounts || []).filter((a) => {
    if (!a || a.kind !== "model") return false;
    if ((a.profileId || "") !== profileId) return false;
    return (a.host || "").toLowerCase() === host;
  });
}

export function agentsForModelProfile(profileId) {
  return defaultAgentsForProfile(profileId);
}

export function modelCompatLabel(accountOrProfileId) {
  let agents;
  if (typeof accountOrProfileId === "string") {
    agents = agentsForModelProfile(accountOrProfileId);
  } else {
    const protocols = accountProtocols(accountOrProfileId);
    agents = accountAgents(accountOrProfileId).filter((id) => {
      const need = agentNeedsProtocol(id);
      return need && protocols.includes(need);
    });
  }
  if (!agents.length) return "未知兼容";
  if (agents.length === 1) return `仅 ${agents[0]}`;
  return `可用于 ${agents.join(" / ")}`;
}
