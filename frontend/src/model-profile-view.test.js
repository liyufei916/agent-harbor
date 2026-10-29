import test from "node:test";
import assert from "node:assert/strict";
import * as api from "./api.js";
import * as mock from "./mock-api.js";
import { mountModels } from "./views/models.js";

test("dsh 向导添加模型使用固定 profile，无需重复填写上游地址", async () => {
  mock.resetMock();
  api.useMock();
  const previousDocument = globalThis.document;
  const nodes = new Map();
  function node(selector) {
    if (!nodes.has(selector)) {
      nodes.set(selector, {
        value: "", innerHTML: "", textContent: "", checked: false,
        classList: { remove() {}, toggle() {} },
        addEventListener() {}, focus() {}, appendChild() {},
        querySelector: node, querySelectorAll: () => [],
      });
    }
    return nodes.get(selector);
  }
  globalThis.document = { createElement: () => node("article") };
  const ctx = {
    state: {
      modelProfileIntent: "dsh-model",
      returnAfterAccount: { kind: "model", route: "wizard" },
      wizardResume: { step: 2, draft: { agentId: "dsh", workspace: "/demo/project" } },
    },
    navigate(route) { this.route = route; },
  };
  try {
    mountModels(node("root"), ctx, { intent: "add" });
    await new Promise((resolve) => setImmediate(resolve));
    assert.match(node("#model-form").innerHTML, /value="dsh-model" selected/);
    // 模拟浏览器按 selected 属性选择固定 profile。
    node("#m-profile").value = "dsh-model";
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
    mock.resetMock();
  }
});
