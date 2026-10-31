import test from "node:test";
import assert from "node:assert/strict";
import {
  DOCKER_CHECK_DEFS,
  ISOLATION_CHECK_DEFS,
  checksFromHealth,
  runIsolationProbe,
} from "./isolation-checks.js";

test("checksFromHealth 成功时七项全 ok", () => {
  const items = checksFromHealth({ available: true, isolationLevel: "seatbelt" });
  assert.equal(items.length, ISOLATION_CHECK_DEFS.length);
  assert.ok(items.every((c) => c.status === "ok"));
  assert.match(items.at(-1).detail, /seatbelt/);
});

test("checksFromHealth docker 成功时用 Docker 文案", () => {
  const items = checksFromHealth({ available: true, isolationLevel: "docker" });
  assert.equal(items.length, DOCKER_CHECK_DEFS.length);
  assert.equal(items[0].name, "Docker CLI");
  assert.match(items.at(-1).detail, /docker/);
});

test("checksFromHealth 失败时首项 bad、探针跳过、闸口 bad", () => {
  const items = checksFromHealth({
    available: false,
    reason: "隔离运行失败。",
  });
  assert.equal(items[0].status, "bad");
  assert.equal(items[0].detail, "隔离运行失败。");
  assert.equal(items[1].status, "skipped");
  assert.equal(items.at(-1).status, "bad");
});

test("checksFromHealth 失败且 preferred docker 时用 Docker 项名", () => {
  const items = checksFromHealth(
    { available: false, reason: "Docker daemon 不可用，已拒绝启动。" },
    "docker",
  );
  assert.equal(items[0].name, "Docker CLI");
  assert.equal(items[0].status, "skipped");
  assert.equal(items[1].name, "Docker daemon");
  assert.equal(items[1].status, "bad");
  assert.match(items[1].detail, /daemon/);
});

test("runIsolationProbe 成功时逐项结束为 ok", async () => {
  const updates = [];
  const health = await runIsolationProbe({
    stepMs: 5,
    getHealth: async () => ({ available: true, isolationLevel: "seatbelt" }),
    onUpdate: (items) => updates.push(items.map((i) => i.status)),
  });
  assert.equal(health.available, true);
  assert.ok(updates.length > 3);
  const last = updates.at(-1);
  assert.ok(last.every((s) => s === "ok"));
});

test("runIsolationProbe 失败时末态含 bad", async () => {
  const health = await runIsolationProbe({
    stepMs: 5,
    getHealth: async () => ({ available: false, reason: "nope" }),
  });
  assert.equal(health.available, false);
  const items = checksFromHealth(health);
  assert.equal(items[0].detail, "nope");
});
