// viewer/fixtures/scope.mjs — which rows an approval covers, and when a record
// run writes at all.
//
// An approval is per row. --only names the rows a reviewer signed off, and
// --record writes a change to no other row: neither a gated change to a `keep`
// value or a `must-change` target, nor a moved `must-change` baseline. Without
// --only the approval names no row at all. --record-all is the explicit act
// that widens it to every row, for a deliberate full re-record.

// The rows an approval does not cover, sorted and each named once.
export function outsideApproval(changedIds, only, recordAll) {
  if (recordAll) return [];
  const named = new Set(only);
  return [...new Set(changedIds)].filter((id) => !named.has(id)).sort();
}

// What a record run must refuse: every row it would change that the approval
// does not name. A `gated` row needs --approve/--reason as well. A `moved` row
// is a `must-change` baseline, which needs naming but no approval block.
// Returns null when the run may write.
export function refusal({ gated, moved, only, recordAll, approval }) {
  const outside = outsideApproval([...gated, ...moved], only, recordAll);
  if (!outside.length) return null;
  const args = onlyArgs([...only, ...outside]);
  const needsApproval = approval || outside.some((id) => gated.includes(id));
  return { outside, line: needsApproval ? approveLine(args, approval) : recordLine(args) };
}

// The FIXTURES_ARGS a re-run needs to approve exactly these rows.
export const onlyArgs = (ids) => `--only ${[...new Set(ids)].sort().join(",")}`;

// The command that records with these FIXTURES_ARGS, under the approval's own
// id and reason when it has them.
export function approveLine(args, approval) {
  const id = approval?.id ?? "<dispatch-id>";
  const reason = approval ? `'${approval.reason.replaceAll("'", "'\\''")}'` : "\"<why this change is authorised>\"";
  return `make record-fixtures APPROVE=${id} REASON=${reason} FIXTURES_ARGS="${args}"`;
}

// The command that records rows which need naming but no approval.
export const recordLine = (args) => `make record-fixtures FIXTURES_ARGS="${args}"`;

// The text expectations.json should hold after a record run, or null when the
// run changes nothing. The date is stamped only on a write that changes
// something, so a run that records nothing leaves the file byte-identical.
export function expectationsText(currentText, next, today) {
  const text = (recordedAt) => `${JSON.stringify({ ...next, meta: { ...next.meta, recordedAt } }, null, 2)}\n`;
  let recordedAt;
  try { recordedAt = JSON.parse(currentText).meta?.recordedAt; } catch { return text(today); }
  return text(recordedAt) === currentText ? null : text(today);
}
