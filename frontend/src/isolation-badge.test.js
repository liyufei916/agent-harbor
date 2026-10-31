import assert from "node:assert/strict";
import { test } from "node:test";
import {
  computeIsolationBadge,
  engineFullLabel,
  engineShortLabel,
  normalizeEngineSource,
  normalizeIsolationBackend,
} from "./isolation-badge.js";

test("normalizeEngineSource 默认托管", () => {
  assert.equal(normalizeEngineSource(""), "harbor-colima");
  assert.equal(normalizeEngineSource("system-docker"), "system-docker");
  assert.equal(normalizeEngineSource("desktop"), "system-docker");
});

test("normalizeIsolationBackend", () => {
  assert.equal(normalizeIsolationBackend("docker"), "docker");
  assert.equal(normalizeIsolationBackend("p2"), "docker");
  assert.equal(normalizeIsolationBackend(""), "seatbelt");
});

test("engine 短标签", () => {
  assert.equal(engineShortLabel("harbor-colima"), "托管");
  assert.equal(engineShortLabel("system-docker"), "本机");
  assert.equal(engineFullLabel("harbor-colima"), "Harbor 托管 Colima");
});

test("Docker 生效时徽章含引擎底座", () => {
  const view = computeIsolationBadge({
    health: { available: true, isolationLevel: "docker" },
    preferredBackend: "docker",
    preferredEngine: "harbor-colima",
    engineSourceAtLaunch: "harbor-colima",
    allowsStart: true,
  });
  assert.equal(view.text, "Docker · 托管");
  assert.equal(view.pendingRestart, false);
  assert.match(view.className, /isolation-docker/);
});

test("换引擎底座时待重启且顶栏应出重启键", () => {
  const view = computeIsolationBadge({
    health: { available: true, isolationLevel: "docker" },
    preferredBackend: "docker",
    preferredEngine: "system-docker",
    engineSourceAtLaunch: "harbor-colima",
    allowsStart: true,
  });
  assert.equal(view.text, "Docker · 本机（待重启）");
  assert.equal(view.pendingRestart, true);
  assert.equal(view.pendingEngine, true);
  assert.equal(view.pendingIsolation, false);
  assert.match(view.title, /须重启后切换/);
});

test("仅改隔离后端 Seatbelt→Docker 待重启", () => {
  const view = computeIsolationBadge({
    health: { available: true, isolationLevel: "seatbelt" },
    preferredBackend: "docker",
    preferredEngine: "harbor-colima",
    engineSourceAtLaunch: "harbor-colima",
    allowsStart: true,
  });
  assert.equal(view.text, "Docker · 托管（待重启）");
  assert.equal(view.pendingIsolation, true);
  assert.equal(view.pendingEngine, false);
});

test("Seatbelt 下改引擎不触发待重启", () => {
  const view = computeIsolationBadge({
    health: { available: true, isolationLevel: "seatbelt" },
    preferredBackend: "seatbelt",
    preferredEngine: "system-docker",
    engineSourceAtLaunch: "harbor-colima",
    allowsStart: true,
  });
  assert.equal(view.text, "Seatbelt");
  assert.equal(view.pendingRestart, false);
});
