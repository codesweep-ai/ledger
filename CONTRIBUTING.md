# Contributing to cs-ledger

Bug reports and pull requests are welcome. These rules apply to humans and coding agents alike. If
you are an agent working in this repository, read this file before you change anything and follow
it.

For a security issue, use GitHub's private vulnerability reporting on this repository's Security
tab, rather than opening a public issue.

## How a change gets in

File a bug or an idea as a GitHub issue on this repository. For a fix that stands on its own, a pull
request on its own is enough. For anything that changes behaviour a user can see, open an issue
first, so the design gets settled before you write it.

1. Fork the repository, and create a branch off `main`.
2. Make the change, with its test.
3. Run `make check`, which is the same gate CI runs.
4. Open a pull request against `main`, and say what the change does and why.

Review asks four questions. Does the change hold the invariants below? Does a test fail without it?
Does every user-visible change land in exactly one document? Does the history read the way this file
describes? Expect comments rather than silence, and expect a small change to move quickly.

By opening a pull request you agree that your contribution ships under the
[Apache 2.0 licence](LICENSE) this project is released under.

## What a change must not break

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

## Before you push

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

Keep one idea per commit. If a change will not fit that shape, it is doing more than one thing, so
split it.

**Subject**, always. Under 60 characters, imperative, no trailing period, completing *"If applied,
this commit will …"*. Say what the change does, in plain English rather than in this project's
internal shorthand. Use no conventional-commit prefix: `fix(proxy):` names a category rather than a
change, and the category is already in the diff.

**Body**, only when the subject leaves a real question a reader would otherwise have to open the
diff to answer. Write the answer in plain English, in whole sentences, addressed to somebody who was
not there. Wrap it at 72 columns. Most commits need no body at all.

Say what the change does and what constrained it. Leave out how the work was scheduled, how it was
tested, and what prompted it. A rule's reason belongs beside the rule in [`SPEC.md`](SPEC.md), and
the investigation that found it belongs in the pull request.

Where a body carries more than one independent point, one line each reads better than a paragraph.
Never reach for another point to fill the shape. A line that restates the subject in different words
is worse than no body, and a body written to a length is the commonest way a message stops being
read.

```
Reject a draft that carries an id
```

```
Re-render the page when the renderer version moves

A ledger records the renderer that wrote its page, so two
binaries claiming one version would render different bytes.
```

```
Key a record on its id, not its file name

- A file can be renamed; the id is what a closure cites.
- The freshness check compares bytes, so order is data.
```

Keep the `Co-Authored-By:` trailer when an agent wrote the change. Drop any trailer linking to the
agent's session or transcript. Such a link is private to whoever ran it and dead to everyone else,
and it cannot be fixed after publication.

## Writing

Six principles carry the voice. Read them before you write a document, and apply them when you edit
one:

1. **Introduce a term where you first use it**, in the same sentence, or link to the page that
   defines it. A reader should never meet a word the docs have not explained.
2. **State the point first, then qualify it.** Opening with the qualifier makes the reader decode
   the sentence backwards.
3. **Give every sentence a subject and a verb.** "Two version numbers, one verdict, one remedy"
   reads as knowing rather than clear. Say what the thing is.
4. **A walkthrough is steps that work.** Put the reasons somewhere else. A reader working through
   one wants commands that run.
5. **Describe what the software does, not how it came to do it.** Leave out what the project used
   to do, what was tried and dropped, and numbers from a run somebody did once.
6. **Do not explain a design by contrast with a worse one.** Say what it is and what you get,
   rather than asking the reader to picture a design nobody proposed.

The mechanical rules are enforced rather than restated here.
[`cs-lint`](https://github.com/codesweep-ai/lint) carries them, and `make check` runs all three of
its linters over this repository:

| Command | Target | What it checks |
|---|---|---|
| `cs-lint docs` | `make docs` | How the documents are written. |
| `cs-lint oss` | `make oss` | What this repository owes a reader as a published project. |
| `cs-lint walkthrough` | `make walkthrough` | Whether the documents still describe the software. |

`--explain` prints what each rule wants and the guidance behind it:

```bash
cs-lint docs --explain
```

That listing is the authority. Where this section and the linter disagree, the linter is right and
this section is a bug. Every knob lives in [`.cs-lint.yaml`](.cs-lint.yaml), and a check that
reports noise is a check to fix rather than a report to work around.

A check turned off here is a waiver, written under `allow` as an identifier and the reason it was
traded away. The reason is required, and it is printed with the finding, because a waiver nobody can
review is a rule deleted in private.

**What not to change.** This project's voice is a strength: concrete, opinionated, free of
marketing padding. These rules are about mechanics. Where one of them fights the voice, the voice
wins, and the exception is worth a sentence in the pull request.

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

## AI-assisted contributions

An agent wrote most of this repository, and you are welcome to use one. The standard is the same
either way: you are responsible for what you submit.

Point your tool at [`AGENTS.md`](AGENTS.md), which routes it to the documents that hold the
conventions, and check three things before you open the pull request:

- You understand every line, and can answer a question about it without going back to the tool.
- You ran `make check` and it passed.
- You cut what the tool added to fill space. A model pads a commit body to the shape it was shown,
  and comments that restate the code around them. Both read as noise to a maintainer, and both are
  yours to remove.

Keep the `Co-Authored-By:` trailer, which is how the work is disclosed. An unattended agent must not
open pull requests or comment on this repository.
