import test from "node:test";
import assert from "node:assert/strict";
import * as api from "./api.js";
import * as mock from "./mock-api.js";
import {
  mountModels,
  customModelOriginError,
  defaultModelOrigin,
} from "./views/models.js";

test("自定义模型拒绝把监听地址当作连接地址", () => {
  assert.match(customModelOriginError("http://0.0.0.0:4000"), /不能使用 0\.0\.0\.0/);
  assert.match(customModelOriginError("http://[::]:4000"), /不能使用/);
  assert.equal(customModelOriginError("http://127.0.0.1:4000"), "");
  assert.equal(customModelOriginError("https://api.example.com"), "");
});

test("编辑模型时保留本机 HTTP 回环协议", () => {
  assert.equal(
    defaultModelOrigin({ host: "127.0.0.1:4000", basePath: "/anthropic" }),
    "http://127.0.0.1:4000/anthropic",
  );
  assert.equal(defaultModelOrigin({ host: "localhost:4000" }), "http://localhost:4000");
  assert.equal(defaultModelOrigin({ host: "api.example.com" }), "https://api.example.com");
});

test("dsh 向导添加模型使用固定 profile，无需重复填写上游地址", async () => {
  mock.resetMock();
  api.useMock();
  const previousDocument = globalThis.document;
  const previousWindow = globalThis.window;
  const nodes = new Map();
  const fixedBtn = {
    getAttribute: (k) =>
      k === "data-fixed" ? "dsh-model" : k === "data-pname" ? "DeepSeek dsh" : "",
    onclick: null,
  };
  const agentBox = {
    getAttribute: (k) => (k === "data-agent" ? "dsh" : ""),
    checked: true,
    addEventListener() {},
    removeEventListener() {},
  };
  function node(selector) {
    if (!nodes.has(selector)) {
      nodes.set(selector, {
        value: "",
        innerHTML: "",
        textContent: "",
        checked: false,
        disabled: false,
        className: "",
        classList: { remove() {}, toggle() {}, add() {}, contains() { return false; } },
        addEventListener() {},
        removeEventListener() {},
        focus() {},
        remove() {},
        setAttribute() {},
        appendChild() {},
        querySelector: node,
        querySelectorAll: (sel) => {
          if (sel === "[data-fixed]") return [fixedBtn];
          if (sel === "[data-agent]" || sel === "[data-agent]:checked") return [agentBox];
          return [];
        },
      });
    }
    return nodes.get(selector);
  }
  const formBody = node("#form-modal-body");
  const modalTitle = { textContent: "" };
  const modalRoot = {
    innerHTML: "",
    classList: {
      _hidden: true,
      contains(c) {
        return c === "hidden" && this._hidden;
      },
      remove(c) {
        if (c === "hidden") this._hidden = false;
      },
      add(c) {
        if (c === "hidden") this._hidden = true;
      },
    },
    querySelector(sel) {
      if (sel === ".modal-card") return { className: "modal-card" };
      if (sel === "#form-modal-body") return formBody;
      if (sel === "#form-modal-title") return modalTitle;
      return null;
    },
  };
  globalThis.document = {
    body: node("body"),
    createElement: () => node(`el-${nodes.size}`),
    getElementById: (id) => (id === "modal-root" ? modalRoot : null),
    querySelectorAll: () => [],
  };
  globalThis.window = {
    addEventListener() {},
    removeEventListener() {},
  };
  const ctx = {
    state: {
      returnAfterAccount: { kind: "model", route: "wizard" },
      wizardResume: { step: 2, draft: { agentId: "dsh", workspace: "/demo/project" } },
    },
    navigate(route) {
      this.route = route;
    },
  };
  try {
    mountModels(node("root"), ctx, { intent: "add" });
    await new Promise((resolve) => setImmediate(resolve));
    assert.match(formBody.innerHTML, /data-fixed="dsh-model"/);
    assert.equal(typeof fixedBtn.onclick, "function");
    fixedBtn.onclick();
    await new Promise((resolve) => setImmediate(resolve));
    assert.match(formBody.innerHTML, /固定 profile：<code>dsh-model<\/code>/);
    node("#m-name").value = "dsh 测试账号";
    node("#m-key").value = "fixture-key";
    await node("#m-save").onclick();
    const accounts = (await api.listAccounts("model")).accounts;
    assert.equal(accounts.length, 1);
    assert.equal(accounts[0].profileId, "dsh-model");
    assert.equal(node("#m-key").value, "");
    assert.equal(ctx.route, "wizard");
    assert.equal(ctx.state.wizardResume.draft.modelAccountId, accounts[0].id);
    assert.equal(ctx.state.wizardResume.draft.agentId, "dsh");
  } finally {
    globalThis.document = previousDocument;
    globalThis.window = previousWindow;
    mock.resetMock();
  }
});
