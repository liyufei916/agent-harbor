import test from "node:test";
import assert from "node:assert/strict";
import * as api from "./api.js";
import * as mock from "./mock-api.js";
import {
  mountSession,
  shouldSubmitComposerEvent,
} from "./views/session.js";

test("输入法确认候选词时不应触发回车发送", () => {
  assert.equal(
    shouldSubmitComposerEvent(
      { key: "Enter", isComposing: true, keyCode: 229 },
      true,
    ),
    false,
  );
  assert.equal(
    shouldSubmitComposerEvent({ key: "Enter", isComposing: false }, true),
    false,
  );
  assert.equal(
    shouldSubmitComposerEvent({ key: "Enter", isComposing: false }),
    true,
  );
  assert.equal(
    shouldSubmitComposerEvent({ key: "Enter", shiftKey: true }),
    false,
  );
});

test(
  "实际 api.js 与工作台在延迟创建响应时保留早到输出和新状态",
  { timeout: 3000 },
  async () => {
    mock.resetMock();
    mock.setScenario("normal");
    api.useMock();
    const nodes = new Map();
    const root = {
      innerHTML: "",
      querySelector(id) {
        if (!nodes.has(id))
          nodes.set(id, {
            textContent: "",
            innerHTML: "",
            classList: { add() {}, remove() {} },
            disabled: false,
          });
        return nodes.get(id);
      },
    };
    const ctx = { state: { sessionId: "", lastSnapshot: null }, showTab() {} };
    const view = mountSession(root, ctx);
    try {
      await api.connectModel({
        profileId: "demo-model",
        name: "fixture",
        apiKey: "fixture",
      });
      await api.connectMcp({
        profileId: "demo-mcp",
        name: "fixture",
        apiKey: "fixture",
      });
      const early = new Promise((resolve) => {
        const off = mock.onEvent((event) => {
          if (event.output) {
            off();
            resolve();
          }
        });
      });
      const response = await api.createSession({
        workspace: "/demo/project",
        agentId: "claude",
        modelAccountId: "mock-model",
        mcpIds: ["mock-mcp"],
        task: "fixture",
      });
      await early;
      assert.equal(nodes.get("#sess-pty").textContent, "");
      ctx.state.sessionId = response.session.sessionId;
      view.resetForNewSession(response.session);
      assert.match(nodes.get("#sess-pty").textContent, /开始执行任务/);
      nodes.get("#sess-pty-input").value = "下一轮问题";
      await nodes.get("#sess-pty-send").onclick();
      assert.match(nodes.get("#sess-pty").textContent, /下一轮问题/);
      nodes.get("#sess-pty").textContent = "";
      view.showSnapshot(ctx.state.lastSnapshot);
      assert.match(nodes.get("#sess-pty").textContent, /开始执行任务/);
      assert.match(nodes.get("#sess-pty").textContent, /下一轮问题/);
      assert.equal(ctx.state.lastSnapshot.state, "running");
      await api.stopSession(ctx.state.sessionId);
      assert.equal(ctx.state.lastSnapshot.state, "stopping");
    } finally {
      view.destroy();
      mock.resetMock();
    }
  },
);
