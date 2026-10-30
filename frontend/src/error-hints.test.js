import test from "node:test";
import assert from "node:assert/strict";
import { formatErrorWithHint, hintForError } from "./error-hints.js";
import { hostsForTier, normalizeTier } from "./security-tier.js";
import { classifySyncPath, ingressFindings } from "./ingress-check.js";

test("hintForError maps known codes", () => {
  assert.match(hintForError("SESSION_LIMIT"), /最多 5/);
  assert.match(formatErrorWithHint({ code: "WORKSPACE_BUSY", message: "busy" }), /同工作区/);
});

test("security tier hosts", () => {
  assert.equal(normalizeTier(""), "standard");
  assert.equal(hostsForTier("strict").length, 0);
  assert.ok(hostsForTier("standard").includes("registry.npmjs.org"));
  assert.ok(hostsForTier("relaxed").length > hostsForTier("standard").length);
});

test("ingress and exit path classify", () => {
  const f = ingressFindings("/tmp/.env");
  assert.ok(f.some((x) => x.level === "warn"));
  assert.equal(classifySyncPath(".git/hooks/pre-commit").highRisk, true);
  assert.equal(classifySyncPath("src/main.go").highRisk, false);
});
