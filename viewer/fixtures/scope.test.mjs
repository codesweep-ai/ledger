// node --test viewer/fixtures/scope.test.mjs — the rule that an approval is per
// row, without a browser. `make fixtures-model` runs it. run.mjs asks scope.mjs
// which changed rows an approval leaves out, and refuses the write when any are.

import assert from "node:assert/strict";
import { test } from "node:test";
import { approveLine, expectationsText, onlyArgs, outsideApproval, refusal } from "./scope.mjs";

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

// A bare approved run on an unchanged tree moved these five baselines, because
// their axe counts grow with this repository's own ledger.
const baselines = ["LF-27", "LF-28", "LF-34", "LF-37", "LF-42"];

test("a moved must-change baseline needs naming like a gated change", () => {
  const r = refusal({ gated: [], moved: baselines, only: [], recordAll: false, approval: { id: "LGR-017", reason: "axe counts grew" } });
  assert.deepEqual(r.outside, baselines);
  assert.equal(r.line, `make record-fixtures APPROVE=LGR-017 REASON='axe counts grew' FIXTURES_ARGS="--only LF-27,LF-28,LF-34,LF-37,LF-42"`);
  assert.equal(refusal({ gated: [], moved: ["LF-27"], only: ["LF-27"], recordAll: false, approval: null }), null);
  assert.equal(refusal({ gated: [], moved: baselines, only: [], recordAll: true, approval: null }), null);
});

test("a baseline refused without an approval asks for naming, not for one", () => {
  assert.equal(refusal({ gated: [], moved: ["LF-27"], only: [], recordAll: false, approval: null }).line, `make record-fixtures FIXTURES_ARGS="--only LF-27"`);
  assert.equal(
    refusal({ gated: ["LF-09"], moved: ["LF-27"], only: [], recordAll: false, approval: null }).line,
    `make record-fixtures APPROVE=<dispatch-id> REASON="<why this change is authorised>" FIXTURES_ARGS="--only LF-09,LF-27"`,
  );
});

test("a run that changes no row leaves expectations.json byte-identical", () => {
  const file = `${JSON.stringify({ meta: { recordedAt: "2026-09-01", clock: "c" }, checks: [{ id: "LF-01", value: 1 }] }, null, 2)}\n`;
  const same = { meta: { recordedAt: undefined, clock: "c" }, checks: [{ id: "LF-01", value: 1 }] };
  assert.equal(expectationsText(file, same, "2026-09-22"), null);
  const moved = { meta: { recordedAt: undefined, clock: "c" }, checks: [{ id: "LF-01", value: 2 }] };
  assert.equal(JSON.parse(expectationsText(file, moved, "2026-09-22")).meta.recordedAt, "2026-09-22");
  assert.equal(JSON.parse(expectationsText(undefined, same, "2026-09-22")).meta.recordedAt, "2026-09-22");
});
