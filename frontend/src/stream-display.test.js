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
