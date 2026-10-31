import test from "node:test";
import assert from "node:assert/strict";
import * as mock from "./mock-api.js";

test("mock Get/SetUIPrefs 读写侧栏偏好", async () => {
  mock.resetMock();
  try {
    const first = await mock.getUIPrefs();
    assert.equal(first.prefs.autoCollapseWorkbench, true);
    assert.equal(first.prefs.askClearOnStop, true);
    assert.equal(first.prefs.theme, "system");
    const saved = await mock.setUIPrefs({
      sidebarCollapsed: true,
      autoCollapseWorkbench: false,
    });
    assert.equal(saved.prefs.sidebarCollapsed, true);
    assert.equal(saved.prefs.autoCollapseWorkbench, false);
    assert.equal(saved.prefs.askClearOnStop, true);
    assert.equal(saved.prefs.theme, "system");
    const again = await mock.getUIPrefs();
    assert.equal(again.prefs.sidebarCollapsed, true);
    assert.equal(again.prefs.autoCollapseWorkbench, false);
    const clear = await mock.setUIPrefs({
      askClearOnStop: false,
      clearOnStopDefault: "clear",
    });
    assert.equal(clear.prefs.askClearOnStop, false);
    assert.equal(clear.prefs.clearOnStopDefault, "clear");
    assert.equal(clear.prefs.sidebarCollapsed, true);
  } finally {
    mock.resetMock();
  }
});

test("mock Get/SetUIPrefs theme 往返 light/dark/system", async () => {
  mock.resetMock();
  try {
    for (const theme of ["light", "dark", "system"]) {
      const saved = await mock.setUIPrefs({ theme });
      assert.equal(saved.prefs.theme, theme);
      const again = await mock.getUIPrefs();
      assert.equal(again.prefs.theme, theme);
    }
    const bad = await mock.setUIPrefs({ theme: "neon" });
    assert.equal(bad.prefs.theme, "system");
  } finally {
    mock.resetMock();
  }
});

test("mock UIPrefs：演示开关默认关；引擎底座可切换", async () => {
  mock.resetMock();
  try {
    const first = await mock.getUIPrefs();
    assert.equal(first.prefs.demoExpandAsk, false);
    assert.equal(first.prefs.engineSource, "harbor-colima");
    assert.equal(first.prefs.isolationBackend, "seatbelt");
    const eng = await mock.setUIPrefs({
      isolationBackend: "docker",
      engineSource: "system-docker",
      demoExpandAsk: true,
    });
    assert.equal(eng.prefs.isolationBackend, "docker");
    assert.equal(eng.prefs.engineSource, "system-docker");
    assert.equal(eng.prefs.demoExpandAsk, true);
    const again = await mock.getUIPrefs();
    assert.equal(again.prefs.engineSource, "system-docker");
    assert.equal(again.prefs.demoExpandAsk, true);
    const off = await mock.setUIPrefs({ demoExpandAsk: false });
    assert.equal(off.prefs.demoExpandAsk, false);
  } finally {
    mock.resetMock();
  }
});
