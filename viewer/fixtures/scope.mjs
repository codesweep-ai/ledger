// viewer/fixtures/scope.mjs — which rows an approval covers.
//
// An approval is per row. --only names the rows a reviewer signed off, and
// --record writes a gated change to no other row. Without --only the approval
// names no row at all. --record-all is the explicit act that widens it to every
// row, for a deliberate full re-record.

// The gated rows an approval does not cover, sorted and each named once.
export function outsideApproval(gatedIds, only, recordAll) {
  if (recordAll) return [];
  const named = new Set(only);
  return [...new Set(gatedIds)].filter((id) => !named.has(id)).sort();
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
