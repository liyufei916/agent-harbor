/**
 * A6：mock createSession / 快照字段形状对齐 CONTRACT SessionSnapshot。
 */
import test from "node:test";
import assert from "node:assert/strict";
import * as mock from "./mock-api.js";

const SNAPSHOT_KEYS = [
  "sessionId",
  "state",
  "seq",
  "workspace",
  "sourceWorkspace",
  "mountMode",
  "modelAccountId",
  "mcpIds",
  "agentId",
  "cleanupComplete",
  "error",
];

test("mock createSession 返回 SessionResponse 形状且快照含契约字段", async () => {
  mock.resetMock();
  mock.setScenario("normal");
  await mock.connectModel({
    profileId: "demo-model",
    name: "shape-model",
    apiKey: "fixture-key",
  });
  await mock.connectMcp({
    profileId: "demo-mcp",
    name: "shape-mcp",
    apiKey: "fixture-key",
  });
  const res = await mock.createSession({
    workspace: "/demo/project",
    agentId: "claude",
    modelAccountId: "mock-model",
    mcpIds: ["mock-mcp"],
    task: "shape check",
    mountMode: "clone",
  });
  assert.equal(res.error, null);
  assert.ok(res.session);
  for (const key of SNAPSHOT_KEYS) {
    assert.ok(key in res.session, `missing session.${key}`);
  }
  assert.equal(typeof res.session.sessionId, "string");
  assert.ok(res.session.sessionId.length > 0);
  assert.equal(res.session.mountMode, "clone");
  assert.equal(res.session.sourceWorkspace, "/demo/project");
  assert.notEqual(res.session.workspace, res.session.sourceWorkspace);
  assert.equal(res.session.agentId, "claude");
  assert.equal(res.session.modelAccountId, "mock-model");
  assert.deepEqual(res.session.mcpIds, ["mock-mcp"]);
  assert.equal(typeof res.session.seq, "number");
  assert.equal(typeof res.session.cleanupComplete, "boolean");
  assert.ok(["provisioning", "running"].includes(res.session.state));

  const got = await mock.getSession(res.session.sessionId);
  assert.equal(got.error, null);
  for (const key of SNAPSHOT_KEYS) {
    assert.ok(key in got.session, `getSession missing ${key}`);
  }

  const listed = await mock.listSessions();
  assert.equal(listed.error, null);
  assert.ok(listed.sessions.length >= 1);
  const row = listed.sessions.find((s) => s.sessionId === res.session.sessionId);
  assert.ok(row);
  for (const key of [
    "sessionId",
    "state",
    "workspace",
    "sourceWorkspace",
    "mountMode",
    "agentId",
    "cleanupComplete",
  ]) {
    assert.ok(key in row, `summary missing ${key}`);
  }

  mock.resetMock();
});
