/**
 * 浏览器 live 选工作区：网页拿不到系统目录绝对路径，用表单 + 本机快捷项。
 */
import { openFormModal } from "./form-modal.js";

export async function pickHttpLiveWorkspace(rpc) {
  let shortcuts = [];
  try {
    const res = await rpc("ListWorkspaceShortcuts", null);
    shortcuts = Array.isArray(res?.shortcuts) ? res.shortcuts : [];
  } catch {
    shortcuts = [];
  }

  return new Promise((resolve) => {
    let settled = false;
    const finish = (result) => {
      if (settled) return;
      settled = true;
      resolve(result);
    };

    const modal = openFormModal({
      title: "选择工作区",
      // 仅未成功提交时视为取消（Escape / 点取消 / 关层）
      onClose: () => finish({ path: "", cancelled: true, error: null }),
    });
    if (!modal?.body) {
      finish({ path: "", cancelled: true, error: null });
      return;
    }
    const demo = shortcuts[0];
    modal.body.innerHTML = `
      <p class="meta">浏览器不能像桌面 App 那样打开系统选目录（网页安全模型不暴露本机绝对路径）。请填写目录绝对路径，或点下方快捷项。</p>
      <label class="field">工作区路径
        <input id="hl-ws-path" type="text" spellcheck="false" placeholder="/Users/…/demos/workspace" value="${escapeAttr(demo?.path || "")}"/>
      </label>
      <div class="row" id="hl-ws-shortcuts" style="flex-wrap:wrap;gap:8px;margin:8px 0"></div>
      <p class="meta" id="hl-ws-status" role="status"></p>
      <div class="actions">
        <button type="button" class="btn" id="hl-ws-cancel">取消</button>
        <button type="button" class="btn primary" id="hl-ws-ok">使用此路径</button>
      </div>`;
    const input = modal.body.querySelector("#hl-ws-path");
    const status = modal.body.querySelector("#hl-ws-status");
    const shortEl = modal.body.querySelector("#hl-ws-shortcuts");
    for (const s of shortcuts) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "btn";
      b.textContent = s.label || s.id;
      b.title = s.path;
      b.addEventListener("click", () => {
        input.value = s.path || "";
        status.textContent = "";
        input.focus();
      });
      shortEl.appendChild(b);
    }
    if (!shortcuts.length) {
      shortEl.innerHTML = `<span class="meta">未发现 demos/workspace（请在仓库根启动 http-live-serve，或手填绝对路径）</span>`;
    }
    modal.body.querySelector("#hl-ws-cancel").onclick = () => {
      modal.close();
    };
    modal.body.querySelector("#hl-ws-ok").onclick = async () => {
      const path = (input.value || "").trim();
      if (!path) {
        status.innerHTML = `<span class="bad">请填写绝对路径</span>`;
        input.focus();
        return;
      }
      status.innerHTML = `<span class="meta">校验中…</span>`;
      const btn = modal.body.querySelector("#hl-ws-ok");
      if (btn) btn.disabled = true;
      try {
        const res = await rpc("SelectWorkspace", { path });
        if (res?.error) {
          status.innerHTML = `<span class="bad">${res.error.code}: ${res.error.message}</span>`;
          if (btn) btn.disabled = false;
          return;
        }
        // 先 settle 成功，再 close（避免 onClose 覆盖成 cancelled）
        finish(res || { path, cancelled: false, error: null });
        modal.close();
      } catch (e) {
        status.innerHTML = `<span class="bad">${e?.message || e}</span>`;
        if (btn) btn.disabled = false;
      }
    };
    input?.focus();
    input?.select();
  });
}

function escapeAttr(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;");
}
