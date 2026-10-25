/**
 * PAGE-DESIGN §5.4 工作台：边界条 + PTY + 摘要；扩权 / 权限一览
 */
import * as api from "../api.js";
import { OutputTail } from "../session-events.js";
import {
  mountPermissionModal,
  hidePermissionModal,
} from "./permission-modal.js";

export function mountSession(root, ctx) {
  root.innerHTML = `
    <div class="boundary" id="boundary">
      <div class="row" style="justify-content:space-between;width:100%">
        <div>
          <span id="sess-state" class="status-pill">无会话</span>
          <span class="meta" id="sess-meta"></span>
        </div>
        <div class="row">
          <label class="field inline">当前模型
            <select id="sess-model"></select>
          </label>
          <button type="button" class="btn" id="btn-perm">权限一览</button>
          <button type="button" class="btn danger" id="btn-stop" disabled>停止</button>
          <button type="button" class="btn" id="btn-to-home">回 HOME</button>
        </div>
      </div>
      <p class="meta" id="sess-hint-line"></p>
    </div>
    <div class="session-grid">
      <div class="pty-panel">
        <div class="pty-toolbar meta">终端 / Agent 输出（PTY）</div>
        <div id="sess-pty" class="pty" tabindex="0"></div>
        <textarea id="sess-pty-input" class="pty-input" rows="2" placeholder="输入后 Enter 发送到 PTY（mock）"></textarea>
      </div>
      <aside class="session-side">
        <h3>本趟摘要</h3>
        <ul class="meta" id="sess-stats">
          <li>输出块：<span id="stat-out">0</span></li>
          <li>审计：<span id="stat-audit">0</span></li>
        </ul>
        <h3>最近活动</h3>
        <ul id="sess-audit" class="audit-list"></ul>
        <p class="meta warn hidden" id="sess-hint"></p>
        <p class="meta" id="sess-error"></p>
      </aside>
    </div>
    <div id="perm-drawer" class="perm-drawer hidden"></div>
  `;

  const els = {
    state: root.querySelector("#sess-state"),
    meta: root.querySelector("#sess-meta"),
    hintLine: root.querySelector("#sess-hint-line"),
    model: root.querySelector("#sess-model"),
    stop: root.querySelector("#btn-stop"),
    pty: root.querySelector("#sess-pty"),
    ptyInput: root.querySelector("#sess-pty-input"),
    audit: root.querySelector("#sess-audit"),
    error: root.querySelector("#sess-error"),
    hint: root.querySelector("#sess-hint"),
    statOut: root.querySelector("#stat-out"),
    statAudit: root.querySelector("#stat-audit"),
    drawer: root.querySelector("#perm-drawer"),
  };

  let tail = new OutputTail();
  let outCount = 0;
  let auditCount = 0;
  const modalRoot =
    typeof document !== "undefined"
      ? document.getElementById("modal-root")
      : null;

  async function fillModels() {
    if (!els.model) return;
    if (typeof document === "undefined" || !document.createElement) return;
    const res = await api.listAccounts("model");
    const accounts = res.accounts || [];
    const options = accounts.map((a) => {
      const o = document.createElement("option");
      o.value = a.id;
      o.textContent = `${a.name} · ${a.host || ""}`;
      return o;
    });
    if (typeof els.model.replaceChildren === "function") {
      els.model.replaceChildren(...options);
    } else {
      els.model.innerHTML = "";
      options.forEach((o) => els.model.appendChild?.(o));
    }
  }

  function renderSnapshot(s) {
    if (!s) {
      els.state.textContent = "无会话";
      els.state.className = "status-pill";
      els.meta.textContent = "";
      els.stop.disabled = true;
      return;
    }
    els.state.textContent = s.state;
    els.state.className = `status-pill ${s.state}`;
    els.meta.textContent = `${s.agentId || ""} · ${s.workspace || ""} · 挂载:${s.mountMode || "direct"} · ${s.sessionId}`;
    els.hintLine.textContent = "● 隔离运行中（边界以权限一览为准）";
    const active =
      ["provisioning", "running", "stopping"].includes(s.state) ||
      (s.state === "failed" && !s.cleanupComplete);
    els.stop.disabled = !active;
    if (s.error) els.error.textContent = `${s.error.code}: ${s.error.message}`;
    else els.error.textContent = "";
    if (s.state === "failed" && !s.cleanupComplete) {
      els.hint.classList.remove("hidden");
      els.hint.textContent = "清理未完成，请再点停止。";
    } else if (s.state === "stopped") {
      els.hint.classList.remove("hidden");
      els.hint.textContent = "已停止。可回 HOME 再开。";
    } else {
      els.hint.classList.add("hidden");
    }
  }

  function appendPty(text) {
    if (!els.pty) return;
    els.pty.textContent += text || "";
    els.pty.scrollTop = els.pty.scrollHeight;
  }

  function onEvent(ev) {
    if (ctx.state.sessionId && ev.sessionId && ev.sessionId !== ctx.state.sessionId)
      return;
    if (ev.type === "state_changed" && ev.state) {
      ctx.state.lastSnapshot = ev.state;
      renderSnapshot(ev.state);
    } else if (ev.type === "output" && ev.output) {
      outCount += 1;
      els.statOut.textContent = String(outCount);
      tail.append(ev.seq, ev.output.text || "");
      appendPty(ev.output.text || "");
    } else if (ev.type === "pty" && (ev.pty || ev.output)) {
      const t = (ev.pty || ev.output).text || "";
      appendPty(t);
    } else if (ev.type === "audit" && ev.audit) {
      auditCount += 1;
      els.statAudit.textContent = String(auditCount);
      const li = document.createElement("li");
      li.textContent = `${ev.audit.result} · ${ev.audit.tool || "-"} @ ${ev.audit.target || "-"}`;
      els.audit.prepend(li);
    } else if (ev.type === "error" && ev.error) {
      els.error.textContent = `${ev.error.code}: ${ev.error.message}`;
    } else if (ev.type === "permission_needed" && ev.permission) {
      const snap = ctx.state.lastSnapshot;
      mountPermissionModal(modalRoot, {
        sessionLabel: `${snap?.agentId || ""} · ${snap?.workspace || ""}`,
        target: ev.permission.target,
        action: ev.permission.action,
        async onDecide(decision) {
          await api.resolvePermission({
            sessionId: ctx.state.sessionId,
            target: ev.permission.target,
            decision,
          });
          hidePermissionModal(modalRoot);
        },
      });
    }
  }

  const unsub = api.onEvent(onEvent);
  fillModels();

  if (els.model) {
    els.model.onchange = async () => {
      if (!ctx.state.sessionId || !els.model.value) return;
      const res = await api.switchModel({
        sessionId: ctx.state.sessionId,
        modelAccountId: els.model.value,
      });
      if (res.error) {
        els.error.textContent = `${res.error.code}: ${res.error.message}`;
      }
    };
  }

  if (els.stop) {
    els.stop.onclick = async () => {
      if (!ctx.state.sessionId) return;
      els.stop.disabled = true;
      const res = await api.stopSession(ctx.state.sessionId);
      if (res.error) {
        els.error.textContent = `${res.error.code}: ${res.error.message}`;
        els.stop.disabled = false;
      }
    };
  }

  const toHome = root.querySelector("#btn-to-home");
  if (toHome) toHome.onclick = () => ctx.navigate?.("home") || ctx.showTab?.("setup");

  const permBtn = root.querySelector("#btn-perm");
  if (permBtn) permBtn.onclick = async () => {
    if (!els.drawer) return;
    const res = await api.getPermissionOverview(ctx.state.sessionId || "");
    const o = res.overview;
    els.drawer.classList.remove("hidden");
    els.drawer.innerHTML = `
      <div class="card">
        <div class="row" style="justify-content:space-between">
          <h3>当前边界</h3>
          <button type="button" class="btn" id="perm-close">关闭</button>
        </div>
        ${
          res.error
            ? `<p class="bad">${res.error.code}: ${res.error.message}</p>`
            : `
          <p class="meta">工作区：${o?.workspace || "—"}</p>
          <h4>网络白名单</h4>
          <ul class="audit-list">${(o?.allowedHosts || []).map((h) => `<li>${h}</li>`).join("") || "<li class='meta'>无</li>"}</ul>
          <h4>工具</h4>
          <ul class="audit-list">${(o?.mcpSummary || []).map((m) => `<li>${m}</li>`).join("") || "<li class='meta'>无</li>"}</ul>
          <h4>不可见类别</h4>
          <ul class="audit-list">${(o?.deniedPathCategories || []).map((m) => `<li>${m}</li>`).join("") || "<li class='meta'>—</li>"}</ul>
        `
        }
      </div>`;
    els.drawer.querySelector("#perm-close").onclick = () => {
      els.drawer.classList.add("hidden");
      els.drawer.innerHTML = "";
    };
  };

  if (els.ptyInput?.addEventListener) {
    els.ptyInput.addEventListener("keydown", async (e) => {
      if (e.key !== "Enter" || e.shiftKey) return;
      e.preventDefault();
      const data = els.ptyInput.value;
      els.ptyInput.value = "";
      if (!ctx.state.sessionId || !data) return;
      appendPty(`\n$ ${data}\n`);
      await api.writePty({ sessionId: ctx.state.sessionId, data: data + "\n" });
    });
  }

  const side = root.querySelector(".session-side");
  if (side && typeof side.appendChild === "function" && typeof document !== "undefined") {
    const demo = document.createElement("button");
    demo.type = "button";
    demo.className = "btn";
    demo.textContent = "演示扩权（mock）";
    demo.style.marginTop = "8px";
    side.appendChild(demo);
    demo.onclick = () => {
      onEvent({
        sessionId: ctx.state.sessionId,
        seq: Date.now(),
        type: "permission_needed",
        permission: { target: "api.npmjs.org", action: "请求访问新地址" },
      });
    };
  }

  return {
    resetForNewSession(snapshot) {
      tail = new OutputTail();
      outCount = 0;
      auditCount = 0;
      els.statOut.textContent = "0";
      els.statAudit.textContent = "0";
      els.pty.textContent = "";
      els.audit.innerHTML = "";
      els.error.textContent = "";
      els.hint.classList.add("hidden");
      hidePermissionModal(modalRoot);
      if (snapshot) {
        ctx.state.lastSnapshot = snapshot;
        api.activateSession(snapshot);
        renderSnapshot(snapshot);
      }
      fillModels();
    },
    destroy() {
      unsub();
      hidePermissionModal(modalRoot);
    },
  };
}
