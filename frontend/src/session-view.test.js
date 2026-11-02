import test from "node:test";
import assert from "node:assert/strict";
import * as api from "./api.js";
import * as mock from "./mock-api.js";
import {
  mountSession,
  modelRouteHint,
  shouldSubmitComposerEvent,
  permissionSessionId,
  isActivePermissionEvent,
  permissionDecisionErrorMessage,
} from "./views/session.js";

test("权限决策使用事件中的权威会话 ID", () => {
  const event = { sessionId: "session-from-event" };
  assert.equal(permissionSessionId(event, "stale-session"), "session-from-event");
  assert.equal(permissionSessionId({}, "active-session"), "active-session");
  assert.equal(isActivePermissionEvent(event, "stale-session"), false);
  assert.equal(isActivePermissionEvent(event, "session-from-event"), true);
  assert.equal(isActivePermissionEvent({}, "active-session"), true);
  assert.match(
    permissionDecisionErrorMessage({ code: "INVALID_ARGUMENT", message: "通用错误" }),
    /权限请求已失效/,
  );
  assert.equal(
    permissionDecisionErrorMessage({ code: "UPSTREAM_TIMEOUT", message: "上游超时" }),
    "上游超时",
  );
});

test("工作台同时说明 Claude 协议模型和实际上游模型", () => {
  assert.equal(modelRouteHint({ profileId: "custom", model: "deepseek-chat" }, "claude"), "上游模型：deepseek-chat · Claude 协议模型：claude-sonnet-4-5");
  assert.equal(modelRouteHint({ profileId: "dsh-model", model: "deepseek-chat" }, "dsh"), "上游模型：deepseek-chat");
  assert.equal(modelRouteHint({ profileId: "custom" }, "claude"), "");
});

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
