import test from "node:test";
import assert from "node:assert/strict";
import {
  clearUsageAnalytics,
  formatEventRow,
  listUsageEvents,
  recordAudit,
  recordPermissionDecision,
  recordSessionOutcome,
  summarizeUsage,
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
