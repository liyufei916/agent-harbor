import assert from "node:assert/strict";
import test from "node:test";
import {
  expectedModelProfile,
  modelFitsAgent,
  filterModelsForAgent,
  modelCompatLabel,
} from "./agent-compat.js";

test("expectedModelProfile 按 Agent 映射", () => {
  assert.equal(expectedModelProfile("claude"), "demo-model");
  assert.equal(expectedModelProfile("codex"), "codex-model");
  assert.equal(expectedModelProfile("dsh"), "dsh-model");
});

test("modelFitsAgent：默认 profile 单绑；agents[] 须匹配 protocols", () => {
  assert.equal(modelFitsAgent({ kind: "model", profileId: "custom" }, "claude"), true);
  assert.equal(modelFitsAgent({ kind: "model", profileId: "custom" }, "codex"), false);
  // 仅勾选 agents、无 protocols（旧 DeepSeek）→ 默认 anthropic，Codex 不可用
  assert.equal(
    modelFitsAgent(
      { kind: "model", profileId: "custom", agents: ["claude", "codex"] },
      "codex",
    ),
    false,
  );
  assert.equal(
    modelFitsAgent(
      {
        kind: "model",
        profileId: "custom",
        agents: ["claude", "codex"],
        protocols: ["anthropic", "openai-responses"],
      },
      "codex",
    ),
    true,
  );
  assert.equal(modelFitsAgent({ kind: "model", profileId: "codex-model" }, "codex"), true);
});

test("filterModelsForAgent 只保留兼容项", () => {
  const list = [
    { id: "1", kind: "model", profileId: "custom" },
    { id: "2", kind: "model", profileId: "codex-model" },
    {
      id: "3",
      kind: "model",
      profileId: "custom",
      agents: ["claude", "codex"],
      protocols: ["anthropic", "openai-responses"],
    },
    {
      id: "4",
      kind: "model",
      profileId: "custom",
      agents: ["claude", "codex"],
      // DeepSeek 误勾 Codex、无 Responses
    },
  ];
  assert.deepEqual(
    filterModelsForAgent(list, "codex").map((a) => a.id),
    ["2", "3"],
  );
});

test("modelCompatLabel", () => {
  assert.equal(modelCompatLabel("dsh-model"), "仅 dsh");
  assert.equal(
    modelCompatLabel({ profileId: "custom", agents: ["claude", "codex"] }),
    "仅 claude",
  );
  assert.equal(
    modelCompatLabel({
      kind: "model",
      profileId: "custom",
      agents: ["claude", "codex"],
      protocols: ["anthropic", "openai-responses"],
    }),
    "可用于 claude / codex",
  );
});
