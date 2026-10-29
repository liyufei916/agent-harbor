// Harbor 多轮桥：长驻 `codex app-server`，stdin 每行一条 JSON 提示字符串。
// 对外输出 exec 风格 JSONL（供 stream-display），并把 agentMessage delta 实时转出。
import { spawn } from "node:child_process";
import readline from "node:readline";

function argValue(flag) {
  const i = process.argv.indexOf(flag);
  if (i < 0 || i + 1 >= process.argv.length) return "";
  return process.argv[i + 1];
}

const codexScript = argValue("--codex");
const model = argValue("--model");
if (!codexScript || !model) {
  process.stderr.write("harbor-turn: missing --codex or --model\n");
  process.exit(2);
}

const cwd = process.cwd();
let nextId = 1;
const pending = new Map();
const turnWaiters = [];
let threadId = null;

function emit(obj) {
  process.stdout.write(JSON.stringify(obj) + "\n");
}

function parsePrompt(line) {
  const trimmed = line.trim();
  if (!trimmed) return "";
  try {
    const v = JSON.parse(trimmed);
    return typeof v === "string" ? v : "";
  } catch {
    return trimmed;
  }
}

function mapItem(item) {
  if (!item || typeof item !== "object") return null;
  const t = item.type;
  if (t === "agentMessage" || t === "agent_message") {
    return { id: item.id, type: "agent_message", text: item.text || "" };
  }
  if (t === "commandExecution" || t === "command_execution") {
    return {
      id: item.id,
      type: "command_execution",
      command: item.command || item.cmd || "",
      status: item.status || "",
      exit_code: item.exitCode ?? item.exit_code,
    };
  }
  if (t === "fileChange" || t === "file_change") {
    return {
      id: item.id,
      type: "file_change",
      path: item.path || item.file_path || "",
    };
  }
  if (t === "error") {
    return { id: item.id, type: "error", message: item.message || item.text || "error" };
  }
  if (t === "userMessage" || t === "user_message" || t === "reasoning") {
    return null;
  }
  return {
    id: item.id,
    type: t || "item",
    text: item.text || item.message || item.command || "",
  };
}

const child = spawn(
  process.execPath,
  [codexScript, "app-server", "--listen", "stdio://"],
  { cwd, env: process.env, stdio: ["pipe", "pipe", "pipe"] },
);

child.on("error", (err) => {
  process.stderr.write(`harbor-turn: app-server spawn failed: ${err.message}\n`);
  process.exit(1);
});
child.stderr.on("data", (chunk) => process.stderr.write(chunk));
child.on("exit", (code, signal) => {
  process.exit(signal ? 1 : code ?? 1);
});

function rpc(method, params) {
  const id = nextId++;
  const payload = params === undefined ? { method, id } : { method, id, params };
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    child.stdin.write(JSON.stringify(payload) + "\n");
  });
}

function notify(method, params) {
  const payload = params === undefined ? { method } : { method, params };
  child.stdin.write(JSON.stringify(payload) + "\n");
}

function handleServerLine(line) {
  const trimmed = line.trim();
  if (!trimmed) return;
  let msg;
  try {
    msg = JSON.parse(trimmed);
  } catch {
    return;
  }

  if (
    msg.id != null &&
    (Object.prototype.hasOwnProperty.call(msg, "result") ||
      Object.prototype.hasOwnProperty.call(msg, "error"))
  ) {
    const waiter = pending.get(msg.id);
    if (!waiter) return;
    pending.delete(msg.id);
    if (msg.error) waiter.reject(new Error(msg.error.message || JSON.stringify(msg.error)));
    else waiter.resolve(msg.result);
    return;
  }

  const method = msg.method;
  const params = msg.params || {};

  if (method === "turn/completed") {
    const status = params.turn?.status;
    if (status && status !== "completed") {
      emit({
        type: "turn.failed",
        error: { message: String(params.turn?.error?.message || status) },
      });
    } else {
      emit({ type: "turn.completed" });
    }
    while (turnWaiters.length) turnWaiters.shift()();
    return;
  }

  if (method === "item/agentMessage/delta") {
    if (params.delta) emit({ type: "agent_message.delta", delta: params.delta });
    return;
  }

  if (method === "item/started" || method === "item/completed") {
    const mapped = mapItem(params.item);
    if (!mapped) return;
    if (mapped.type === "agent_message") {
      if (method === "item/completed") {
        emit({ type: "item.completed", item: mapped, streamed: true });
      }
      return;
    }
    emit({
      type: method === "item/started" ? "item.started" : "item.completed",
      item: mapped,
    });
    return;
  }

  if (method === "error") {
    emit({ type: "error", message: params.message || "error" });
  }
}

const serverOut = readline.createInterface({
  input: child.stdout,
  crlfDelay: Infinity,
});
serverOut.on("line", handleServerLine);

function waitTurnCompleted() {
  return new Promise((resolve) => {
    turnWaiters.push(resolve);
  });
}

async function bootstrap() {
  await rpc("initialize", {
    clientInfo: { name: "agent_harbor", title: "Agent Harbor", version: "0.1.0" },
  });
  notify("initialized");
  const started = await rpc("thread/start", {
    model,
    cwd,
    sandbox: "danger-full-access",
    approvalPolicy: "never",
  });
  threadId = started?.thread?.id;
  if (!threadId) throw new Error("thread/start missing thread id");
  emit({ type: "thread.started", thread_id: threadId });
}

async function runTurn(prompt) {
  emit({ type: "turn.started" });
  const done = waitTurnCompleted();
  await rpc("turn/start", {
    threadId,
    input: [{ type: "text", text: prompt }],
  });
  await done;
}

await bootstrap();

const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
for await (const line of rl) {
  const prompt = parsePrompt(line);
  if (!prompt) continue;
  try {
    await runTurn(prompt);
  } catch (err) {
    emit({ type: "error", message: err.message || String(err) });
    emit({ type: "turn.failed", error: { message: err.message || String(err) } });
  }
}

try {
  child.stdin.end();
} catch {
  /* ignore */
}
