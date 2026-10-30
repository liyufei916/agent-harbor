import test from "node:test";
import assert from "node:assert/strict";
import {
  clearUsageAnalytics,
  recordAudit,
  recordSessionOutcome,
  summarizeUsage,
} from "./usage-analytics.js";

test("usage analytics aggregates success deny and failures locally", () => {
  clearUsageAnalytics();
  recordSessionOutcome({ sessionId: "s1", state: "stopped" });
  recordSessionOutcome({ sessionId: "s2", state: "failed", error: { code: "UPSTREAM_TIMEOUT" } });
  recordAudit({ result: "allowed", tool: "read", target: "gate" }, "s1");
  recordAudit({ result: "denied", tool: "bash", target: "evil" }, "s1");
  recordAudit({ result: "failed", tool: "search", target: "mcp" }, "s2");
  const s = summarizeUsage();
  assert.equal(s.sessions, 2);
  assert.match(s.firstTaskSuccessRate, /50%/);
  assert.match(s.denyRate, /33%/);
  assert.match(s.mcpFailRate, /33%/);
  assert.ok(s.topFailures.some((f) => f.label.includes("denied") || f.label.includes("UPSTREAM")));
  clearUsageAnalytics();
  assert.equal(summarizeUsage().sessions, 0);
});
