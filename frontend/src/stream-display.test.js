import test from "node:test";
import assert from "node:assert/strict";
import { createStreamDisplay } from "./stream-display.js";

test("stream-display 提取 assistant 文本并折叠协议噪声", () => {
  const d = createStreamDisplay();
  const chunk =
    JSON.stringify({
      type: "system",
      subtype: "init",
      model: "claude-sonnet-4-5",
      tools: ["Read", "Write"],
    }) +
    "\n" +
    JSON.stringify({
      type: "assistant",
      message: {
        content: [
          { type: "thinking", thinking: "secret" },
          { type: "text", text: "你好" },
          {
            type: "tool_use",
            name: "Read",
            input: { file_path: "/tmp/a.md" },
          },
        ],
      },
    }) +
    "\n" +
    JSON.stringify({
      type: "user",
      message: {
        content: [
          { type: "tool_result", content: "big file...", is_error: false },
        ],
      },
    }) +
    "\n" +
    JSON.stringify({
      type: "result",
      subtype: "success",
      result: "短结论",
    }) +
    "\n";

  const out = d.push(chunk);
  assert.match(out, /会话就绪/);
  assert.match(out, /你好/);
  assert.match(out, /⚙ Read/);
  assert.doesNotMatch(out, /secret/);
  assert.doesNotMatch(out, /big file/);
  assert.match(out, /短结论/);
});

test("stream-display 展示 Read 缺 file_path 的完整要点", () => {
  const d = createStreamDisplay();
  const chunk =
    JSON.stringify({
      type: "user",
      message: {
        content: [
          {
            type: "tool_result",
            is_error: true,
            content:
              "<tool_use_error>InputValidationError: Read failed due to the following issue:\nThe required parameter `file_path` is missing</tool_use_error>",
          },
        ],
      },
    }) + "\n";
  const out = d.push(chunk);
  assert.match(out, /读取缺少文件路径|file_path/);
});

test("stream-display 将 No such tool available 中文化", () => {
  const d = createStreamDisplay();
  const chunk =
    JSON.stringify({
      type: "user",
      message: {
        content: [
          {
            type: "tool_result",
            is_error: true,
            content: "No such tool available: mcp__harbor-2__resolve-library-id",
          },
        ],
      },
    }) + "\n";
  const out = d.push(chunk);
  assert.match(out, /未授权该工具|白名单/);
  assert.doesNotMatch(out, /No such tool available/i);
});

test("stream-display 将 File does not exist 中文化", () => {
  const d = createStreamDisplay();
  const chunk =
    JSON.stringify({
      type: "user",
      message: {
        content: [
          {
            type: "tool_result",
            is_error: true,
            content: "<tool_use_error>File does not exist.</tool_use_error>",
          },
        ],
      },
    }) + "\n";
  const out = d.push(chunk);
  assert.match(out, /找不到该文件/);
});

test("stream-display 跨 chunk 拼完整 JSON 行", () => {
  const d = createStreamDisplay();
  const line =
    JSON.stringify({
      type: "assistant",
      message: { content: [{ type: "text", text: "跨行" }] },
    }) + "\n";
  assert.equal(d.push(line.slice(0, 10)), "");
  const out = d.push(line.slice(10));
  assert.match(out, /跨行/);
});

test("stream-display 提取 Codex exec --json 文本与错误", () => {
  const d = createStreamDisplay();
  const chunk = [
    { type: "thread.started", thread_id: "t1" },
    { type: "turn.started" },
    {
      type: "item.completed",
      item: { id: "item_0", type: "command_execution", command: "ls -la" },
    },
    {
      type: "item.completed",
      item: { id: "item_1", type: "agent_message", text: "HELLO_FROM_MOCK" },
    },
    { type: "turn.completed", usage: { input_tokens: 3, output_tokens: 3 } },
    {
      type: "error",
      message: "Reconnecting... 1/5 (unexpected status 401)",
    },
    {
      type: "turn.failed",
      error: { message: "stream disconnected before completion" },
    },
  ]
    .map((o) => JSON.stringify(o))
    .join("\n") + "\n";
  const out = d.push(chunk);
  assert.match(out, /Codex 会话就绪/);
  assert.match(out, /本轮开始/);
  assert.match(out, /⚙ exec · ls -la/);
  assert.match(out, /HELLO_FROM_MOCK/);
  assert.match(out, /本轮结束/);
  assert.match(out, /Reconnecting/);
  assert.match(out, /stream disconnected/);
});

test("stream-display 流式输出 Codex agent_message.delta", () => {
  const d = createStreamDisplay();
  let out = d.push(
    [
      { type: "turn.started" },
      { type: "agent_message.delta", delta: "你" },
      { type: "agent_message.delta", delta: "好" },
      {
        type: "item.completed",
        item: { type: "agent_message", text: "你好" },
        streamed: true,
      },
      { type: "turn.completed" },
    ]
      .map((o) => JSON.stringify(o))
      .join("\n") + "\n",
  );
  assert.match(out, /你好/);
  assert.match(out, /本轮结束/);
  // 流式后不应再整段重复打印
  assert.equal((out.match(/你好/g) || []).length, 1);
});

test("stream-display 展示 dsh conversation JSONL", () => {
  const d = createStreamDisplay();
  const out = d.push(
    [
      { type: "ready", sessionId: "harbor-1" },
      { type: "text-delta", text: "深" },
      { type: "text-delta", text: "度" },
      { type: "reasoning-delta", text: "hidden" },
      { type: "final", sessionId: "harbor-1", round: 1, text: "深度" },
    ]
      .map((o) => JSON.stringify(o))
      .join("\n") + "\n",
  );
  assert.match(out, /DeepSeek dsh 会话就绪/);
  assert.match(out, /深度/);
  assert.match(out, /本轮结束/);
  assert.doesNotMatch(out, /hidden/);
  assert.equal((out.match(/深度/g) || []).length, 1);
});

test("stream-display dsh 无流式时展示 final 全文", () => {
  const d = createStreamDisplay();
  const out = d.push(
    JSON.stringify({ type: "final", round: 1, text: "仅终稿" }) + "\n",
  );
  assert.match(out, /仅终稿/);
  assert.match(out, /本轮结束/);
});

test("stream-display 折叠 Codex rmcp MCP 502 噪声", () => {
  const d = createStreamDisplay();
  const noise =
    '2026-10-30T11:53:02.557165Z ERROR rmcp::transport::worker: worker quit with fatal: Transport channel closed, when UnexpectedServer Response("HTTP 502: write |1: broken pipe\\n")\n';
  const out = d.push(noise + noise + noise);
  assert.match(out, /MCP 连接曾中断/);
  assert.equal((out.match(/MCP 连接曾中断/g) || []).length, 1);
  assert.doesNotMatch(out, /rmcp::transport/);
});

test("stream-display dsh error 行", () => {
  const d = createStreamDisplay();
  const out = d.push(
    JSON.stringify({ type: "error", message: "dsh 会话执行失败。" }) + "\n",
  );
  assert.match(out, /dsh 会话执行失败/);
});
