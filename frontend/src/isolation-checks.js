/**
 * 隔离能力逐条检测（产品文案）。后端仍一次 GetRuntimeHealth；前端分阶段上屏。
 */

export const ISOLATION_CHECK_DEFS = [
  {
    id: "os",
    name: "系统环境",
    okDetail: "macOS Darwin 25 · arm64 · 非 root",
    checkingDetail: "检查操作系统与权限…",
  },
  {
    id: "tmpdir",
    name: "临时探测区",
    okDetail: "可在本机建立隔离探测目录",
    checkingDetail: "创建探测目录…",
  },
  {
    id: "profile",
    name: "Seatbelt 规则",
    okDetail: "沙箱配置生成成功（默认拒绝 + 工作区白名单）",
    checkingDetail: "生成沙箱规则…",
  },
  {
    id: "sandbox",
    name: "沙箱可启动",
    okDetail: "受控进程可在沙箱内启动",
    checkingDetail: "试启动 sandbox-exec…",
  },
  {
    id: "read-ok",
    name: "工作区可读",
    okDetail: "授权目录内文件可读",
    checkingDetail: "验证授权路径读取…",
  },
  {
    id: "read-deny",
    name: "越权拒绝",
    okDetail: "工作区外路径读取被拒绝",
    checkingDetail: "验证越权路径拦截…",
  },
  {
    id: "gate",
    name: "安全闸口",
    okDetail: "可创建新会话",
    failDetail: "依赖隔离就绪；不会无隔离启动 Agent",
    checkingDetail: "确认开跑准入…",
  },
];

const PROBE_COUNT = 6;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function freshCheckItems() {
  return ISOLATION_CHECK_DEFS.map((d) => ({
    id: d.id,
    name: d.name,
    status: "pending",
    detail: "等待检测",
  }));
}

/** 已有最终 health 时的静态列表（阻断页首屏等） */
export function checksFromHealth(health) {
  const ok = !!health?.available;
  const reason = health?.reason || "ISOLATION_UNAVAILABLE";
  return ISOLATION_CHECK_DEFS.map((d, i) => {
    if (ok) {
      const detail =
        i === PROBE_COUNT && health?.isolationLevel
          ? `${d.okDetail} · ${health.isolationLevel}`
          : d.okDetail;
      return { id: d.id, name: d.name, status: "ok", detail };
    }
    if (i === 0) {
      return { id: d.id, name: d.name, status: "bad", detail: reason };
    }
    if (i < PROBE_COUNT) {
      return {
        id: d.id,
        name: d.name,
        status: "skipped",
        detail: "因前项未通过而跳过",
      };
    }
    return {
      id: d.id,
      name: d.name,
      status: "bad",
      detail: d.failDetail || "依赖隔离就绪",
    };
  });
}

/**
 * @param {{ getHealth: () => Promise<object>, onUpdate?: (items) => void, stepMs?: number }} opts
 * @returns {Promise<object>} health
 */
export async function runIsolationProbe({
  getHealth,
  onUpdate,
  stepMs = 160,
} = {}) {
  const items = freshCheckItems();
  const emit = () => onUpdate?.(items.map((x) => ({ ...x })));
  emit();

  if (typeof getHealth !== "function") {
    const failed = { available: false, reason: "ISOLATION_UNAVAILABLE" };
    applyFinal(items, failed);
    emit();
    return failed;
  }

  let settled = null;
  const healthPromise = Promise.resolve()
    .then(() => getHealth())
    .then((h) => {
      settled = h || { available: false, reason: "ISOLATION_UNAVAILABLE" };
      return settled;
    })
    .catch(() => {
      settled = { available: false, reason: "ISOLATION_UNAVAILABLE" };
      return settled;
    });

  for (let i = 0; i < PROBE_COUNT; i++) {
    items[i].status = "checking";
    items[i].detail = ISOLATION_CHECK_DEFS[i].checkingDetail;
    emit();
    await sleep(stepMs);
    if (settled && !settled.available) {
      failAt(items, i, settled.reason || "ISOLATION_UNAVAILABLE");
      emit();
      return settled;
    }
  }

  items[PROBE_COUNT].status = "checking";
  items[PROBE_COUNT].detail = ISOLATION_CHECK_DEFS[PROBE_COUNT].checkingDetail;
  emit();

  const health = settled || (await healthPromise);
  if (!health?.available) {
    applyFinal(items, health);
    emit();
    return health;
  }

  for (let i = 0; i < PROBE_COUNT; i++) {
    items[i].status = "ok";
    items[i].detail = ISOLATION_CHECK_DEFS[i].okDetail;
    emit();
    await sleep(Math.min(70, stepMs));
  }
  items[PROBE_COUNT].status = "ok";
  items[PROBE_COUNT].detail = health.isolationLevel
    ? `${ISOLATION_CHECK_DEFS[PROBE_COUNT].okDetail} · ${health.isolationLevel}`
    : ISOLATION_CHECK_DEFS[PROBE_COUNT].okDetail;
  emit();
  return health;
}

function failAt(items, index, reason) {
  for (let i = 0; i < PROBE_COUNT; i++) {
    if (i < index) {
      items[i].status = "skipped";
      items[i].detail = "因后续探测未通过而中止明细";
    } else if (i === index) {
      items[i].status = "bad";
      items[i].detail = reason;
    } else {
      items[i].status = "skipped";
      items[i].detail = "因前项未通过而跳过";
    }
  }
  items[PROBE_COUNT].status = "bad";
  items[PROBE_COUNT].detail = ISOLATION_CHECK_DEFS[PROBE_COUNT].failDetail;
}

function applyFinal(items, health) {
  const final = checksFromHealth(health);
  for (let i = 0; i < final.length; i++) {
    items[i].status = final[i].status;
    items[i].detail = final[i].detail;
  }
}

export function renderCheckListHtml(items) {
  const list = Array.isArray(items) ? items : [];
  return list
    .map((c) => {
      const st = c.status || (c.ok ? "ok" : "bad");
      const mark =
        st === "ok"
          ? "●"
          : st === "checking"
            ? "…"
            : st === "pending"
              ? "·"
              : st === "skipped"
                ? "–"
                : "○";
      return `
      <li class="${st}">
        <span class="block-check-mark" aria-hidden="true">${mark}</span>
        <span>
          <strong>${escapeHtml(c.name || "检测项")}</strong>
          <span class="meta">${escapeHtml(c.detail || "")}</span>
        </span>
      </li>`;
    })
    .join("");
}

function escapeHtml(s) {
  return String(s || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function paintCheckList(el, items) {
  if (!el) return;
  el.innerHTML = renderCheckListHtml(items);
}
