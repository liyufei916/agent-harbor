import test from "node:test";
import assert from "node:assert/strict";
import * as mock from "./mock-api.js";
import * as api from "./api.js";
import { healthAllowsStart, healthBlockReason } from "./health-ready.js";

test("healthAllowsStart：provisional 禁止开跑", () => {
  assert.equal(healthAllowsStart({ available: true }), true);
  assert.equal(healthAllowsStart({ available: true, provisional: true }), false);
  assert.equal(healthAllowsStart({ available: false, provisional: true }), false);
  assert.equal(healthAllowsStart({ available: false }), false);
  assert.match(healthBlockReason({ provisional: true, reason: "占位" }), /占位|未验证/);
});

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
