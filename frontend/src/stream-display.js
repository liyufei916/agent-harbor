/**
 * 把 Agent 协议 JSON 压成可读行；非 JSON 原样透出。
 * 支持 Claude --print stream-json、Codex exec/app-server JSON、dsh conversation JSONL。
 */

/** 将 Agent 英文工具错误压成可操作中文（不含 Key）。 */
export function humanizeAgentToolError(raw) {
  const text = String(raw || "").replace(/\s+/g, " ").trim();
  const lower = text.toLowerCase();
  if (/not authorized|not allowed|permission|tool.*not.*permit|unknown tool|not in.*allow/i.test(text)) {
    return "本次会话未授权该工具；请到「工具」页确认白名单后重开会话。";
  }
  if (/file does not exist|no such file|enoent|not found.*file/i.test(text)) {
    return "找不到该文件；确认路径在当前工作区（clone 模式下为副本目录），或先创建再读取。";
  }
  if (/file_path.*missing|required parameter.*file_path/i.test(text)) {
    return "读取缺少文件路径参数；请改用工作区内的明确路径重试。";
  }
  if (/inputvalidationerror/i.test(text)) {
    const brief = text.replace(/^inputvalidationerror:\s*/i, "").slice(0, 120);
    return `工具参数不合法：${brief || "请检查路径与必填项后重试"}。`;
  }
  if (/eperm|operation not permitted|sandbox/i.test(text) || lower.includes("denied")) {
    return "系统隔离拒绝该操作（工作区外读写或未授权出站）；改回工作区内路径，或在扩权弹窗中决策。";
  }
  const lines = text.split(/[.\n]/).map((l) => l.trim()).filter(Boolean);
  const err = (lines.slice(0, 2).join(" · ") || text || "工具失败").slice(0, 180);
  return err;
}

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
        out += `✗ ${humanizeAgentToolError(raw)}\r\n`;
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

/** dsh harbor-conversation.mjs：ready / text-delta / final / error */
function formatDshEvent(obj, state) {
  const type = obj.type;
  if (type === "ready") {
    return `● DeepSeek dsh 会话就绪\r\n`;
  }
  if (type === "text-delta" && obj.text) {
    state.streaming = true;
    return String(obj.text);
  }
  if (type === "reasoning-delta") {
    return "";
  }
  if (type === "final") {
    const streamed = state.streaming;
    state.streaming = false;
    if (streamed) {
      return `\r\n—— 本轮结束\r\n`;
    }
    const text = obj.text != null ? String(obj.text).trim() : "";
    if (text) {
      return `${text}\r\n—— 本轮结束\r\n`;
    }
    return `—— 本轮结束\r\n`;
  }
  if (type === "error" && obj.message) {
    const suffix = state.streaming ? "\r\n" : "";
    state.streaming = false;
    return `${suffix}✗ ${String(obj.message).slice(0, 240)}\r\n`;
  }
  return null;
}

function formatEvent(obj, state) {
  if (!obj || typeof obj !== "object") return "";
  const dsh = formatDshEvent(obj, state);
  if (dsh != null) return dsh;
  const claude = formatClaudeEvent(obj);
  if (claude != null) return claude;
  const codex = formatCodexEvent(obj, state);
  if (codex != null) return codex;
  return "";
}

/** Codex rmcp 连 MCP 失败时打到 stderr 的噪声（常因宿主 stdio 二次 initialize）。 */
function isCodexMcpTransportNoise(line) {
  const s = String(line || "");
  if (/rmcp::transport::worker/i.test(s)) return true;
  if (/Transport channel closed/i.test(s) && /502|broken pipe/i.test(s)) return true;
  return false;
}

export function createStreamDisplay() {
  let buf = "";
  const state = { streaming: false, mcpNoiseHinted: false };
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
        if (isCodexMcpTransportNoise(trimmed)) {
          if (!state.mcpNoiseHinted) {
            state.mcpNoiseHinted = true;
            out += `△ MCP 连接曾中断（多见于宿主 stdio）；本轮可继续。若工具不可用，请到「工具」页检查后重开。\r\n`;
          }
          continue;
        }
        out += `${line}\r\n`;
      }
      return out;
    },
    reset() {
      buf = "";
      state.streaming = false;
      state.mcpNoiseHinted = false;
    },
  };
}
