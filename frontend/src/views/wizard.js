/**
 * PAGE-DESIGN §5.3 新会话四步：工作区 → Agent → 模型 → 工具（可跳过）
 * 模型按账号 agents[] / profile 过滤；MCP 可跨 Agent 勾选。
 */
import * as api from "../api.js";
import { expectedModelProfile, filterModelsForAgent } from "../agent-compat.js";
import { AGENT_INSTALL, copyText } from "../agent-install.js";
import { showToast } from "../toast.js";

const STEPS = ["workspace", "agent", "model", "tools"];

export function mountWizard(root, ctx, opts = {}) {
  if (opts.fresh) {
    ctx.state.wizardResume = null;
  }

  let defaultMount = "clone";
  try {
    defaultMount = localStorage.getItem("harbor-default-mount") || "clone";
  } catch (_) {
    /* ignore */
  }
  if (defaultMount !== "direct" && defaultMount !== "clone") defaultMount = "clone";

  const resume = ctx.state.wizardResume;
  let step = Number.isInteger(resume?.step) ? resume.step : 0;
  if (step < 0 || step > 3) step = 0;

  const draft = {
    workspace: resume?.draft?.workspace || ctx.state.workspace || "",
    mountMode: resume?.draft?.mountMode || defaultMount,
    agentId: resume?.draft?.agentId || ctx.state.agentId || "claude",
    modelAccountId: resume?.draft?.modelAccountId || ctx.state.modelAccount?.id || "",
    mcpIds: Array.isArray(resume?.draft?.mcpIds)
      ? [...resume.draft.mcpIds]
      : ctx.state.mcpAccount
        ? [ctx.state.mcpAccount.id]
        : [],
    task:
      resume?.draft?.task ||
      ctx.state.lastTask ||
      "在工作区完成一项只读检查并写一行摘要到 summary.txt。",
  };

  const wrap = document.createElement("div");
  wrap.className = "wizard";
  root.replaceChildren(wrap);

  function persist() {
    ctx.state.wizardResume = {
      step,
      draft: {
        workspace: draft.workspace,
        mountMode: draft.mountMode,
        agentId: draft.agentId,
        modelAccountId: draft.modelAccountId,
        mcpIds: [...draft.mcpIds],
        task: draft.task,
      },
    };
  }

  async function refreshAccounts() {
    const res = await api.listAccounts("");
    ctx.state.accounts = res.accounts || [];
  }

  async function refreshAgents() {
    const res = await api.listAgents();
    ctx.state.agents = res.agents || [];
    const cur = ctx.state.agents.find((a) => a.id === draft.agentId);
    if (cur) ctx.state.agentAvailable = !!cur.available;
  }

  function renderSteps() {
    return `<ol class="wizard-steps">${STEPS.map((s, i) => {
      const labels = { workspace: "工作区", agent: "Agent", model: "模型", tools: "工具" };
      return `<li class="${i === step ? "on" : i < step ? "done" : ""}">${labels[s]}</li>`;
    }).join("")}</ol>`;
  }

  function goAddAccount(kind) {
    persist();
    ctx.state.returnAfterAccount = { kind, route: "wizard" };
    ctx.navigate(kind === "model" ? "models" : "tools", { intent: "add" });
  }

  async function paint() {
    persist();
    await Promise.all([refreshAgents(), refreshAccounts()]);
    const agents = ctx.state.agents || [];
    const allModels = (ctx.state.accounts || []).filter((a) => a.kind === "model");
    const models = filterModelsForAgent(allModels, draft.agentId, agents);
    const mcps = (ctx.state.accounts || []).filter((a) => a.kind === "mcp");
    if (!draft.modelAccountId && models[0]) {
      draft.modelAccountId = models[0].id;
    }
    if (draft.modelAccountId && !models.some((m) => m.id === draft.modelAccountId)) {
      draft.modelAccountId = models[0]?.id || "";
    }
    let body = "";

    if (step === 0) {
      body = `
        <h2>选择工作区</h2>
        <p class="sub">选择要隔离运行的项目文件夹。</p>
        <div class="card drop-zone">
          <button type="button" class="btn primary" id="wz-pick">选择文件夹（mock）</button>
          <button type="button" class="btn" id="wz-pick-b" title="第二条演示路径，用于双会话">演示路径 B</button>
          <p class="meta" id="wz-path">${draft.workspace || "未选择"}</p>
        </div>
        <p class="meta">浏览器 mock 无系统选目录；桌面 live 才弹面板。SSH 密钥等敏感目录不会进入隔离环境。</p>
        <h3>文件怎么进隔离环境？</h3>
        <label class="radio"><input type="radio" name="mount" value="clone" ${draft.mountMode === "clone" ? "checked" : ""}/> 安全副本（推荐）— 在副本里改，工作台可一键导回原文件夹</label>
        <label class="radio"><input type="radio" name="mount" value="direct" ${draft.mountMode === "direct" ? "checked" : ""}/> 直接编辑 — 改动立刻写回该文件夹</label>
        ${draft.mountMode === "direct" ? `<p class="meta warn">直接编辑会修改所选文件夹中的文件。</p>` : ""}
      `;
    } else if (step === 1) {
      const agents = ctx.state.agents || [];
      body = `
        <h2>选择 Agent</h2>
        <p class="sub">预置列表；未安装灰显。<button type="button" class="btn" id="wz-redetect">重新检测</button></p>
        <p class="meta warn">须本机 PATH 可调用对应 CLI（Claude：<code>claude</code>；Codex：<code>codex</code>+<code>node</code>；dsh：固定预览版）。隔离仅 Darwin 25 · arm64 · 非 root。</p>
        <div class="agent-grid" id="wz-agents">
          ${agents
            .map((a) => {
              const install = AGENT_INSTALL[a.id];
              return `
            <div class="agent-card-wrap">
              <button type="button" class="agent-card ${draft.agentId === a.id ? "on" : ""} ${a.available ? "" : "disabled"}" data-id="${a.id}" ${a.available ? "" : "disabled"}>
                <strong>${a.name}</strong>
                <span class="meta">${a.available ? "● 可用" : "○ 未安装"} ${a.version || ""}</span>
                ${a.available ? "" : `<span class="meta">${a.reason || "请安装后重试"}</span>`}
              </button>
              ${
                !a.available && install
                  ? `<button type="button" class="btn" data-copy-install="${a.id}">复制安装命令</button>`
                  : ""
              }
            </div>`;
            })
            .join("")}
        </div>
      `;
    } else if (step === 2) {
      const needProfile = expectedModelProfile(draft.agentId, agents);
      body = `
        <h2>本会话使用的模型</h2>
        <p class="sub">同一时间只用一个。列表仅显示<strong>勾选了当前 Agent</strong>的模型（可在「模型」页多选 Claude/Codex/dsh）。</p>
        <p class="meta">当前 Agent 默认 profile：<code>${needProfile}</code>。多 Agent 共用须上游同时支持对应协议。</p>
        <label class="field">模型
          <select id="wz-model" ${models.length ? "" : "disabled"}>
            ${models.map((m) => `<option value="${m.id}" ${draft.modelAccountId === m.id ? "selected" : ""}>${m.name} · ${m.host || m.profileId}</option>`).join("")}
          </select>
        </label>
        ${
          models.length
            ? `<p class="meta"><button type="button" class="btn" id="wz-go-models">再添加一个</button></p>`
            : `<p class="meta bad">没有与当前 Agent 匹配的模型（需要 ${needProfile}${draft.agentId === "claude" ? " 或 custom" : ""}）。<button type="button" class="btn" id="wz-go-models">去添加</button></p>`
        }
      `;
    } else {
      body = `
        <h2>需要哪些工具？（可跳过）</h2>
        <p class="sub">MCP 可跨 Agent 挂载（与模型不同）。未添加的请先去「工具」页。</p>
        <p class="meta">可挂远程 HTTP MCP，或工具页添加的宿主 stdio（本机进程经回环桥）。</p>
        <div id="wz-mcps">
          ${
            mcps.length
              ? mcps
                  .map(
                    (m) => `
            <label class="check-row">
              <input type="checkbox" data-mcp="${m.id}" ${draft.mcpIds.includes(m.id) ? "checked" : ""}/>
              ${m.name} · ● 已连接
            </label>`,
                  )
                  .join("")
              : `<p class="meta">尚未添加 MCP。<button type="button" class="btn" id="wz-go-tools">去添加</button> 或直接跳过。</p>`
          }
        </div>
        <label class="field">任务
          <textarea id="wz-task">${draft.task}</textarea>
        </label>
        <p class="meta" id="wz-msg"></p>
      `;
    }

    const nav =
      step < 3
        ? `<div class="actions">
            <button type="button" class="btn" id="wz-back" ${step === 0 ? "disabled" : ""}>上一步</button>
            <button type="button" class="btn" id="wz-home">返回 HOME</button>
            <button type="button" class="btn primary" id="wz-next">下一步</button>
          </div>`
        : `<div class="actions">
            <button type="button" class="btn" id="wz-back">上一步</button>
            <button type="button" class="btn" id="wz-skip">跳过工具</button>
            <button type="button" class="btn primary" id="wz-start">开始运行</button>
            <button type="button" class="btn" id="wz-home">返回 HOME</button>
          </div>`;

    wrap.innerHTML = `${renderSteps()}${body}${nav}`;

    wrap.querySelector("#wz-home").onclick = () => {
      persist();
      ctx.navigate("home");
    };
    const back = wrap.querySelector("#wz-back");
    if (back) {
      back.onclick = () => {
        step = Math.max(0, step - 1);
        paint();
      };
    }

    if (step === 0) {
      const applyPath = (path) => {
        draft.workspace = path;
        ctx.state.workspace = path;
        paint();
      };
      wrap.querySelector("#wz-pick").onclick = async () => {
        const res = await api.selectWorkspace();
        if (!res.cancelled && !res.error) applyPath(res.path);
      };
      wrap.querySelector("#wz-pick-b").onclick = () => applyPath("/demo/project-b");
      wrap.querySelectorAll('input[name="mount"]').forEach((el) => {
        el.onchange = () => {
          draft.mountMode = el.value;
          paint();
        };
      });
      wrap.querySelector("#wz-next").onclick = () => {
        if (!draft.workspace) return;
        step = 1;
        paint();
      };
      wrap.querySelector("#wz-next").disabled = !draft.workspace;
    } else if (step === 1) {
      wrap.querySelector("#wz-redetect").onclick = async () => {
        const btn = wrap.querySelector("#wz-redetect");
        if (btn) {
          btn.disabled = true;
          btn.textContent = "检测中…";
        }
        await refreshAgents();
        await paint();
      };
      wrap.querySelectorAll(".agent-card[data-id]").forEach((btn) => {
        btn.onclick = () => {
          const next = btn.getAttribute("data-id");
          if (next !== draft.agentId) {
            draft.agentId = next;
            ctx.state.agentId = draft.agentId;
            // 换 Agent 后旧模型多半不兼容，清空以免带到下一步。
            draft.modelAccountId = "";
            ctx.state.modelAccount = null;
          }
          paint();
        };
      });
      wrap.querySelectorAll("[data-copy-install]").forEach((btn) => {
        btn.onclick = async () => {
          const id = btn.getAttribute("data-copy-install");
          const okCopy = await copyText(AGENT_INSTALL[id]?.cmd || "");
          showToast(okCopy ? "已复制安装命令" : "复制失败，请手动选择命令", okCopy ? "ok" : "bad");
        };
      });
      const ok = (ctx.state.agents || []).some((a) => a.id === draft.agentId && a.available);
      wrap.querySelector("#wz-next").disabled = !ok;
      wrap.querySelector("#wz-next").onclick = () => {
        step = 2;
        paint();
      };
    } else if (step === 2) {
      const sel = wrap.querySelector("#wz-model");
      if (sel) {
        sel.onchange = () => {
          draft.modelAccountId = sel.value;
          persist();
        };
      }
      const go = wrap.querySelector("#wz-go-models");
      if (go) go.onclick = () => goAddAccount("model");
      wrap.querySelector("#wz-next").disabled = !models.length;
      wrap.querySelector("#wz-next").onclick = () => {
        draft.modelAccountId = sel?.value || draft.modelAccountId;
        if (!draft.modelAccountId) return;
        step = 3;
        paint();
      };
    } else {
      const go = wrap.querySelector("#wz-go-tools");
      if (go) go.onclick = () => goAddAccount("mcp");
      const syncMcp = () => {
        draft.mcpIds = [...wrap.querySelectorAll("[data-mcp]:checked")].map((el) =>
          el.getAttribute("data-mcp"),
        );
      };
      wrap.querySelectorAll("[data-mcp]").forEach((el) => {
        el.onchange = syncMcp;
      });
      const start = async (skipTools) => {
        syncMcp();
        if (skipTools) draft.mcpIds = [];
        draft.task = wrap.querySelector("#wz-task")?.value.trim() || draft.task;
        const msg = wrap.querySelector("#wz-msg");
        if (!draft.workspace || !draft.modelAccountId || !draft.task) {
          msg.innerHTML = `<span class="bad">请补全工作区、模型与任务</span>`;
          return;
        }
        msg.textContent = "创建会话…";
        const createSpec = {
          workspace: draft.workspace,
          agentId: draft.agentId,
          modelAccountId: draft.modelAccountId,
          mcpIds: draft.mcpIds,
          task: draft.task,
          mountMode: draft.mountMode,
        };
        const res = await api.createSession(createSpec);
        if (res.error) {
          let hint = "";
          let cta = "";
          if (res.error.code === "SESSION_LIMIT") {
            hint = "运行中已满 2 个。回 HOME 停止其中一个后再试。";
            cta = `<button type="button" class="btn" id="wz-to-home">回 HOME</button>`;
          } else if (res.error.code === "WORKSPACE_BUSY") {
            hint =
              "该文件夹已有会话在跑。打开已有工作台，或上一步改选「演示路径 B」。";
            cta = `<button type="button" class="btn" id="wz-to-home">回 HOME 打开会话</button>`;
          } else if (res.error.code === "WORKSPACE_DENIED") {
            hint =
              "工作区须是已存在的普通目录；不要选 Home、.ssh、系统目录。演示可用 demos/workspace 的绝对路径。";
          } else if (res.error.code === "INVALID_ARGUMENT") {
            if (draft.agentId === "codex") {
              hint =
                "Codex 需要已通过 Responses（/v1/responses）探测的模型账号。MCP 可选（工具:0 正常）。DeepSeek Anthropic 账号不可用；请确认「模型」页该账号勾选了 Codex。";
              cta = `<button type="button" class="btn" id="wz-to-models">去模型</button>`;
            } else {
              hint =
                "该模型可能只勾了 Codex。Claude 请选用 Anthropic 账号，或到「模型」勾选 Claude 后重存。自定义 MCP 须带工具列表。";
              cta = `<button type="button" class="btn" id="wz-to-models">去模型</button> <button type="button" class="btn" id="wz-to-tools">去工具</button>`;
            }
          }
          msg.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>${hint ? `<p class="meta">${hint}</p>` : ""}${cta ? `<div class="actions">${cta}</div>` : ""}`;
          msg.querySelector("#wz-to-home")?.addEventListener("click", () =>
            ctx.navigate("home"),
          );
          msg.querySelector("#wz-to-tools")?.addEventListener("click", () =>
            ctx.navigate("tools"),
          );
          msg.querySelector("#wz-to-models")?.addEventListener("click", () =>
            ctx.navigate("models", { intent: "add" }),
          );
          return;
        }
        ctx.state.lastTask = draft.task;
        ctx.state.wizardResume = null;
        ctx.state.returnAfterAccount = null;
        ctx.onSessionCreated(res.session, createSpec);
      };
      wrap.querySelector("#wz-skip").onclick = () => start(true);
      wrap.querySelector("#wz-start").onclick = () => start(false);
    }
  }

  paint();
  return {
    refreshAgents: () => paint(),
  };
}
