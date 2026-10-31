import test from "node:test";
import assert from "node:assert/strict";
import * as api from "./api.js";
import * as mock from "./mock-api.js";
import { mountTools } from "./views/tools.js";

function installDom(root) {
  const previousDocument = globalThis.document;
  globalThis.document = {
    body: { appendChild() {}, removeChild() {} },
    createElement: () => ({
      className: "",
      style: {},
      appendChild() {},
      setAttribute() {},
      addEventListener() {},
    }),
    getElementById: () => null,
    querySelectorAll: () => [],
  };
  return () => {
    globalThis.document = previousDocument;
  };
}

test("工具页展示预置 MCP 与远程/stdio 空态引导", async () => {
  mock.resetMock();
  api.useMock();
  const restore = installDom();
  try {
    const root = {
      innerHTML: "",
      querySelector(sel) {
        if (!this._nodes) this._nodes = new Map();
        if (!this._nodes.has(sel)) {
          this._nodes.set(sel, {
            innerHTML: "",
            textContent: "",
            addEventListener() {},
            onclick: null,
            onchange: null,
            checked: false,
            value: "",
            disabled: false,
            classList: { toggle() {}, add() {}, remove() {}, contains() { return false; } },
            querySelectorAll: () => [],
            getAttribute: () => "",
          });
        }
        return this._nodes.get(sel);
      },
      querySelectorAll(sel) {
        if (sel === "[data-preset-check]") {
          return [];
        }
        return [];
      },
    };
    const ctx = {
      state: {
        mcpAccounts: [],
        mcpAccount: null,
        returnAfterAccount: null,
      },
      navigate() {},
    };
    mountTools(root, ctx);
    // paint 是 async load 后触发；等微任务
    await new Promise((r) => setTimeout(r, 30));
    assert.match(root.innerHTML, /预置 MCP/);
    assert.match(root.innerHTML, /远程 HTTP/);
    assert.match(root.innerHTML, /宿主 stdio|stdio/);
    assert.match(root.innerHTML, /自定义添加/);
    const presets = await mock.listMcpPresets();
    assert.ok((presets.presets || []).length >= 3);
  } finally {
    restore();
    mock.resetMock();
  }
});
