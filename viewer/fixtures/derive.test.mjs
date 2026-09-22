// node --test viewer/fixtures/derive.test.mjs — the model derive.mjs keeps of
// the viewer, held to the frozen sandbox rows without a browser. `make
// fixtures-model` runs it. The runner makes the same comparison on every run;
// this catches a wrong model before anybody needs a Chromium to find out.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { DERIVED, canon, expected, frozenPart, pageData } from "./derive.mjs";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, "..", "..");
const expectations = JSON.parse(readFileSync(path.join(HERE, "expectations.json"), "utf8"));
const rows = Object.fromEntries(expectations.checks.map((c) => [c.id, c]));
const derivedRows = expectations.checks.filter((c) => c.derive);
// The committed page is current: `make ledger` fails a commit whose page is stale.
const sandbox = { data: pageData(readFileSync(path.join(REPO, "fixtures", "sandbox", "ledger", "ledger.html"), "utf8")), clock: expectations.meta.clock, prefix: "SBX" };
const own = { data: pageData(readFileSync(path.join(REPO, "ledger", "ledger.html"), "utf8")), clock: expectations.meta.clock, prefix: "LGR" };

test("every derived row names a probe with a model and a frozen sandbox twin", () => {
  assert.ok(derivedRows.length > 0, "no row in expectations.json is derived");
  for (const row of derivedRows) {
    assert.ok(DERIVED[row.derive.probe], `${row.id}: no model for probe ${row.derive.probe}`);
    const twin = rows[row.derive.twin];
    assert.ok(twin && twin.status === "keep" && !twin.derive, `${row.id}: twin ${row.derive.twin} is not a frozen keep row`);
  }
});

test("the model reproduces every frozen sandbox twin", () => {
  for (const row of derivedRows) {
    const twin = rows[row.derive.twin];
    const { probe } = row.derive;
    assert.deepEqual(canon(expected(probe, sandbox, frozenPart(probe, twin.value))), canon(twin.value), `${row.id}'s model disagrees with ${twin.id}`);
  }
});

test("a derived row freezes only what its probe does not derive", () => {
  for (const row of derivedRows) {
    const fields = DERIVED[row.derive.probe];
    if (fields === "*") assert.equal(row.value, undefined, `${row.id} derives its whole value, yet freezes one`);
    else for (const k of fields) assert.ok(!(k in (row.value ?? {})), `${row.id} freezes ${k}, which its probe derives`);
  }
});

// The point of deriving: the ledger moving is not the viewer moving. Filing a
// record and starting work on another both change what the own rows expect.
test("a filed record and a record in progress move the expectation with them", () => {
  const board = rows[derivedRows.find((r) => r.derive.probe === "structure/board").id].value;
  const before = expected("structure/board", own, board);
  const filed = { id: "LGR-999", title: "A record filed after the rows were frozen", type: "defect", severity: "high", status: "in-progress", foundBy: "test", opened: "2026-09-30", resolved: null, stint: "test", evidence: { commits: [], integrated: [], verified: null }, resolution: null, details: "", notes: [], links: [] };
  const grown = { ...own, data: { ...own.data, records: [...own.data.records, filed], derived: { ...own.data.derived, lastActivity: { ...own.data.derived.lastActivity, "LGR-999": "2026-09-30" } } } };
  const after = expected("structure/board", grown, board);
  assert.equal(after.records, before.records + 1);
  assert.ok(after.ids.includes("LGR-999") && after.severityLabels.includes("high") && after.statusLabels.includes("in progress"));
  assert.deepEqual(after.toolbar, before.toolbar, "the frozen toolbar does not move with the corpus");
  const open = expected("open", grown, frozenPart("open", { detailOpens: true, idMatches: true, escapeCloses: true, hashOpensDetail: true }));
  assert.equal(open.clicked, "LGR-999", "the In progress group comes first in the list, so its card is the first one clicked");
  assert.equal(expected("status", grown).afterClick2.label, "status · in prog");
});
