import test from "node:test";
import assert from "node:assert/strict";
import * as mock from "./mock-api.js";
import * as api from "./api.js";

test("mock getRuntimeHealth：normal 可用，isolation 不可用", async () => {
  mock.resetMock();
  mock.setScenario("normal");
  try {
    const ok = await mock.getRuntimeHealth();
    assert.equal(ok.available, true);
    assert.equal(ok.isolationLevel, "mock-seatbelt");
    mock.setScenario("isolation");
    const bad = await mock.getRuntimeHealth();
    assert.equal(bad.available, false);
    assert.match(bad.reason, /隔离/);
  } finally {
    mock.resetMock();
    mock.setScenario("normal");
  }
});

test("api.getRuntimeHealth 经 mock transport；listSessions 空列表", async () => {
  mock.resetMock();
  api.useMock();
  try {
    const health = await api.getRuntimeHealth();
    assert.equal(health.available, true);
    const list = await api.listSessions();
    assert.deepEqual(list.sessions, []);
    assert.equal(list.error, null);
  } finally {
    mock.resetMock();
  }
});
