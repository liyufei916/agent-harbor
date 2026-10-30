import test from "node:test";
import assert from "node:assert/strict";
import { mountBlock } from "./views/block.js";

function fakeRoot() {
  const handlers = {};
  const el = {
    innerHTML: "",
    querySelector(sel) {
      const id = sel.startsWith("#") ? sel.slice(1) : null;
      if (!id) return null;
      if (!handlers[id]) {
        handlers[id] = {
          disabled: false,
          textContent: "",
          innerHTML: "",
          classList: {
            _set: new Set(),
            toggle(name, on) {
              if (on) this._set.add(name);
              else this._set.delete(name);
            },
          },
          onclick: null,
        };
      }
      return handlers[id];
    },
  };
  el._handlers = handlers;
  return el;
}

test("阻断页无「仍要继续」，主按钮进设置改引擎", async () => {
  const root = fakeRoot();
  let available = false;
  let toSettings = null;
  mountBlock(root, {
    reason: "隔离能力不可用（模拟）",
    health: { available: false, reason: "隔离能力不可用（模拟）" },
    preferredLevel: "docker",
    getHealth: async () => ({ available: true, isolationLevel: "seatbelt" }),
    onAvailable() {
      available = true;
    },
    onSettings(opts) {
      toSettings = opts || {};
    },
  });
  assert.match(root.innerHTML, /无法安全启动/);
  assert.match(root.innerHTML, /不会在无隔离时启动/);
  assert.match(root.innerHTML, /也没有「仍要继续」/);
  assert.match(root.innerHTML, /本机 Docker|Seatbelt/);
  assert.match(root.innerHTML, /改隔离 \/ 引擎设置/);
  assert.doesNotMatch(root.innerHTML, /id="btn-.*continue"/);
  assert.match(root._handlers["block-checks"].innerHTML, /Docker CLI|系统环境/);
  assert.ok(root._handlers["btn-block-recheck"]);
  assert.ok(root._handlers["btn-block-settings"]);
  await root._handlers["btn-block-recheck"].onclick();
  assert.equal(available, true);
  assert.match(root._handlers["block-checks"].innerHTML, /安全闸口/);
  root._handlers["btn-block-settings"].onclick();
  assert.equal(toSettings?.focusIsolation, true);
});
