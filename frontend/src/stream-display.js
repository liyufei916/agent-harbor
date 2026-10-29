/**
 * 把 Agent 协议 JSON 压成可读行；非 JSON 原样透出。
 * 支持 Claude --print stream-json 与 Codex exec --json。
 */

function contentParts(message) {
  const parts = message?.content;
  if (!Array.isArray(parts)) return [];
  return parts;
}

function formatClaudeEvent(obj) {
  const type = obj.type;

  if (type === "system" && obj.subtype === "init") {
    const tools = Array.isArray(obj.tools) ? obj.tools.join(", ") : "";
    return `● 会话就绪 · ${obj.model || "?"} · 工具[${tools || "无"}]\r\n`;
  }

  if (type === "assistant" && obj.message) {
    let out = "";
    for (const part of contentParts(obj.message)) {
      if (part?.type === "text" && part.text) {
        out += `${part.text}\r\n`;
      } else if (part?.type === "tool_use") {
        const name = part.name || "tool";
        const input = part.input && typeof part.input === "object" ? part.input : {};
        const hint =
          input.file_path ||
          input.path ||
          input.command ||
          input.pattern ||
          "";
        out += `⚙ ${name}${hint ? ` · ${String(hint).slice(0, 80)}` : ""}\r\n`;
      }
    }
    return out;
  }

  if (type === "user" && obj.message?.content) {
    let out = "";
    for (const part of contentParts(obj.message)) {
      if (part?.type !== "tool_result") continue;
      if (part.is_error) {
        const raw = String(part.content || "error")
          .replace(/<\/?tool_use_error>/g, "")
          .trim();
        const lines = raw.split("\n").map((l) => l.trim()).filter(Boolean);
        const err = (lines.slice(0, 3).join(" · ") || "error").slice(0, 240);
        out += `✗ ${err}\r\n`;
      }
    }
    return out;
  }

  if (type === "result") {
    const text = obj.result != null ? String(obj.result).trim() : "";
    if (!text) {
      return obj.is_error ? `✗ 本轮失败\r\n` : "";
    }
    if (text.length > 400) {
      return `—— 本轮结束\r\n`;
    }
    return `—— ${text}\r\n`;
  }

  return null;
}

function formatCodexItem(item) {
  if (!item || typeof item !== "object") return "";
  const kind = item.type || "item";
  if (kind === "agent_message" && item.text) {
    return `${item.text}\r\n`;
  }
  if (kind === "error") {
    const msg = item.message || item.text || "error";
    return `✗ ${String(msg).slice(0, 240)}\r\n`;
  }
  if (kind === "command_execution") {
    const cmd = item.command || item.cmd || "";
    let status = "";
    if (item.exit_code != null) status = `exit ${item.exit_code}`;
    else if (item.status) status = String(item.status);
    return `⚙ exec${cmd ? ` · ${String(cmd).slice(0, 80)}` : ""}${status ? ` · ${status}` : ""}\r\n`;
  }
  if (kind === "file_change") {
    const path = item.path || item.file_path || item.filename || "";
    return `⚙ patch${path ? ` · ${String(path).slice(0, 80)}` : ""}\r\n`;
  }
  if (kind === "mcp_tool") {
    const name = item.name || item.tool || "mcp";
    return `⚙ ${name}\r\n`;
  }
  if (kind === "reasoning") {
    return "";
  }
  const hint = item.text || item.message || item.command || item.path || "";
  return hint ? `· ${kind}: ${String(hint).slice(0, 120)}\r\n` : "";
}

function formatCodexEvent(obj, state) {
  const type = obj.type;

  if (type === "thread.started") {
    return `● Codex 会话就绪\r\n`;
  }
  if (type === "turn.started") {
    return `—— 本轮开始\r\n`;
  }
  if (type === "turn.completed") {
    const suffix = state.streaming ? "\r\n" : "";
    state.streaming = false;
    return `${suffix}—— 本轮结束\r\n`;
  }
  if (type === "turn.failed") {
    const msg = obj.error?.message || obj.message || "本轮失败";
    const suffix = state.streaming ? "\r\n" : "";
    state.streaming = false;
    return `${suffix}✗ ${String(msg).slice(0, 240)}\r\n`;
  }
  if (type === "error" && obj.message) {
    return `✗ ${String(obj.message).slice(0, 240)}\r\n`;
  }
  // app-server 流式增量：直接吐字，不换行
  if (type === "agent_message.delta" && obj.delta) {
    state.streaming = true;
    return String(obj.delta);
  }
  if (type === "item.completed" || type === "item.started" || type === "item.updated") {
    if (obj.item?.type === "agent_message" && (obj.streamed || state.streaming)) {
      // 正文已通过 delta 输出，只补换行
      state.streaming = false;
      return type === "item.completed" ? `\r\n` : "";
    }
    return formatCodexItem(obj.item);
  }
  return null;
}

function formatEvent(obj, state) {
  if (!obj || typeof obj !== "object") return "";
  const claude = formatClaudeEvent(obj);
  if (claude != null) return claude;
  const codex = formatCodexEvent(obj, state);
  if (codex != null) return codex;
  return "";
}

export function createStreamDisplay() {
  let buf = "";
  const state = { streaming: false };
  return {
    push(chunk) {
      if (!chunk) return "";
      buf += chunk;
      let out = "";
      for (;;) {
        const nl = buf.indexOf("\n");
        if (nl < 0) break;
        const line = buf.slice(0, nl).replace(/\r$/, "");
        buf = buf.slice(nl + 1);
        const trimmed = line.trim();
        if (!trimmed) continue;
        if (trimmed.startsWith("{")) {
          try {
            out += formatEvent(JSON.parse(trimmed), state);
            continue;
          } catch {
            out += `${trimmed.slice(0, 160)}${trimmed.length > 160 ? "…" : ""}\r\n`;
            continue;
          }
        }
        out += `${line}\r\n`;
      }
      return out;
    },
    reset() {
      buf = "";
      state.streaming = false;
    },
  };
}
