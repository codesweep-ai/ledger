# Viewer fixtures — the behavioural oracle

`viewer/fixtures` holds the behaviour the ledger viewer (the single-file
`ledger.html` that `cs-ledger render` writes) must keep while it is rebuilt on
`@codesweep-ai/ui` components. Pixel parity is not a gate. Behaviour,
accessibility, keyboard reach and page size are.

The suite renders real ledgers with the `cs-ledger` binary, drives each page in
headless Chromium and compares what it measures with `expectations.json`.

## Run it

On the host:

```sh
make build                       # or: make build-go — the runner needs bin/cs-ledger
cd viewer && npm ci              # puppeteer-core comes from the viewer's devDependencies
LEDGER_FIXTURES_BROWSER=~/.cache/ms-playwright/chromium-1234/chrome-linux64/chrome npm run fixtures
```

In the guest VM the browser is in `$CHROME_BIN`, so `npm run fixtures` (or
`make fixtures`) works as is. `make fixtures` is deliberately not part of
`make check`; the orchestrator runs it explicitly. Chromium is launched with
`--no-sandbox`, which the VMs need.

Flags (after `npm run fixtures --`):

| flag | effect |
|---|---|
| `--strict` | also fail on any serious or critical axe violation |
| `--json out.json` | write the table and every live value to a file |
| `--only LF-09,LF-17` | run a subset of checks |
| `--ledger sandbox=/path` | point a named ledger (`own`, `sandbox`) at another directory |
| `--bin ./bin/cs-ledger` | renderer to use (default `bin/cs-ledger`) |
| `--keep` | keep the rendered pages in the temp directory and print its path |
| `--allow-skip` | a missing ledger skips its checks instead of failing |
| `--verbose` | print every live value |
| `--record` | rewrite the values in `expectations.json` from this run (oracle author only, see below) |

Environment:

| variable | meaning |
|---|---|
| `LEDGER_FIXTURES_BROWSER`, then `CHROME_BIN`, then `PUPPETEER_EXECUTABLE_PATH` | Chromium executable (required) |
| `LEDGER_FIXTURES_BIN` | renderer binary (same as `--bin`) |
| `LEDGER_FIXTURES_AXE` | path to `axe.min.js`; otherwise `axe-core` from `node_modules`, otherwise the vendored copy in `vendor/axe-core` |
| `LEDGER_FIXTURES_OUT` | render into this directory instead of a temp directory |

Exit status is 0 when every `keep` check matches (and, with `--strict`, no
serious or critical axe violation remains) and 1 otherwise. A ledger the runner
cannot find is reported as `SKIP` and fails the run unless `--allow-skip`.

Inputs: the repo's own `ledger/` (11 records) and `fixtures/sandbox/ledger`
(15 records). **Both are carried by this repo**, so the suite runs in any clone
and in CI. Each is copied to a temp directory before `cs-ledger render` runs, so
the sources are never touched.

## What is measured

Every check has a stable id. The table prints `id · status · result · check ·
live → note`. Results: `PASS`/`FAIL` for `keep` checks, `DONE`/`OPEN` for
`must-change` checks (with the delta from the recorded baseline), `SKIP`, and
`NEW` on `--record`.

| ids | area | what |
|---|---|---|
| LF-01 – LF-08 | structure | per ledger × view, both themes: visible record count, visible id set, severity and status labels, view switcher and toolbar control labels, search present, board lane / list group labels with counts |
| LF-09 – LF-24 | interactions | per ledger, list view with every group expanded: search `wait`, `SAC-06`, `<prefix>-00` → id sets; status and severity cyclers → label + count after one and two clicks; sort cycler → first five ids after 0/1/2/3 clicks; reset → back to all, search cleared; click the first card → detail shows its id and title, Escape closes, `#<id>` deep link opens it |
| LF-25 | theme | toggle cycles light → dark → system, `localStorage["ledger-theme"]` follows, and the choice survives a reload without `?theme=` |
| LF-26 – LF-33 | accessibility | axe-core (WCAG 2.x A/AA + best-practice) per ledger × view, light and dark, violations by rule id with impact and node count |
| LF-34 – LF-41 | keyboard | board view: first 20 Tab stop labels; whether Tab reaches a card and a lane header; Enter on a focused card opens its detail with focus inside and Escape closes; focusable records and lane headers vs visible |
| LF-42 – LF-44 | size | bytes, gzip bytes and the script / style / data split of `ledger.html` per ledger against the 1,000,000-byte budget |
| LF-45 – LF-46 | isolation | no request leaves the `file://` page across all four views and a detail; zero page errors |

Measurement conditions, recorded in `expectations.json` under `meta`: viewport
1280 × 900, timezone UTC, `prefers-color-scheme: light`, and the page's clock
frozen at `meta.clock` so "stale Nd" and "new since last visit" never drift.
Every check runs in a fresh browser context (empty `localStorage`).

## What is frozen, what may change

`expectations.json` is a record of measured facts. Each check carries
`{ id, name, value, status, target?, note? }`:

- `status: "keep"` — the live value must equal `value`. These are the viewer's
  behaviour: which records a view shows, what search and the filters return,
  what opening a record shows, how the theme persists, that nothing leaves the
  page.
- `status: "must-change"` — `value` is today's baseline, `target` is where the
  rebuild has to land: zero serious/critical axe nodes, every record and lane
  header keyboard-reachable, Enter opening a record, `ledger.html` within
  1,000,000 bytes. The runner reports progress; `--strict` turns the
  accessibility targets into failures.

A viewer developer **may** edit `selectors.mjs`. It maps each behavioural role
(the search box, a record card, a lane header, the detail dialog, ...) to
today's DOM, one commented entry per role, and it is the only place the DOM is
named. When the markup changes, point the role at the new element; the runner
then re-measures the same behaviour.

A viewer developer **must not** edit `expectations.json` or the check
definitions in `run.mjs`. Re-recording (`--record`) is for the oracle's author,
after a reviewer has agreed that a behaviour genuinely changed; the diff of
`expectations.json` is what gets reviewed. `--record` keeps each check's
`status`, `target` and `note` and replaces only `value`.

Labels that are part of a check's value (`status · open`, `sort · activity`,
lane names such as `Closed / retired`) are behaviour too: a control that reads
differently is a change a reader sees.

## Files

| file | role |
|---|---|
| `run.mjs` | the runner: render, drive, measure, compare, report |
| `selectors.mjs` | role → DOM map (the one file a viewer developer edits) |
| `expectations.json` | frozen values and targets, with the measurement conditions under `meta` |
| `vendor/axe-core/` | `axe.min.js` 4.13.0 and its MPL-2.0 licence, so the guest needs no extra install |
