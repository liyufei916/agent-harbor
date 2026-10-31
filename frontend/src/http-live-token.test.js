import test from "node:test";
import assert from "node:assert/strict";
import { ingestHttpLiveTokenFromSearch } from "./http-live-api.js";

test("http-live：URL token 迁入存储后从查询串剥离", () => {
  let saved = "";
  const got = ingestHttpLiveTokenFromSearch(
    "?transport=http-live&token=secret-tok&bridge=http://127.0.0.1:18765",
    "",
    (tok) => {
      saved = tok;
    },
  );
  assert.equal(got.token, "secret-tok");
  assert.equal(got.stripped, true);
  assert.equal(saved, "secret-tok");
  assert.doesNotMatch(got.search, /token=/);
  assert.match(got.search, /transport=http-live/);
  assert.match(got.search, /bridge=/);
});

test("http-live：已有存储 token 时不读 URL", () => {
  const got = ingestHttpLiveTokenFromSearch(
    "?transport=http-live&token=other",
    "kept",
    () => {
      throw new Error("should not set");
    },
  );
  assert.equal(got.token, "kept");
  assert.equal(got.stripped, false);
});
