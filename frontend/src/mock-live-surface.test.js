/**
 * A6：mock 与 live transport 方法面收敛。
 * 列表须与 live-api.js createLiveTransport 返回键一致；新增绑定先改 live，再补 mock。
 */
import test from "node:test";
import assert from "node:assert/strict";
import * as mock from "./mock-api.js";

const LIVE_SURFACE = [
  "listAgents",
  "selectWorkspace",
  "connectModel",
  "connectMcp",
  "createSession",
  "getSession",
  "stopSession",
  "listAccounts",
  "removeAccount",
  "listSessions",
  "destroySession",
  "syncBackWorkspace",
  "getRuntimeHealth",
  "getPermissionOverview",
  "listRememberedHosts",
  "revokeRememberedHost",
  "resolvePermission",
  "getUIPrefs",
  "setUIPrefs",
  "switchModel",
  "startMcpOAuth",
  "cancelMcpOAuth",
  "writePty",
  "resizePty",
  "onEvent",
];

test("mock-api 导出覆盖 live transport 全部方法", () => {
  for (const name of LIVE_SURFACE) {
    assert.equal(typeof mock[name], "function", `missing mock.${name}`);
  }
});
