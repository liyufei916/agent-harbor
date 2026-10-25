import * as api from "../api.js";

/** 评审 / D4 固定演示任务（非敏感）；live 必须改写工具名，见 demos/README */
export const DEMO_TASK =
  "在工作区创建或覆盖 hello.txt，写入一行 Hello from Agent Harbor mock。然后用已连接的 MCP 做一次只读 search，把查询结论用两句话总结。不要修改其它文件，不要请求扩权。";

/** live 任务若仍含这些词，开始前给出提示（不强制拦截） */
const LIVE_TASK_STALE = /\bmock\b|\bsearch\b/i;

/**
 * 配置页：连接模型/MCP、选工作区、任务、启动
 * @param {{ embedded?: boolean }} [opts] 嵌在新会话向导时隐藏顶栏标题
 */
export function mountSetup(root, ctx, opts = {}) {
  const embedded = !!opts.embedded;
  root.innerHTML = `
    ${
      embedded
        ? ""
        : `<h2>配置</h2>
    <p class="sub">连接返回后清空凭据；切换 Agent 后需要重新连接对应模型。Claude 用 demo-model，dsh 用 dsh-model；MCP 固定 demo-mcp。</p>`
    }
    <div class="grid">
      <section class="card">
        <h3>Agent</h3>
        <label class="field">执行 Agent
          <select id="agent-select"></select>
        </label>
        <p class="meta" id="agent-status">检测中…</p>
        <div class="actions">
          <button type="button" class="btn" id="btn-refresh-agents">重新检测</button>
        </div>
      </section>
      <section class="card" id="mock-scenario-card">
        <h3>模拟场景（仅 mock）</h3>
        <label class="field">场景
          <select id="scenario">
            <option value="normal">正常闭环</option>
            <option value="auth">认证失败</option>
            <option value="busy">SESSION_BUSY</option>
            <option value="isolation">ISOLATION_UNAVAILABLE</option>
            <option value="cleanup">cleanupComplete=false</option>
          </select>
        </label>
        <p class="meta">对应验收 D4 四个失败场景 + 正常流。切 auth 后需重新点连接。</p>
      </section>
      <section class="card">
        <h3>模型连接</h3>
        <label class="field">显示名
          <input id="model-name" value="演示模型" maxlength="100" />
        </label>
        <label class="field">profileId
          <input id="model-profile" value="demo-model" readonly />
        </label>
        <label class="field">API Key（提交后清空）
          <input id="model-key" type="password" placeholder="示例：mock-model-key-not-secret" autocomplete="off" />
        </label>
        <div class="row">
          <button type="button" class="btn primary" id="btn-connect-model">连接模型</button>
          <span class="meta" id="model-status">未连接</span>
        </div>
      </section>
      <section class="card">
        <h3>MCP 连接</h3>
        <label class="field">显示名
          <input id="mcp-name" value="演示 MCP" maxlength="100" />
        </label>
        <label class="field">profileId
          <input id="mcp-profile" value="demo-mcp" readonly />
        </label>
        <label class="field">API Key / Token（提交后清空）
          <input id="mcp-key" type="password" placeholder="示例：mock-mcp-token-not-secret" autocomplete="off" />
        </label>
        <div class="row">
          <button type="button" class="btn primary" id="btn-connect-mcp">连接 MCP</button>
          <span class="meta" id="mcp-status">未连接</span>
        </div>
      </section>
    </div>
    <section class="card" style="margin-top:12px">
      <h3>工作区与任务</h3>
      <p class="meta warn">direct 模式会直接修改所选文件夹中的文件。</p>
      <div class="row" style="margin:10px 0">
        <button type="button" class="btn" id="btn-pick-ws">选择工作区</button>
        <span class="meta" id="ws-path">未选择</span>
      </div>
      <label class="field">任务（将发给 Agent）
        <textarea id="task">${DEMO_TASK}</textarea>
      </label>
      <p class="meta" id="task-hint"></p>
      <div class="actions">
        <button type="button" class="btn primary" id="btn-start" disabled>开始运行</button>
        <span class="meta" id="setup-msg"></span>
      </div>
    </section>
  `;

  const els = {
    agentSelect: root.querySelector("#agent-select"),
    modelProfile: root.querySelector("#model-profile"),
    agentStatus: root.querySelector("#agent-status"),
    scenarioCard: root.querySelector("#mock-scenario-card"),
    scenario: root.querySelector("#scenario"),
    modelName: root.querySelector("#model-name"),
    modelKey: root.querySelector("#model-key"),
    modelStatus: root.querySelector("#model-status"),
    mcpName: root.querySelector("#mcp-name"),
    mcpKey: root.querySelector("#mcp-key"),
    mcpStatus: root.querySelector("#mcp-status"),
    wsPath: root.querySelector("#ws-path"),
    task: root.querySelector("#task"),
    taskHint: root.querySelector("#task-hint"),
    start: root.querySelector("#btn-start"),
    msg: root.querySelector("#setup-msg"),
    btnModel: root.querySelector("#btn-connect-model"),
    btnMcp: root.querySelector("#btn-connect-mcp"),
    btnWs: root.querySelector("#btn-pick-ws"),
  };

  function syncModeChrome() {
    const mock = api.isMock();
    if (els.scenarioCard?.classList?.toggle) {
      els.scenarioCard.classList.toggle("hidden", !mock);
    }
    if (els.taskHint) {
      els.taskHint.innerHTML = mock
        ? `模拟模式：可用默认任务。正式 I2 请改用 <code>demos/workspace</code> 与真实工具名（见 demos/README）。`
        : `<span class="warn">真实绑定：请改写任务，去掉 mock/search，工具名须与 HARBOR_DEMO_MCP_TOOLS 一致；工作区选 demos/workspace 绝对路径。</span>`;
    }
  }
  syncModeChrome();

  function startBlockReason() {
    const missing = [];
    // isolation 场景 Agent 会标不可用，仍允许点「开始」以便展示 CreateSession 的 ISOLATION_UNAVAILABLE
    if (!ctx.state.agentAvailable && api.getMockScenario() !== "isolation") {
      missing.push("可用 Agent");
    }
    if (!ctx.state.modelAccount) missing.push("模型连接");
    if (!ctx.state.mcpAccount) missing.push("MCP 连接");
    if (!ctx.state.workspace) missing.push("工作区");
    if (!els.task.value.trim()) missing.push("任务");
    return missing;
  }

  function refreshStart() {
    const missing = startBlockReason();
    els.start.disabled = missing.length > 0;
    if (missing.length) {
      els.msg.textContent = `开始前还需：${missing.join("、")}`;
    } else if (
      !els.msg.querySelector(".bad") &&
      !els.msg.querySelector(".ok")
    ) {
      els.msg.textContent = "就绪，可开始运行";
    }
  }

  let agents = [];
  function selectAgent(id) {
    const a = agents.find((item) => item.id === id);
    const changed = ctx.state.agentId !== id;
    ctx.state.agentId = id;
    ctx.state.agentAvailable = !!a?.available;
    els.modelProfile.value = a?.modelProfileId || (id === "dsh" ? "dsh-model" : "demo-model");
    if (changed) {
      ctx.state.modelAccount = null;
      els.modelKey.value = "";
      els.modelStatus.textContent = "已切换 Agent，请重新连接模型";
      els.msg.textContent = "";
    }
    els.agentStatus.textContent = a
      ? `${a.name} ${a.version || ""} · ${a.available ? "可用" : `不可用：${a.reason || ""}`}`
      : "无可用 Agent";
    refreshStart();
  }
  els.agentSelect.onchange = () => selectAgent(els.agentSelect.value);

  async function refreshAgents() {
    const res = await api.listAgents();
    if (res.error) {
      els.agentStatus.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
      ctx.state.agentAvailable = false;
      refreshStart();
      return;
    }
    agents = res.agents || [];
    els.agentSelect.replaceChildren(...agents.map((a) => {
      const option = document.createElement("option");
      option.value = a.id;
      option.textContent = `${a.name}${a.available ? "" : "（不可用）"}`;
      return option;
    }));
    const id = agents.some((a) => a.id === ctx.state.agentId)
      ? ctx.state.agentId : agents[0]?.id || "";
    els.agentSelect.value = id;
    selectAgent(id);
  }

  els.scenario.value = api.getMockScenario();
  els.scenario.onchange = () => {
    api.setMockScenario(els.scenario.value);
    const tip =
      els.scenario.value === "auth"
        ? "已切 auth：请重新连接模型/MCP 以看到 UPSTREAM_AUTH_FAILED"
        : `已切换场景：${els.scenario.value}`;
    els.msg.textContent = tip;
    refreshAgents();
  };

  root.querySelector("#btn-refresh-agents").onclick = refreshAgents;

  els.btnModel.onclick = async () => {
    const key = els.modelKey.value;
    const name = els.modelName.value.trim();
    if (!name || !key) {
      els.modelStatus.innerHTML = `<span class="bad">请填写显示名与 API Key</span>`;
      return;
    }
    els.btnModel.disabled = true;
    els.agentSelect.disabled = true;
    try {
      const res = await api.connectModel({
        profileId: els.modelProfile.value,
        name,
        apiKey: key,
      });
      els.modelKey.value = "";
      if (res.error) {
        ctx.state.modelAccount = null;
        els.modelStatus.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
      } else {
        ctx.state.modelAccount = res.account;
        els.modelStatus.innerHTML = `<span class="ok">已连接 · ${res.account.id} · ${res.account.host}</span>`;
      }
    } finally {
      els.btnModel.disabled = false;
      els.agentSelect.disabled = false;
      refreshStart();
    }
  };

  els.btnMcp.onclick = async () => {
    const key = els.mcpKey.value;
    const name = els.mcpName.value.trim();
    if (!name || !key) {
      els.mcpStatus.innerHTML = `<span class="bad">请填写显示名与 Token</span>`;
      return;
    }
    els.btnMcp.disabled = true;
    try {
      const res = await api.connectMcp({
        profileId: "demo-mcp",
        name,
        apiKey: key,
      });
      els.mcpKey.value = "";
      if (res.error) {
        ctx.state.mcpAccount = null;
        els.mcpStatus.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
      } else {
        ctx.state.mcpAccount = res.account;
        els.mcpStatus.innerHTML = `<span class="ok">已连接 · ${res.account.id} · ${res.account.host}</span>`;
      }
    } finally {
      els.btnMcp.disabled = false;
      refreshStart();
    }
  };

  els.btnWs.onclick = async () => {
    els.btnWs.disabled = true;
    try {
      const res = await api.selectWorkspace();
      if (res.error) {
        els.wsPath.innerHTML = `<span class="bad">${res.error.message}</span>`;
        ctx.state.workspace = "";
      } else if (res.cancelled) {
        els.wsPath.textContent = "已取消";
      } else {
        ctx.state.workspace = res.path;
        els.wsPath.textContent = res.path;
      }
    } finally {
      els.btnWs.disabled = false;
      refreshStart();
    }
  };

  els.task.oninput = () => {
    els.msg.textContent = "";
    refreshStart();
  };

  els.start.onclick = async () => {
    const missing = startBlockReason();
    if (missing.length) {
      els.msg.innerHTML = `<span class="bad">无法开始：缺少 ${missing.join("、")}</span>`;
      return;
    }
    const taskText = els.task.value.trim();
    if (!api.isMock() && LIVE_TASK_STALE.test(taskText)) {
      els.msg.innerHTML =
        `<span class="warn">任务仍含 mock/search。真机验收请先改成白名单工具名后再点开始（详见 demos/README）。</span>`;
      return;
    }
    els.start.disabled = true;
    els.msg.textContent = "创建会话…";
    try {
      const res = await api.createSession({
        workspace: ctx.state.workspace,
        agentId: ctx.state.agentId || "claude",
        modelAccountId: ctx.state.modelAccount.id,
        mcpIds: [ctx.state.mcpAccount.id],
        task: taskText,
      });
      if (res.error) {
        els.msg.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
        return;
      }
      ctx.state.sessionId = res.session.sessionId;
      ctx.onSessionCreated(res.session);
      els.msg.innerHTML = `<span class="ok">已进入 ${res.session.state}</span>`;
    } finally {
      refreshStart();
    }
  };

  refreshAgents();
  return { refreshStart, refreshAgents, syncModeChrome };
}
