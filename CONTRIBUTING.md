# Contributing to ledger

Bug reports and pull requests are welcome. For a security issue, use GitHub's private
vulnerability reporting on this repository's Security tab, rather than opening a public issue.

## What this project will not trade away

Two properties hold the design together, and a change that weakens either needs a very good reason.

**The rendered page is a pure function of the records.** It reads no wall-clock timestamp, no
environment and no network. That is what lets `check` prove the page is current by rendering again
and comparing bytes, and it is why a conflict in `ledger.html` is never hand-merged.

**A closed record proves its claim.** `evidence.verified` says what was measured, and
`evidence.commits` says which commit did it. A ledger whose closed records assert rather than prove
is worth less than no ledger.

## Getting set up

```bash
make build     # bin/cs-ledger
make check     # the full local gate
```

`make check` runs the gofmt check, `go vet`, `golangci-lint`, `deadcode`, the Go suite, the
viewer's JavaScript suite, the prose linter and the publication linter. Run it before pushing. CI
runs that target and one gate on top: the freshness check on both ledger corpora.

Two of those do not come with Go. Install them once, pinning `golangci-lint` to the version CI
runs, so a newer release's added checks arrive when you move the pin rather than on an unrelated
pull request:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
go install golang.org/x/tools/cmd/deadcode@latest
```

This repo keeps a **ledger** of open issues in `ledger/`. Read
[`ledger/AGENTS.md`](ledger/AGENTS.md) before you start work, and follow it as you go. A commit
that touches `ledger/` needs `cs-ledger render && cs-ledger check` to pass first.

The viewer's assets live in `viewer/` as real files and are embedded into the binary at build time.
To iterate on them without recompiling:

```bash
bin/cs-ledger render ledger --assets ./viewer
```

Those renders are dev-stamped and `check` refuses them, so a page built this way cannot reach a
commit by accident.

## Testing

The Go suite covers validation, rendering and the command-line surface, each rule against its own
synthetic corpus. Two real corpora act as scale fixtures: this repository's own ledger, and
`fixtures/sandbox`, a frozen copy of the sandbox project's.

A change to validation needs a case on both sides: a record that should pass and one that should
fail, each asserting the message a reader would act on. A change to the renderer needs the two
corpora to still round-trip through the freshness check.

`schema/issue.v1.json` documents the record shape for readers and agents, while the binary enforces
it natively. The suite pins the two to the same verdicts. If you change one, change the other in
the same commit.

## Commits

Keep one idea per commit. A commit that fixes a bug and tidies neighbouring code is two commits.

Write the subject in the imperative and keep it under 60 characters: "Reject a draft that carries
an id", not "Fixed draft validation". Most commits need no body. Write one when the reason is not
visible in the diff, and say why rather than what.

Drop any trailer linking to the agent's session or transcript. A reader of the history has no
access to it, and a link that resolves for nobody is noise.

A change that touches the ledger directory needs `cs-ledger render && cs-ledger check` to pass in
the same commit, because the freshness gate compares the committed page against the records beside
it.

## Writing

The docs drift into a style that reads as terse and knowing rather than clear. These rules push
back. `scripts/lint-docs.py` enforces the mechanical ones, and `make check` runs it.
`scripts/lint-oss.py` is its sibling, and `make oss` runs it. It checks what this repository has to
satisfy as a published project, and `--explain` lists every rule it applies. Its knobs live beside
it in `scripts/lint-oss.config.py`.

1. **Write to the reader, in second person.** "Run `cs-ledger check` before committing", not "the
   check should be run before committing".

2. **Introduce a term where you first use it.** A reader meeting *draft*, *stint* or *queue* for
   the first time needs it introduced. Give a definition on the spot, an entry in a glossary
   table, or a link to the page that defines it. The linter carries the project's vocabulary. It
   reports a term used before anything explains it.

3. **One em-dash per paragraph at most.** Two or three read as a writer who will not commit to a
   sentence. A colon or a full stop nearly always works better.

4. **Sentences under 30 words.** Longer than that and a sentence is carrying two ideas.

5. **Every sentence has a verb.** "Two version numbers, one verdict" is an epigram, not a
   sentence. It sounds knowing and tells the reader nothing.

6. **Delete the frame.** "It is worth noting that", "put simply", "in other words", "to be clear".
   Each one comments on the writing instead of getting on with it. Say the thing.

7. **Do not say a thing twice in one sentence.** "Which command you type is the difference, and it
   is the difference that decides the outcome" circles and lands nowhere.

8. **Show a file before running it.** A block that runs `./build.sh` has to have shown the reader
   what is in `build.sh`.

9. **Explain the case, or leave it out.** If a walkthrough has two shapes, walk through both fully,
   or pick one. Half an explanation, hedged, is worse than either.

10. **Do not mention what does not happen.** "The `--assets` flag is ignored here" makes a reader
    wonder why they were told. Cut it.

11. **Do not document the absence of a feature** as a section of its own. A "What it does not do"
    list mostly answers questions nobody asked. Non-goals belong in the spec, where a reader is
    looking for the boundary.

12. **Prefer a concrete example to a general statement.** A record with real field values teaches
    the schema faster than a paragraph about it.

13. **Say what it costs.** If a flag makes output uncommittable, or a rule blocks a merge, say so
    where the reader meets it.

Run the linter on its own while you write:

```bash
python3 scripts/lint-docs.py            # check
python3 scripts/lint-docs.py --stats    # per-file measurements
python3 scripts/lint-docs.py --list     # which files are checked
```

The knobs live beside it in `scripts/lint-docs.config.py`: `GLOSSARY`, `SKIP_EXTRA`,
`LOWERCASE_STARTERS` and `PROJECT_VERBS`. When a real sentence trips the verb check, add the verb.
When a report is noise, fix the check. A linter that cries wolf gets ignored, and then it protects
nothing.

The linter itself is vendored and stays byte-identical across projects. A fix to a check belongs in
the shared copy, and comes back here the next time it is copied out.

## Which document does a change belong in?

| If you are writing | It goes in |
|---|---|
| Why someone would want a ledger, and the first five minutes | `README.md` |
| How to get the binary and scaffold a ledger with it | `INSTALL.md` |
| What a verb does, what a flag means, what an error means | `MANUAL.md` |
| A rule the tool enforces, or the reason a rule exists | `SPEC.md` |
| What an agent should do while working in a ledger-keeping repo | `GUIDE.md` |
| Where an agent working in this repository looks first | `AGENTS.md` |

`GUIDE.md` is embedded in the binary and materialized into target repositories by `init` and
`render`. Changing it changes what every ledger-keeping repository ships, and `check` compares the
materialized copy against the binary's, down to the `<!-- LEDGER:PROJECT -->` marker. Rebuild and
run `cs-ledger render ledger` in the same commit as an edit to it. `MANUAL.md` is embedded too,
for `cs-ledger manual`, but no repository carries a copy.

`AGENTS.md` ships with nothing. It is the filename agent harnesses discover on their own, so it
routes an agent to the documents above and holds no knowledge that could go stale against them.

## Versioning

`RendererVersion` in `internal/ledger/render.go` is the number a ledger records. Bump it when the
rendered output changes for the same records. Every ledger on the old number then gets a warning
from `check` naming the renderer that wrote its page, and `cs-ledger render` brings it across.

Say what it costs: skip the bump and two binaries claim the same version while rendering different
bytes. `check` reports that as a stale page rather than as version skew, and sends the reader after
the wrong problem. Nothing catches a forgotten bump for you.
