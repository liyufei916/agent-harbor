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

test("阻断页无「仍要继续」，有重新检测与设置", async () => {
  const root = fakeRoot();
  let available = false;
  let toSettings = false;
  mountBlock(root, {
    reason: "隔离能力不可用（模拟）",
    health: { available: false, reason: "隔离能力不可用（模拟）" },
    getHealth: async () => ({ available: true, isolationLevel: "seatbelt" }),
    onAvailable() {
      available = true;
    },
    onSettings() {
      toSettings = true;
    },
  });
  assert.match(root.innerHTML, /无法安全启动/);
  assert.match(root.innerHTML, /不会在无隔离时启动/);
  assert.match(root.innerHTML, /也没有「仍要继续」/);
  assert.doesNotMatch(root.innerHTML, /id="btn-.*continue"/);
  assert.match(root._handlers["block-checks"].innerHTML, /系统环境/);
  assert.ok(root._handlers["btn-block-recheck"]);
  assert.ok(root._handlers["btn-block-settings"]);
  await root._handlers["btn-block-recheck"].onclick();
  assert.equal(available, true);
  assert.match(root._handlers["block-checks"].innerHTML, /安全闸口/);
  root._handlers["btn-block-settings"].onclick();
  assert.equal(toSettings, true);
});
