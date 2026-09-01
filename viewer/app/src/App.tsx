import { useEffect, useMemo, useRef, useState } from "react";
import {
  AppShell,
  Button,
  Card,
  CardGroup,
  Chip,
  Footer,
  Header,
  Modal,
  SearchInput,
  SegmentedControl,
  StatusBadge,
  ThemeToggle,
} from "@codesweep-ai/ui";
import { MarkdownViewer } from "@codesweep-ai/ui/markdown";
import { CodeBlock } from "@codesweep-ai/ui/code";
import bash from "highlight.js/lib/languages/bash";

type View = "brief" | "board" | "list" | "activity";
type GroupBy = "status" | "stint" | "type";
type SortBy = "id" | "date" | "sev";

interface Note { date: string; text: string }
interface Evidence { commits?: string[]; integrated?: string[]; verified?: string }
interface RecordItem {
  id?: string;
  member?: string;
  slug?: string;
  title: string;
  type: "defect" | "improvement" | "feature-idea";
  severity: "critical" | "high" | "med" | "low";
  status: string;
  opened?: string;
  resolved?: string;
  details?: string;
  resolution?: string;
  foundBy?: string;
  stint?: string;
  notes?: Note[];
  evidence?: Evidence;
  links?: string[];
  _draft?: boolean;
  _idea?: boolean;
  _terminal?: boolean;
  _last?: string;
  _age?: number;
  _stale?: boolean;
  _new?: boolean;
}
interface QueueItem { id: string; why?: string }
interface EventItem { d: string; kind: string; k: string; x?: string }
interface LedgerData {
  project: string;
  staleAfterDays: number;
  records: RecordItem[];
  drafts: RecordItem[];
  queue: { updated?: string; items: QueueItem[] } | null;
  description: string | null;
  links: { label: string; url: string }[];
  commitUrlTemplate: string | null;
  rendererVersion: string;
  uiVersion: string;
  lastActivity: string;
  derived: {
    terminalStatuses: string[];
    lastActivity: Record<string, string>;
    events: EventItem[];
    daily: Record<string, { f: number; x: number }>;
    firstEventDate: string;
    queuePredates: number;
    draftsAwaiting: { k: string; title: string }[];
    unqueuedCriticals: string[];
  };
}

function readLedgerData(): LedgerData {
  const element = document.getElementById("ledger-data")!;
  const data = JSON.parse(element.textContent || "{}") as LedgerData;
  return data;
}
const DATA = readLedgerData();
const MS_DAY = 86_400_000;
const TYPES = ["defect", "improvement", "feature-idea"] as const;
const SEVERITIES = ["critical", "high", "med", "low"] as const;
const SEV_ORDER: Record<string, number> = { critical: 0, high: 1, med: 2, low: 3 };

const keyOf = (record: RecordItem) => record.id ?? `draft:${record.member}/${record.slug}`;
const todayLocal = () => {
  const now = new Date();
  return new Date(now.getTime() - now.getTimezoneOffset() * 60_000).toISOString().slice(0, 10);
};

function enrich(record: RecordItem, draft: boolean, previousVisit: string | null): RecordItem {
  const last = DATA.derived.lastActivity[keyOf(record)] ?? "";
  const age = last ? Math.floor((Date.now() - new Date(`${last}T00:00:00`).getTime()) / MS_DAY) : 0;
  const terminal = DATA.derived.terminalStatuses.includes(record.status);
  return {
    ...record,
    _draft: draft,
    _idea: !draft && record.type === "feature-idea" && record.status === "open",
    _terminal: terminal,
    _last: last,
    _age: age,
    _stale: DATA.staleAfterDays > 0 && !terminal && age > DATA.staleAfterDays,
    _new: Boolean(previousVisit && last && last > previousVisit),
  };
}

function Severity({ value }: { value: string }) {
  const appearance = {
    critical: { status: "error", emphasis: "label" },
    high: { status: "severe", emphasis: "label" },
    med: { status: "warning" },
    low: { status: "neutral" },
  }[value] ?? { status: "neutral" };
  return <StatusBadge
    className="sev"
    label={value === "critical" ? "crit" : value}
    status={appearance.status as "error" | "severe" | "warning" | "neutral"}
    emphasis={appearance.emphasis as "label" | undefined}
    size="sm"
  />;
}
function TypeTag({ value }: { value: string }) {
  return <StatusBadge className="tag" label={value === "feature-idea" ? "idea" : value} status="neutral" size="sm" />;
}

// Highlighting arrives through `codeRenderers`, which the lightweight entry
// supports — it is wired into the shared component map inside
// `createMarkdownViewer`, so it works on both entries. The rich entry buys the
// full CommonMark/GFM parser and its plugin seam; this viewer needs neither.
// See ui's Patterns -> Markdown Viewer ladder.
//
// The records carry one language-tagged fence, `bash`. CodeBlock builds on
// highlight.js/lib/core with nothing pre-registered, so only that grammar ships.
//
// Constraint accepted with this: footnotes and bare autolinks render as literal
// text. Tables, task lists, strikethrough, blockquotes and alerts are unaffected.
const codeRenderers = {
  bash: ({ code }: { code: string }) => (
    <CodeBlock code={code} language="bash" languages={{ bash }} inline />
  ),
};

function Markdown({ content, onRecord }: { content?: string; onRecord?: (id: string) => void }) {
  if (!content) return null;
  return <MarkdownViewer
    content={content}
    inline
    className="prose"
    codeRenderers={codeRenderers}
    onLinkClick={(href) => {
      if (href.startsWith("#") && onRecord) onRecord(href.slice(1));
      else if (/^https?:\/\//.test(href)) window.open(href, "_blank", "noopener");
    }}
  />;
}

function RecordCard({ record, rank, why, open }: { record: RecordItem; rank?: number; why?: string; open: (id: string) => void }) {
  const meta: string[] = [];
  if (record._draft && record.member) meta.push(record.member);
  if (["wont-fix", "moved-to-roadmap"].includes(record.status)) meta.push(record.status);
  if (record.stint) meta.push(record.stint);
  if (record._stale) meta.push(`stale ${record._age}d`);
  if (record._terminal) meta.push(`✓ ${record.resolved ?? ""}`);
  return <Card
    as="article"
    variant="tight"
    interactive
    className={`bcard${record._idea ? " idea" : ""}`}
    data-record={keyOf(record)}
    onActivate={() => open(keyOf(record))}
  >
    <div className="bhead">
      {rank ? <span className="rank">{rank}</span> : null}
      <span className="idm">{record.id ?? "draft"}</span>
      {!record._idea && <Severity value={record.severity} />}
      <TypeTag value={record.type} />
      {record.status === "in-progress" && <StatusBadge className="ip" label="in progress" status="info" size="sm" />}
      {record._new && <span className="newb">new</span>}
    </div>
    <div className="bttl">{record.title}</div>
    {why && <div className="bwhy">{why}</div>}
    {meta.length > 0 && <div className="bmeta">{meta.join(" · ")}</div>}
  </Card>;
}

function Detail({ record, open }: { record: RecordItem; open: (id: string) => void }) {
  const evidence = record.evidence ?? {};
  const commits = (items: string[]) => items.map((sha, index) => DATA.commitUrlTemplate
    ? <span key={sha}>{index > 0 && ", "}<a className="mono shalink" href={DATA.commitUrlTemplate.replace("{sha}", sha)}>{sha}</a></span>
    : <span key={sha}>{index > 0 && ", "}<span className="mono">{sha}</span></span>);
  return <div className="detail">
    {record.details && <><div className="dlbl">details</div><Markdown content={record.details} onRecord={open} /></>}
    {record.notes?.length ? <><div className="dlbl">notes</div>{record.notes.map((note, i) =>
      <div className="note" key={`${note.date}-${i}`}><div className="d">{note.date}</div><Markdown content={note.text} onRecord={open} /></div>)}</> : null}
    {record.resolution && <><div className="dlbl">resolution</div><Markdown content={record.resolution} onRecord={open} /></>}
    {(evidence.commits?.length || evidence.integrated?.length || evidence.verified) && <>
      <div className="dlbl">evidence</div><div className="evd">
        {evidence.commits?.length ? <div><b>commits</b> {commits(evidence.commits)}</div> : null}
        {evidence.integrated?.length ? <div><b>integrated</b> {commits(evidence.integrated)}</div> : null}
        {evidence.verified && <div><b>verified</b> — <Markdown content={evidence.verified} onRecord={open} /></div>}
      </div>
    </>}
    {record.links?.length ? <><div className="dlbl">links</div><div className="linksrow">{record.links.map((link) =>
      <Button variant="ghost" size="sm" key={link} onClick={() => open(link)}>{link}</Button>)}</div></> : null}
    <div className="dmeta">opened {record.opened} · status {record.status}{record.resolved ? ` · resolved ${record.resolved}` : ""} · last activity {record._last}</div>
  </div>;
}

function Help() {
  const entries: [string, string][] = [
    ["brief", "The gist: trend, the queue's top picks, and what needs a human."],
    ["board", "Current state by status; queued records carry their rank badge."],
    ["list", "Every record, groupable by status, stint, or type and sortable."],
    ["activity", "Every dated fact, newest first."],
    ["record", "One tracked defect, improvement, or idea, with evidence when closed."],
    ["up next", "The queue's ordered recommendation of what to fix next."],
    ["needs you", "Drafts awaiting an id, stale in-progress work, and unqueued criticals."],
    ["stale", "No activity for the repository's configured window."],
    ["new", "Activity since this browser's last visit."],
  ];
  return <div className="introbody">
    <p className="introdesc">A ledger is a repository issue tracker kept as files. This page is a view of those files.</p>
    {entries.map(([term, definition]) => <div className="gent" key={term}><div className="gterm">{term}</div><div className="gdef">{definition}</div></div>)}
    <p className="introfoot">All views are generated from ledger.json, issues, drafts, and queue.json. Click a card for its full narrative, notes, and closure evidence.</p>
  </div>;
}

function Sparkline() {
  const days = Object.keys(DATA.derived.daily).sort();
  const maximum = Math.max(1, ...days.map((day) => DATA.derived.daily[day].f + DATA.derived.daily[day].x));
  const width = Math.max(8, days.length * 8);
  return <div className="barwrap" id="spark"><svg width={width} height="46" viewBox={`0 0 ${width} 46`} role="img" aria-label="daily found and resolved">
    {days.map((day, index) => {
      const point = DATA.derived.daily[day];
      const found = point.f * 40 / maximum;
      const fixed = point.x * 40 / maximum;
      return <g key={day}><title>{day}: {point.f} found · {point.x} resolved</title>
        {found > 0 && <rect className="barf" x={index * 8 + 1} y={43 - found} width="6" height={found} />}
        {fixed > 0 && <rect className="barx" x={index * 8 + 1} y={43 - found - fixed} width="6" height={fixed} />}
      </g>;
    })}
    <line className="basel" x1="0" x2={width} y1="43.5" y2="43.5" />
  </svg></div>;
}

export function App() {
  const [previousVisit] = useState<string | null>(() => {
    try { return localStorage.getItem(`ledger-last-visit:${DATA.project}`); } catch { return null; }
  });
  const records = useMemo(() => DATA.records.map((item) => enrich(item, false, previousVisit)), [previousVisit]);
  const drafts = useMemo(() => DATA.drafts.map((item) => enrich(item, true, previousVisit)), [previousVisit]);
  const all = useMemo(() => [...records, ...drafts], [records, drafts]);
  const byKey = useMemo(() => Object.fromEntries(all.map((record) => [keyOf(record), record])), [all]);
  const queueRank = useMemo(() => Object.fromEntries((DATA.queue?.items ?? []).map((item, index) => [item.id, { rank: index + 1, why: item.why ?? "" }])), []);
  const initialView = new URLSearchParams(location.search).get("view");
  const [view, setView] = useState<View>(["brief", "board", "list", "activity"].includes(initialView ?? "") ? initialView as View : "brief");
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [typeMode, setTypeMode] = useState(0);
  const [severityMode, setSeverityMode] = useState(0);
  const [staleOnly, setStaleOnly] = useState(false);
  const [newOnly, setNewOnly] = useState(false);
  const [groupBy, setGroupBy] = useState<GroupBy>("status");
  const [sortBy, setSortBy] = useState<SortBy>("id");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({ closed: true, retired: true, "bl-done": true });
  const [modal, setModal] = useState<string | "help" | null>(() => decodeURIComponent(location.hash.slice(1)) || null);
  const searchRoot = useRef<HTMLDivElement>(null);

  const typeSets = [TYPES, ["defect"], ["improvement"], ["feature-idea"]] as readonly (readonly string[])[];
  const severitySets = [SEVERITIES, ["critical"], ["critical", "high"], ["critical", "high", "med"]] as readonly (readonly string[])[];
  const typeFaces = ["all", "defects", "improv", "ideas"];
  const severityFaces = ["all", "crit", "high+", "med+"];

  useEffect(() => {
    try { localStorage.setItem(`ledger-last-visit:${DATA.project}`, todayLocal()); } catch {}
  }, []);
  useEffect(() => { document.body.classList.toggle("briefmode", view === "brief"); }, [view]);
  useEffect(() => {
    const input = searchRoot.current?.querySelector<HTMLInputElement>("[data-search-input]");
    if (input) input.type = "search";
  }, [view]);
  useEffect(() => {
    try { history.replaceState(null, "", modal && modal !== "help" ? `#${modal}` : location.pathname + location.search); } catch {}
  }, [modal]);

  const openRecord = (id: string) => { if (byKey[id]) setModal(id); };
  const passes = (record: RecordItem, ignoreStatus = false) => {
    if (!ignoreStatus && status !== "all") {
      const matches = status === "open" ? record.status === "open" && !record._idea && !record._draft
        : status === "in-progress" ? record.status === "in-progress"
        : status === "closed" ? record.status === "closed"
        : status === "ideas" ? record._idea
        : status === "drafts" ? record._draft
        : ["wont-fix", "moved-to-roadmap"].includes(record.status);
      if (!matches) return false;
    }
    if (!typeSets[typeMode].includes(record.type) || !severitySets[severityMode].includes(record.severity)) return false;
    if (staleOnly && !record._stale) return false;
    if (newOnly && !record._new) return false;
    if (query) {
      const text = [keyOf(record), record.title, record.details, record.foundBy, ...(record.notes ?? []).map((note) => note.text)].join(" ").toLowerCase();
      if (!text.includes(query.toLowerCase())) return false;
    }
    return true;
  };
  const compare = (left: RecordItem, right: RecordItem) => {
    if (sortBy === "sev" && SEV_ORDER[left.severity] !== SEV_ORDER[right.severity]) return SEV_ORDER[left.severity] - SEV_ORDER[right.severity];
    if (sortBy === "date" && left._last !== right._last) return (right._last ?? "").localeCompare(left._last ?? "");
    return keyOf(left).localeCompare(keyOf(right));
  };
  const reset = () => { setQuery(""); setStatus("all"); setTypeMode(0); setSeverityMode(0); setStaleOnly(false); setNewOnly(false); };
  const activeFilters = query || status !== "all" || typeMode || severityMode || staleOnly || newOnly;
  const statuses = ["all", "open", "in-progress", "closed", "ideas", "retired", "drafts"].filter((candidate) => candidate === "all" || all.some((record) =>
    candidate === "ideas" ? record._idea : candidate === "drafts" ? record._draft : candidate === "retired" ? ["wont-fix", "moved-to-roadmap"].includes(record.status) : record.status === candidate));
  const statusFaces: Record<string, string> = { "in-progress": "in prog" };

  const groupList = () => {
    const items = all.filter((record) => passes(record));
    const groups: { key: string; label: string; items: RecordItem[]; ideas?: boolean }[] = [];
    if (groupBy === "status") {
      const definitions: [string, string, (record: RecordItem) => boolean, boolean?][] = [
        ["in-progress", "In progress", (r) => r.status === "in-progress"], ["open", "Open", (r) => r.status === "open" && !r._idea && !r._draft],
        ["drafts", "Drafts — unfiled (member branches)", (r) => Boolean(r._draft), true], ["ideas", "Ideas / roadmap", (r) => Boolean(r._idea), true],
        ["closed", "Closed", (r) => r.status === "closed"], ["retired", "Retired (wont-fix · moved-to-roadmap)", (r) => ["wont-fix", "moved-to-roadmap"].includes(r.status)],
      ];
      definitions.forEach(([key, label, match, ideas]) => { const found = items.filter(match); if (found.length) groups.push({ key, label, items: found, ideas }); });
    } else {
      const keys = groupBy === "type" ? TYPES : [...new Set(items.map((item) => item.stint ?? "unassigned"))].sort();
      keys.forEach((key) => { const found = items.filter((item) => (groupBy === "type" ? item.type : item.stint ?? "unassigned") === key); if (found.length) groups.push({ key: `${groupBy}-${key}`, label: groupBy === "type" ? key : key === "unassigned" ? "No stint" : `Stint ${key}`, items: found }); });
    }
    groups.forEach((group) => {
      if (sortBy === "id" && (group.key === "closed" || group.key === "retired")) {
        group.items.sort((left, right) => (right.resolved ?? "").localeCompare(left.resolved ?? "") || keyOf(right).localeCompare(keyOf(left)));
      } else group.items.sort(compare);
    });
    return groups;
  };

  const renderBrief = () => {
    const queued = (DATA.queue?.items ?? []).flatMap((item) => { const record = records.find((candidate) => candidate.id === item.id && !candidate._terminal); return record ? [{ item, record }] : []; });
    const needs = [...DATA.derived.draftsAwaiting.map((item) => ({ key: item.k, label: `draft awaiting an id: ${item.title}` })),
      ...records.filter((record) => record.status === "in-progress" && record._stale).map((record) => ({ key: keyOf(record), label: `${record.id} — in progress but stale ${record._age}d` })),
      ...DATA.derived.unqueuedCriticals.map((id) => ({ key: id, label: `${id} — critical but not in the queue` }))];
    return <div className="brief">
      <div className="bblock btrend"><div className="bhead2"><span>trend · daily</span><span className="cnt"><span className="lg"><i className="swf" />{records.length} found</span> <span className="lg"><i className="swx" />{records.filter((r) => r._terminal).length} resolved</span></span></div><Sparkline /></div>
      <div className="bblock"><div className="bhead2"><span>up next</span>{DATA.queue?.updated && <span className="cnt">queue · {Math.min(3, queued.length)} of {queued.length} · as of {DATA.queue.updated}{DATA.derived.queuePredates > 0 && <span className="qstale"> · predates {DATA.derived.queuePredates} newer events</span>}</span>}</div>
        {queued.length ? queued.slice(0, 3).map(({ item, record }, index) => <RecordCard key={item.id} record={record} rank={index + 1} why={item.why} open={openRecord} />) : <div className="laneempty">no live queue recommendation — see the board</div>}
      </div>
      <div className="bblock"><div className="bhead2"><span>needs you</span></div>{needs.length ? needs.slice(0, 5).map((item) => <div className="actrow" key={item.key} onClick={() => openRecord(item.key)}><span className="ttl">{item.label}</span></div>) : <div className="laneempty">nothing — the agents have it</div>}</div>
      <div className="bviews"><SegmentedControl aria-label="Choose a ledger view" value={view} onChange={(next) => setView(next as View)} options={(["board", "list", "activity"] as View[]).map((next) => ({ value: next, label: next }))} /></div>
    </div>;
  };
  const renderBoard = () => {
    const items = all.filter((record) => passes(record, true));
    const definitions: [string, string, (record: RecordItem) => boolean, boolean?][] = [
      ["open", "Open", (r) => (r.status === "open" || r.status === "in-progress") && !r._idea && !r._draft],
      ["drafts", "Drafts", (r) => Boolean(r._draft), true], ["ideas", "Ideas", (r) => Boolean(r._idea), true], ["done", "Closed / retired", (r) => Boolean(r._terminal)],
    ];
    const lanes = definitions.map(([key, label, match, ideas]) => ({ key, label, ideas, items: items.filter((record) => match(record) && (status === "all" || passes(record))).sort(key === "done" ? (left, right) => (right.resolved ?? "").localeCompare(left.resolved ?? "") || keyOf(right).localeCompare(keyOf(left)) : compare) })).filter((lane) => lane.items.length);
    if (!lanes.length) return <Empty reset={reset} />;
    return <CardGroup fill={false} className="stack">{lanes.map((lane) => {
      const isCollapsed = Boolean(collapsed[`bl-${lane.key}`]);
      const toggle = () => setCollapsed({ ...collapsed, [`bl-${lane.key}`]: !isCollapsed });
      return <Card
        className={`lane${lane.ideas ? " ideas" : ""}`}
        key={lane.key}
        variant="tight"
        collapsed={isCollapsed}
        onToggle={toggle}
        header={<div
          className="lane-header"
          tabIndex={0}
          role="button"
          aria-expanded={!isCollapsed}
          onClick={toggle}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === " ") { event.preventDefault(); toggle(); }
          }}
        ><span>{isCollapsed ? "▸" : "▾"} {lane.label}</span><span className="cnt">{lane.items.length}</span></div>}
      ><CardGroup fill={false} className="cards">{lane.items.map((record) => <RecordCard key={keyOf(record)} record={record} rank={record.id && queueRank[record.id] && !record._terminal ? queueRank[record.id].rank : undefined} open={openRecord} />)}</CardGroup></Card>;
    })}</CardGroup>;
  };
  const renderList = () => { const groups = groupList(); return groups.length ? <>{groups.map((group) => {
    const toggle = () => setCollapsed({ ...collapsed, [group.key]: !collapsed[group.key] });
    return <section className={`grp${group.ideas ? " ideas" : ""}`} key={group.key}><header tabIndex={0} role="button" aria-expanded={!collapsed[group.key]} onClick={toggle} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); toggle(); } }}><span>{collapsed[group.key] ? "▸" : "▾"} {group.label}</span><span className="cnt">{group.items.length}</span></header>{!collapsed[group.key] && <CardGroup fill={false} className="cards listcards">{group.items.map((record) => <RecordCard key={keyOf(record)} record={record} rank={record.id && queueRank[record.id] && !record._terminal ? queueRank[record.id].rank : undefined} open={openRecord} />)}</CardGroup>}</section>;
  })}</> : <Empty reset={reset} />; };
  const renderActivity = () => {
    const allowed = new Set(all.filter((record) => passes(record)).map(keyOf));
    const events = DATA.derived.events.filter((event) => allowed.has(event.k)).slice(0, 100);
    return events.length ? <section className="grp"><header><span>▾ Activity — newest first</span><span className="cnt">{events.length}</span></header><div className="grpbody">{events.map((event, index) => { const record = byKey[event.k]; const eventLabel = event.kind === "moved-to-roadmap" ? "roadmap" : event.kind === "wont-fix" ? "wontfix" : event.kind; return <div className="actrow" key={`${event.k}-${event.d}-${index}`} onClick={() => openRecord(event.k)}><StatusBadge className="actk" label={eventLabel} status={event.kind === "closed" ? "success" : event.kind === "note" ? "info" : "neutral"} size="sm" /><span className="idm">{record.id ?? "draft"}</span><span className="ttl">{event.d} · {record.title}</span></div>; })}</div></section> : <Empty reset={reset} />;
  };

  const selected = modal && modal !== "help" ? byKey[modal] : null;
  return <AppShell>
    <Header title={`cs-ledger · ${DATA.project}`} titleHref="?view=brief" actions={<ThemeToggle storageKey="ledger-theme" />} />
    <main>
    <div className="page">
      <div className="controls viewbar"><span className="chipset"><span className="cl">view</span><SegmentedControl aria-label="Choose a ledger view" value={view} onChange={(next) => setView(next as View)} options={(["brief", "board", "list", "activity"] as View[]).map((item) => ({ value: item, label: item }))} /></span><Chip className="aux" title="how to read this ledger" onClick={() => setModal("help")}>?</Chip></div>
      {(DATA.description || DATA.links.length > 0) && <div className="masthead">{DATA.description && <div className="bdesc">{DATA.description}</div>}{DATA.links.length > 0 && <div className="mastlinks">{DATA.links.map((link) => <a className="plink" href={link.url} key={link.url}>{link.label}</a>)}</div>}</div>}
      {view !== "brief" && <div className={`controls filterbar${view !== "list" ? " boardmode" : ""}`}>
        <SearchInput ref={searchRoot} className="ledger-search" placeholder="search id, title, details…" value={query} onChange={setQuery} onSearch={setQuery} />
        <Chip id="cycStatus" className={`cyc${status !== "all" ? " hasactive" : ""}`} pressed={status !== "all"} onClick={() => setStatus(statuses[(statuses.indexOf(status) + 1) % statuses.length])}>status · {statusFaces[status] ?? status}</Chip>
        <Chip className={`cyc${typeMode ? " hasactive" : ""}`} pressed={Boolean(typeMode)} onClick={() => setTypeMode((typeMode + 1) % typeSets.length)}>type · {typeFaces[typeMode]}</Chip>
        <Chip className={`cyc${severityMode ? " hasactive" : ""}`} pressed={Boolean(severityMode)} onClick={() => setSeverityMode((severityMode + 1) % severitySets.length)}>sev · {severityFaces[severityMode]}</Chip>
        <Chip className="aux" pressed={staleOnly} onClick={() => setStaleOnly(!staleOnly)}>stale only</Chip>
        <Chip className="aux" disabled={!activeFilters} onClick={reset}>reset</Chip>
        <span className="arrange listonly"><Chip id="cycGroup" className={`cyc2${groupBy !== "status" ? " hasactive" : ""}`} pressed={groupBy !== "status"} onClick={() => setGroupBy(({ status: "stint", stint: "type", type: "status" })[groupBy] as GroupBy)}>group · {groupBy}</Chip><Chip id="cycSort" className={`cyc2${sortBy !== "id" ? " hasactive" : ""}`} pressed={sortBy !== "id"} onClick={() => setSortBy(({ id: "date", date: "sev", sev: "id" })[sortBy] as SortBy)}>sort · {sortBy === "date" ? "activity" : sortBy === "sev" ? "severity" : "id"}</Chip></span>
        {all.some((record) => record._new) && <Chip className="aux" pressed={newOnly} onClick={() => setNewOnly(!newOnly)}>new since last visit ({all.filter((record) => record._new).length})</Chip>}
      </div>}
      <div id="groups">{view === "brief" ? renderBrief() : view === "board" ? renderBoard() : view === "activity" ? renderActivity() : renderList()}</div>
    </div>
    </main>
    <Footer>cs-ledger v{DATA.rendererVersion} · @codesweep-ai/ui v{DATA.uiVersion}</Footer>
    {modal && <Modal title={modal === "help" ? "How to read this ledger" : selected?.title ?? "Record"} className={`ledger-modal${modal === "help" ? " intro" : ""}`} onClose={() => setModal(null)}>{modal === "help" ? <Help /> : selected ? <><div className="mhead"><span className="idm">{selected.id ?? "draft"}</span>{!selected._idea && <Severity value={selected.severity} />}<TypeTag value={selected.type} />{selected.member && <StatusBadge className="tag" label={selected.member} status="neutral" size="sm" />}{selected.stint && <StatusBadge className="wo" label={selected.stint} status="info" size="sm" />}{selected._stale && <StatusBadge className="stale" label={`stale ${selected._age}d`} status="warning" size="sm" />}<span className="mmeta">{selected._terminal ? `✓ ${selected.resolved ?? ""}` : selected.foundBy}</span></div><Detail record={selected} open={openRecord} /></> : null}</Modal>}
  </AppShell>;
}

function Empty({ reset }: { reset: () => void }) {
  return <div className="empty">No records match the current filters.<br /><Button variant="ghost" size="sm" onClick={reset}>reset filters</Button></div>;
}
