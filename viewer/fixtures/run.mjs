#!/usr/bin/env node
// viewer/fixtures/run.mjs — behavioural oracle for the cs-ledger viewer.
//
// Renders real ledgers with the cs-ledger binary, drives the single-file page in
// headless Chromium and compares what it measures against
// viewer/fixtures/expectations.json. See README.md.
//
// puppeteer-core, not puppeteer: the browser is always one this repository was
// pointed at rather than one npm fetched, so the full package would download a
// Chrome for Testing that nothing here ever launches -- on every install, in
// every CI job.
//
//   node viewer/fixtures/run.mjs [--strict] [--json out.json] [--only LF-01,LF-09]
//                                [--ledger sandbox=/path/to/ledger] [--bin ./bin/cs-ledger]
//                                [--keep] [--allow-skip] [--verbose]
//                                [--record [--approve <dispatch-id> --reason "<text>"]]
//
// --record rewrites expectations.json from what was just measured. Frozen-value
// contract (per the campaign's frozen-values document): a change to a `keep`
// row's value or to a `must-change` row's target is gated — it needs BOTH
// --approve <id> and --reason "<text>", and the row records an `approval`
// block carrying id, reason, previous and measured values. Without both flags
// the runner exits non-zero, writes nothing at all, names every row it would
// have written, and prints the authorising command. A byte-identical write
// needs no flags. A row the run could not measure keeps its existing value and
// approval verbatim; a row that can neither be measured nor carried forward is
// an error, never a blank. A new row is not an approved write but is disclosed
// with an `origin` field whose id is the --approve id, or the member name
// ("ledger") when no flags are given — never a placeholder.
//
// Exit status: 0 when every `keep` check matches (and, with --strict, no serious or
// critical axe violation remains); 1 otherwise.

import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { gzipSync } from "node:zlib";
import puppeteer from "puppeteer-core";
import { focusRoles, selectors } from "./selectors.mjs";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, "..", "..");
const EXPECTATIONS = path.join(HERE, "expectations.json");
const require = createRequire(import.meta.url);

// ---- options ---------------------------------------------------------------
const argv = process.argv.slice(2);
const flag = (name) => argv.includes(name);
const opt = (name) => { const i = argv.indexOf(name); return i >= 0 ? argv[i + 1] : undefined; };
const opts = {
  strict: flag("--strict"), record: flag("--record"), keep: flag("--keep"), allowSkip: flag("--allow-skip"),
  verbose: flag("--verbose"), json: opt("--json"), only: (opt("--only") ?? "").split(",").filter(Boolean),
  approve: opt("--approve"), reason: opt("--reason"),
  bin: opt("--bin") ?? process.env.LEDGER_FIXTURES_BIN ?? path.join(REPO, "bin", "cs-ledger"),
  ledgerOverrides: Object.fromEntries(argv.flatMap((a, i) => a === "--ledger" ? [argv[i + 1].split("=")] : [])),
};
// --approve and --reason: both or neither; one without the other is an error.
if ((opts.approve === undefined) !== (opts.reason === undefined)) {
  console.error("fixtures: --approve <id> and --reason \"<text>\" must be given together — one without the other is an error");
  process.exit(2);
}
if (opts.approve !== undefined && !opts.record) console.error("fixtures: note: --approve/--reason have no effect without --record");
if (flag("--help") || flag("-h")) {
  console.log(readFileSync(fileURLToPath(import.meta.url), "utf8").split("\n").slice(1, 27).map((l) => l.replace(/^\/\/ ?/, "")).join("\n"));
  process.exit(0);
}

// ---- constants every expectation was measured under -------------------------
const VIEWPORT = { width: 1280, height: 900 };
const VIEWS = ["brief", "board", "list", "activity"];
const THEMES = ["light", "dark"];
// Raised from 600,000 on a recorded maintainer ruling, 2026-08-25. The old
// number was a record-count ceiling in disguise: the rendered shell is a fixed
// ~406 KB on every page and only record data varies, at ~3,025 B/record, so
// 600,000 capped a ledger at ~64 records, and any ledger past that missed the
// budget for having records rather than for being wasteful. 1,000,000 is
// headroom to ~196 records on today's shell. It is not a re-baseline to fit one
// artifact: the recorded `value` of every size row is untouched and no rendered
// page changed size because of this edit.
// expectations.json's targets, notes and meta.sizeBudgetBytes are generated
// from this constant, so this is the only place to change it.
const SIZE_BUDGET = 1_000_000;
const AXE_TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"];
// Both ledgers this suite renders are carried by this repo, so the fixtures run
// in any clone and in CI. The second one used to be a sibling `campaign`
// checkout: that resolved only on a machine holding one, and the public
// campaign's ledger is not the one these values were frozen against, so the
// lookup found the wrong data as readily as the right data. Fixtures never
// name a host path, and now they never leave the repo either.
const LEDGERS = {
  own: { dir: opts.ledgerOverrides.own ?? path.join(REPO, "ledger"), label: "repo ledger" },
  sandbox: { dir: opts.ledgerOverrides.sandbox ?? path.join(REPO, "fixtures", "sandbox", "ledger"), label: "sandbox fixture" },
};

const expected = existsSync(EXPECTATIONS) ? JSON.parse(readFileSync(EXPECTATIONS, "utf8")) : { meta: {}, checks: [] };
// The page's clock is frozen so "stale Nd" / "new since last visit" never drift.
const CLOCK = expected.meta?.clock ?? "2026-08-21T12:00:00Z";

// ---- browser -----------------------------------------------------------------
const executablePath = process.env.LEDGER_FIXTURES_BROWSER ?? process.env.CHROME_BIN ?? process.env.PUPPETEER_EXECUTABLE_PATH;
if (!executablePath || !existsSync(executablePath)) {
  console.error("fixtures: set LEDGER_FIXTURES_BROWSER (or CHROME_BIN / PUPPETEER_EXECUTABLE_PATH) to a Chromium executable");
  process.exit(2);
}
function axePath() {
  const candidates = [process.env.LEDGER_FIXTURES_AXE, path.join(HERE, "vendor", "axe-core", "axe.min.js")].filter(Boolean);
  try { candidates.splice(1, 0, require.resolve("axe-core/axe.min.js")); } catch {}
  return candidates.find((c) => existsSync(c));
}

// ---- render -----------------------------------------------------------------
const outDir = process.env.LEDGER_FIXTURES_OUT ?? mkdtempSync(path.join(tmpdir(), "ledger-fixtures-"));
function render(name) {
  const { dir } = LEDGERS[name];
  if (!dir || !existsSync(path.join(dir, "ledger.json"))) return null;
  if (!existsSync(opts.bin)) throw new Error(`cs-ledger binary not found at ${opts.bin} — run 'make build' or set LEDGER_FIXTURES_BIN`);
  const copy = path.join(outDir, name);
  rmSync(copy, { recursive: true, force: true });
  cpSync(dir, copy, { recursive: true }); // render writes ledger.html/json in place; never touch the source
  execFileSync(opts.bin, ["render", copy], { stdio: opts.verbose ? "inherit" : "pipe" });
  const file = path.join(copy, "ledger.html");
  const html = readFileSync(file, "utf8");
  const data = JSON.parse(html.match(/<script id=["']ledger-data["'][^>]*>([\s\S]*?)<\/script>/)[1].replace(/\\u003c/g, "<"));
  return { name, file, html, url: pathToFileURL(file).href, records: data.records.length, drafts: data.drafts.length, project: data.project };
}

// ---- page helpers -------------------------------------------------------------
const PAGE_HELPERS = `window.__lf = {
  raw: (s) => (s || "").replace(/\\s+/g, " ").trim(),
  norm: (s) => (s || "").replace(/[\\u25B8\\u25BE\\u25B9\\u25BF]/g, "").replace(/\\s+/g, " ").trim(),
  visible: (el) => !!el && el.getClientRects().length > 0,
  q(spec) {
    const css = typeof spec === "string" ? spec : spec.css;
    let els = [...document.querySelectorAll(css)];
    if (typeof spec !== "string") {
      const t = (spec.text ?? spec.textPrefix).toLowerCase();
      els = els.filter((el) => { const x = this.raw(el.textContent).toLowerCase(); return spec.text != null ? x === t : x.startsWith(t); });
    }
    return els;
  },
  one(spec) { return this.q(spec)[0] ?? null; },
  label(el) {
    if (!el || el === document.body) return "BODY";
    const name = el.getAttribute("aria-label") || this.norm(el.textContent) || el.getAttribute("placeholder") || "";
    return el.tagName.toLowerCase() + (el.id ? "#" + el.id : "") + (el.getAttribute("role") ? "[" + el.getAttribute("role") + "]" : "") + ' "' + name.slice(0, 32) + '"';
  },
  focusable: 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
};`;
const clockScript = (iso) => `(() => { const fixed = ${new Date(iso).getTime()}; const Real = Date;
  class Frozen extends Real { constructor(...a) { super(...(a.length ? a : [fixed])); } static now() { return fixed; } }
  Frozen.UTC = Real.UTC; Frozen.parse = Real.parse; window.Date = Frozen; })();`;

async function newPage(browser, log) {
  const context = await browser.createBrowserContext();
  const page = await context.newPage();
  await page.setViewport(VIEWPORT);
  await page.emulateTimezone("UTC");
  await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: "light" }]);
  await page.evaluateOnNewDocument(clockScript(CLOCK));
  await page.evaluateOnNewDocument(PAGE_HELPERS);
  page.on("pageerror", (e) => log?.errors.push(String(e?.message ?? e)));
  page.on("console", (m) => { if (m.type() === "error") log?.errors.push(m.text()); });
  page.on("request", (r) => log?.requests.push(r.url()));
  page.close2 = async () => { await page.close(); await context.close(); };
  return page;
}
async function open(page, ledger, { view = "brief", theme = "light", hash = "" } = {}, waitUntil = "load") {
  await page.goto(`${ledger.url}?theme=${theme}&view=${view}${hash}`, { waitUntil });
  await page.waitForFunction((sel) => { const b = document.querySelector(sel); return b && b.children.length > 0; }, { timeout: 15000 }, selectors.viewBody);
  await settle(page);
}
async function settle(page) {
  let last = null;
  for (let i = 0; i < 25; i++) {
    await new Promise((r) => setTimeout(r, 120));
    const now = await page.evaluate((sel) => document.querySelector(sel)?.innerText.length ?? -1, selectors.viewBody);
    if (now === last && i >= 1) return;
    last = now;
  }
}
async function handle(page, spec) {
  const h = await page.evaluateHandle((s) => window.__lf.one(s), spec);
  const el = h.asElement();
  if (!el) throw new Error(`role not found in page: ${JSON.stringify(spec)}`);
  return el;
}
async function click(page, spec) { await (await handle(page, spec)).click(); await settle(page); }
async function typeSearch(page, text) {
  const el = await handle(page, selectors.search);
  await el.evaluate((input) => { const set = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set; set.call(input, ""); input.dispatchEvent(new Event("input", { bubbles: true })); });
  await el.focus();
  if (text) await el.type(text);
  await settle(page);
}
async function expandAll(page) {
  for (const spec of [selectors.collapsedLaneHeader, selectors.collapsedGroupHeader]) {
    for (let i = 0; i < 12; i++) {
      const h = await page.evaluateHandle((s) => window.__lf.one(s), spec);
      const el = h.asElement();
      if (!el) break;
      await el.click(); await settle(page);
    }
  }
}
const visibleIds = (page) => page.evaluate((S) => [...new Set(window.__lf.q(S.recordId).filter((el) => el.closest(S.viewBody) && window.__lf.visible(el)).map((el) => window.__lf.norm(el.textContent)).filter((t) => t && t !== "draft"))].sort(), selectors);
const visibleRecordCount = (page) => page.evaluate((S) => window.__lf.q(S.card).filter((el) => window.__lf.visible(el)).length + window.__lf.q(S.activityRow).filter((el) => window.__lf.visible(el)).length, selectors);
const detailState = (page) => page.evaluate((S) => { const d = window.__lf.one(S.detail); return { open: !!d && window.__lf.visible(d), id: d ? window.__lf.norm(window.__lf.one(S.detailId)?.textContent) : null, title: d ? window.__lf.norm(window.__lf.one(S.detailTitle)?.textContent) : null }; }, selectors);

// ---- probes -------------------------------------------------------------------
// Each probe yields { id, value } pairs. Ids are stable; names/status/targets live in CHECKS.
async function structure(page, ledger, view) {
  const perTheme = {};
  for (const theme of THEMES) {
    await open(page, ledger, { view, theme });
    perTheme[theme] = await page.evaluate((S, view) => {
      const L = window.__lf;
      const inBody = (el) => el.closest(S.viewBody) && L.visible(el);
      const set = (spec, filter = () => true) => [...new Set(L.q(spec).filter((el) => L.visible(el) && filter(el)).map((el) => L.norm(el.textContent)).filter(Boolean))].sort();
      const headers = (spec) => L.q(spec).filter(L.visible).map((h) => { const c = h.cloneNode(true); c.querySelectorAll(S.headerCount).forEach((n) => n.remove()); return { label: L.norm(c.textContent), count: Number(L.norm(h.querySelector(S.headerCount)?.textContent) || 0) }; });
      const out = {
        theme: document.documentElement.getAttribute("data-theme"),
        records: L.q(S.card).filter(inBody).length + L.q(S.activityRow).filter(inBody).length,
        ids: [...new Set(L.q(S.recordId).filter(inBody).map((el) => L.norm(el.textContent)).filter((t) => t && t !== "draft"))].sort(),
        severityLabels: set(S.severityLabel, inBody),
        statusLabels: set(S.statusLabel, inBody),
        viewSwitch: L.q(S.viewSwitch).filter(L.visible).map((b) => L.norm(b.textContent)),
        toolbar: L.q(S.toolbarControls).filter(L.visible).map((c) => c.tagName === "INPUT" ? `[${c.type}] ${c.placeholder || c.getAttribute("aria-label") || ""}`.trim() : L.norm(c.textContent)),
        search: !!L.one(S.search) && L.visible(L.one(S.search)),
      };
      if (view === "board") out.lanes = headers(S.laneHeader);
      if (view === "list") out.groups = headers(S.groupHeader);
      return out;
    }, selectors, view);
  }
  const themes = Object.fromEntries(THEMES.map((t) => [t, perTheme[t].theme]));
  for (const t of THEMES) delete perTheme[t].theme;
  return { themes, ...perTheme.light, darkDiffers: JSON.stringify(perTheme.light) !== JSON.stringify(perTheme.dark) ? perTheme.dark : undefined };
}

async function interactions(page, ledger, ownPrefix) {
  const out = {};
  await open(page, ledger, { view: "list" });
  await expandAll(page);
  const all = await visibleRecordCount(page);
  for (const q of ["wait", "SAC-06", ownPrefix]) { await typeSearch(page, q); out[`search:${q}`] = await visibleIds(page); }
  await typeSearch(page, "");
  const cyc = async (spec) => { await click(page, spec); return { label: await labelOf(page, spec), count: await visibleRecordCount(page) }; };
  out.statusCycle = { all, afterClick1: await cyc(selectors.statusCycle), afterClick2: await cyc(selectors.statusCycle) };
  await click(page, selectors.resetButton);
  out.severityCycle = { all, afterClick1: await cyc(selectors.severityCycle), afterClick2: await cyc(selectors.severityCycle) };
  await click(page, selectors.resetButton);
  const first5 = () => visibleIdsInOrder(page).then((ids) => ids.slice(0, 5));
  out.sortCycle = { id: await first5() };
  await click(page, selectors.sortCycle); out.sortCycle.afterClick1 = { label: await labelOf(page, selectors.sortCycle), first5: await first5() };
  await click(page, selectors.sortCycle); out.sortCycle.afterClick2 = { label: await labelOf(page, selectors.sortCycle), first5: await first5() };
  await click(page, selectors.sortCycle); out.sortCycle.afterClick3 = { label: await labelOf(page, selectors.sortCycle), first5: await first5() };
  await typeSearch(page, "wait"); await click(page, selectors.statusCycle);
  const before = await visibleRecordCount(page);
  await click(page, selectors.resetButton);
  out.reset = { beforeReset: before, afterReset: await visibleRecordCount(page), equalsAll: (await visibleRecordCount(page)) === all, searchCleared: (await page.evaluate((S) => window.__lf.one(S.search).value, selectors)) === "" };
  // open the first card by click, read the detail, close with Escape
  const firstKey = await page.evaluate((S) => { const c = window.__lf.one(S.card); return { key: c.getAttribute(S.cardKeyAttr), id: window.__lf.norm(c.querySelector(S.recordId)?.textContent) }; }, selectors);
  await click(page, selectors.card);
  const opened = await detailState(page);
  await page.keyboard.press("Escape"); await settle(page);
  const closed = await detailState(page);
  out.openRecord = { clicked: firstKey.id, detailOpens: opened.open, detailId: opened.id, detailTitle: opened.title, idMatches: opened.id === firstKey.id, escapeCloses: !closed.open };
  // hash deep link (fresh document: a same-URL hash change would be a fragment navigation, not a load)
  await page.goto("about:blank");
  await open(page, ledger, { view: "list", hash: `#${encodeURIComponent(firstKey.key)}` });
  out.hashOpensDetail = (await detailState(page)).open;
  return out;
}
const labelOf = (page, spec) => page.evaluate((s) => window.__lf.norm(window.__lf.one(s).textContent), spec);
const visibleIdsInOrder = (page) => page.evaluate((S) => window.__lf.q(S.card).filter((el) => window.__lf.visible(el)).map((el) => window.__lf.norm(el.querySelector(S.recordId)?.textContent)).filter((t) => t && t !== "draft"), selectors);

async function themeToggle(page, ledger) {
  await open(page, ledger, { view: "board", theme: "light" });
  const state = () => page.evaluate(() => ({ theme: document.documentElement.getAttribute("data-theme"), stored: localStorage.getItem("ledger-theme") }));
  const out = { initial: await state(), toggle: await page.evaluate((S) => window.__lf.label(window.__lf.one(S.themeToggle)), selectors) };
  await click(page, selectors.themeToggle); out.afterClick1 = await state();
  await page.goto(`${ledger.url}?view=board`, { waitUntil: "load" }); await settle(page); out.afterReloadNoParam = await state();
  await click(page, selectors.themeToggle); out.afterClick2 = await state();
  await click(page, selectors.themeToggle); out.afterClick3 = await state();
  out.persists = out.afterClick1.theme === out.afterReloadNoParam.theme && out.afterReloadNoParam.theme !== out.initial.theme;
  return out;
}

async function axe(page, ledger, view, axeFile) {
  const out = {};
  for (const theme of THEMES) {
    await open(page, ledger, { view, theme });
    await page.addScriptTag({ content: readFileSync(axeFile, "utf8") });
    const res = await page.evaluate(async (tags) => { const r = await window.axe.run(document, { runOnly: { type: "tag", values: tags } }); return r.violations.map((v) => [v.id, { impact: v.impact, nodes: v.nodes.length }]); }, AXE_TAGS);
    out[theme] = Object.fromEntries(res.sort((a, b) => a[0].localeCompare(b[0])));
  }
  return out;
}
const seriousNodes = (byTheme) => Object.values(byTheme).reduce((n, rules) => n + Object.values(rules).filter((r) => r.impact === "serious" || r.impact === "critical").reduce((m, r) => m + r.nodes, 0), 0);

async function keyboard(page, ledger) {
  await open(page, ledger, { view: "board" });
  const out = { view: "board" };
  const stops = []; let reachedRecord = false, reachedLane = false, cycled = false;
  await page.evaluate(() => { window.__lfSeen = new Set(); });
  for (let i = 0; i < 400 && !cycled; i++) {
    await page.keyboard.press("Tab");
    const s = await page.evaluate((R) => { const L = window.__lf; const e = document.activeElement; const seen = window.__lfSeen.has(e); window.__lfSeen.add(e); return { label: L.label(e), record: !!e.closest?.(R.record), lane: !!e.closest?.(R.laneHeader), seen, body: e === document.body }; }, focusRoles);
    if (s.seen) { cycled = true; break; }
    if (stops.length < 20) stops.push(s.label + (s.record ? " (record)" : s.lane ? " (lane)" : ""));
    reachedRecord ||= s.record; reachedLane ||= s.lane;
  }
  out.first20 = stops; out.recordReachable = reachedRecord; out.laneHeaderReachable = reachedLane;
  out.focusable = await page.evaluate((S) => { const L = window.__lf; const cards = L.q(S.card).filter(L.visible); return { records: cards.length, focusableRecords: cards.filter((c) => c.matches(L.focusable) || c.querySelector(L.focusable)).length, laneHeaders: L.q(S.laneHeader).filter(L.visible).length, focusableLaneHeaders: L.q(S.laneHeader).filter((h) => L.visible(h) && (h.matches(L.focusable) || h.querySelector(L.focusable))).length, ariaExpanded: L.q(S.laneHeader).filter((h) => h.hasAttribute("aria-expanded") || h.querySelector("[aria-expanded]")).length }; }, selectors);
  // Enter on a focused card
  await page.evaluate((S) => { const L = window.__lf; const c = L.one(S.card); const t = c.matches(L.focusable) ? c : c.querySelector(L.focusable) ?? c; t.focus(); }, selectors);
  const focusedCard = await page.evaluate((R) => !!document.activeElement?.closest(R.record), focusRoles);
  await page.keyboard.press("Enter"); await settle(page);
  const afterEnter = await detailState(page);
  let escapeCloses = null, focusInsideDetail = null;
  if (afterEnter.open) {
    focusInsideDetail = await page.evaluate((S) => !!document.activeElement?.closest(S.detail), selectors);
    await page.keyboard.press("Escape"); await settle(page); escapeCloses = !(await detailState(page)).open;
  }
  out.enter = { cardTakesFocus: focusedCard, opensDetail: afterEnter.open, detailId: afterEnter.id, focusInsideDetail, escapeCloses };
  return out;
}

function size(ledger) {
  const html = ledger.html;
  const scripts = [...html.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/g)];
  const data = scripts.find((m) => /id=["']ledger-data["']/.test(m[0]));
  const bytes = Buffer.byteLength(html);
  return { bytes, gzipBytes: gzipSync(html).length, scriptBytes: scripts.filter((m) => m !== data).reduce((a, m) => a + Buffer.byteLength(m[1]), 0), styleBytes: [...html.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)].reduce((a, m) => a + Buffer.byteLength(m[1]), 0), dataBytes: data ? Buffer.byteLength(data[1]) : 0, records: ledger.records, withinBudget: bytes <= SIZE_BUDGET };
}

// Open a record detail by hash deep link and inspect what the markdown viewer
// rendered. Every descent is anchored at MarkdownViewer's documented
// [data-markdown-content] hook; selectors live here rather than in
// selectors.mjs because they are internal to these two probes.
async function markdownDetail(page, ledger, hash) {
  await open(page, ledger, { view: "list", hash });
  return page.evaluate((S) => {
    const d = window.__lf.one(S.detail);
    if (!d || !window.__lf.visible(d)) return { error: "detail did not open" };
    const links = {};
    for (const a of d.querySelectorAll("[data-markdown-content] a")) links[a.textContent.trim()] = a.getAttribute("href") ?? "<no-href>";
    const fences = [...d.querySelectorAll("[data-markdown-content] pre code")].map((c) => ({
      // The language is carried either as the `language-*` class a highlighter
      // plugin writes, or as CodeBlock's documented `data-language` hook when
      // the fence is rendered through a codeRenderer. Same fact, two spellings.
      lang:
        [...c.classList].find((x) => x.startsWith("language-"))?.replace("language-", "") ??
        c.getAttribute("data-language") ??
        null,
      // rehype-highlight marks highlighted code with the hljs class and wraps
      // tokens in span children; unhighlighted code has neither. DOM
      // properties, not selectors — the token spans are our plugin's output.
      tokens: c.classList.contains("hljs") && c.childElementCount > 0 ? c.childElementCount : 0,
    }));
    const tagged = fences.filter((f) => f.lang);
    return { links, fences: fences.length, tagged: tagged.map((f) => f.lang), everyTaggedHighlighted: tagged.length > 0 && tagged.every((f) => f.tokens > 0) };
  }, selectors);
}

async function network(page, ledger, log) {
  await open(page, ledger, { view: "brief" }, ["load", "networkidle0"]);
  for (const view of ["board", "list", "activity"]) await click(page, { css: selectors.viewSwitch, text: view });
  await click(page, selectors.activityRow); await page.keyboard.press("Escape"); await settle(page);
  await page.waitForNetworkIdle({ idleTime: 500 });
  const own = new Set([ledger.url, ...[...new Set(log.requests)].filter((u) => u.startsWith(ledger.url))]);
  const external = [...new Set(log.requests)].filter((u) => !own.has(u) && !/^(data|blob|about):/.test(u));
  return { requests: log.requests.length, external, pageErrors: log.errors.length, errors: log.errors.slice(0, 5) };
}

// ---- check catalogue ---------------------------------------------------------
// status/target/note defaults are used on --record only; expectations.json is the record of truth afterwards.
const CHECKS = [];
let n = 0;
const def = (name, status, extra = {}) => { const id = `LF-${String(++n).padStart(2, "0")}`; CHECKS.push({ id, name, status, ...extra }); return id; };
const judges = {};
const ids = { structure: {}, interactions: {}, axe: {}, keyboard: {}, size: {}, network: {} };
for (const L of ["own", "sandbox"]) for (const v of VIEWS) ids.structure[`${L}/${v}`] = def(`structure ${L}/${v} (records, ids, labels, toolbar, lanes) in both themes`, "keep", { note: "Behaviour freeze: same visible record set, labels and controls on brief/board/list/activity, light and dark." });
for (const L of ["own", "sandbox"]) {
  const I = ids.interactions[L] = {};
  I.searchWait = def(`search "wait" → visible id set (${L}, list, all groups expanded)`, "keep");
  I.searchSac = def(`search "SAC-06" → visible id set (${L})`, "keep");
  I.searchOwn = def(`search "<ledger prefix>-00" → visible id set (${L})`, "keep");
  I.status = def(`status cycle → label + count after 1 and 2 clicks (${L}, list, all groups expanded)`, "keep");
  I.severity = def(`severity cycle → label + count after 1 and 2 clicks (${L})`, "keep");
  I.sort = def(`sort cycle → first-5 id order after 0/1/2/3 clicks (${L})`, "keep");
  I.reset = def(`reset → count returns to all, search cleared (${L})`, "keep");
  I.open = def(`click first card → detail shows its id + title; Escape closes; #hash deep link opens (${L})`, "keep");
}
ids.theme = def("theme toggle cycles light → dark → system and persists in localStorage ledger-theme across reload (sandbox)", "keep");
for (const L of ["own", "sandbox"]) for (const v of VIEWS) { ids.axe[`${L}/${v}`] = def(`axe ${L}/${v} violations by rule (light + dark)`, "must-change", { target: { seriousOrCriticalNodes: 0 }, note: "Baseline = today's violations; the target is zero serious/critical nodes in both themes. Run with --strict to fail on any." }); judges[ids.axe[`${L}/${v}`]] = (live) => seriousNodes(live) === 0; }
for (const L of ["own", "sandbox"]) {
  const K = ids.keyboard[L] = {};
  K.stops = def(`Tab walk: first 20 stop labels (${L}, board)`, "must-change", { target: { includesRecordOrLaneHeader: true }, note: "Baseline records today's chrome-only Tab loop; the target is a record or lane header among the stops." });
  judges[K.stops] = (live) => live.some((l) => / \((record|lane)\)$/.test(l));
  K.reach = def(`Tab reaches a board card and a lane header (${L})`, "must-change", { target: { recordReachable: true, laneHeaderReachable: true } });
  judges[K.reach] = (live) => live.recordReachable && live.laneHeaderReachable;
  K.enter = def(`Enter on a focused card opens its detail, focus moves inside, Escape closes (${L})`, "must-change", { target: { opensDetail: true, escapeCloses: true, focusInsideDetail: true } });
  judges[K.enter] = (live) => live.opensDetail && live.escapeCloses && live.focusInsideDetail;
  K.focusable = def(`focusable records / lane headers vs visible (${L}, board)`, "must-change", { target: { allRecordsFocusable: true, allLaneHeadersFocusable: true }, note: "Target: every visible record and lane header is keyboard-focusable." });
  judges[K.focusable] = (live) => live.focusableRecords === live.records && live.focusableLaneHeaders === live.laneHeaders;
}
for (const L of ["own", "sandbox"]) { ids.size[L] = def(`ledger.html size (${L})`, "must-change", { target: { bytes: SIZE_BUDGET }, note: `Budget ${SIZE_BUDGET.toLocaleString()} bytes uncompressed.` }); judges[ids.size[L]] = (live) => live.bytes <= SIZE_BUDGET; }
for (const L of ["own", "sandbox"]) ids.network[L] = def(`no external requests, zero page errors on file:// (${L}, all views + a detail)`, "keep", { note: "Compared on external request list and error count only." });
ids.mdLinks = def(`markdown link safety: javascript:/data: hrefs emptied, ordinary https link survives (sandbox, SBX-015 detail)`, "keep", { note: "The viewer's URL allowlist, watched in our tree: dropped schemes become empty hrefs. SBX-015 is a synthetic record carrying the three link kinds." });
ids.mdHighlight = def(`every language-tagged code fence renders highlighted (own, LGR-003 detail; exactly one, bash)`, "keep", { note: "The shipped page carries exactly one tagged fence (```bash); rehype-highlight must mark it. A new tagged fence in the ledger changes this row and forces review." });

// ---- run ---------------------------------------------------------------------
const wanted = (id) => !opts.only.length || opts.only.includes(id);
const live = {}; const skipped = [];
const skip = (idList, why) => { for (const id of idList) { skipped.push({ id, why }); } };
const browser = await puppeteer.launch({ executablePath, headless: true, args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-gpu", "--disable-dev-shm-usage"] });
const t0 = Date.now();
try {
  const rendered = {};
  for (const name of Object.keys(LEDGERS)) { rendered[name] = render(name); if (!rendered[name]) console.error(`fixtures: ${name} ledger not found (${LEDGERS[name].dir}) — its checks are skipped`); }
  const axeFile = axePath();
  const run = async (id, fn) => { if (!wanted(id)) return; try { live[id] = await fn(); } catch (e) { live[id] = { error: String(e?.message ?? e) }; } if (opts.verbose) console.error(`${id} ${JSON.stringify(live[id]).slice(0, 300)}`); };
  const withPage = async (fn) => { const log = { errors: [], requests: [] }; const page = await newPage(browser, log); try { return await fn(page, log); } finally { await page.close2(); } };

  for (const L of ["own", "sandbox"]) {
    const ledger = rendered[L];
    const byLedger = (map) => Object.entries(map).filter(([k]) => k.startsWith(`${L}/`)).map(([, id]) => id);
    const all = [...byLedger(ids.structure), ...Object.values(ids.interactions[L]), ...byLedger(ids.axe), ...Object.values(ids.keyboard[L]), ids.size[L], ids.network[L], ...(L === "sandbox" ? [ids.theme] : []), ...(L === "own" ? [ids.mdHighlight] : [])];
    if (!ledger) { skip(all, `${L} ledger missing`); continue; }
    for (const v of VIEWS) await run(ids.structure[`${L}/${v}`], () => withPage((p) => structure(p, ledger, v)));
    const prefix = `${ledger.html.match(/"id":"([A-Z]{2,6})-\d{3}"/)?.[1] ?? "XX"}-00`;
    const I = ids.interactions[L];
    if (Object.values(I).some(wanted)) {
      const r = await withPage((p) => interactions(p, ledger, prefix));
      if (r) { live[I.searchWait] = r["search:wait"]; live[I.searchSac] = r["search:SAC-06"]; live[I.searchOwn] = { query: prefix, ids: r[`search:${prefix}`] }; live[I.status] = r.statusCycle; live[I.severity] = r.severityCycle; live[I.sort] = r.sortCycle; live[I.reset] = r.reset; live[I.open] = { ...r.openRecord, hashOpensDetail: r.hashOpensDetail }; }
    }
    if (L === "sandbox") await run(ids.theme, () => withPage((p) => themeToggle(p, ledger)));
    for (const v of VIEWS) { const id = ids.axe[`${L}/${v}`]; if (!axeFile) skip([id], "axe-core not found (LEDGER_FIXTURES_AXE / npm axe-core / vendor)"); else await run(id, () => withPage((p) => axe(p, ledger, v, axeFile))); }
    const K = ids.keyboard[L];
    if (Object.values(K).some(wanted)) { const r = await withPage((p) => keyboard(p, ledger)); live[K.stops] = r.first20; live[K.reach] = { recordReachable: r.recordReachable, laneHeaderReachable: r.laneHeaderReachable }; live[K.enter] = r.enter; live[K.focusable] = r.focusable; }
    await run(ids.size[L], () => size(ledger));
    await run(ids.network[L], () => withPage((p, log) => network(p, ledger, log)));
    if (L === "own") await run(ids.mdHighlight, () => withPage((p) => markdownDetail(p, ledger, "#LGR-003")));
  }
  if (rendered.sandbox) {
    await run(ids.size.sandbox, () => size(rendered.sandbox));
    await run(ids.mdLinks, () => withPage((p) => markdownDetail(p, rendered.sandbox, "#SBX-015")));
  } else skip([ids.size.sandbox, ids.mdLinks], "sandbox ledger missing");
} finally {
  await browser.close();
  if (!opts.keep && !process.env.LEDGER_FIXTURES_OUT) rmSync(outDir, { recursive: true, force: true });
  else console.error(`fixtures: rendered pages kept in ${outDir}`);
}

// ---- compare + report --------------------------------------------------------
const canon = (v) => JSON.stringify(v, (k, x) => x && typeof x === "object" && !Array.isArray(x) ? Object.fromEntries(Object.keys(x).sort().map((key) => [key, x[key]])) : x);
const brief = (v, max = 70) => { if (v === undefined) return "-"; let s = Array.isArray(v) ? `[${v.length}] ${v.slice(0, 4).map((x) => typeof x === "string" ? x : JSON.stringify(x)).join(", ")}${v.length > 4 ? " …" : ""}` : typeof v === "object" && v ? Object.entries(v).map(([k, x]) => `${k}=${Array.isArray(x) ? `[${x.length}]` : typeof x === "object" && x ? brief(x, 40) : String(x)}`).join(" ") : String(v); return s.length > max ? s.slice(0, max - 1) + "…" : s; };
function diff(a, b) {
  if (Array.isArray(a) && Array.isArray(b)) { const A = new Set(a.map(canon)), B = new Set(b.map(canon)); const missing = b.filter((x) => !A.has(canon(x))), extra = a.filter((x) => !B.has(canon(x))); if (missing.length || extra.length) return `missing ${JSON.stringify(missing).slice(0, 80)} extra ${JSON.stringify(extra).slice(0, 80)}`; return "order differs"; }
  if (a && b && typeof a === "object" && typeof b === "object") return Object.keys({ ...a, ...b }).filter((k) => canon(a[k]) !== canon(b[k])).map((k) => `${k}: ${brief(a[k], 40)} ≠ ${brief(b[k], 40)}`).join("; ");
  return `${brief(a)} ≠ ${brief(b)}`;
}
const expectedById = Object.fromEntries((expected.checks ?? []).map((c) => [c.id, c]));
const rows = []; let failures = 0, strictFailures = 0;
for (const check of CHECKS) {
  if (!wanted(check.id)) continue;
  const exp = expectedById[check.id] ?? (opts.record ? check : null);
  const sk = skipped.find((s) => s.id === check.id);
  const value = live[check.id];
  let result, detail = "";
  if (sk) { result = "SKIP"; detail = sk.why; if (!opts.allowSkip) failures++; }
  else if (opts.record && !expectedById[check.id]) { result = "NEW"; detail = "recorded"; }
  else if (!exp) { result = "FAIL"; detail = "no expectation recorded"; failures++; }
  else if (value === undefined || value?.error) { result = "FAIL"; detail = value?.error ?? "not measured"; failures++; }
  else if (exp.status === "keep") { if (canon(value) === canon(exp.value)) result = "PASS"; else { result = "FAIL"; failures++; detail = diff(value, exp.value); } }
  else { const done = judges[check.id]?.(value) ?? false; result = done ? "DONE" : "OPEN"; const moved = exp.value !== undefined && canon(value) !== canon(exp.value); detail = `target ${brief(exp.target, 50)}${moved ? " · changed from baseline" : " · unchanged"}`; if (check.id.startsWith("LF") && ids.axe && Object.values(ids.axe).includes(check.id)) { const s = seriousNodes(value), b = exp.value ? seriousNodes(exp.value) : s; detail = `serious/critical nodes ${s} (baseline ${b}, Δ${s - b})`; if (opts.strict && s > 0) { strictFailures++; result = "FAIL"; } } }
  rows.push({ id: check.id, status: exp?.status ?? check.status, result, name: check.name, live: brief(value), detail });
}
const w = { id: 5, status: 11, result: 6, name: 78 };
console.log(`fixtures · ${rows.length} checks · ${((Date.now() - t0) / 1000).toFixed(1)}s · clock ${CLOCK} · viewport ${VIEWPORT.width}×${VIEWPORT.height}${opts.strict ? " · --strict" : ""}`);
console.log(["id".padEnd(w.id), "status".padEnd(w.status), "result".padEnd(w.result), "check".padEnd(w.name), "live → note"].join("  "));
for (const r of rows) console.log([r.id.padEnd(w.id), r.status.padEnd(w.status), r.result.padEnd(w.result), r.name.slice(0, w.name).padEnd(w.name), `${r.live}${r.detail ? `  → ${r.detail}` : ""}`].join("  "));
const counts = rows.reduce((m, r) => ((m[r.result] = (m[r.result] ?? 0) + 1), m), {});
console.log(`summary: ${Object.entries(counts).map(([k, v]) => `${k} ${v}`).join(" · ")} · must-change progress ${rows.filter((r) => r.status === "must-change" && r.result === "DONE").length}/${rows.filter((r) => r.status === "must-change").length}`);

if (opts.record) {
  // Frozen-value write path. Gated: any change to a `keep` row's value, and any
  // change to a `must-change` row's target — both need --approve/--reason, and
  // the row records the approval block. Anything unmeasured carries forward
  // verbatim; a row that can neither be measured nor carried forward is an
  // error. Refusal is loud and total: nothing is written at all.
  const approval = opts.approve !== undefined ? { id: opts.approve, reason: opts.reason } : null;
  const proposed = [];    // rows this write would contain (the wanted portion)
  const gated = [];       // { id, kind, previous, measured } — require --approve/--reason
  const wouldWrite = [];  // every row whose written content differs from the file
  const created = [];     // new rows: not an approved write, but disclosed
  const impossible = [];  // neither measured nor carried forward — an error, never a blank
  for (const c of CHECKS) {
    if (!wanted(c.id)) continue;
    const prev = expectedById[c.id];
    const v = live[c.id];
    const measuredOk = v !== undefined && !v?.error;
    if (!prev) {
      if (!measuredOk) { impossible.push(c.id); continue; }
      const e = { id: c.id, name: c.name, value: v, status: c.status };
      if (c.target) e.target = c.target;
      if (c.note) e.note = c.note;
      e.origin = approval
        ? { id: approval.id, reason: approval.reason }
        : { id: "ledger", reason: "new measured check recorded for the first time" };
      proposed.push(e); created.push(e); wouldWrite.push(`${c.id} (new row)`);
      continue;
    }
    // Carry the existing row forward verbatim (value, target, note, approval);
    // only a fresh measurement, a catalogue target or prose (name) may move,
    // and only as the contract allows.
    const e = { ...prev, name: c.name, status: prev.status ?? c.status };
    const rowGated = [];
    if (measuredOk && canon(v) !== canon(prev.value)) {
      e.value = v;
      if (e.status === "keep") rowGated.push({ kind: "keep value change", previous: prev.value, measured: v });
      else wouldWrite.push(`${c.id} (must-change value: progress measurement)`);
    }
    if (c.target && canon(c.target) !== canon(prev.target)) {
      e.target = c.target;
      rowGated.push({ kind: "must-change target change", previous: prev.target, measured: c.target });
    }
    for (const g of rowGated) gated.push({ id: c.id, ...g });
    if (rowGated.length) {
      wouldWrite.push(...rowGated.map((g) => `${c.id} (${g.kind})`));
      if (approval) {
        // One permission slip per row; for a must-change row it describes the target change.
        const g = rowGated[rowGated.length - 1];
        e.approval = { id: approval.id, reason: approval.reason, previous: g.previous, measured: g.measured };
      }
    }
    proposed.push(e);
  }
  if (impossible.length) {
    console.error(`fixtures: --record error: ${impossible.join(", ")} — not measured and no prior expectation to carry forward; a row is never written from nothing. NOTHING was written.`);
    process.exit(1);
  }
  if (gated.length && !approval) {
    console.error("fixtures: --record REFUSED — gated frozen value(s) would change and no --approve/--reason was given. NOTHING was written.");
    for (const g of gated) console.error(`fixtures:   gated: ${g.id} — ${g.kind}: ${brief(g.previous, 60)} → ${brief(g.measured, 60)}`);
    console.error(`fixtures: rows this write would have written (${wouldWrite.length} changed of ${proposed.length}):`);
    for (const wrow of wouldWrite) console.error(`fixtures:   ${wrow}`);
    console.error("fixtures: authorise this exact write with:");
    console.error(`fixtures:   node viewer/fixtures/run.mjs --record --approve <dispatch-id> --reason "<why this change is authorised>"`);
    process.exit(1);
  }
  // Rows outside --only, or no longer in the catalogue, pass through verbatim.
  const catalogue = new Set(CHECKS.map((c) => c.id));
  const keep = (expected.checks ?? []).filter((c) => !wanted(c.id) || !catalogue.has(c.id));
  const out = { meta: { ...(expected.meta ?? {}), recordedAt: new Date().toISOString().slice(0, 10), clock: CLOCK, viewport: VIEWPORT, sizeBudgetBytes: SIZE_BUDGET, axeTags: AXE_TAGS, note: "Values are measured facts about the viewer's behaviour. A viewer developer may edit selectors.mjs, never these values; re-record only with the reviewer's sign-off." }, checks: [...keep, ...proposed].sort((a, b) => a.id.localeCompare(b.id)) };
  writeFileSync(EXPECTATIONS, `${JSON.stringify(out, null, 2)}\n`);
  console.log(`recorded ${proposed.length} expectation(s) → ${path.relative(REPO, EXPECTATIONS)}`);
  for (const e of created) console.log(`fixtures: new row ${e.id} created — origin: ${JSON.stringify(e.origin)} — list it in the reply`);
  for (const g of gated) console.log(`fixtures: approved ${g.id} — ${g.kind} — approval block recorded on the row`);
}
if (opts.json) writeFileSync(opts.json, `${JSON.stringify({ meta: { clock: CLOCK, viewport: VIEWPORT, strict: opts.strict }, rows, live }, null, 2)}\n`);
process.exit(failures || strictFailures ? 1 : 0);
