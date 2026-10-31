/**
 * 标题栏隔离徽章：Seatbelt / Docker·托管|本机；待重启含后端或引擎底座切换。
 */

export function normalizeEngineSource(v) {
  const s = String(v || "")
    .toLowerCase()
    .trim();
  if (
    s === "system-docker" ||
    s === "system" ||
    s === "docker" ||
    s === "desktop"
  ) {
    return "system-docker";
  }
  return "harbor-colima";
}

export function normalizeIsolationBackend(v) {
  const s = String(v || "")
    .toLowerCase()
    .trim();
  return s === "docker" || s === "p2" || s === "container"
    ? "docker"
    : "seatbelt";
}

export function engineShortLabel(src) {
  return normalizeEngineSource(src) === "system-docker" ? "本机" : "托管";
}

export function engineFullLabel(src) {
  return normalizeEngineSource(src) === "system-docker"
    ? "本机 Docker"
    : "Harbor 托管 Colima";
}

export function isolationLevelFromHealth(health) {
  const levelRaw = String(health?.isolationLevel || "").toLowerCase();
  if (levelRaw === "docker" || levelRaw.includes("docker")) return "docker";
  if (
    levelRaw === "seatbelt" ||
    levelRaw.includes("seatbelt") ||
    levelRaw.includes("mock-seatbelt")
  ) {
    return "seatbelt";
  }
  return "";
}

/**
 * @param {{
 *   health: { available?: boolean, provisional?: boolean, isolationLevel?: string } | null,
 *   preferredBackend: string,
 *   preferredEngine: string,
 *   engineSourceAtLaunch: string,
 *   allowsStart: boolean,
 * }} p
 */
export function computeIsolationBadge(p) {
  const preferred = normalizeIsolationBackend(p.preferredBackend);
  const preferredEngine = normalizeEngineSource(p.preferredEngine);
  const atLaunch = normalizeEngineSource(
    p.engineSourceAtLaunch || p.preferredEngine,
  );
  const fromHealth = isolationLevelFromHealth(p.health);
  const pendingIsolation = !!fromHealth && fromHealth !== preferred;
  const pendingEngine =
    preferred === "docker" && preferredEngine !== atLaunch;
  const pendingRestart = pendingIsolation || pendingEngine;
  const verified = !!p.allowsStart && !!fromHealth;

  const backendLabel = (v) => (v === "docker" ? "Docker" : "Seatbelt");
  const dockerText = (engine, suffix) => {
    const base = `Docker · ${engineShortLabel(engine)}`;
    return suffix ? `${base}（${suffix}）` : base;
  };

  let text;
  let className;
  if (pendingRestart) {
    text =
      preferred === "docker"
        ? dockerText(preferredEngine, "待重启")
        : `${backendLabel(preferred)}（待重启）`;
    className = "badge isolation-badge isolation-pending";
  } else if ((fromHealth || preferred) === "docker") {
    text = verified
      ? dockerText(preferredEngine)
      : dockerText(preferredEngine, "偏好");
    className = `badge isolation-badge ${verified ? "isolation-docker" : "isolation-pending"}`;
  } else {
    text = verified
      ? backendLabel("seatbelt")
      : `${backendLabel("seatbelt")}（偏好）`;
    className = `badge isolation-badge ${verified ? "isolation-seatbelt" : "isolation-pending"}`;
  }

  const bits = [];
  if (fromHealth) {
    bits.push(`当前生效：${backendLabel(fromHealth)}`);
  } else {
    bits.push(
      `设置偏好：${backendLabel(preferred)}（尚未由健康检查确认）`,
    );
  }
  if (preferred === "docker" || fromHealth === "docker") {
    bits.push(`引擎偏好：${engineFullLabel(preferredEngine)}`);
    if (pendingEngine) {
      bits.push(
        `本进程启动时为 ${engineFullLabel(atLaunch)}，须重启后切换`,
      );
    }
  }
  if (pendingIsolation) {
    bits.push(`偏好已改为 ${backendLabel(preferred)}，须重启应用后生效`);
  }
  bits.push("设置 → 隔离后端 / 容器引擎 可切换");

  return {
    text,
    className,
    title: bits.join(" · "),
    pendingRestart,
    pendingIsolation,
    pendingEngine,
  };
}
