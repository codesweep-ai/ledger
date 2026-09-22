// viewer/fixtures/derive.mjs — what the viewer must show for a given corpus.
//
// The repo's own ledger gains a record every time somebody files one, so a row
// that froze its counts measured the corpus rather than the viewer, and went red
// whenever the ledger moved. For the `own` rows, run.mjs asks this module what
// the page should show for the records it was rendered from. The parts of a row
// that do not depend on the records, such as the toolbar labels and the
// behavioural booleans, stay frozen in expectations.json.
//
// This is a model of the filters, lanes, groups and sorts in
// viewer/app/src/App.tsx. It is held to the frozen sandbox corpus: each derived
// row names a sandbox twin, and the runner fails the row when this model,
// applied to the sandbox records, disagrees with the twin's frozen value. A
// viewer change that moves a sandbox row moves this model in the same review.
//
// Pure: no browser, no filesystem. derive.test.mjs runs it with `node --test`.

// The fields each probe computes from the corpus. "*" is the whole value.
// Every other field of the row is frozen in expectations.json.
export const DERIVED = {
  "structure/brief": ["records", "ids", "severityLabels", "statusLabels", "darkDiffers"],
  "structure/board": ["records", "ids", "severityLabels", "statusLabels", "lanes", "darkDiffers"],
  "structure/list": ["records", "ids", "severityLabels", "statusLabels", "groups", "darkDiffers"],
  "structure/activity": ["records", "ids", "severityLabels", "statusLabels", "darkDiffers"],
  "search:wait": "*",
  "search:SAC-06": "*",
  "search:own": "*",
  status: "*",
  severity: "*",
  sort: "*",
  reset: ["beforeReset", "afterReset"],
  open: ["clicked", "detailId", "detailTitle"],
};

export function pageData(html) {
  return JSON.parse(html.match(/<script id=["']ledger-data["'][^>]*>([\s\S]*?)<\/script>/)[1].replace(/\\u003c/g, "<"));
}

// ---- the viewer's rules, as App.tsx states them ------------------------------
const TYPES = ["defect", "improvement", "feature-idea"];
const SEVERITY_SETS = [["critical", "high", "med", "low"], ["critical"], ["critical", "high"], ["critical", "high", "med"]];
const SEV_ORDER = { critical: 0, high: 1, med: 2, low: 3 };
const RETIRED = ["wont-fix", "moved-to-roadmap"];
const STATUS_CYCLE = ["all", "open", "in-progress", "closed", "ideas", "retired", "drafts"];
const STATUS_FACES = { "in-progress": "in prog" };
const LANES = [
  ["open", "Open", (r) => (r.status === "open" || r.status === "in-progress") && !r.idea && !r.draft],
  ["drafts", "Drafts", (r) => r.draft],
  ["ideas", "Ideas", (r) => r.idea],
  ["done", "Closed / retired", (r) => r.terminal],
];
const GROUPS = [
  ["in-progress", "In progress", (r) => r.status === "in-progress"],
  ["open", "Open", (r) => r.status === "open" && !r.idea && !r.draft],
  ["drafts", "Drafts — unfiled (member branches)", (r) => r.draft],
  ["ideas", "Ideas / roadmap", (r) => r.idea],
  ["closed", "Closed", (r) => r.status === "closed"],
  ["retired", "Retired (wont-fix · moved-to-roadmap)", (r) => RETIRED.includes(r.status)],
];
// Collapsed on first load: the board's done lane, the list's closed and retired groups.
const COLLAPSED_LANES = new Set(["done"]);
const COLLAPSED_GROUPS = new Set(["closed", "retired"]);

const norm = (s) => (s || "").replace(/[▸▾▹▿]/g, "").replace(/\s+/g, " ").trim();
const cmp = (a, b) => (a ?? "").localeCompare(b ?? "", "en-US");
const idText = (r) => r.id ?? "draft";
const set = (xs) => [...new Set(xs.filter(Boolean))].sort();
const ids = (cards) => set(cards.map(idText).filter((t) => t !== "draft"));
const severities = (cards) => set(cards.filter((r) => !r.idea).map((r) => (r.severity === "critical" ? "crit" : r.severity)));
const inProgressMarks = (cards) => (cards.some((r) => r.status === "in-progress") ? ["in progress"] : []);

function enrich(data, clock, previousVisit) {
  const now = new Date(clock).getTime();
  const terminal = new Set(data.derived.terminalStatuses);
  const one = (r, draft) => {
    const key = r.id ?? `draft:${r.member}/${r.slug}`;
    const last = data.derived.lastActivity[key] ?? "";
    // The page runs in UTC, so its local midnight is this one.
    const age = last ? Math.floor((now - new Date(`${last}T00:00:00Z`).getTime()) / 86_400_000) : 0;
    const isTerminal = terminal.has(r.status);
    return {
      ...r, key, draft, last, terminal: isTerminal,
      idea: !draft && r.type === "feature-idea" && r.status === "open",
      stale: data.staleAfterDays > 0 && !isTerminal && age > data.staleAfterDays,
      isNew: Boolean(previousVisit && last && last > previousVisit),
    };
  };
  return [...data.records.map((r) => one(r, false)), ...data.drafts.map((r) => one(r, true))];
}

function statusMatches(r, status) {
  if (status === "all") return true;
  if (status === "open") return r.status === "open" && !r.idea && !r.draft;
  if (status === "in-progress" || status === "closed") return r.status === status;
  if (status === "ideas") return r.idea;
  if (status === "drafts") return r.draft;
  return RETIRED.includes(r.status);
}
function passes(r, { status = "all", sev = 0, query = "" } = {}) {
  if (!statusMatches(r, status) || !TYPES.includes(r.type) || !SEVERITY_SETS[sev].includes(r.severity)) return false;
  const text = [r.key, r.title, r.details, r.foundBy, ...(r.notes ?? []).map((n) => n.text)].join(" ").toLowerCase();
  return !query || text.includes(query.toLowerCase());
}
const byResolvedDesc = (a, b) => cmp(b.resolved, a.resolved) || cmp(b.key, a.key);
const compareBy = (sortBy) => (a, b) => {
  if (sortBy === "sev" && SEV_ORDER[a.severity] !== SEV_ORDER[b.severity]) return SEV_ORDER[a.severity] - SEV_ORDER[b.severity];
  if (sortBy === "date" && a.last !== b.last) return cmp(b.last, a.last);
  return cmp(a.key, b.key);
};

function lanes(all) {
  return LANES.map(([key, label, match]) => ({ key, label, items: all.filter((r) => passes(r) && match(r)).sort(key === "done" ? byResolvedDesc : compareBy("id")) }))
    .filter((lane) => lane.items.length);
}
function groups(all, filter = {}, sortBy = "id") {
  const items = all.filter((r) => passes(r, filter));
  return GROUPS.map(([key, label, match]) => {
    const found = items.filter(match);
    return { key, label, items: found.sort(sortBy === "id" && (key === "closed" || key === "retired") ? byResolvedDesc : compareBy(sortBy)) };
  }).filter((group) => group.items.length);
}

// One theme pass of the structure probe: what #groups shows on first load.
function view(data, all, name) {
  if (name === "brief") {
    const queued = (data.queue?.items ?? []).flatMap((item) => all.filter((r) => !r.draft && r.id === item.id && !r.terminal).slice(0, 1));
    const cards = queued.slice(0, 3);
    const needs = [...data.derived.draftsAwaiting, ...all.filter((r) => !r.draft && r.status === "in-progress" && r.stale), ...data.derived.unqueuedCriticals].slice(0, 5);
    return { records: cards.length + needs.length, ids: ids(cards), severityLabels: severities(cards), statusLabels: inProgressMarks(cards) };
  }
  if (name === "board") {
    const shown = lanes(all);
    const cards = shown.filter((lane) => !COLLAPSED_LANES.has(lane.key)).flatMap((lane) => lane.items);
    return { records: cards.length, ids: ids(cards), severityLabels: severities(cards), statusLabels: set([...shown.map((lane) => lane.label), ...inProgressMarks(cards)]), lanes: shown.map((lane) => ({ label: lane.label, count: lane.items.length })) };
  }
  if (name === "list") {
    const shown = groups(all);
    const cards = shown.filter((group) => !COLLAPSED_GROUPS.has(group.key)).flatMap((group) => group.items);
    return { records: cards.length, ids: ids(cards), severityLabels: severities(cards), statusLabels: set([...shown.map((group) => group.label), ...inProgressMarks(cards)]), groups: shown.map((group) => ({ label: group.label, count: group.items.length })) };
  }
  const byKey = Object.fromEntries(all.map((r) => [r.key, r]));
  const allowed = new Set(all.filter((r) => passes(r)).map((r) => r.key));
  const events = data.derived.events.filter((e) => allowed.has(e.k)).slice(0, 100);
  const kind = (k) => (k === "moved-to-roadmap" ? "roadmap" : k === "wont-fix" ? "wontfix" : k);
  if (!events.length) return { records: 0, ids: [], severityLabels: [], statusLabels: [] };
  return { records: events.length, ids: ids(events.map((e) => byKey[e.k])), severityLabels: [], statusLabels: set(["Activity — newest first", ...events.map((e) => kind(e.kind))]) };
}

// The structure probe opens the view in light, then in dark in the same
// context. The light load stores the visit, so the dark load marks every record
// with activity after the frozen clock's date as new, and the toolbar grows a
// "new since last visit" chip. The brief has no toolbar.
function structure(corpus, frozen, name) {
  const { data, clock } = corpus;
  const light = { ...view(data, enrich(data, clock, null), name) };
  const fresh = enrich(data, clock, new Date(clock).toISOString().slice(0, 10)).filter((r) => r.isNew).length;
  const chrome = { viewSwitch: frozen.viewSwitch, toolbar: frozen.toolbar, search: frozen.search };
  const lightPart = { ...light, ...chrome };
  const darkPart = { ...lightPart, toolbar: name !== "brief" && fresh ? [...frozen.toolbar, `new since last visit (${fresh})`] : frozen.toolbar };
  const differs = JSON.stringify(canon(lightPart)) !== JSON.stringify(canon(darkPart));
  return { themes: frozen.themes, ...lightPart, ...(differs ? { darkDiffers: darkPart } : {}) };
}

// The interactions probe: list view, light, every group expanded, then search,
// the status and severity cyclers, the sort cycler, reset, and the first card.
function interactions(corpus, frozen, probe) {
  const { data, clock, prefix } = corpus;
  const all = enrich(data, clock, null);
  const cards = (filter, sortBy) => groups(all, filter, sortBy).flatMap((group) => group.items);
  const count = (filter) => cards(filter).length;
  const statuses = STATUS_CYCLE.filter((s) => s === "all" || all.some((r) => (s === "ideas" ? r.idea : s === "drafts" ? r.draft : s === "retired" ? RETIRED.includes(r.status) : r.status === s)));
  const status = (click) => statuses[click % statuses.length];
  const cycled = (click) => ({ label: `status · ${STATUS_FACES[status(click)] ?? status(click)}`, count: count({ status: status(click) }) });
  const first5 = (sortBy) => cards({}, sortBy).map(idText).filter((t) => t !== "draft").slice(0, 5);
  const total = count({});
  switch (probe) {
    case "search:wait": return ids(cards({ query: "wait" }));
    case "search:SAC-06": return ids(cards({ query: "SAC-06" }));
    case "search:own": return { query: `${prefix}-00`, ids: ids(cards({ query: `${prefix}-00` })) };
    case "status": return { all: total, afterClick1: cycled(1), afterClick2: cycled(2) };
    case "severity": return { all: total, afterClick1: { label: "sev · crit", count: count({ sev: 1 }) }, afterClick2: { label: "sev · high+", count: count({ sev: 2 }) } };
    case "sort": return { id: first5("id"), afterClick1: { label: "sort · activity", first5: first5("date") }, afterClick2: { label: "sort · severity", first5: first5("sev") }, afterClick3: { label: "sort · id", first5: first5("id") } };
    case "reset": return { ...frozen, beforeReset: count({ query: "wait", status: status(1) }), afterReset: total };
    case "open": { const first = cards({}, "id")[0]; return { ...frozen, clicked: idText(first), detailId: idText(first), detailTitle: norm(first.title) }; }
    default: throw new Error(`derive: no model for probe ${probe}`);
  }
}

// ---- the runner's entry points ------------------------------------------------
// canon matches run.mjs: key order never makes two values differ.
export const canon = (v) => (v && typeof v === "object" && !Array.isArray(v) ? Object.fromEntries(Object.keys(v).sort().map((k) => [k, canon(v[k])])) : Array.isArray(v) ? v.map(canon) : v);

// The part of a measured value that stays frozen: everything the probe does not derive.
export function frozenPart(probe, value) {
  const fields = DERIVED[probe];
  if (fields === "*" || value === undefined) return undefined;
  return Object.fromEntries(Object.entries(value).filter(([k]) => !fields.includes(k)));
}

// The value a derived row must measure: its frozen fields, completed from the corpus.
// corpus = { data: pageData(html), clock: ISO string, prefix: the id prefix, e.g. "LGR" }.
export function expected(probe, corpus, frozen = {}) {
  if (probe.startsWith("structure/")) return structure(corpus, frozen, probe.slice("structure/".length));
  return interactions(corpus, frozen, probe);
}
