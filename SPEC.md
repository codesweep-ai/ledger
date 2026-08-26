# The cs-ledger specification

This document specifies the ledger format and the behaviour of the `cs-ledger` tool. It is the
contract between the tool, the records agents write, and the page humans read.

**Audience.** Anyone implementing against the format, reviewing a change to the tool, or deciding
whether a repository's ledger is well formed. If you want to operate a ledger rather than
implement one, read [GUIDE.md](GUIDE.md) for the practice and [MANUAL.md](MANUAL.md) for the
command surface.

**Normative language.** **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT** and **MAY** carry their
RFC 2119 meanings. The numbered requirements (**R1**, **R2**, …) are the testable statements, and
the prose between them explains why each one is there.

## 1. Vocabulary

| Term | Meaning |
|---|---|
| **ledger** | A directory of JSON records describing one repository's defects, improvements and ideas, plus the page rendered from them. |
| **record** | One issue, held in one JSON file. The unit the format specifies. |
| **draft** | An observation written on a branch that may not allocate IDs. A record without an `id`. |
| **queue** | The ordered recommendation of what to fix next, held in `queue.json`. |
| **stint** | Whatever selected a record for work: an orchestrated run, a branch, or a dated session. |
| **viewer** | The HTML, CSS and JavaScript that `cs-ledger` embeds in the rendered page. |
| **pin** | The renderer version recorded in `ledger.json` as `toolVersion`. |
| **terminal status** | `closed`, `wont-fix` or `moved-to-roadmap`. A record that has stopped moving. |

## 2. The model

A ledger is data first. Agents write records as they work; the tool validates them and renders a
page; a human opens the page to see where the repository stands. Nothing runs in the background and
nothing is served.

**R1.** A ledger **MUST** be self-contained in one directory, and that directory **MUST** be committed to the
repository it describes.

**R2.** The rendered page **MUST** open from `file://` with no network request, no build step and no
server.

**R3.** The tool **MUST NOT** read or write anything outside the ledger directory, and **MUST NOT** reach the
network.

The point of R1 is that the ledger travels with the code. A clone carries the issue history the way
it carries the source, and a reviewer sees a record change in the same diff as the fix.

## 3. Repository layout

```
ledger/
  ledger.json           config, including the pin (§8)
  issues/<ID>.json      one record per issue (§4)
  drafts/<member>/*.json un-numbered observations (§6)
  queue.json            optional: the fix-next recommendation (§5)
  ledger.html           GENERATED: a pure function of the above (§7)
  GUIDE.md              GENERATED above the project marker: the operating guide (§9)
  AGENTS.md             the router pointing at the guide, written once by init
```

**R4.** The default ledger directory is `./ledger`. A tool invocation **MAY** name another path.

**R5.** `ledger.json` **MUST** carry `project` (a non-empty string), `idPrefix` (matching `^[A-Z]{2,6}$`)
and `schemaVersion` (exactly `issue.v1`).

**R6.** `ledger.json` **MAY** carry any of the following:

| Key | Value |
|---|---|
| `staleAfterDays` | A number of at least zero. How long an untouched record stays fresh. |
| `description` | A non-empty sentence describing the project. |
| `links` | An array of `{label, url}`. |
| `commitUrlTemplate` | An `http` or `https` URL containing the `{sha}` placeholder. |
| `toolVersion` | The pin (§8). |

**R7.** A `links` entry's `url` **MUST** be `http`, `https`, or a relative path. Any other scheme is an
error.

The description and links are the repository's front door. They render into the page's masthead, so
someone who opens `ledger.html` first sees what the project is before seeing what is wrong with it.

## 4. The record (`issue.v1`)

One JSON object per file, at `issues/<ID>.json`.

| Field | Type | Notes |
|---|---|---|
| `id` | `<PREFIX>-NNN` | Stable forever. Never renumbered, never reused. |
| `title` | non-empty string | One line. |
| `type` | `defect` \| `improvement` \| `feature-idea` | Roadmap directions are `feature-idea` records, not a separate construct. |
| `severity` | `low` \| `med` \| `high` \| `critical` | Ascending, so `critical` is the top level. |
| `status` | `open` \| `in-progress` \| `closed` \| `wont-fix` \| `moved-to-roadmap` | Lifecycle in §6. |
| `foundBy` | non-empty string | Provenance: who or what found this. |
| `opened` | `YYYY-MM-DD` | The filing date. Must be a real calendar date. |
| `resolved` | `YYYY-MM-DD` \| `null` | The date the record reached a terminal status. |
| `stint` | string \| `null` | What selected it for work. |
| `evidence` | `{commits: [string], integrated: [string], verified: string\|null}` | How the fix was proved. |
| `resolution` | string \| `null` | Why an abandoned record ended. |
| `details` | string | The narrative, in Markdown, written for a reader with no context. |
| `notes` | `[{date, text}]` | Append-only dated updates. May be empty. |
| `links` | `[id]` | Stated relationships to other records. May be empty. |

**R8.** Every field above **MUST** be present on a record, including those whose value is `null` or an
empty array. Absence is an error; an explicit empty value is not.

**R9.** A record's filename **MUST** equal its `id` with a `.json` suffix. The filename is the ID claim.

**R10.** An `id` **MUST** match `^<idPrefix>-\d{3,}$`, taking the prefix from `ledger.json`.

**R11.** Two records **MUST NOT** share an `id`.

**R12.** `opened` and every `notes[].date` **MUST** be a real date in `YYYY-MM-DD` form. A date that
parses as text but not as a day, such as `2026-02-30`, is an error.

**R13.** `notes[].date` **MUST NOT** decrease down the array. Notes are an append-only timeline.

**R14.** `links` entries **MUST** be well-formed IDs. A link to a record that does not exist **SHOULD** be
reported as a warning rather than an error, because partial migrations are a normal state.

**R15.** A field the schema does not name **SHOULD** be reported as a warning and **MUST NOT** fail
validation. Evolution within `issue.v1` is additive.

**R16.** `details` **SHOULD** be non-empty. An empty one is a warning, because a record with no narrative
is a row rather than an observation.

Prose fields hold Markdown. The rendered page is the reading surface, so readability of the raw
JSON is secondary to the quality of what it says.

## 5. The queue

`queue.json` is optional. It holds an ordered opinion about what to fix next.

```json
{ "recommendedBy": "orchestrator", "updated": "2026-08-14",
  "items": [ { "id": "MYS-012", "why": "blocks the 0.4 release" } ] }
```

**R17.** When present, `queue.json` **MUST** carry a non-empty `recommendedBy`, an `updated` date, and an
`items` array.

**R18.** Every item **MUST** carry an `id` that resolves to a record in this ledger and a non-empty `why`.

**R19.** An item **MUST NOT** name a record in a terminal status, and **MUST NOT** repeat an ID already in the
array.

**R20.** An item naming an `in-progress` record **SHOULD** be reported as a warning. The recommendation is
redundant once someone has claimed the work.

**R21.** An `open` `critical` record that is not a `feature-idea` and is absent from the queue **SHOULD**
be reported as a warning. Nothing scheduled it and nobody claimed it.

Ordering lives in this one file rather than as a priority number on each record. Per-record
integers renumber badly and conflict on every branch, whereas a single ordered list has one writer
and merges as one conflict.

## 6. Lifecycle and invariants

```
open ──> in-progress ──> closed
  │            │
  ├────────────┴──> wont-fix
  └───────────────> moved-to-roadmap
```

**R22.** A record in a terminal status **MUST** carry a non-null `resolved` date.

**R23.** A record in a non-terminal status **MUST** carry `resolved: null`.

**R24.** A record with `status: in-progress` **MUST** carry a non-null `stint`.

**R25.** A record with `status: closed` **MUST** carry a non-empty `evidence.verified`.

**R26.** A record with `status: closed` **MUST** carry either a non-empty `evidence.commits` or non-empty
`links`.

**R27.** A record with `status: wont-fix` or `status: moved-to-roadmap` **MUST** carry a non-empty
`resolution`.

**R28.** Records in a terminal status **SHOULD NOT** be edited except to append a note.

R25 and R26 are the rules the format exists for. A closed record has to say how the fix was proved,
and it has to point at the commit that did it. The `links` alternative in R26 covers an umbrella
record closed by delegation to its children, where no single commit resolves it; `verified` must
then explain the delegation.

Staleness is deliberately not validated. Whether an issue has sat too long is project-management
policy rather than a property of the data, so the viewer surfaces it and the tool stays out of it.

### ID allocation across branches

**R29.** Only the integration branch **MAY** create `issues/<ID>.json`. A branch that may not allocate IDs
**MUST** write to `drafts/<member>/<slug>.json` instead.

**R30.** A draft **MUST** have the same shape as a record, minus `id`. A draft carrying an `id` is an
error.

**R31.** Drafts **MUST** be validated for shape, and **MUST NOT** be validated for the ID and lifecycle rules
that assume a numbered record.

R29 is load-bearing rather than tidy. Under `git merge`, two branches minting the same ID collide
as an add/add conflict, which is loud. Under pathspec checkout, which is what integration actually
uses, the second checkout silently overwrites the first, and validation cannot detect it: the
resulting tree holds one well-formed file. The draft protocol is the protection, and git and the
tool are only partial backstops.

## 7. Determinism and freshness

**R32.** `ledger.html` **MUST** be a pure function of the records, `ledger.json` and the renderer version.

**R33.** The rendered page **MUST NOT** contain a wall-clock timestamp. Any "last updated" shown **MUST**
derive from the dates in the records.

**R34.** `check` **MUST** verify freshness by rendering to memory and comparing bytes against the
committed page, and **MUST** do so only when the page was written by this renderer.

**R35.** Every derivation over the record set **MUST** be computed by the tool at render time and embedded
in the page. The browser **MAY** compute only what depends on the reader's clock or history, such as
staleness ages and new-since-last-visit badges.

Determinism is what makes R34 possible, and it makes a merge conflict in `ledger.html` a non-event:
the file is never hand-merged, only rendered again. R35 keeps one implementation of each
derivation, so a number in the page cannot disagree with the same number computed elsewhere.

The embedded derivations are the activity events, the daily found-and-resolved buckets, and each
record's last-activity date. Alongside them sit the queue-predates signal, the count of drafts
awaiting IDs, the unqueued criticals, and the set of terminal statuses.

## 8. Versioning and the pin

**R36.** `ledger.json` **SHOULD** carry `toolVersion`, the renderer version that last rendered the page.

**R37.** The renderer version **MUST** change whenever rendered output can differ between builds, whether
the cause is render code or an embedded viewer asset.

**R38.** Where the recorded version differs from this binary's, `check` **MUST** report it as a warning,
**MUST NOT** compare bytes, and **MUST NOT** fail on that account.

**R39.** `render` **MUST** write the page, record this binary's renderer as `toolVersion`, and refresh the
generated part of a materialized `GUIDE.md`. Where the guide or the router is absent, `render`
**MUST** write it, so a ledger scaffolded by an older binary picks it up.

**R40.** `render --assets` **MUST** write the page alone, leaving `toolVersion` and the documents as they
were, because a dev-stamped page certifies nothing.

**R41.** A ledger with no recorded version **MUST** be accepted, because R36 makes `toolVersion` optional.

`toolVersion` describes rather than gates. It says which renderer wrote the committed page, which
is what R38 needs to tell version skew from a stale page. Two people on different builds will
rewrite `ledger.html` past each other, and the cost of that is a churning diff rather than a
blocked commit. Neither of them is ever stopped, and `render` always resolves it.

R37 is what makes the recorded version mean anything. Skip a bump and two binaries reporting the
same version render different bytes. `check` then reports the divergence as a stale page rather
than version skew, which sends the reader after the wrong problem.

## 9. The tool

**R42.** `cs-ledger` **MUST** be a single static binary with no runtime dependencies.

**R43.** The viewer assets, the man page and the operating guide **MUST** be embedded in the binary, so a
target repository needs no vendored files.

**R44.** `render` **MUST** write the page even when records fail validation, and **MUST** report the failures.
`check` fails on them separately.

**R45.** `render --assets DIR` **MUST** load viewer assets from disk and **MUST** mark its output as
dev-stamped.

**R46.** `check` **MUST** reject a dev-stamped page, so only a release binary's render is committable.

**R47.** `check` **MUST** reject a materialized `GUIDE.md` whose generated part differs from the binary's
copy, and **MUST NOT** compare what follows the project marker.

**R48.** `init` **MUST** refuse to overwrite an existing `ledger.json`.

**R49.** Exit status **MUST** be 0 on success, 1 on a failed check or an error, and 2 on a usage error.

**R50.** `init` and `render` **MUST** materialize the guide as `LEDGERDIR/GUIDE.md` and a router as
`LEDGERDIR/AGENTS.md`.

A ledger nobody is told about is a ledger nobody keeps, and R50 is the whole of discovery. Agent
harnesses read the `AGENTS.md` nearest the file being edited, so a router beside the records is
what an agent that opens one will find. Putting it there rather than in the repository's own
`AGENTS.md` keeps R3 intact: the tool writes inside the ledger directory and nowhere else.

## 10. The viewer

The rendered page is one column at every viewport. Records render as cards, and details open in a
shared modal that link chips navigate between. A `feature-idea` card is muted and dashed, so a
roadmap direction never reads as defect backlog.

**R51.** The page **MUST** inline all CSS, JavaScript and record data, and **MUST** make no external request.

**R52.** The page **MUST** offer a light and a dark theme.

**R53.** Styling **MUST** go through the pinned design tokens rather than through a component framework or
a build step.

**R54.** A view **MUST** be reachable by a `?view=` parameter, and a record by a `#<id>` fragment.

The full `@codesweep-ai/ui` package is React and Tailwind behind a private registry, which a
no-build artifact cannot use. So the project keeps a pinned copy of that design system's
`tokens.css` and `base.css` under `ui/codesweep-ui/`, adapts it in `viewer/`, and embeds the
result. The token version appears in the page footer next to the renderer version, and moving it is
a release followed by `cs-ledger render`.

Four views are defined. **brief** is the landing view: the masthead, a today-anchored trend of
daily found and resolved counts, the top of the queue with its rationales, and a derived block of
what needs a human. **board** stacks records by status in collapsible sections. **list** is the
full inventory, grouped and sorted on demand. **activity** is every dated event, newest first.

Controls are chips whose face shows the current selection, and they occupy a fixed footprint in
every state so the page never reflows as you filter. A filter that is active but hidden is
impossible by construction, because every control shows its own value.

Markdown rendering is a small built-in subset: headings, emphasis, code, lists, links and tables.
No library is vendored for it.

## 11. Testing

The Go suite covers validation, rendering and the command-line surface, each rule against its own
synthetic corpus. Two real corpora act as scale fixtures: this repository's own ledger, and
`fixtures/sandbox`, a frozen copy of the sandbox project's. A renderer change has to leave both
round-tripping through the freshness check.

### Coverage

Every test target writes coverage into its own tier under `.coverage/`, so running several
aggregates rather than overwrites. `make coverage` merges what is there and prints the report.

`make coverage-check` runs inside `make check` and in CI. It fails when a package
`.coverage-baseline` lists stops being reached: presence, not a percentage. What it catches is a
suite that stopped running while the tests still report green. When a package is meant to lose its
coverage, rerun `make coverage-baseline` and commit the result.

The CLI tests build `cs-ledger` with `-cover`, so what the real binary runs counts too.

## 12. Conformance

An implementation conforms when it satisfies R1–R54. The tool's own test suite is the reference. It
validates a corpus of well-formed and malformed records, renders both this repository's ledger and
the `fixtures/sandbox` corpus, and asserts the freshness comparison.

This repository self-hosts. Its own ledger lives at `ledger/` and is checked by the same binary, as
a standing gate in the suite. `schema/issue.v1.json` documents §4 for readers and agents; the
binary enforces §4 natively, and the suite pins the two to the same verdicts so they cannot drift
apart.

## 13. Non-goals

- **No record types beyond issues.** Decisions, stints and specifications stay in Markdown.
- **No migration tooling.** The tool reads no other issue format. `fixtures/sandbox` is a test
  corpus rather than a conversion path.
- **No multi-repository view.** The tool reads one ledger directory.
- **No harness integration.** `stint` is a string, and it is the only link to whatever selected the
  work. The tool never reads a harness's run directories.
- **No server and no build step**, ever, for the rendered page.

## 14. Open questions

1. **Drafts have no promotion command.** An orchestrator moves a draft to `issues/<ID>.json` by
   hand. Whether that deserves a verb is open.
2. **`schemaVersion` is fixed at `issue.v1`.** What a v2 would change, and whether the tool would
   read both, has not been decided.
