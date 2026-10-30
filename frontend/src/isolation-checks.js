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

/** P2 Docker 后端探测文案（仍一次 GetRuntimeHealth，仅展示映射不同）。 */
export const DOCKER_CHECK_DEFS = [
  {
    id: "os",
    name: "Docker CLI",
    okDetail: "本机 docker 可执行",
    checkingDetail: "查找 docker CLI…",
  },
  {
    id: "tmpdir",
    name: "Docker daemon",
    okDetail: "daemon 可响应",
    checkingDetail: "探测 Docker daemon…",
  },
  {
    id: "profile",
    name: "Harbor 网络",
    okDetail: "agent-harbor-p2（internal）就绪",
    checkingDetail: "确保 internal 网络…",
  },
  {
    id: "sandbox",
    name: "运行时镜像",
    okDetail: "agent-harbor/runtime:dev 可用",
    checkingDetail: "校验 Harbor 镜像…",
  },
  {
    id: "read-ok",
    name: "Gate 入口",
    okDetail: "仅 harbor.gate（host-gateway）",
    checkingDetail: "确认 Gate 别名策略…",
  },
  {
    id: "read-deny",
    name: "别名拉黑",
    okDetail: "host.docker.internal 已拉黑",
    checkingDetail: "确认常见 SSRF 别名拉黑…",
  },
  {
    id: "gate",
    name: "安全闸口",
    okDetail: "可创建新会话",
    failDetail: "依赖 Docker 隔离就绪；不会无隔离启动 Agent",
    checkingDetail: "确认开跑准入…",
  },
];

const PROBE_COUNT = 6;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function defsFor(level) {
  return String(level || "").toLowerCase() === "docker"
    ? DOCKER_CHECK_DEFS
    : ISOLATION_CHECK_DEFS;
}

export function freshCheckItems(level) {
  return defsFor(level).map((d) => ({
    id: d.id,
    name: d.name,
    status: "pending",
    detail: "等待检测",
  }));
}

/** 已有最终 health 时的静态列表（阻断页首屏等） */
export function checksFromHealth(health, preferredLevel = "") {
  const ok = !!health?.available;
  const reason = health?.reason || "ISOLATION_UNAVAILABLE";
  const level = ok
    ? health?.isolationLevel
    : health?.isolationLevel || preferredLevel;
  const defs = defsFor(level);
  return defs.map((d, i) => {
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
 * @param {{ getHealth: () => Promise<object>, onUpdate?: (items) => void, stepMs?: number, preferredLevel?: string }} opts
 * @returns {Promise<object>} health
 */
export async function runIsolationProbe({
  getHealth,
  onUpdate,
  stepMs = 160,
  preferredLevel = "",
} = {}) {
  let defs = defsFor(preferredLevel);
  const items = defs.map((d) => ({
    id: d.id,
    name: d.name,
    status: "pending",
    detail: "等待检测",
  }));
  const emit = () => onUpdate?.(items.map((x) => ({ ...x })));
  emit();

  if (typeof getHealth !== "function") {
    const failed = { available: false, reason: "ISOLATION_UNAVAILABLE" };
    applyFinal(items, failed, preferredLevel);
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
    items[i].detail = defs[i].checkingDetail;
    emit();
    await sleep(stepMs);
    if (settled && !settled.available) {
      failAt(items, i, settled.reason || "ISOLATION_UNAVAILABLE", defs);
      emit();
      return settled;
    }
  }

  items[PROBE_COUNT].status = "checking";
  items[PROBE_COUNT].detail = defs[PROBE_COUNT].checkingDetail;
  emit();

  const health = settled || (await healthPromise);
  if (health?.isolationLevel) {
    defs = defsFor(health.isolationLevel);
    for (let i = 0; i < defs.length; i++) {
      items[i].id = defs[i].id;
      items[i].name = defs[i].name;
    }
  }
  if (!health?.available) {
    applyFinal(items, health, preferredLevel);
    emit();
    return health;
  }

  for (let i = 0; i < PROBE_COUNT; i++) {
    items[i].status = "ok";
    items[i].detail = defs[i].okDetail;
    emit();
    await sleep(Math.min(70, stepMs));
  }
  items[PROBE_COUNT].status = "ok";
  items[PROBE_COUNT].detail = health.isolationLevel
    ? `${defs[PROBE_COUNT].okDetail} · ${health.isolationLevel}`
    : defs[PROBE_COUNT].okDetail;
  emit();
  return health;
}

function failAt(items, index, reason, defs) {
  const list = defs || ISOLATION_CHECK_DEFS;
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
  items[PROBE_COUNT].detail = list[PROBE_COUNT].failDetail;
}

function applyFinal(items, health, preferredLevel) {
  const final = checksFromHealth(health, preferredLevel);
  for (let i = 0; i < final.length; i++) {
    items[i].status = final[i].status;
    items[i].detail = final[i].detail;
    items[i].name = final[i].name;
    items[i].id = final[i].id;
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
