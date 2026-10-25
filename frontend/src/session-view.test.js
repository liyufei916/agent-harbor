import test from "node:test";
import assert from "node:assert/strict";
import * as api from "./api.js";
import * as mock from "./mock-api.js";
import { mountSession } from "./views/session.js";

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
      assert.equal(ctx.state.lastSnapshot.state, "running");
      await api.stopSession(ctx.state.sessionId);
      assert.equal(ctx.state.lastSnapshot.state, "stopping");
    } finally {
      view.destroy();
      mock.resetMock();
    }
  },
);
