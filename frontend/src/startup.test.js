import assert from "node:assert/strict";
import test from "node:test";
import { initializeTransport } from "./startup.js";

function fakeApi() {
  const calls = [];
  return {
    calls,
    async useLive() {
      calls.push("live");
    },
    async useHttpLive() {
      calls.push("http-live");
    },
    useMock() {
      calls.push("mock");
    },
    setMockScenario(name) {
      calls.push(`scenario:${name}`);
    },
  };
}

test("initializeTransport：默认 mock", async () => {
  const api = fakeApi();
  await initializeTransport(api, { search: "" });
  assert.deepEqual(api.calls, ["mock"]);
});

test("initializeTransport：transport=http-live", async () => {
  const api = fakeApi();
  await initializeTransport(api, {
    search: "?transport=http-live&token=x&bridge=http://127.0.0.1:18765",
  });
  assert.deepEqual(api.calls, ["http-live"]);
});

test("initializeTransport：shell 优先 live", async () => {
  const api = fakeApi();
  await initializeTransport(api, { shell: true, search: "?transport=http-live" });
  assert.deepEqual(api.calls, ["live"]);
});
