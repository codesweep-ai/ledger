# Contributing to cs-ledger

Bug reports and pull requests are welcome. For a security issue, use GitHub's private
vulnerability reporting on this repository's Security tab, rather than opening a public issue.

## What this project will not trade away

Two properties hold the design together, and a change that weakens either needs a very good reason.

**The rendered page is a pure function of the records.** It reads no wall-clock timestamp, no
environment and no network. That is what lets `check` prove the page is current by rendering again
and comparing bytes, and it is why a conflict in `ledger.html` is never hand-merged.
`TestRenderDeterministic` renders one corpus twice and compares, and
`TestRenderEmbedsDataNoTimestamps` holds the second half.

**A closed record proves its claim.** `evidence.verified` says what was measured, and
`evidence.commits` says which commit did it. A ledger whose closed records assert rather than prove
is worth less than no ledger. `TestClosedRequiresVerified` and `TestClosedRequiresCommitsOrLinks`
reject a record that closes without either.

## Getting set up

```bash
make build     # bin/cs-ledger
make check     # the full local gate
```

`make check` runs the gofmt check, `go vet`, `golangci-lint`, `deadcode`, the Go suite, the
viewer's JavaScript suite, and the three linters `cs-lint` carries. Run it before pushing. CI runs
that target and one gate on top: the freshness check on both ledger corpora.

Three of those do not come with Go. Install them once, pinning `golangci-lint` to the version CI
runs, so a newer release's added checks arrive when you move the pin rather than on an unrelated
pull request:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
go install golang.org/x/tools/cmd/deadcode@latest
go install github.com/codesweep-ai/lint/cmd/cs-lint@latest
```

`cs-lint` is deliberately not pinned. It is this family's own linter, and CI installs it from source
the same way, so a check it gains reaches you on the day it lands.

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

## Tests are part of the change

Every behavior change ships with test coverage. A change with no test is only acceptable when the
behavior genuinely cannot be observed in a test. Say so in the PR.

The Go suite covers validation, rendering and the command-line surface, each rule against its own
synthetic corpus. Two real corpora act as scale fixtures: this repository's own ledger, and
`fixtures/sandbox`, a frozen copy of the sandbox project's.

A change to validation needs a case on both sides: a record that should pass and one that should
fail, each asserting the message a reader would act on. A change to the renderer needs the two
corpora to still round-trip through the freshness check.

`schema/issue.v1.json` documents the record shape for readers and agents, while the binary enforces
it natively. The suite pins the two to the same verdicts. If you change one, change the other in
the same commit.

### Coverage

Every test target writes coverage into its own tier under `.coverage/`, so running several
aggregates rather than overwrites. `make coverage` merges what is there and prints the report.

`make coverage-check` runs inside `make check` and in CI. It fails when a package
`.coverage-baseline` lists stops being reached: presence, not a percentage. What it catches is a
suite that stopped running while the tests still report green. When a package is meant to lose its
coverage, rerun `make coverage-baseline` and commit the result.

The CLI tests build `cs-ledger` with `-cover`, so what the real binary runs counts too.

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
back. [`cs-lint`](https://github.com/codesweep-ai/lint) enforces the mechanical ones. It carries
three linters, and `make check` runs all three:

| Command | Target | What it checks |
|---|---|---|
| `cs-lint docs` | `make docs` | How the documents are written. |
| `cs-lint oss` | `make oss` | What this repository owes a reader as a published project. |
| `cs-lint walkthrough` | `make walkthrough` | Whether the documents still describe the software. |

The third checks the claims rather than the prose. Every command the docs name goes against the
binary's help tree, every setting against the code that reads it, and every sample output against
the command re-run now. `--run` lists every command the documents tell a reader to run, in reading
order, and `--review` prints the half that needs a reader.

Read what a rule wants with `--explain`, which prints the guidance behind each one rather than
leaving you to argue with the tool:

```bash
cs-lint oss --explain
```

1. **Write to the reader, in second person.** "Run `cs-ledger check` before committing", not "the
   check should be run before committing".

2. **Introduce a term where you first use it.** A reader meeting *draft*, *stint* or *queue* for
   the first time needs it introduced. Give a definition on the spot, an entry in a glossary
   table, or a link to the page that defines it. The linter carries the project's vocabulary. It
   reports a term used before anything explains it.

3. **No em-dash.** The aside one introduces is a full stop, a comma, or a cut. It is also the
   first punctuation a model reaches for, so a page full of them reads as unedited whoever wrote it.

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

14. **Describe what the software does, not how it came to do it.** Leave out what the project used
    to do, what was tried and dropped, and numbers from a run someone did once. The reason a rule
    exists belongs beside the rule in `SPEC.md`; the investigation that found it belongs in the PR.

15. **State the point first, then qualify it.** Opening with the qualifier makes the reader decode
    the sentence backwards. "Appended and never rewritten, so an interrupted run leaves the records
    intact" names its subject last. Start with the file, and let the consequence follow it.

16. **Do not explain a design by contrast with a worse one.** "A directory, so a change reads as a
    diff rather than as one unreadable line" asks the reader to picture a format nobody proposed.
    Say what it is and what you get.

17. **A walkthrough is steps that work.** Put the reasons somewhere else. A reader working through
    one wants commands that run, not an account of which field the schema used to spell differently.

18. **Do not make the reader hold two halves of a sentence apart.** "What the records say may
    change; what the page shows may not" is a puzzle. Name the subject in each clause.

19. **Do not write in the register a model defaults to.** Untouched model output has a signature
    readers now recognise and discount. `cs-lint docs --explain` lists the words this house
    declines and what to write instead, so the table lives in one place rather than here. Two
    shapes matter as much as the words. Negative parallelism sets up a contrast nobody asked for.
    The rule of three is a rhythm rather than an argument, and a reader stops counting the third
    item as information.

These rules are about mechanics, and this project's voice is a strength: concrete, opinionated, and
free of padding. Where a rule fights the voice, the voice wins. Say so in the PR when it does.

Run the linter on its own while you write:

```bash
cs-lint docs              # check
cs-lint docs --stats      # per-file measurements
cs-lint docs --list       # which files are checked
cs-lint docs --explain    # what each rule wants, and the guidance behind it
```

Every knob lives in [`.cs-lint.yaml`](.cs-lint.yaml) at the repository root, one section per
linter. The `docs` section carries `glossary`, `skipExtra`, `lowercaseStarters` and `projectVerbs`.
When a real sentence trips the verb check, add the verb. When a report is noise, fix the config. A
linter that cries wolf gets ignored, and then it protects nothing.

A rule turned off for this repository is a waiver: a rule identifier and the reason it was traded
away, under `allow`. The reason is required, and it is printed with the finding, because a waiver
nobody can review is a rule deleted in private.

The linter is a project of its own, shared across this family. A fix to a check belongs there, and
reaches this repository the next time somebody installs it.

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
