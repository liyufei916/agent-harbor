/**
 * 记录·计量：单价表单与时序悬停的纯逻辑（视图层可单测）。
 */
import { lockedPriceCurrency } from "./usage-analytics.js";

/**
 * 新建：下拉只露出尚未配置单价的模型；编辑：只显示当前模型。
 * @param {{
 *   accountModels: { value: string, label: string }[],
 *   prices: { model: string }[],
 *   editingModel?: string,
 *   draftModel?: string,
 * }} p
 * @returns {{
 *   modelChoices: { value: string, label: string }[],
 *   draftModel: string,
 *   emptyReason: "" | "no-accounts" | "all-priced",
 * }}
 */
export function priceModelChoices(p) {
  const accountModels = Array.isArray(p.accountModels) ? p.accountModels : [];
  const prices = Array.isArray(p.prices) ? p.prices : [];
  const editingModel = String(p.editingModel || "").trim();
  let draftModel = String(p.draftModel || "").trim();
  const pricedModels = new Set(prices.map((x) => x.model).filter(Boolean));

  /** @type {{ value: string, label: string }[]} */
  let modelChoices = [];
  if (editingModel) {
    const hit =
      accountModels.find((m) => m.value === editingModel) ||
      (pricedModels.has(editingModel)
        ? { value: editingModel, label: editingModel }
        : null);
    if (hit) modelChoices = [hit];
  } else {
    modelChoices = accountModels.filter((m) => !pricedModels.has(m.value));
    if (draftModel && pricedModels.has(draftModel)) draftModel = "";
  }

  let emptyReason = /** @type {"" | "no-accounts" | "all-priced"} */ ("");
  if (!editingModel) {
    if (accountModels.length === 0) emptyReason = "no-accounts";
    else if (modelChoices.length === 0) emptyReason = "all-priced";
  }

  return { modelChoices, draftModel, emptyReason };
}

/**
 * 币种按钮态：列表已有 CNY/USD 则锁定；编辑时排除自身再判锁。
 * @param {{
 *   prices: { model: string, currency?: string }[],
 *   editingModel?: string,
 *   draftCcy?: string,
 *   lockedCcy?: string,
 * }} p
 */
export function priceCurrencyFormState(p) {
  const prices = Array.isArray(p.prices) ? p.prices : [];
  const editingModel = String(p.editingModel || "").trim();
  let draftCcy = String(p.draftCcy || "").toUpperCase();
  const lockedCcy = String(p.lockedCcy || "").toUpperCase();
  const editing = prices.find((x) => x.model === editingModel);
  const peersForCcy = editingModel
    ? prices.filter((x) => x.model !== editingModel)
    : prices;
  const peerLocked = lockedPriceCurrency(peersForCcy);
  const formCcyDefault =
    editing?.currency ||
    (peerLocked === "CNY" || peerLocked === "USD"
      ? peerLocked
      : lockedCcy === "CNY" || lockedCcy === "USD"
        ? lockedCcy
        : "CNY");
  const ccySelectLocked = peerLocked === "CNY" || peerLocked === "USD";
  if (ccySelectLocked) {
    draftCcy = peerLocked;
  } else if (draftCcy !== "CNY" && draftCcy !== "USD") {
    draftCcy = formCcyDefault === "USD" ? "USD" : "CNY";
  }
  const formCcy = ccySelectLocked ? peerLocked : draftCcy || formCcyDefault;
  return {
    peerLocked,
    formCcyDefault: String(formCcyDefault || "CNY").toUpperCase() === "USD" ? "USD" : "CNY",
    ccySelectLocked,
    draftCcy: formCcy === "USD" ? "USD" : "CNY",
    formCcy: formCcy === "USD" ? "USD" : "CNY",
    cnyDisabled: ccySelectLocked && formCcy !== "CNY",
    usdDisabled: ccySelectLocked && formCcy !== "USD",
  };
}

/**
 * 分模型时序柱悬停文案（数量 / 价格）。
 * @param {{
 *   mode: "qty" | "price",
 *   label: string,
 *   model: string,
 *   inputTokens?: number,
 *   outputTokens?: number,
 *   inputCost?: number,
 *   outputCost?: number,
 *   currency?: string,
 * }} p
 */
export function formatMeterBucketTip(p) {
  const label = String(p.label || "");
  const model = String(p.model || "");
  if (p.mode === "price") {
    const inC = Number(p.inputCost) || 0;
    const outC = Number(p.outputCost) || 0;
    const ccy = String(p.currency || "");
    return `${label} · ${model}\n入估 ${inC.toFixed(4)} ${ccy}\n出估 ${outC.toFixed(4)} ${ccy}\n合计 ${(inC + outC).toFixed(4)}`;
  }
  const inT = Number(p.inputTokens) || 0;
  const outT = Number(p.outputTokens) || 0;
  return `${label} · ${model}\n入 ${inT} token\n出 ${outT} token\n合计 ${inT + outT} token`;
}

/**
 * 悬停 tip 视口夹紧（桌面 WebView 无原生 title）。
 * @param {{ clientX: number, clientY: number, tipW: number, tipH: number, vw: number, vh: number, pad?: number }} p
 */
export function clampMeterTipPosition(p) {
  const pad = Number(p.pad) >= 0 ? Number(p.pad) : 12;
  let left = p.clientX + pad;
  let top = p.clientY + pad;
  if (left + p.tipW > p.vw - 8) left = Math.max(8, p.clientX - p.tipW - pad);
  if (top + p.tipH > p.vh - 8) top = Math.max(8, p.clientY - p.tipH - pad);
  return { left, top };
}
