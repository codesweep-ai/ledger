# cs-ledger(1) — manual

## Name

**cs-ledger** — validate a repository's issue records and render them as a single HTML page.

## Synopsis

```
cs-ledger check   [LEDGERDIR]
cs-ledger render  [LEDGERDIR] [--assets DIR]
cs-ledger init    [LEDGERDIR] --project NAME --prefix ABC
cs-ledger manual
cs-ledger guide
cs-ledger version
cs-ledger help

LEDGERDIR defaults to ./ledger.
```

## Description

A **ledger** is a directory of JSON files that records what is wrong with a repository, what could
be better, and what to do next. Agents write those files. `cs-ledger` checks that they are well
formed and renders them into `ledger.html`, a self-contained page a human opens straight from disk.

Everything the ledger holds is committed alongside the code it describes: the records, the config,
and the rendered page. There is no server and no database. The tool is one static binary with no
runtime dependencies, and the page it writes makes no network request.

The rendered page is a pure function of the records, the config and the renderer version. Nothing
in it comes from the wall clock. That is what lets `check` prove the page is current by rendering
again and comparing bytes.

For the record schema and the rules `check` enforces, see [SPEC.md](SPEC.md). For how an agent is
expected to work in a ledger day to day, read [GUIDE.md](GUIDE.md) or run `cs-ledger guide`. This
manual covers the command surface; the guide covers the practice.

## Commands

### check

```
cs-ledger check [LEDGERDIR]
```

The standing gate. Run it before every commit that touches the ledger. It validates every record
and draft against the schema and the lifecycle rules, and validates `queue.json` if the ledger has
one. It then confirms that `ledger.html` is what these records render to right now.

Errors fail the run. Warnings print and pass, because they flag judgement calls rather than broken
data. A link to a record that does not exist yet is a warning; a `closed` record with no evidence
is an error.

Two more states fail. One is a page rendered in dev mode with `--assets`, which would put an
unreproducible page into a commit. The other is a materialized `GUIDE.md` whose generated half no
longer matches the copy inside the binary. Whatever sits below the `<!-- LEDGER:PROJECT -->`
marker belongs to the project, and `check` leaves it alone.

A page written by a different renderer is a warning rather than a failure. Freshness is only
assertable within one renderer version, because the version is stamped into the page and the bytes
differ whatever the records say. `check` reports which renderer wrote the page and skips the
comparison. Run `cs-ledger render` to bring both to yours.

### render

```
cs-ledger render [LEDGERDIR] [--assets DIR]
```

The one command that writes. It rewrites `LEDGERDIR/ledger.html`, records this binary's renderer
in `ledger.json` as `toolVersion`, and brings the ledger's own documents up to the binary. Running
it twice changes nothing the second time.

Validation errors do not stop a render. They print, and `check` fails on them afterwards, so you
can look at a page whose records are still being fixed. A `ledger.json` the tool cannot read does
stop it, because there is nothing to render without one.

`--assets DIR` loads the viewer's CSS and JavaScript from disk instead of from inside the binary,
so you can iterate on the viewer without recompiling. The page it writes carries a dev stamp and
`check` refuses it, which keeps a hand-built page out of a commit. Dev mode writes the page alone,
leaving `toolVersion` and the documents as they were, because a dev-stamped page must not certify
anything.

### init

```
cs-ledger init [LEDGERDIR] --project NAME --prefix ABC
```

Scaffolds a new ledger: a `ledger.json` holding the project name, the record ID prefix and the
version pin, an empty `issues/`, a materialized `GUIDE.md` and `AGENTS.md`, and a first render.
Both flags are required. The prefix is two to six upper-case letters and becomes the first part of
every record ID.

`AGENTS.md` is the router, and it is how the ledger is found. Agent harnesses read the `AGENTS.md`
nearest the file being edited, so an agent that opens a record walks up to this one and is sent to
the guide. It holds no knowledge of its own, which is why nothing gates it.

A prefix in any other shape is refused before anything is written, so a typo costs you the command
rather than a half-built directory. If `LEDGERDIR/ledger.json` already exists, `init` refuses rather
than overwrite it.

### manual

```
cs-ledger manual | less
```

Prints this manual, the command surface, from inside the binary. A machine that has `cs-ledger`
has its reference, with no checkout to read and no page to fetch.

### guide

```
cs-ledger guide | less
```

Prints [GUIDE.md](GUIDE.md), the guide to keeping a ledger: the moves an agent makes, what
provenance and evidence have to say, and how the queue gets re-triaged. Read it before touching a
ledger. `init` and `render` also materialize it as `LEDGERDIR/GUIDE.md`, so a target repo carries
a copy that `check` holds to the binary's.

The two documents answer different questions. Reach for `manual` when you need a verb or a flag,
and `guide` when you need to know what to write in a record.

### version

Prints three versions: the tool's own, the renderer's, and the pinned design tokens'.

```console
$ cs-ledger version
cs-ledger v1-complete-110-g7ea68dc (renderer 0.3.1, ui tokens 1.12.0)
```

The first field is a `git describe` of the build, so yours will read differently. The renderer
version is the one that matters to a ledger. It is what `toolVersion` records and what `render`
moves.

### help

```
cs-ledger help
cs-ledger --help
```

Prints the verb list and the flags. Both spellings work and exit 0. Running with no verb at all
prints the same text on stderr and exits 2.

## Options

| Option | Applies to | Meaning |
|---|---|---|
| `--assets DIR` | render | Load viewer assets from `DIR`. The output is dev-stamped and not committable. |
| `--project NAME` | init | The project name shown on the rendered page. Required. |
| `--prefix ABC` | init | The record ID prefix, matching `^[A-Z]{2,6}$`. Required. |

## Files

| Path | What it is |
|---|---|
| `ledger/ledger.json` | Config. `project`, `idPrefix` and `schemaVersion` are required; `toolVersion`, `staleAfterDays`, `description`, `links` and `commitUrlTemplate` are optional. |
| `ledger/issues/<ID>.json` | One record per issue. The filename must equal the record's `id`. |
| `ledger/drafts/<member>/<slug>.json` | An observation from a branch that may not mint IDs. Same shape, without `id`. |
| `ledger/queue.json` | Optional. The ordered recommendation of what to fix next. |
| `ledger/ledger.html` | Generated. Never edit it and never hand-merge it; render it again instead. |
| `ledger/GUIDE.md` | The guide, materialized by `init` and `render`. Generated above the `<!-- LEDGER:PROJECT -->` marker, yours below it. |
| `ledger/AGENTS.md` | The router pointing at the guide. Written once by `init`, then yours. |

## Exit status

| Code | Meaning |
|---|---|
| 0 | Success. `check` found no errors. |
| 1 | `check` failed, or the run hit an error such as an unreadable config or an unwritable file. |
| 2 | No verb, or a verb the tool does not have. Usage goes to stderr. |

## Diagnostics

**`ledger.html: STALE — records changed without re-render`** — the committed page is not what these
records render to. Run `cs-ledger render` and commit the page with the records.

**`ledger.html: rendered in dev mode (--assets)`** — the page came from viewer files on disk rather
than from the binary. Render again without `--assets`.

**`ledger.html was rendered by X and this binary renders Y`** — a warning. Another renderer wrote
the committed page, so `check` cannot tell a stale page from version skew and does not try. Run
`cs-ledger render` to bring the page and `toolVersion` to your binary.

**`GUIDE.md: does not match this binary's embedded guide`** — the generated half of the
materialized guide has drifted from the binary's copy. `cs-ledger render` refreshes it and keeps
your conventions below the marker.

**`filename must equal id`** — a record's `id` field and its filename disagree. The filename is the
ID claim, so one of the two is a typo.

**`status "closed" requires non-empty evidence.verified`** — a record was closed without saying how
it was proved. Write what you measured, or reopen it.

**`is an open critical missing from the queue — needs triage`** — a warning. Something critical is
open and nothing scheduled it. Add it to `queue.json`, or lower the severity and say why in a note.

## Notes for agents

- Every verb is non-interactive and exits with a meaningful status. Nothing prompts.
- `cs-ledger guide` prints the operating doctrine from the binary. Read it there when the repo has
  no materialized copy.
- `check` is the gate to run before any commit that touches the ledger. Its messages name the file
  and the rule, so they can be acted on without reading the source.
- `render` writes only inside the ledger directory: the page, `ledger.json` and the two documents.
  It reads nothing outside that directory, apart from an `--assets` directory you name, and it
  reaches no network.
- Never edit `ledger.html`. It is generated, and `check` compares it byte for byte against a fresh
  render.
- Records accrete. Append to `notes` rather than rewriting `details`, and never renumber an ID.

## Examples

Start a ledger in a repository that has none:

```bash
cs-ledger init ledger --project my-service --prefix MYS
```

The gate an agent runs before committing:

```bash
cs-ledger render && cs-ledger check
```

Check a ledger that is not at the default path:

```bash
cs-ledger check fixtures/sandbox/ledger
```

Iterate on the viewer without recompiling, then produce a committable page:

```bash
cs-ledger render ledger --assets ./viewer   # dev-stamped; check refuses it
cs-ledger render ledger                     # from the embedded assets
```

Move a ledger onto a newer binary:

```bash
cs-ledger render ledger
git add ledger && git commit -m "Re-render the ledger on renderer 0.3.1"
```

## See also

[GUIDE.md](GUIDE.md) for how to keep a ledger, [README.md](README.md) for the tour,
[INSTALL.md](INSTALL.md) for getting the binary, and [SPEC.md](SPEC.md) for the record schema and
the rules. [AGENTS.md](AGENTS.md) says where an agent working in this repository looks first, and
[CONTRIBUTING.md](CONTRIBUTING.md) covers working on `cs-ledger` itself.
