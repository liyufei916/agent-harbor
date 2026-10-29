/**
 * PAGE-DESIGN §5.7 模型：自由添加 OpenAPI（名 + 地址 + Key）
 * Live 自定义 origin 须 Anthropic 兼容 /v1/messages。
 * Codex Agent 须另存 profileId=codex-model（Responses；Origin 来自宿主机 env）。
 */
import * as api from "../api.js";
import { defaultAgentsForProfile, modelCompatLabel } from "../agent-compat.js";
import { showToast } from "../toast.js";

const AGENT_OPTS = [
  { id: "claude", label: "Claude" },
  { id: "codex", label: "Codex" },
  { id: "dsh", label: "dsh" },
];

const PRESETS = [
  { name: "Anthropic", origin: "https://api.anthropic.com", model: "claude-haiku-4-5-20251001" },
  // DeepSeek Anthropic 兼容基址（探测 POST …/anthropic/v1/messages，模型 deepseek-chat）
  { name: "DeepSeek", origin: "https://api.deepseek.com/anthropic", model: "deepseek-chat" },
  // 两套七牛网关 Key 不互通：官方 Token API vs LiteLLM
  { name: "七牛官方", origin: "https://api.qnaigc.com", model: "" },
  // Codex/OpenAI 兼容网关常用小写 id（如 gpt-5.6-sol）；Anthropic 路由另填 Claude 模型名
  { name: "七牛LiteLLM", origin: "https://llmapi.qiniu.io", model: "gpt-5.6-sol" },
];

/** 固定 profile：不填 Origin，走宿主机 HARBOR_* 登记上游（Codex / dsh）。 */
const FIXED_PRESETS = [
  {
    name: "OpenAI Codex",
    profileId: "codex-model",
    hint: "Responses API；须已 export HARBOR_CODEX_MODEL_ORIGIN（如 https://api.openai.com）",
  },
  {
    name: "DeepSeek dsh",
    profileId: "dsh-model",
    hint: "须已 export HARBOR_DSH_MODEL_ORIGIN 等",
  },
];

export function mountModels(root, ctx, opts = {}) {
  const returning = ctx.state.returnAfterAccount?.kind === "model";
  let editing = opts.intent === "add" || returning ? "new" : null;
  let pendingDelete = "";
  let listStatus = "";
  /** null=自定义 Anthropic；否则为固定 profileId（codex-model / dsh-model） */
  let fixedProfileId = null;

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
          <p class="sub">给 Agent 一把「嘴」。可勾选兼容多个 Agent（须上游同时支持对应协议）；列表显示可用范围。</p>
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
            <span class="meta">${a.host || a.profileId || ""} · ${modelCompatLabel(a)}</span>
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
      fixedProfileId = null;
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
        const acc = (ctx.state.modelAccounts || []).find((a) => a.id === editing);
        fixedProfileId =
          acc?.profileId === "codex-model" || acc?.profileId === "dsh-model"
            ? acc.profileId
            : null;
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
        ? { name: "", host: "", profileId: "custom" }
        : (ctx.state.modelAccounts || []).find((a) => a.id === editing) || {};
    const originValue =
      acc.origin ||
      (acc.host ? "https://" + acc.host + (acc.basePath || "") : "");
    form.classList.remove("hidden");
    const isFixed = !!fixedProfileId;
    const fixedMeta = FIXED_PRESETS.find((p) => p.profileId === fixedProfileId);
    const profileForAgents = fixedProfileId || acc.profileId || "custom";
    const selectedAgents =
      Array.isArray(acc.agents) && acc.agents.length
        ? acc.agents
        : defaultAgentsForProfile(profileForAgents);
    form.innerHTML = `
      <section class="card">
        <h3>${editing === "new" ? "添加模型" : "编辑模型"}</h3>
        <label class="field">名称<input id="m-name" value="${acc.name || ""}" maxlength="100"/></label>
        <div id="m-custom-fields" class="${isFixed ? "hidden" : ""}">
          <label class="field">API 地址<input id="m-origin" value="${originValue}" placeholder="https://api.deepseek.com/anthropic"/></label>
          <p class="meta">须 Anthropic 兼容 Messages API（探测 <code>POST …/v1/messages</code>）。七牛两套 Key 不能混用。若同时勾选 Codex，地址须为<strong>同时提供 Messages + Responses</strong> 的网关。</p>
          <label class="field">模型名（可选）<input id="m-model" value="${acc.model || ""}" placeholder="空则按主机推断，如 deepseek-chat"/></label>
        </div>
        <div id="m-fixed-fields" class="${isFixed ? "" : "hidden"}">
          <p class="meta ok">固定 profile：<code>${fixedProfileId || ""}</code>。${fixedMeta?.hint || "上游由启动环境变量登记，页面不填 Origin。"}</p>
        </div>
        <p class="meta">可用于哪些 Agent（可多选）：</p>
        <div class="row" id="m-agents">
          ${AGENT_OPTS.map(
            (o) =>
              `<label class="check-row"><input type="checkbox" data-agent="${o.id}" ${selectedAgents.includes(o.id) ? "checked" : ""}/> ${o.label}</label>`,
          ).join("")}
        </div>
        <label class="field">API Key（提交后清空）<input id="m-key" type="password" autocomplete="off" spellcheck="false" placeholder="${editing === "new" ? "粘贴完整 sk-… Key" : "重贴新 Key（不回显旧值）"}"/></label>
        <p class="meta" id="m-key-hint">${
          editing !== "new" && acc.keyHint
            ? `已保存 · 末四位 ${acc.keyHint}`
            : editing !== "new"
              ? "已保存 · Key 在本机（重存一次后可显示末四位）"
              : ""
        }</p>
        <p class="meta" id="m-key-len"></p>
        <p class="meta">Anthropic 兼容快捷填入：
          ${PRESETS.map((p) => `<button type="button" class="btn" data-preset="${p.origin}" data-pname="${p.name}" data-model="${p.model || ""}">${p.name}</button>`).join(" ")}
        </p>
        <p class="meta">固定 Agent 上游（Codex / dsh）：
          ${FIXED_PRESETS.map((p) => `<button type="button" class="btn" data-fixed="${p.profileId}" data-pname="${p.name}">${p.name}</button>`).join(" ")}
          <button type="button" class="btn" id="m-use-custom">改回自定义 Anthropic</button>
        </p>
        <label class="check-row ${isFixed ? "hidden" : ""}"><input type="checkbox" id="m-skip-probe"/> 跳过上游探测并保存（仅自定义；会话时仍可能因 Key 无效失败）</label>
        <div class="actions">
          <button type="button" class="btn" id="m-cancel">取消</button>
          <button type="button" class="btn primary" id="m-save">保存</button>
        </div>
        <p class="meta" id="m-status" role="status"></p>
      </section>`;
    const nameEl = form.querySelector("#m-name");
    const originEl = form.querySelector("#m-origin");
    const modelEl = form.querySelector("#m-model");
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
    form.querySelectorAll("[data-preset]").forEach((btn) => {
      btn.onclick = () => {
        fixedProfileId = null;
        paintForm();
        const o = form.querySelector("#m-origin");
        const m = form.querySelector("#m-model");
        const n = form.querySelector("#m-name");
        if (o) o.value = btn.getAttribute("data-preset") || "";
        if (m) {
          const presetModel = btn.getAttribute("data-model") || "";
          if (presetModel) m.value = presetModel;
        }
        if (n && !n.value.trim()) {
          n.value = "我的 " + (btn.getAttribute("data-pname") || "");
        }
        form.querySelector("#m-key")?.focus();
      };
    });
    form.querySelectorAll("[data-fixed]").forEach((btn) => {
      btn.onclick = () => {
        fixedProfileId = btn.getAttribute("data-fixed");
        if (!nameEl.value.trim()) {
          nameEl.value = "我的 " + (btn.getAttribute("data-pname") || "");
        }
        paintForm();
        form.querySelector("#m-key")?.focus();
      };
    });
    form.querySelector("#m-use-custom")?.addEventListener("click", () => {
      fixedProfileId = null;
      paintForm();
    });
    form.querySelector("#m-cancel").onclick = () => {
      editing = null;
      fixedProfileId = null;
      if (returning) {
        clearReturn();
        ctx.navigate("wizard");
        return;
      }
      paint();
    };
    nameEl.addEventListener("input", clearStatus);
    originEl?.addEventListener("input", clearStatus);
    modelEl?.addEventListener("input", clearStatus);
    keyEl.addEventListener("input", () => {
      clearStatus();
      updateKeyLen();
    });
    keyEl.addEventListener("paste", () => setTimeout(updateKeyLen, 0));
    updateKeyLen();
    form.querySelector("#m-save").onclick = async () => {
      const name = nameEl.value.trim();
      const origin = (originEl?.value || "").trim().replace(/\/+$/, "");
      const model = (modelEl?.value || "").trim();
      const key = keyEl.value.trim();
      const agents = [...form.querySelectorAll("[data-agent]:checked")].map((el) =>
        el.getAttribute("data-agent"),
      );
      const missing = [];
      if (!name) missing.push("名称");
      if (!fixedProfileId && !origin) missing.push("API 地址");
      if (!key) missing.push("API Key");
      if (!agents.length) missing.push("至少一个 Agent");
      if (missing.length) {
        status.innerHTML = `<span class="bad">请填写：${missing.join("、")}（灰色提示不是已填内容）</span>`;
        if (!fixedProfileId && !origin) originEl?.focus();
        else if (!key) keyEl.focus();
        else nameEl.focus();
        return;
      }
      const originLow = origin.toLowerCase();
      if (
        !fixedProfileId &&
        agents.includes("codex") &&
        (originLow.includes("deepseek") || originLow.includes("/anthropic"))
      ) {
        const msg =
          "Codex 不能使用 DeepSeek 或 …/anthropic 地址。请改用「OpenAI Codex」，或取消 Codex 勾选后保存给 Claude。";
        status.innerHTML = `<span class="bad">INVALID_ARGUMENT: ${msg}</span>`;
        showToast(msg, "bad");
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
        const res = await api.connectModel(
          fixedProfileId
            ? { profileId: fixedProfileId, name, apiKey: key, agents }
            : {
                profileId: "custom",
                name,
                apiKey: key,
                origin,
                model,
                skipProbe,
                agents,
              },
        );
        if (res.error) {
          const wantsCodex =
            agents.includes("codex") || fixedProfileId === "codex-model";
          let hint = "";
          if (res.error.code === "UPSTREAM_AUTH_FAILED") {
            hint = wantsCodex
              ? " Responses 认证失败时请核对 Key 与 Origin（Codex 不用 DeepSeek Anthropic Key）。"
              : fixedProfileId
                ? " 核对 Key 与 HARBOR_*_MODEL_ORIGIN 是否同一套上游。"
                : " 请用终端验证：export KEY='完整Key'; ./scripts/probe-model-auth.sh " +
                  origin.replace(/'/g, "");
          } else if (res.error.code === "UPSTREAM_UNAVAILABLE") {
            hint = wantsCodex
              ? " Codex 依赖 POST /v1/responses；DeepSeek 仅 Anthropic 时请改用「OpenAI Codex」快捷项。"
              : fixedProfileId
                ? " 确认已 export 对应 HARBOR_*_MODEL_ORIGIN，且上游支持登记协议。"
                : " 上游可达但协议/模型名可能不对；可改「模型名」后重试。";
          } else if (res.error.code === "UPSTREAM_TIMEOUT") {
            hint = wantsCodex
              ? " 七牛/代理上 Responses 首包可能很慢；确认模型名 gpt-5.6-sol、网络可达，稍后重试。仍超时则网关可能未真正支持 /v1/responses。"
              : " 网络或上游过慢，请稍后重试。";
          } else if (res.error.code === "INVALID_ARGUMENT") {
            hint = wantsCodex
              ? " 常见：未 export HARBOR_CODEX_MODEL_ORIGIN，或跳过探测时勾了 Codex。"
              : fixedProfileId
                ? " 常见：未启动前登记 HARBOR_*_MODEL_ORIGIN，profile 不可用。"
                : "";
          }
          const line = `${res.error.code}: ${res.error.message}`;
          status.innerHTML = `<span class="bad">${line}</span>${hint ? `<br/><span class="meta">${hint}</span>` : ""}`;
          showToast(line, "bad");
          return;
        }
        keyEl.value = "";
        editing = null;
        fixedProfileId = null;
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
