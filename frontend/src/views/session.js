/**
 * PAGE-DESIGN §5.4 工作台：边界条 + PTY + 摘要；扩权 / 权限一览
 */
import * as api from "../api.js";
import { OutputTail } from "../session-events.js";
import { createPtyTerm } from "../pty-term.js";
import { createStreamDisplay } from "../stream-display.js";
import {
  mountPermissionModal,
  hidePermissionModal,
} from "./permission-modal.js";
import {
  mountPermissionOverviewModal,
  hidePermissionOverviewModal,
} from "./permission-overview-modal.js";
import { askExitReview } from "./exit-review-modal.js";
import { mountRunReportModal } from "./run-report-modal.js";
import { showToast } from "../toast.js";
import { askConfirm } from "../confirm-dialog.js";
import {
  filterModelsForAgent,
  filterModelsForSwitch,
} from "../agent-compat.js";
import { recordAudit, recordPermissionDecision } from "../usage-analytics.js";
import { formatErrorWithHint } from "../error-hints.js";
import { markTutorialStep } from "../tutorial-progress.js";

const SWITCH_MODEL_META =
  "仅可切换同 profile / 同主机账号；跨协议或改固定模型名不支持。";

function escapeAttr(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;");
}

export function mountSession(root, ctx) {
  const healthOk = ctx.state.health?.available !== false;

  root.innerHTML = `
    <div id="sess-empty" class="empty-state">
      <p>当前没有打开的会话。从 HOME 进入工作台，或创建新会话。</p>
      <div class="actions">
        <button type="button" class="btn" id="sess-empty-home">返回 HOME</button>
        <button type="button" class="btn primary" id="sess-empty-wizard" ${healthOk ? "" : "disabled"}>新会话</button>
      </div>
    </div>
    <div id="sess-workbench" class="hidden">
      <div class="boundary" id="boundary">
        <div class="row" style="justify-content:space-between;width:100%">
          <div>
            <span id="sess-state" class="status-pill">无会话</span>
            <span class="meta" id="sess-meta"></span>
          </div>
          <div class="row">
            <label class="field inline">当前模型
              <select id="sess-model"></select>
              <span class="meta" id="sess-model-meta" style="display:block;max-width:22rem"></span>
            </label>
            <button type="button" class="btn" id="btn-perm">权限一览</button>
            <button type="button" class="btn hidden" id="btn-syncback" disabled>导回原文件夹</button>
            <button type="button" class="btn" id="btn-to-home">返回 HOME</button>
            <button type="button" class="btn danger" id="btn-stop" disabled>停止</button>
          </div>
        </div>
        <p class="meta" id="sess-hint-line"></p>
      </div>
      <div class="session-grid">
        <div class="pty-panel">
          <div class="pty-toolbar meta">对话（已过滤协议 JSON）· 请在下方输入框发下一轮（点终端无效）</div>
          <div id="sess-pty" class="pty" tabindex="0"></div>
          <div class="pty-composer">
            <textarea id="sess-pty-input" class="pty-input" rows="2" placeholder="在此输入下一轮，Enter 发送（Shift+Enter 换行）"></textarea>
            <button type="button" class="btn primary" id="sess-pty-send">发送</button>
          </div>
          <p class="meta pty-composer-hint" id="sess-pty-status"></p>
        </div>
        <aside class="session-side">
          <h3>本趟摘要</h3>
          <ul class="meta" id="sess-stats">
            <li>输出块：<span id="stat-out">0</span></li>
            <li>审计：<span id="stat-audit">0</span></li>
          </ul>
          <h3>扩权 / 边界</h3>
          <div id="sess-perm-summary" class="perm-summary">
            <p class="meta">加载中…</p>
          </div>
          <h3>最近活动</h3>
          <ul id="sess-audit" class="audit-list"></ul>
          <p class="meta warn hidden" id="sess-hint"></p>
          <p class="meta" id="sess-error"></p>
        </aside>
      </div>
    </div>
  `;

  const els = {
    empty: root.querySelector("#sess-empty"),
    workbench: root.querySelector("#sess-workbench"),
    boundary: root.querySelector("#boundary"),
    state: root.querySelector("#sess-state"),
    meta: root.querySelector("#sess-meta"),
    hintLine: root.querySelector("#sess-hint-line"),
    model: root.querySelector("#sess-model"),
    modelMeta: root.querySelector("#sess-model-meta"),
    syncBack: root.querySelector("#btn-syncback"),
    stop: root.querySelector("#btn-stop"),
    pty: root.querySelector("#sess-pty"),
    ptyInput: root.querySelector("#sess-pty-input"),
    ptySend: root.querySelector("#sess-pty-send"),
    ptyStatus: root.querySelector("#sess-pty-status"),
    audit: root.querySelector("#sess-audit"),
    permSummary: root.querySelector("#sess-perm-summary"),
    error: root.querySelector("#sess-error"),
    hint: root.querySelector("#sess-hint"),
    statOut: root.querySelector("#stat-out"),
    statAudit: root.querySelector("#stat-audit"),
  };

  let tail = new OutputTail();
  let streamDisplay = createStreamDisplay();
  let outCount = 0;
  let auditCount = 0;
  let term = null;
  let sending = false;
  const modalRoot =
    typeof document !== "undefined"
      ? document.getElementById("modal-root")
      : null;

  function ensureTerm() {
    if (term || !els.pty) return term;
    // 输出用 xterm；用户输入走下方输入框，避免按键逐字 WritePty。
    term = createPtyTerm(els.pty, {
      onResize(cols, rows) {
        if (!ctx.state.sessionId) return;
        api
          .resizePty({ sessionId: ctx.state.sessionId, cols, rows })
          .catch(() => {});
      },
    });
    return term;
  }

  function syncComposer(snapshot) {
    const running = snapshot?.state === "running";
    const canSend = running && !sending && !!ctx.state.sessionId;
    if (els.ptyInput) {
      els.ptyInput.disabled = !canSend;
      els.ptyInput.placeholder = running
        ? "在此输入下一轮，Enter 发送（Shift+Enter 换行）"
        : snapshot?.state
          ? `会话 ${snapshot.state}，无法发送`
          : "无会话";
    }
    if (els.ptySend) els.ptySend.disabled = !canSend;
    if (els.ptyStatus) {
      if (sending) els.ptyStatus.textContent = "发送中…";
      else if (!snapshot) els.ptyStatus.textContent = "";
      else if (!running)
        els.ptyStatus.textContent =
          "当前不可交互：请确认会话为 running；已结束请停止后新开。";
      else
        els.ptyStatus.textContent =
          "点终端区域无效；在输入框打字后点「发送」或按 Enter。";
    }
  }

  async function sendRound() {
    const raw = els.ptyInput?.value ?? "";
    const data = raw.trim();
    const snap = ctx.state.lastSnapshot;
    if (!ctx.state.sessionId || !data || sending) return;
    if (snap?.state !== "running") {
      const msg = "会话未在运行，无法发送";
      if (els.error) els.error.textContent = msg;
      showToast(msg, "bad");
      syncComposer(snap);
      return;
    }
    sending = true;
    syncComposer(snap);
    appendPty(`\r\n› ${data}\r\n`, { raw: true });
    try {
      const res = await api.writePty({
        sessionId: ctx.state.sessionId,
        data: data + "\n",
      });
      if (res?.error) {
        const msg = `${res.error.code}: ${res.error.message}`;
        if (els.error) els.error.textContent = msg;
        showToast(
          res.error.code === "INVALID_ARGUMENT"
            ? "发送失败：Agent 可能已结束本轮，请停止后新开会话"
            : `发送失败：${res.error.message}`,
          "bad",
        );
        return;
      }
      if (els.ptyInput) els.ptyInput.value = "";
      if (els.error) els.error.textContent = "";
    } finally {
      sending = false;
      syncComposer(ctx.state.lastSnapshot);
    }
  }

  function showEmpty() {
    els.empty?.classList.remove("hidden");
    els.workbench?.classList.add("hidden");
  }

  function showWorkbench() {
    els.empty?.classList.add("hidden");
    els.workbench?.classList.remove("hidden");
  }

  const emptyHome = root.querySelector("#sess-empty-home");
  if (emptyHome && typeof emptyHome.addEventListener === "function") {
    emptyHome.addEventListener("click", () => ctx.navigate?.("home"));
  }
  const emptyWizard = root.querySelector("#sess-empty-wizard");
  if (emptyWizard && typeof emptyWizard.addEventListener === "function") {
    emptyWizard.addEventListener("click", () => {
      if (ctx.state.health?.available === false) {
        ctx.showBlock?.(ctx.state.health.reason);
        return;
      }
      ctx.navigate?.("wizard", { fresh: true });
    });
  }

  function setAwaiting(on) {
    const list = els.boundary?.classList;
    if (list && typeof list.toggle === "function") {
      list.toggle("awaiting", !!on);
    }
  }

  function notifyResize() {
    if (!ctx.state.sessionId) return;
    const t = ensureTerm();
    if (!t?.fit) return;
    const size = t.fit();
    if (size?.cols && size?.rows) {
      api
        .resizePty({
          sessionId: ctx.state.sessionId,
          cols: size.cols,
          rows: size.rows,
        })
        .catch(() => {});
    }
  }

  async function refreshPermSummary() {
    if (!els.permSummary) return;
    const sid = ctx.state.sessionId || "";
    if (!sid) {
      els.permSummary.innerHTML = `<p class="meta">无会话</p>`;
      return;
    }
    try {
      const [res, hostsRes] = await Promise.all([
        api.getPermissionOverview(sid),
        api.listRememberedHosts(sid),
      ]);
      if (res?.error) {
        els.permSummary.innerHTML = `<p class="bad">${res.error.code}: ${res.error.message}</p>`;
        return;
      }
      const o = res.overview || {};
      const remembered = hostsRes.hosts || [];
      const hosts = (o.allowedHosts || []).slice(0, 6);
      const hostLis =
        hosts.map((h) => `<li>${h}</li>`).join("") ||
        `<li class="meta">无固定白名单</li>`;
      const remLis =
        remembered
          .map(
            (h) =>
              `<li class="row audit-revokable" style="justify-content:space-between;gap:8px;align-items:center">
                <span>${escapeAttr(h.host)} · ${h.scope === "project" ? "本项目" : "本会话"}</span>
                <button type="button" class="btn" data-rev-host="${escapeAttr(h.host)}" data-rev-ws="${escapeAttr(h.workspace || o.workspace || "")}">撤销</button>
              </li>`,
          )
          .join("") || `<li class="meta">尚无扩权记住项</li>`;
      els.permSummary.innerHTML = `
        <p class="meta">工作区 ${o.workspace || "—"}</p>
        <p class="meta">网络白名单</p>
        <ul class="audit-list">${hostLis}</ul>
        <p class="meta">已记住（扩权）</p>
        <ul class="audit-list" id="sess-rem-hosts">${remLis}</ul>
        <p class="meta">完整边界点顶栏「权限一览」</p>`;
      bindRevokeButtons(els.permSummary);
    } catch (e) {
      els.permSummary.innerHTML = `<p class="meta">边界摘要暂不可用</p>`;
    }
  }

  function bindRevokeButtons(scope) {
    scope?.querySelectorAll("[data-rev-host]").forEach((btn) => {
      btn.onclick = async () => {
        btn.disabled = true;
        try {
          await api.revokeRememberedHost({
            host: btn.getAttribute("data-rev-host"),
            workspace: btn.getAttribute("data-rev-ws"),
          });
          showToast("已撤销记住的允许", "ok");
          await refreshPermSummary();
        } catch (e) {
          showToast(e?.message || "撤销失败", "bad");
          btn.disabled = false;
        }
      };
    });
  }

  function prependActivity(innerHtml) {
    if (!els.audit) return;
    const li = document.createElement("li");
    li.innerHTML = innerHtml;
    els.audit.prepend(li);
    bindRevokeButtons(li);
  }

  async function fillModels(selectedId) {
    if (!els.model) return;
    if (typeof document === "undefined" || !document.createElement) return;
    const res = await api.listAccounts("model");
    const all = res.accounts || [];
    const agentId = ctx.state.lastSnapshot?.agentId || ctx.state.agentId || "";
    const agents = ctx.state.agents || [];
    const want =
      selectedId ||
      ctx.state.lastSnapshot?.modelAccountId ||
      ctx.state.modelAccount?.id ||
      "";
    const current =
      all.find((a) => a.id === want) ||
      (ctx.state.modelAccount?.id === want ? ctx.state.modelAccount : null);
    const running = ctx.state.lastSnapshot?.state === "running";
    // 运行中：CONTRACT / Gate 同 profileId + host；否则向导期按 Agent 兼容过滤。
    let accounts = running
      ? filterModelsForSwitch(all, current)
      : filterModelsForAgent(all, agentId, agents);
    if (running && current && !accounts.some((a) => a.id === current.id)) {
      accounts = [current, ...accounts];
    }
    const alternates = accounts.filter((a) => a.id !== want);
    const canSwitch = running && alternates.length > 0;
    if (els.modelMeta) {
      els.modelMeta.textContent = running
        ? canSwitch
          ? SWITCH_MODEL_META
          : `${SWITCH_MODEL_META} 当前无其他同 profile / 同主机账号可切换。`
        : "";
    }
    els.model.disabled = running && !canSwitch;
    if (running && !canSwitch) {
      // 无备选：只保留当前项，避免误选不可切换账号
      accounts = current ? [current] : accounts.filter((a) => a.id === want);
    }
    const options = accounts.map((a) => {
      const o = document.createElement("option");
      o.value = a.id;
      o.textContent = `${a.name} · ${a.host || a.profileId || ""}`;
      if (a.id === want) o.selected = true;
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
      showEmpty();
      setAwaiting(false);
      if (els.state) {
        els.state.textContent = "无会话";
        els.state.className = "status-pill";
      }
      if (els.meta) els.meta.textContent = "";
      if (els.stop) els.stop.disabled = true;
      return;
    }
    showWorkbench();
    ensureTerm();
    els.state.textContent = s.state;
    els.state.className = `status-pill ${s.state}`;
    const toolN = (s.mcpIds || []).length;
    const isClone = s.mountMode === "clone";
    const source = s.sourceWorkspace || "";
    els.meta.textContent = `${s.agentId || ""} · ${isClone && source ? `原:${source} · 副本:${s.workspace || ""}` : s.workspace || ""} · 文件:${isClone ? "安全副本" : "直接编辑"} · 工具:${toolN}`;
    els.hintLine.textContent = `● 隔离运行中 · ${s.sessionId || ""}`;
    const active =
      ["provisioning", "running", "stopping"].includes(s.state) ||
      (s.state === "failed" && !s.cleanupComplete);
    els.stop.disabled = !active;
    // 运行中或已停止未销毁均可试导回；Destroy 后会话记录消失。
    const canSync = isClone && !!source && s.state !== "stopping";
    if (els.syncBack) {
      if (els.syncBack.classList?.toggle) {
        els.syncBack.classList.toggle("hidden", !isClone);
      } else if (isClone) {
        els.syncBack.className = (els.syncBack.className || "")
          .replace(/\bhidden\b/g, "")
          .trim();
      } else {
        els.syncBack.className = `${els.syncBack.className || ""} hidden`.trim();
      }
      els.syncBack.disabled = !canSync;
    }
    if (s.error) els.error.textContent = `${s.error.code}: ${s.error.message}`;
    else els.error.textContent = "";
    if (s.state === "failed" && !s.cleanupComplete) {
      els.hint.classList.remove("hidden");
      els.hint.textContent = "清理未完成，请再点停止。";
    } else if (s.state === "stopped") {
      els.hint.classList.remove("hidden");
      els.hint.textContent = isClone
        ? "本轮已结束。请点「导回原文件夹」把副本改动写回源目录，再到 Finder 确认新文件；然后可销毁副本。"
        : "本轮已结束。可回 HOME「再次开始」或开新会话。";
    } else if (isClone && active) {
      els.hint.classList.remove("hidden");
      els.hint.textContent = "安全副本模式：改动在副本中；需要时点「导回原文件夹」覆盖写回原目录。";
    } else {
      els.hint.classList.add("hidden");
    }
    if (s.modelAccountId) fillModels(s.modelAccountId);
    refreshPermSummary();
    syncComposer(s);
  }

  function appendPty(text, { raw = false } = {}) {
    const t = ensureTerm();
    if (!t) return;
    if (!text) return;
    if (raw) {
      t.write(text);
      return;
    }
    const pretty = streamDisplay.push(text);
    if (pretty) t.write(pretty);
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
      markTutorialStep(3);
      tail.append(ev.seq, ev.output.text || "");
      appendPty(ev.output.text || "");
    } else if (ev.type === "pty" && (ev.pty || ev.output)) {
      const t = (ev.pty || ev.output).text || "";
      markTutorialStep(3);
      appendPty(t);
    } else if (ev.type === "audit" && ev.audit) {
      auditCount += 1;
      els.statAudit.textContent = String(auditCount);
      recordAudit(ev.audit, ev.sessionId || "");
      const li = document.createElement("li");
      li.textContent = `${ev.audit.result} · ${ev.audit.tool || "-"} @ ${ev.audit.target || "-"}`;
      els.audit.prepend(li);
    } else if (ev.type === "error" && ev.error) {
      els.error.textContent = formatErrorWithHint(ev.error);
    } else if (ev.type === "permission_needed" && ev.permission) {
      const snap = ctx.state.lastSnapshot;
      const sid = ctx.state.sessionId || snap?.sessionId || "";
      const uiDemo = !!ev.permission.uiDemo;
      setAwaiting(true);
      hidePermissionOverviewModal(modalRoot);
      mountPermissionModal(modalRoot, {
        sessionLabel: `${snap?.agentId || "—"} · ${snap?.workspace || "—"} · ${sid}`,
        target: ev.permission.target,
        action: ev.permission.action,
        onPendingChange: (pending) => ctx.setPermissionPending?.(pending),
        async onDecide(decision) {
          // uiDemo：仅前端演练，无 Orch 挂起；live 不得调 ResolvePermission（会 INVALID_ARGUMENT）。
          // mock 真事件 / live Gate 真事件：必须走 resolvePermission。
          if (!(uiDemo && !api.isMock())) {
            const res = await api.resolvePermission({
              sessionId: sid,
              target: ev.permission.target,
              decision,
            });
            if (res?.error) {
              throw new Error(`${res.error.code}: ${res.error.message}`);
            }
          }
          recordPermissionDecision({
            sessionId: sid,
            target: ev.permission.target,
            action: ev.permission.action,
            decision,
            uiDemo: uiDemo && !api.isMock(),
          });
          hidePermissionModal(modalRoot);
          setAwaiting(false);
          const tag = uiDemo ? "演示扩权" : "扩权";
          const wroteProject =
            decision === "project" && !(uiDemo && !api.isMock());
          const ws =
            snap?.workspace ||
            ctx.state.lastSnapshot?.workspace ||
            "";
          if (wroteProject) {
            prependActivity(
              `<span class="row audit-revokable" style="justify-content:space-between;gap:8px;align-items:center;width:100%">
                <span>${tag} ${decision} · ${escapeAttr(ev.permission.target)}</span>
                <button type="button" class="btn" data-rev-host="${escapeAttr(ev.permission.target)}" data-rev-ws="${escapeAttr(ws)}">撤销</button>
              </span>`,
            );
          } else {
            prependActivity(
              `<span>${tag} ${decision} · ${escapeAttr(ev.permission.target)} · ${escapeAttr(sid)}</span>`,
            );
          }
          if (uiDemo && !api.isMock() && decision === "project") {
            // live UI 演示：权限一览仍以本机记住表为准，不写入假 project
            els.error.textContent =
              "这是界面演示，未写入本机记住表；真扩权须由 Gate 跨主机重定向触发。";
          }
          await refreshPermSummary();
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
        const line = `${res.error.code}: ${res.error.message}`;
        els.error.textContent = line;
        showToast(line, "bad");
        // 切失败时下拉仍停在新选项，拉回当前会话账号以免误导
        fillModels(ctx.state.lastSnapshot?.modelAccountId);
      }
    };
  }

  if (els.syncBack) {
    els.syncBack.onclick = async () => {
      if (!ctx.state.sessionId) return;
      const snap = ctx.state.lastSnapshot;
      const dest = snap?.sourceWorkspace || "";
      const ok = await askExitReview(modalRoot, {
        sourceWorkspace: dest,
        paths: [],
      });
      if (!ok) return;
      els.syncBack.disabled = true;
      showToast("正在导回…", "");
      try {
        const res = await api.syncBackWorkspace(ctx.state.sessionId);
        if (res.error) {
          showToast(`${res.error.code}: ${res.error.message}`, "bad");
          if (els.error)
            els.error.textContent = `${res.error.code}: ${res.error.message}`;
          return;
        }
        if (res.session) {
          ctx.state.lastSnapshot = res.session;
          renderSnapshot(res.session);
        }
        const destHint = dest ? `：${dest}` : "";
        showToast(
          api.isMock()
            ? `已导回原文件夹（模拟）${destHint}`
            : `已导回原文件夹${destHint}（已在访达中显示）`,
          "ok",
          { durationMs: 6000 },
        );
        if (els.hint) {
          els.hint.classList.remove("hidden");
          els.hint.textContent =
            "导回成功。请在源目录核对后，再决定是否销毁隔离副本。";
        }
      } catch (e) {
        showToast(e?.message || "导回失败", "bad");
      } finally {
        const s = ctx.state.lastSnapshot;
        els.syncBack.disabled = !(
          s?.mountMode === "clone" && s?.sourceWorkspace
        );
      }
    };
  }

  if (els.stop) {
    els.stop.onclick = async () => {
      if (!ctx.state.sessionId) return;
      const prefs = ctx.state.uiPrefs || {};
      const isClone = ctx.state.lastSnapshot?.mountMode === "clone";
      const stopOk = await askConfirm(
        {
          title: "停止会话",
          body: "停止后 Agent 不再运行。可稍后再开或销毁隔离数据。",
          confirmLabel: "停止",
          cancelLabel: "取消",
          danger: true,
        },
        modalRoot,
      );
      if (!stopOk) return;
      let destroyAfter = false;
      if (prefs.askClearOnStop !== false) {
        destroyAfter = await askConfirm(
          {
            title: "清除隔离数据？",
            body: isClone
              ? "确定 = 清除并停止（未导回的安全副本改动会丢失）\n取消 = 仅停止（停止后仍可导回）"
              : "确定 = 清除隔离环境数据并停止（本机密钥保留）\n取消 = 仅停止",
            confirmLabel: "清除并停止",
            cancelLabel: "仅停止",
            danger: true,
          },
          modalRoot,
        );
      } else {
        destroyAfter = prefs.clearOnStopDefault === "clear";
      }
      if (destroyAfter && isClone) {
        const syncFirst = await askConfirm(
          {
            title: "销毁前导回？",
            body: "确定 = 先导回原文件夹再清除\n取消 = 直接清除（不导回）",
            confirmLabel: "先导回再清除",
            cancelLabel: "直接清除",
          },
          modalRoot,
        );
        if (syncFirst) {
          showToast("正在导回…", "");
          const syncRes = await api.syncBackWorkspace(ctx.state.sessionId);
          if (syncRes.error) {
            showToast(`${syncRes.error.code}: ${syncRes.error.message}`, "bad");
            const still = await askConfirm(
              {
                title: "导回失败",
                body: "仍要清除副本吗？未导回的改动会丢失。",
                confirmLabel: "仍要清除",
                cancelLabel: "取消停止",
                danger: true,
              },
              modalRoot,
            );
            if (!still) return;
          } else {
            const src =
              syncRes.session?.sourceWorkspace ||
              ctx.state.lastSnapshot?.sourceWorkspace ||
              "";
            showToast(
              api.isMock()
                ? `已导回原文件夹（模拟）${src ? `：${src}` : ""}`
                : `已导回原文件夹${src ? `：${src}` : ""}（已在访达中显示）`,
              "ok",
              { durationMs: 6000 },
            );
          }
        }
      }
      els.stop.disabled = true;
      showToast("正在停止…", "");
      try {
        const res = await api.stopSession(ctx.state.sessionId);
        if (res.error) {
          showToast(`${res.error.code}: ${res.error.message}`, "bad");
          if (els.error)
            els.error.textContent = `${res.error.code}: ${res.error.message}`;
          els.stop.disabled = false;
          return;
        }
        if (res.session) {
          ctx.state.lastSnapshot = res.session;
          renderSnapshot(res.session);
        }
        showToast(destroyAfter ? "已停止，正在清理…" : "已停止", "ok");
        const reportSnap = ctx.state.lastSnapshot || res.session;
        mountRunReportModal(modalRoot, {
          sessionId: ctx.state.sessionId || reportSnap?.sessionId || "",
          workspace: reportSnap?.workspace || reportSnap?.sourceWorkspace || "",
          agentId: reportSnap?.agentId || "",
        });
        if (destroyAfter && ctx.state.sessionId) {
          const sid = ctx.state.sessionId;
          const waitClean = () =>
            new Promise((resolve) => {
              const off = api.onEvent((ev) => {
                if (
                  ev.sessionId === sid &&
                  ev.type === "state_changed" &&
                  ev.state?.cleanupComplete
                ) {
                  off();
                  resolve();
                }
              });
              setTimeout(() => {
                off();
                resolve();
              }, 8000);
            });
          await waitClean();
          await ctx.destroySession?.(sid);
          showToast("已清除隔离数据", "ok");
        }
      } catch (e) {
        showToast(e?.message || "停止失败", "bad");
        els.stop.disabled = false;
      }
    };
  }

  const toHome = root.querySelector("#btn-to-home");
  if (toHome) {
    toHome.onclick = () => {
      ctx.navigate("home");
    };
  }

  const permBtn = root.querySelector("#btn-perm");
  if (permBtn) {
    permBtn.onclick = () => {
      hidePermissionModal(modalRoot);
      mountPermissionOverviewModal(modalRoot, {
        sessionId: ctx.state.sessionId || "",
        mountMode: ctx.state.lastSnapshot?.mountMode || "",
        onChanged: () => refreshPermSummary(),
      });
    };
  }

  if (els.ptyInput?.addEventListener) {
    els.ptyInput.addEventListener("keydown", (e) => {
      if (e.key !== "Enter" || e.shiftKey) return;
      e.preventDefault();
      sendRound();
    });
  }
  if (els.ptySend) els.ptySend.onclick = () => sendRound();
  syncComposer(ctx.state.lastSnapshot);

  const side = root.querySelector(".session-side");
  if (side && typeof side.appendChild === "function" && typeof document !== "undefined") {
    const demo = document.createElement("button");
    demo.type = "button";
    demo.className = "btn";
    demo.style.marginTop = "8px";
    // 真实挂起（PromptPermission）；与竞品 ask 对齐，不依赖上游 302。
    demo.textContent = api.isMock() ? "请求扩权（mock）" : "请求扩权";
    side.appendChild(demo);
    demo.onclick = async () => {
      if (!ctx.state.sessionId) {
        els.error.textContent = "请先打开会话再请求扩权";
        return;
      }
      const target = "registry.npmjs.org";
      if (typeof api.promptPermission === "function") {
        const res = await api.promptPermission({
          sessionId: ctx.state.sessionId,
          target,
        });
        if (res?.error) {
          els.error.textContent = formatErrorWithHint(res.error);
          return;
        }
        els.error.textContent = "";
        return;
      }
      // 旧绑定回退：仅界面演练
      onEvent({
        sessionId: ctx.state.sessionId,
        seq: Date.now(),
        type: "permission_needed",
        permission: {
          target,
          action: "connect",
          uiDemo: true,
        },
      });
    };
  }

  if (typeof window !== "undefined") {
    window.addEventListener("resize", notifyResize);
  }

  // 初始无会话
  showEmpty();

  return {
    resetForNewSession(snapshot) {
      tail = new OutputTail();
      streamDisplay = createStreamDisplay();
      outCount = 0;
      auditCount = 0;
      if (els.statOut) els.statOut.textContent = "0";
      if (els.statAudit) els.statAudit.textContent = "0";
      ensureTerm()?.clear();
      if (els.audit) els.audit.innerHTML = "";
      if (els.error) els.error.textContent = "";
      if (els.permSummary) els.permSummary.innerHTML = `<p class="meta">加载中…</p>`;
      els.hint?.classList.add("hidden");
      hidePermissionModal(modalRoot);
      setAwaiting(false);
      if (snapshot) {
        ctx.state.lastSnapshot = snapshot;
        api.activateSession(snapshot);
        renderSnapshot(snapshot);
        notifyResize();
      } else {
        renderSnapshot(null);
      }
      fillModels(snapshot?.modelAccountId);
    },
    showEmpty,
    destroy() {
      unsub();
      hidePermissionModal(modalRoot);
      setAwaiting(false);
      term?.dispose?.();
      term = null;
      if (typeof window !== "undefined") {
        window.removeEventListener("resize", notifyResize);
      }
    },
  };
}
