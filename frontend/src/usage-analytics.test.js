import test from "node:test";
import assert from "node:assert/strict";
import {
  clearUsageAnalytics,
  estimateCostStatus,
  formatEventRow,
  listUsageEvents,
  loadModelPrices,
  lockedPriceCurrency,
  recordAudit,
  recordPermissionDecision,
  recordSessionOutcome,
  removeModelPrice,
  saveModelPrices,
  summarizeUsage,
  upsertModelPrice,
} from "./usage-analytics.js";

test("usage analytics aggregates success deny and failures locally", () => {
  clearUsageAnalytics();
  recordSessionOutcome({ sessionId: "s1", state: "stopped" });
  recordSessionOutcome({
    sessionId: "s2",
    state: "failed",
    error: { code: "UPSTREAM_TIMEOUT" },
  });
  recordAudit({ result: "allowed", tool: "read", target: "gate" }, "s1");
  recordAudit({ result: "denied", tool: "bash", target: "evil" }, "s1");
  recordAudit({ result: "failed", tool: "search", target: "mcp" }, "s2");
  const s = summarizeUsage();
  assert.equal(s.sessions, 2);
  assert.match(s.firstTaskSuccessRate, /50%/);
  assert.match(s.denyRate, /33%/);
  assert.match(s.mcpFailRate, /33%/);
  assert.ok(
    s.topFailures.some(
      (f) => f.label.includes("denied") || f.label.includes("UPSTREAM"),
    ),
  );
  clearUsageAnalytics();
  assert.equal(summarizeUsage().sessions, 0);
});

test("records channels split blocks vs permissions", () => {
  clearUsageAnalytics();
  recordAudit({ result: "denied", tool: "net", target: "bad.example" }, "s1");
  recordAudit({ result: "allowed", tool: "read", target: "ok" }, "s1");
  recordPermissionDecision({
    sessionId: "s1",
    target: "cdn.example",
    action: "connect",
    decision: "project",
  });
  recordPermissionDecision({
    sessionId: "s1",
    target: "demo",
    decision: "deny",
    uiDemo: true,
  });
  const blocks = listUsageEvents("blocks");
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0].result, "denied");
  const perms = listUsageEvents("permissions");
  assert.equal(perms.length, 2);
  assert.match(formatEventRow(perms[0]), /deny|project/);
  assert.equal(summarizeUsage().permissionCount, 2);
  clearUsageAnalytics();
});

test("estimateCostStatus uses collected tokens when prices exist", () => {
  clearUsageAnalytics();
  saveModelPrices([]);
  recordAudit({ result: "allowed", tool: "model", target: "api.example", inputTokens: 1000, outputTokens: 500 }, "s1");
  let st = estimateCostStatus();
  assert.match(st.status, /有 token/);
  saveModelPrices([
    { model: "demo", inputPer1M: 1, outputPer1M: 2, currency: "CNY", asOf: "2026-10-30" },
  ]);
  st = estimateCostStatus();
  assert.equal(st.status, "已估算（参考）");
  assert.match(st.detail, /CNY/);
  clearUsageAnalytics();
  saveModelPrices([]);
});

test("model prices upsert/edit/delete and reject CNY/USD mix", () => {
  saveModelPrices([]);
  assert.equal(lockedPriceCurrency(), "");
  const a = upsertModelPrice([], {
    model: "deepseek-chat",
    inputPer1M: 1,
    outputPer1M: 2,
    currency: "CNY",
  });
  assert.equal(a.ok, true);
  assert.equal(loadModelPrices().length, 1);
  const mixed = upsertModelPrice(loadModelPrices(), {
    model: "gpt",
    inputPer1M: 1,
    outputPer1M: 1,
    currency: "USD",
  });
  assert.equal(mixed.ok, false);
  assert.match(mixed.error, /勿与 USD 混加|混加/);
  const upd = upsertModelPrice(loadModelPrices(), {
    model: "deepseek-chat",
    inputPer1M: 3,
    outputPer1M: 4,
    currency: "CNY",
  });
  assert.equal(upd.ok, true);
  assert.equal(loadModelPrices()[0].inputPer1M, 3);
  removeModelPrice(loadModelPrices(), "deepseek-chat");
  assert.equal(loadModelPrices().length, 0);
  assert.equal(lockedPriceCurrency(), "");
});
