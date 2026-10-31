import test from "node:test";
import assert from "node:assert/strict";
import {
  clampMeterTipPosition,
  formatMeterBucketTip,
  priceCurrencyFormState,
  priceModelChoices,
} from "./records-price.js";

test("priceModelChoices hides already-priced models when creating", () => {
  const r = priceModelChoices({
    accountModels: [
      { value: "deepseek-chat", label: "ds" },
      { value: "gpt-4o-mini", label: "gpt" },
    ],
    prices: [{ model: "deepseek-chat" }],
    draftModel: "deepseek-chat",
  });
  assert.deepEqual(
    r.modelChoices.map((m) => m.value),
    ["gpt-4o-mini"],
  );
  assert.equal(r.draftModel, "");
  assert.equal(r.emptyReason, "");
});

test("priceModelChoices reports all-priced and no-accounts", () => {
  assert.equal(
    priceModelChoices({
      accountModels: [{ value: "a", label: "a" }],
      prices: [{ model: "a" }],
    }).emptyReason,
    "all-priced",
  );
  assert.equal(
    priceModelChoices({ accountModels: [], prices: [] }).emptyReason,
    "no-accounts",
  );
});

test("priceModelChoices edit mode only shows current model", () => {
  const r = priceModelChoices({
    accountModels: [
      { value: "deepseek-chat", label: "ds" },
      { value: "gpt", label: "gpt" },
    ],
    prices: [{ model: "deepseek-chat" }, { model: "gpt" }],
    editingModel: "deepseek-chat",
  });
  assert.deepEqual(
    r.modelChoices.map((m) => m.value),
    ["deepseek-chat"],
  );
  assert.equal(r.emptyReason, "");
});

test("priceCurrencyFormState locks peer currency and disables other button", () => {
  const locked = priceCurrencyFormState({
    prices: [{ model: "a", currency: "CNY" }],
    draftCcy: "USD",
  });
  assert.equal(locked.ccySelectLocked, true);
  assert.equal(locked.formCcy, "CNY");
  assert.equal(locked.draftCcy, "CNY");
  assert.equal(locked.usdDisabled, true);
  assert.equal(locked.cnyDisabled, false);

  const editingUnlock = priceCurrencyFormState({
    prices: [{ model: "a", currency: "CNY" }],
    editingModel: "a",
    draftCcy: "USD",
  });
  assert.equal(editingUnlock.ccySelectLocked, false);
  assert.equal(editingUnlock.formCcy, "USD");
});

test("formatMeterBucketTip covers qty and price modes", () => {
  assert.equal(
    formatMeterBucketTip({
      mode: "qty",
      label: "10:00",
      model: "deepseek-chat",
      inputTokens: 12,
      outputTokens: 3,
    }),
    "10:00 · deepseek-chat\n入 12 token\n出 3 token\n合计 15 token",
  );
  assert.equal(
    formatMeterBucketTip({
      mode: "price",
      label: "10:00",
      model: "deepseek-chat",
      inputCost: 0.1,
      outputCost: 0.2,
      currency: "CNY",
    }),
    "10:00 · deepseek-chat\n入估 0.1000 CNY\n出估 0.2000 CNY\n合计 0.3000",
  );
});

test("clampMeterTipPosition flips near viewport edges", () => {
  const mid = clampMeterTipPosition({
    clientX: 40,
    clientY: 40,
    tipW: 100,
    tipH: 40,
    vw: 800,
    vh: 600,
  });
  assert.equal(mid.left, 52);
  assert.equal(mid.top, 52);

  const edge = clampMeterTipPosition({
    clientX: 790,
    clientY: 590,
    tipW: 120,
    tipH: 50,
    vw: 800,
    vh: 600,
  });
  assert.ok(edge.left < 790);
  assert.ok(edge.top < 590);
  assert.ok(edge.left >= 8);
  assert.ok(edge.top >= 8);
});
