import assert from "node:assert/strict";
import test from "node:test";
import {
  expectedModelProfile,
  modelFitsAgent,
  filterModelsForAgent,
  agentsForModelProfile,
  modelCompatLabel,
} from "./agent-compat.js";

test("expectedModelProfile 按 Agent 映射", () => {
  assert.equal(expectedModelProfile("claude"), "demo-model");
  assert.equal(expectedModelProfile("codex"), "codex-model");
  assert.equal(expectedModelProfile("dsh"), "dsh-model");
  assert.equal(
    expectedModelProfile("claude", [{ id: "claude", modelProfileId: "demo-model" }]),
    "demo-model",
  );
});

test("modelFitsAgent：Claude 允 custom，Codex/dsh 拒绝异 profile", () => {
  assert.equal(modelFitsAgent({ kind: "model", profileId: "custom" }, "claude"), true);
  assert.equal(modelFitsAgent({ kind: "model", profileId: "demo-model" }, "claude"), true);
  assert.equal(modelFitsAgent({ kind: "model", profileId: "custom" }, "codex"), false);
  assert.equal(modelFitsAgent({ kind: "model", profileId: "codex-model" }, "codex"), true);
  assert.equal(modelFitsAgent({ kind: "model", profileId: "dsh-model" }, "codex"), false);
  assert.equal(modelFitsAgent({ kind: "model", profileId: "demo-model" }, "dsh"), false);
});

test("filterModelsForAgent 只保留兼容项", () => {
  const list = [
    { id: "1", kind: "model", profileId: "custom" },
    { id: "2", kind: "model", profileId: "codex-model" },
    { id: "3", kind: "mcp", profileId: "demo-mcp" },
  ];
  assert.deepEqual(
    filterModelsForAgent(list, "codex").map((a) => a.id),
    ["2"],
  );
  assert.deepEqual(
    filterModelsForAgent(list, "claude").map((a) => a.id),
    ["1"],
  );
});

test("agentsForModelProfile / label", () => {
  assert.deepEqual(agentsForModelProfile("codex-model"), ["codex"]);
  assert.equal(modelCompatLabel("dsh-model"), "仅 dsh");
  assert.equal(modelCompatLabel("custom"), "仅 claude");
});
