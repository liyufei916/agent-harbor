/**
 * PAGE-DESIGN §5.7 模型：自由添加 OpenAPI（名 + 地址 + Key）
 */
import * as api from "../api.js";

const PRESETS = [
  { name: "DeepSeek", origin: "https://api.deepseek.com" },
  { name: "Anthropic", origin: "https://api.anthropic.com" },
];

export function mountModels(root, ctx) {
  let editing = null; // account id or "new"

  async function load() {
    const res = await api.listAccounts("model");
    ctx.state.modelAccounts = res.accounts || [];
    // 兼容旧单账号字段
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
      <div id="model-list" class="session-list"></div>
      ${list.length === 0 && editing !== "new" ? `<p class="meta">还没有模型。点「添加」配置第一个。</p>` : ""}
      <div id="model-form" class="${editing ? "" : "hidden"}"></div>
    `;

    const listEl = root.querySelector("#model-list");
    list.forEach((a) => {
      const row = document.createElement("article");
      row.className = "card session-card";
      row.innerHTML = `
        <div class="row" style="justify-content:space-between">
          <div>
            <strong>${a.name}</strong>
            <span class="meta">${a.host || a.profileId || ""}</span>
            ${a.invalid ? `<span class="bad">▲ 无效</span>` : ""}
          </div>
          <div class="row">
            <button type="button" class="btn" data-edit="${a.id}">编辑</button>
            <button type="button" class="btn danger" data-del="${a.id}">删除</button>
          </div>
        </div>`;
      listEl.appendChild(row);
    });

    root.querySelector("#btn-add-model").onclick = () => {
      editing = "new";
      paint();
      paintForm();
    };
    listEl.querySelectorAll("[data-edit]").forEach((btn) => {
      btn.onclick = () => {
        editing = btn.getAttribute("data-edit");
        paint();
        paintForm();
      };
    });
    listEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = async () => {
        const id = btn.getAttribute("data-del");
        const acc = list.find((x) => x.id === id);
        if (!confirm(`删除模型「${acc?.name || id}」及其本机 API Key？`)) return;
        await api.removeAccount(id);
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
    form.classList.remove("hidden");
    form.innerHTML = `
      <section class="card">
        <h3>${editing === "new" ? "添加模型" : "编辑模型"}</h3>
        <label class="field">名称<input id="m-name" value="${acc.name || ""}" maxlength="100"/></label>
        <label class="field">API 地址<input id="m-origin" value="${acc.origin || (acc.host ? `https://${acc.host}` : "")}" placeholder="https://api.example.com"/></label>
        <label class="field">API Key（提交后清空）<input id="m-key" type="password" autocomplete="off" placeholder="粘贴 Key"/></label>
        <p class="meta">快捷填入默认地址：
          ${PRESETS.map((p) => `<button type="button" class="btn" data-preset="${p.origin}" data-pname="${p.name}">${p.name}</button>`).join(" ")}
        </p>
        <div class="actions">
          <button type="button" class="btn" id="m-cancel">取消</button>
          <button type="button" class="btn primary" id="m-save">保存</button>
          <span class="meta" id="m-status"></span>
        </div>
      </section>`;
    form.querySelectorAll("[data-preset]").forEach((btn) => {
      btn.onclick = () => {
        form.querySelector("#m-origin").value = btn.getAttribute("data-preset");
        if (!form.querySelector("#m-name").value) {
          form.querySelector("#m-name").value = `我的 ${btn.getAttribute("data-pname")}`;
        }
      };
    });
    form.querySelector("#m-cancel").onclick = () => {
      editing = null;
      paint();
    };
    form.querySelector("#m-save").onclick = async () => {
      const name = form.querySelector("#m-name").value.trim();
      const origin = form.querySelector("#m-origin").value.trim();
      const key = form.querySelector("#m-key").value;
      const status = form.querySelector("#m-status");
      if (!name || !origin || !key) {
        status.innerHTML = `<span class="bad">请填写名称、地址与 Key</span>`;
        return;
      }
      if (editing !== "new") await api.removeAccount(editing);
      const res = await api.connectModel({
        profileId: "custom",
        name,
        apiKey: key,
        origin,
      });
      form.querySelector("#m-key").value = "";
      if (res.error) {
        status.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
        return;
      }
      editing = null;
      ctx.onAccountsChanged?.();
      await load();
    };
  }

  load();
}
