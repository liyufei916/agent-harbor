import test from "node:test";
import assert from "node:assert/strict";
import { projectWorkspaceOf } from "./remembered-hosts.js";

test("projectWorkspaceOf 优先 source / project，不用 clone 副本", () => {
  assert.equal(
    projectWorkspaceOf({
      workspace: "/tmp/clone",
      sourceWorkspace: "/Users/a/proj",
      mountMode: "clone",
    }),
    "/Users/a/proj",
  );
  assert.equal(
    projectWorkspaceOf(
      { sessionId: "s1", workspace: "/tmp/clone" },
      {
        state: {
          sessionId: "s1",
          sessionHistory: [
            { sessionId: "s1", projectWorkspace: "/Users/a/proj" },
          ],
        },
      },
    ),
    "/Users/a/proj",
  );
});
