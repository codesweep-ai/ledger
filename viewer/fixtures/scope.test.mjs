// node --test viewer/fixtures/scope.test.mjs — the rule that an approval is per
// row, without a browser. `make fixtures-model` runs it. run.mjs asks scope.mjs
// which gated rows an approval leaves out, and refuses the write when any are.

import assert from "node:assert/strict";
import { test } from "node:test";
import { approveLine, onlyArgs, outsideApproval } from "./scope.mjs";

// Both deps-maintenance pilots ran with these seven rows failing for a reason
// nobody had approved: the own ledger had grown from 12 records to 14.
const stale = ["LF-02", "LF-03", "LF-04", "LF-12", "LF-13", "LF-14", "LF-15"];

test("an approval without --only covers no row", () => {
  assert.deepEqual(outsideApproval(["LF-09", ...stale], [], false), ["LF-02", "LF-03", "LF-04", "LF-09", "LF-12", "LF-13", "LF-14", "LF-15"]);
});

test("an approval covers exactly the rows --only names", () => {
  assert.deepEqual(outsideApproval(["LF-09"], ["LF-09"], false), []);
  assert.deepEqual(outsideApproval(["LF-09", ...stale], ["LF-09"], false), stale);
  assert.deepEqual(outsideApproval([], [], false), [], "nothing gated needs no approval");
});

test("--record-all is what applies one approval to every row", () => {
  assert.deepEqual(outsideApproval(["LF-09", ...stale], [], true), []);
});

test("a row with several gated changes is named once, in order", () => {
  assert.deepEqual(outsideApproval(["LF-44", "LF-25", "LF-44"], [], false), ["LF-25", "LF-44"]);
  assert.equal(onlyArgs(["LF-44", "LF-25", "LF-44"]), "--only LF-25,LF-44");
});

test("the refusal prints a line that approves exactly the rows it lists", () => {
  assert.equal(
    approveLine(onlyArgs(["LF-44", "LF-25"]), { id: "LGR-015", reason: "records grew from 12 to 14" }),
    `make record-fixtures APPROVE=LGR-015 REASON='records grew from 12 to 14' FIXTURES_ARGS="--only LF-25,LF-44"`,
  );
  assert.equal(
    approveLine(onlyArgs(["LF-25"]), { id: "x", reason: "the reviewer's call" }),
    `make record-fixtures APPROVE=x REASON='the reviewer'\\''s call' FIXTURES_ARGS="--only LF-25"`,
  );
  assert.equal(
    approveLine("--record-all", null),
    `make record-fixtures APPROVE=<dispatch-id> REASON="<why this change is authorised>" FIXTURES_ARGS="--record-all"`,
  );
});
