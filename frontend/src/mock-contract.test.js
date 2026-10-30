import test from "node:test";
import assert from "node:assert/strict";
import * as mock from "./mock-api.js";

test("mock 的事件 seq、会话 ID、错误码和 Stop 幂等遵循真实契约", async () => {
  mock.resetMock();
  mock.setScenario("auth");
  const auth = await mock.connectModel({
    profileId: "demo-model",
    name: "fixture",
    apiKey: "fixture",
  });
  assert.equal(auth.error.code, "UPSTREAM_AUTH_FAILED");
  assert.equal(auth.error.retryable, false);
  mock.setScenario("normal");
  await mock.connectModel({
    profileId: "demo-model",
    name: "fixture",
    apiKey: "fixture",
  });
  await mock.connectMcp({
    profileId: "demo-mcp",
    name: "fixture",
    apiKey: "fixture",
  });
  const events = [];
  const off = mock.onEvent((event) => events.push(event));
  try {
    const spec = {
      workspace: "/demo/project",
      agentId: "claude",
      modelAccountId: "mock-model",
      mcpIds: ["mock-mcp"],
      task: "fixture",
    };
    const first = await mock.createSession(spec);
    const stopped = new Promise((resolve) => {
      const un = mock.onEvent((event) => {
        if (event.state?.state === "stopped") {
          un();
          resolve();
        }
      });
    });
    await mock.stopSession(first.session.sessionId);
    const count = events.length;
    await mock.stopSession(first.session.sessionId);
    assert.equal(events.length, count, "重复 Stop 产生重复状态事件");
    await stopped;
    const second = await mock.createSession(spec);
    assert.notEqual(first.session.sessionId, second.session.sessionId);
    for (const event of events.filter((event) => event.state))
      assert.equal(event.seq, event.state.seq);
  } finally {
    off();
    mock.resetMock();
  }
});

test("mock Health 在 isolationBackend=docker 时返回 isolationLevel=docker", async () => {
  mock.resetMock();
  try {
    const seat = await mock.getRuntimeHealth();
    assert.match(String(seat.isolationLevel || ""), /seatbelt/);
    await mock.setUIPrefs({ isolationBackend: "docker" });
    const dock = await mock.getRuntimeHealth();
    assert.equal(dock.available, true);
    assert.equal(dock.isolationLevel, "docker");
    assert.match(String(dock.reason || ""), /Docker/i);
  } finally {
    mock.resetMock();
  }
});
