/**
 * PAGE-DESIGN §5.3 新会话四步：工作区 → Agent → 模型 → 工具（可跳过）
 */
import * as api from "../api.js";

const STEPS = ["workspace", "agent", "model", "tools"];

export function mountWizard(root, ctx) {
  let step = 0;
  const draft = {
    workspace: ctx.state.workspace || "",
    mountMode: "clone",
    agentId: ctx.state.agentId || "claude",
    modelAccountId: ctx.state.modelAccount?.id || "",
    mcpIds: ctx.state.mcpAccount ? [ctx.state.mcpAccount.id] : [],
    task: ctx.state.lastTask || "在工作区完成一项只读检查并写一行摘要到 summary.txt。",
  };

  const wrap = document.createElement("div");
  wrap.className = "wizard";
  root.replaceChildren(wrap);

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

  async function paint() {
    await Promise.all([refreshAgents(), refreshAccounts()]);
    const models = (ctx.state.accounts || []).filter((a) => a.kind === "model");
    const mcps = (ctx.state.accounts || []).filter((a) => a.kind === "mcp");
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
        <label class="radio"><input type="radio" name="mount" value="clone" ${draft.mountMode === "clone" ? "checked" : ""}/> 安全副本（推荐）— 在副本里改，需要时再导回</label>
        <label class="radio"><input type="radio" name="mount" value="direct" ${draft.mountMode === "direct" ? "checked" : ""}/> 直接编辑 — 改动立刻写回该文件夹</label>
        ${draft.mountMode === "direct" ? `<p class="meta warn">直接编辑会修改所选文件夹中的文件。</p>` : ""}
      `;
    } else if (step === 1) {
      const agents = ctx.state.agents || [];
      body = `
        <h2>选择 Agent</h2>
        <p class="sub">预置列表；未安装灰显。<button type="button" class="btn" id="wz-redetect">重新检测</button></p>
        <div class="agent-grid" id="wz-agents">
          ${agents
            .map(
              (a) => `
            <button type="button" class="agent-card ${draft.agentId === a.id ? "on" : ""} ${a.available ? "" : "disabled"}" data-id="${a.id}" ${a.available ? "" : "disabled"}>
              <strong>${a.name}</strong>
              <span class="meta">${a.available ? "● 可用" : "○ 未安装"} ${a.version || ""}</span>
              ${a.available ? "" : `<span class="meta">${a.reason || "请安装后重试"}</span>`}
            </button>`,
            )
            .join("")}
        </div>
      `;
    } else if (step === 2) {
      body = `
        <h2>本会话使用的模型</h2>
        <p class="sub">同一时间只用一个；进入工作台后仍可切换。选项来自「模型」页已添加项。</p>
        <label class="field">模型
          <select id="wz-model" ${models.length ? "" : "disabled"}>
            ${models.map((m) => `<option value="${m.id}" ${draft.modelAccountId === m.id ? "selected" : ""}>${m.name} · ${m.host || m.profileId}</option>`).join("")}
          </select>
        </label>
        ${models.length ? "" : `<p class="meta bad">还没有模型。<button type="button" class="btn" id="wz-go-models">去添加</button></p>`}
      `;
    } else {
      body = `
        <h2>需要哪些工具？（可跳过）</h2>
        <p class="sub">与模型无关；这里是 MCP。未添加的请先去「工具」页。</p>
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

    wrap.querySelector("#wz-home").onclick = () => ctx.navigate("home");
    const back = wrap.querySelector("#wz-back");
    if (back) back.onclick = () => {
      step = Math.max(0, step - 1);
      paint();
    };

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
      wrap.querySelector("#wz-redetect").onclick = () => paint();
      wrap.querySelectorAll(".agent-card[data-id]").forEach((btn) => {
        btn.onclick = () => {
          draft.agentId = btn.getAttribute("data-id");
          ctx.state.agentId = draft.agentId;
          paint();
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
      if (sel) sel.onchange = () => {
        draft.modelAccountId = sel.value;
      };
      const go = wrap.querySelector("#wz-go-models");
      if (go) go.onclick = () => ctx.navigate("models");
      wrap.querySelector("#wz-next").disabled = !models.length;
      wrap.querySelector("#wz-next").onclick = () => {
        draft.modelAccountId = sel?.value || draft.modelAccountId;
        if (!draft.modelAccountId) return;
        step = 3;
        paint();
      };
    } else {
      const go = wrap.querySelector("#wz-go-tools");
      if (go) go.onclick = () => ctx.navigate("tools");
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
        const res = await api.createSession({
          workspace: draft.workspace,
          agentId: draft.agentId,
          modelAccountId: draft.modelAccountId,
          mcpIds: draft.mcpIds,
          task: draft.task,
          mountMode: draft.mountMode,
        });
        if (res.error) {
          const hint =
            res.error.code === "SESSION_LIMIT"
              ? "请先在 HOME 停止一个会话。"
              : res.error.code === "WORKSPACE_BUSY"
                ? "请换「演示路径 B」或打开已有会话。"
                : "";
          msg.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>${hint ? `<p class="meta">${hint}</p>` : ""}`;
          return;
        }
        ctx.state.lastTask = draft.task;
        ctx.onSessionCreated(res.session);
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
