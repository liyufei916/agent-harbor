import test from "node:test";
import assert from "node:assert/strict";
import {
  findBusyWorkspaceSession,
  normalizeWorkspaceKey,
} from "./workspace-busy.js";

test("normalizeWorkspaceKey 去尾斜杠", () => {
  assert.equal(
    normalizeWorkspaceKey("/Users/a/proj/"),
    "/Users/a/proj",
  );
});

test("findBusyWorkspaceSession：running + projectWorkspace", () => {
  const hit = findBusyWorkspaceSession(
    [
      {
        sessionId: "s1",
        state: "running",
        cleanupComplete: false,
        projectWorkspace: "/Users/a/proj",
        workspace: "/tmp/clone",
        mountMode: "clone",
      },
    ],
    "/Users/a/proj/",
  );
  assert.equal(hit?.sessionId, "s1");
});

test("findBusyWorkspaceSession：不与 clone 副本路径误匹配", () => {
  const hit = findBusyWorkspaceSession(
    [
      {
        sessionId: "s1",
        state: "running",
        cleanupComplete: false,
        projectWorkspace: "/Users/a/proj",
        workspace: "/tmp/clone",
        mountMode: "clone",
      },
    ],
    "/tmp/clone",
  );
  assert.equal(hit, null);
});

test("findBusyWorkspaceSession：已清理终态不计", () => {
  const hit = findBusyWorkspaceSession(
    [
      {
        sessionId: "s1",
        state: "stopped",
        cleanupComplete: true,
        projectWorkspace: "/Users/a/proj",
        workspace: "/Users/a/proj",
        mountMode: "direct",
      },
    ],
    "/Users/a/proj",
  );
  assert.equal(hit, null);
});
