/**
 * PAGE-DESIGN §5.7 模型：自定义 OpenAPI 或预置 Agent profile。
 * Live 自定义 origin 须 Anthropic 兼容 /v1/messages；预置 profile 的
 * 上游由宿主环境登记，页面只接收显示名和 Key。
 */
import * as api from "../api.js";

const PRESETS = [
  { name: "Anthropic", origin: "https://api.anthropic.com", model: "claude-haiku-4-5-20251001" },
  // DeepSeek Anthropic 兼容基址（探测 POST …/anthropic/v1/messages，模型 deepseek-chat）
  { name: "DeepSeek", origin: "https://api.deepseek.com/anthropic", model: "deepseek-chat" },
  // 两套七牛网关 Key 不互通：官方 Token API vs LiteLLM
  { name: "七牛官方", origin: "https://api.qnaigc.com", model: "" },
  { name: "七牛LiteLLM", origin: "https://llmapi.qiniu.io", model: "GPT-5.6" },
];

export function mountModels(root, ctx, opts = {}) {
  const returning = ctx.state.returnAfterAccount?.kind === "model";
  let editing = opts.intent === "add" || returning ? "new" : null;
  let pendingDelete = "";
  let listStatus = "";

  function clearReturn() {
    if (ctx.state.returnAfterAccount?.kind === "model") {
      ctx.state.returnAfterAccount = null;
    }
  }

  function resumeWizard(accountId) {
    const ret = ctx.state.returnAfterAccount;
    if (!ret || ret.kind !== "model" || ret.route !== "wizard") return false;
    const prev = ctx.state.wizardResume || { step: 2, draft: {} };
    ctx.state.wizardResume = {
      step: 2,
      draft: {
        ...(prev.draft || {}),
        modelAccountId: accountId || prev.draft?.modelAccountId || "",
      },
    };
    ctx.state.returnAfterAccount = null;
    ctx.navigate("wizard");
    return true;
  }

  async function load() {
    const res = await api.listAccounts("model");
    ctx.state.modelAccounts = res.accounts || [];
    if (ctx.state.modelAccounts[0]) ctx.state.modelAccount = ctx.state.modelAccounts[0];
    paint();
  }

  function paint() {
    const list = ctx.state.modelAccounts || [];
    root.innerHTML = `
      <div class="page-head row" style="justify-content:space-between">
        <div>
          <h2>模型</h2>
          <p class="sub">给 Agent 一把「嘴」。自行添加：名称、API 地址、API Key。无浏览器登录。</p>
        </div>
        <button type="button" class="btn primary" id="btn-add-model">+ 添加</button>
      </div>
      ${
        returning
          ? `<p class="meta ok" id="model-return-banner">添加成功后将回到「新会话」模型步骤。
             <button type="button" class="btn" id="m-back-wizard">直接返回</button></p>`
          : ""
      }
      <div id="model-list" class="session-list"></div>
      ${listStatus ? `<p class="meta" id="model-list-status">${listStatus}</p>` : ""}
      ${
        list.length === 0 && editing !== "new"
          ? `<div class="empty-state"><p>还没有模型。自行添加名称、API 地址与 Key；无浏览器登录。</p>
             <div class="actions"><button type="button" class="btn primary" id="empty-add-model">+ 添加第一个</button></div></div>`
          : ""
      }
      <div id="model-form" class="${editing ? "" : "hidden"}"></div>
    `;

    const listEl = root.querySelector("#model-list");
    list.forEach((a) => {
      const row = document.createElement("article");
      row.className = "card session-card";
      const confirming = pendingDelete === a.id;
      row.innerHTML = `
        <div class="row" style="justify-content:space-between">
          <div>
            <strong>${a.name}</strong>
            <span class="meta">${a.host || a.profileId || ""}</span>
            ${a.invalid ? `<span class="bad">▲ 无效</span>` : ""}
          </div>
          <div class="row">
            <button type="button" class="btn" data-edit="${a.id}">编辑</button>
            ${
              confirming
                ? `<button type="button" class="btn danger" data-del-confirm="${a.id}">确认删除</button>
                   <button type="button" class="btn" data-del-cancel="${a.id}">取消</button>`
                : `<button type="button" class="btn danger" data-del="${a.id}">删除</button>`
            }
          </div>
        </div>`;
      listEl.appendChild(row);
    });

    const startAdd = () => {
      editing = "new";
      pendingDelete = "";
      listStatus = "";
      paint();
      paintForm();
    };
    root.querySelector("#btn-add-model").onclick = startAdd;
    root.querySelector("#empty-add-model")?.addEventListener("click", startAdd);
    root.querySelector("#m-back-wizard")?.addEventListener("click", () => {
      clearReturn();
      ctx.navigate("wizard");
    });
    listEl.querySelectorAll("[data-edit]").forEach((btn) => {
      btn.onclick = () => {
        editing = btn.getAttribute("data-edit");
        pendingDelete = "";
        paint();
        paintForm();
      };
    });
    listEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = () => {
        pendingDelete = btn.getAttribute("data-del") || "";
        listStatus = "再点一次「确认删除」将移除本机 Key（桌面内不用系统确认框）。";
        paint();
      };
    });
    listEl.querySelectorAll("[data-del-cancel]").forEach((btn) => {
      btn.onclick = () => {
        pendingDelete = "";
        listStatus = "";
        paint();
      };
    });
    listEl.querySelectorAll("[data-del-confirm]").forEach((btn) => {
      btn.onclick = async () => {
        const id = btn.getAttribute("data-del-confirm");
        pendingDelete = "";
        listStatus = `<span class="meta">正在删除…</span>`;
        paint();
        const res = await api.removeAccount(id);
        if (res?.error) {
          listStatus = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
          paint();
          return;
        }
        listStatus = "";
        if (editing === id) editing = null;
        await load();
      };
    });
    if (editing) paintForm();
  }

  function paintForm() {
    const form = root.querySelector("#model-form");
    const acc =
      editing === "new"
        ? { name: "", host: "", profileId: ctx.state.modelProfileIntent || "custom" }
        : (ctx.state.modelAccounts || []).find((a) => a.id === editing) || {};
    const originValue =
      acc.origin ||
      (acc.host ? "https://" + acc.host + (acc.basePath || "") : "");
    form.classList.remove("hidden");
    form.innerHTML = `
      <section class="card">
        <h3>${editing === "new" ? "添加模型" : "编辑模型"}</h3>
        <label class="field">名称<input id="m-name" value="${acc.name || ""}" maxlength="100"/></label>
        <label class="field">Agent profile
          <select id="m-profile" ${editing !== "new" ? "disabled" : ""}>
            <option value="custom" ${acc.profileId === "custom" ? "selected" : ""}>自定义模型（Claude）</option>
            <option value="demo-model" ${acc.profileId === "demo-model" ? "selected" : ""}>Claude Code（预置上游）</option>
            <option value="dsh-model" ${acc.profileId === "dsh-model" ? "selected" : ""}>DeepSeek dsh</option>
            <option value="codex-model" ${acc.profileId === "codex-model" ? "selected" : ""}>OpenAI Codex</option>
          </select>
        </label>
        <div id="custom-model-fields">
          <label class="field">API 地址<input id="m-origin" value="${originValue}" placeholder="https://api.deepseek.com/anthropic"/></label>
          <p class="meta">自定义模型须 Anthropic 兼容 Messages API（探测 <code>POST …/v1/messages</code>）。</p>
          <label class="field">模型名（可选）<input id="m-model" value="${acc.model || ""}" placeholder="空则按主机推断，如 deepseek-chat"/></label>
        </div>
        <p class="meta" id="fixed-profile-hint">预置 profile 的地址和模型由启动环境登记，页面不重复填写。</p>
        <label class="field">API Key（提交后清空）<input id="m-key" type="password" autocomplete="off" spellcheck="false" placeholder="${editing === "new" ? "粘贴完整 sk-… Key" : "重贴新 Key（不回显旧值）"}"/></label>
        <p class="meta" id="m-key-hint">${
          editing !== "new" && acc.keyHint
            ? `已保存 · 末四位 ${acc.keyHint}`
            : editing !== "new"
              ? "已保存 · Key 在本机（重存一次后可显示末四位）"
              : ""
        }</p>
        <p class="meta" id="m-key-len"></p>
        <p class="meta">快捷填入（可选）：
          ${PRESETS.map((p) => `<button type="button" class="btn" data-preset="${p.origin}" data-pname="${p.name}" data-model="${p.model || ""}">${p.name}</button>`).join(" ")}
        </p>
        <label class="check-row"><input type="checkbox" id="m-skip-probe"/> 跳过上游探测并保存（仅自定义；会话时仍可能因 Key 无效失败）</label>
        <div class="actions">
          <button type="button" class="btn" id="m-cancel">取消</button>
          <button type="button" class="btn primary" id="m-save">保存</button>
          <span class="meta" id="m-status"></span>
        </div>
      </section>`;
    const nameEl = form.querySelector("#m-name");
    const originEl = form.querySelector("#m-origin");
    const modelEl = form.querySelector("#m-model");
    const profileEl = form.querySelector("#m-profile");
    const customFields = form.querySelector("#custom-model-fields");
    const fixedHint = form.querySelector("#fixed-profile-hint");
    const keyEl = form.querySelector("#m-key");
    const status = form.querySelector("#m-status");
    const keyLenEl = form.querySelector("#m-key-len");
    const updateKeyLen = () => {
      const n = keyEl.value.trim().length;
      if (!n) {
        keyLenEl.textContent = "";
        return;
      }
      keyLenEl.innerHTML = `<span class="meta">已输入 ${n} 字节${keyEl.value.trim().startsWith("sk-") ? "，前缀 sk-" : ""}（LiteLLM 短 Key 也常见）</span>`;
    };
    const clearStatus = () => {
      status.textContent = "";
    };
    const syncProfileFields = () => {
      const fixed = profileEl?.value !== "custom";
      customFields?.classList.toggle("hidden", fixed);
      if (fixedHint) {
        fixedHint.classList.toggle("hidden", !fixed);
        fixedHint.textContent = profileEl?.value === "dsh-model"
          ? "dsh 使用 HARBOR_DSH_* 环境变量登记的上游。"
          : "Codex 使用 HARBOR_CODEX_* 环境变量登记的 Responses 上游。";
      }
    };
    profileEl?.addEventListener("change", () => {
      syncProfileFields();
      clearStatus();
    });
    syncProfileFields();
    form.querySelectorAll("[data-preset]").forEach((btn) => {
      btn.onclick = () => {
        originEl.value = btn.getAttribute("data-preset") || "";
        const presetModel = btn.getAttribute("data-model") || "";
        if (presetModel) modelEl.value = presetModel;
        if (!nameEl.value.trim()) {
          nameEl.value = "我的 " + (btn.getAttribute("data-pname") || "");
        }
        clearStatus();
        keyEl.focus();
      };
    });
    form.querySelector("#m-cancel").onclick = () => {
      editing = null;
      if (returning) {
        clearReturn();
        ctx.navigate("wizard");
        return;
      }
      paint();
    };
    nameEl.addEventListener("input", clearStatus);
    originEl.addEventListener("input", clearStatus);
    modelEl.addEventListener("input", clearStatus);
    keyEl.addEventListener("input", () => {
      clearStatus();
      updateKeyLen();
    });
    keyEl.addEventListener("paste", () => setTimeout(updateKeyLen, 0));
    updateKeyLen();
    form.querySelector("#m-save").onclick = async () => {
      const name = nameEl.value.trim();
      const profileId = profileEl?.value || "custom";
      const origin = originEl?.value.trim().replace(/\/+$/, "") || "";
      const model = modelEl.value.trim();
      const key = keyEl.value.trim();
      const missing = [];
      if (!name) missing.push("名称");
      if (profileId === "custom" && !origin) missing.push("API 地址");
      if (!key) missing.push("API Key");
      if (missing.length) {
        status.innerHTML = `<span class="bad">请填写：${missing.join("、")}（灰色提示不是已填内容）</span>`;
        if (!origin) originEl.focus();
        else if (!key) keyEl.focus();
        else nameEl.focus();
        return;
      }
      status.innerHTML = `<span class="meta">正在保存…</span>`;
      const saveBtn = form.querySelector("#m-save");
      saveBtn.disabled = true;
      try {
        if (editing !== "new") {
          const rm = await api.removeAccount(editing);
          if (rm?.error) {
            status.innerHTML = `<span class="bad">更新前删除旧账号失败：${rm.error.code}: ${rm.error.message}</span>`;
            return;
          }
        }
        const skipProbe = !!form.querySelector("#m-skip-probe")?.checked;
        const res = await api.connectModel({
          profileId,
          name,
          apiKey: key,
          ...(profileId === "custom" ? { origin, model, skipProbe } : {}),
        });
        if (res.error) {
          let hint = "";
          if (res.error.code === "UPSTREAM_AUTH_FAILED") {
            hint =
              " 请用终端验证：export KEY='完整Key'; ./scripts/probe-model-auth.sh " +
              origin.replace(/'/g, "");
          } else if (res.error.code === "UPSTREAM_UNAVAILABLE") {
            hint = " 上游可达但协议/模型名可能不对；可改「模型名」后重试。";
          }
          status.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>${hint ? `<span class="meta">${hint}</span>` : ""}`;
          return;
        }
        keyEl.value = "";
        editing = null;
        ctx.state.modelProfileIntent = null;
        ctx.onAccountsChanged?.();
        if (resumeWizard(res.account?.id)) return;
        await load();
      } finally {
        saveBtn.disabled = false;
      }
    };
  }

  load();
}
