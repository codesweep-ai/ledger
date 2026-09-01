# Contributing to cs-ledger

Bug reports and pull requests are welcome. These rules apply to humans and coding agents alike. If
you are an agent working in this repository, read this file before you change anything and follow
it.

For a security issue, use GitHub's private vulnerability reporting on this repository's Security
tab, rather than opening a public issue.

## Submitting a change

File a bug or an idea as a GitHub issue on this repository. For a fix that stands on its own, a pull
request on its own is enough. For anything that changes behaviour a user can see, open an issue
first, so the design gets settled before you write it.

1. Fork the repository, and create a branch off `main`.
2. Make the change, with its test.
3. Run `make ci`, which is every gate CI runs.
4. Open a pull request against `main`, and say what the change does and why.

Expect comments rather than silence, and expect a small change to move quickly. A reviewer asks
whether the change keeps the design rules below, whether a test fails without it, and where a reader
would find it documented.

By opening a pull request you agree that your contribution ships under the
[Apache 2.0 licence](LICENSE) this project is released under.

## Design rules

Two properties hold the design together. A change that weakens either needs a very good reason, and
each names the test that holds it.

**The rendered page is a pure function of the records.** It reads no wall-clock timestamp, no
environment and no network. That is what lets `check` prove the page is current by rendering again
and comparing bytes, and it is why a conflict in `ledger.html` is never hand-merged.
`TestRenderDeterministic` renders one corpus twice and compares, and
`TestRenderEmbedsDataNoTimestamps` holds the second half.

**A closed record proves its claim.** `evidence.verified` says what was measured, and
`evidence.commits` says which commit did it. A ledger whose closed records assert rather than prove
is worth less than no ledger. `TestClosedRequiresVerified` and `TestClosedRequiresCommitsOrLinks`
reject a record that closes without either, and `TestEvidenceShasResolveAgainstTheRepository`
rejects one whose sha names no commit, which is what an invented citation looks like.

## Before you push

One command:

```bash
make ci
```

That is every gate the CI workflow has, on this machine and in the order the workflow takes them,
so a green run here is a green run there. `make check` is the faster subset to keep beside you
while you work, and `make ci` is the one that has to pass.

No linter needs installing. Every one the gates shell out to is pinned and built from the module
cache on first use: `golangci-lint`, `deadcode`, `actionlint` and `cs-lint`. `make repin` moves the
`cs-lint` pin to the branch tip, and `make versions` says which builds the gates used.

Moving a linter pin is an edit to `go.mod`, or to `go.golangci.mod` for `golangci-lint`. A linter
release reaches you when you ask for it, not on an unrelated pull request.

Two programs are expected on the PATH, and only `make ci` requires them. `goreleaser` validates
the release manifest, and `make build` falls back to `go build` where it is absent. Rebuilding the
viewer needs npm, and `make ci` fails without it rather than pass on a bundle it could not check.
`make check` needs neither.

The viewer is a React application under `viewer/app/`, built against the pinned
`@codesweep-ai/ui` package into the single self-contained `viewer/index.html` the Go binary embeds.
That file is committed, so building the binary needs Go alone. Changing the viewer needs Node
22.13 or newer with npm, which is the floor `@codesweep-ai/ui` sets. The viewer's `package.json`
sits beside its source in `viewer/`, so npm runs there, and `make` does that for you:

```bash
make viewer-build
```

`make build` rebuilds the viewer on its own when its sources have moved, so this is the explicit
form rather than an extra step. Re-render the page with `bin/cs-ledger render ledger` afterwards:
the viewer's bytes are part of what `toolVersion` pins.

This repository keeps a **ledger** of open issues in `ledger/`. Read
[`ledger/AGENTS.md`](ledger/AGENTS.md) before you start work, and follow it as you go. A commit
that touches `ledger/` needs `cs-ledger render && cs-ledger check` to pass first, and
`make ledger` runs the check half.

## Tests

Ship a test with your change. Where a behaviour genuinely cannot be observed in a test, say so in
the pull request.

A change to validation needs a case on both sides: a record that should pass and one that should
fail, each asserting the message a reader would act on. A change to the renderer needs both real
corpora to still round-trip through the freshness check.

`schema/issue.v1.json` documents the record shape for readers and agents, while the binary enforces
it natively. If you change one, change the other in the same commit.

Test the contract, not the implementation: the exit code, the message a reader acts on, and the
bytes the page renders to. Say why the case matters in a comment when it is not obvious.

Never lower a coverage baseline to make a run green. [`SPEC.md`](SPEC.md#11-testing) holds what the
suite covers, the two real corpora it runs against, and how coverage is measured and gated.

## Commits

**Keep it short.** One idea per commit, and a message a reader takes in at a glance. If a change
will not fit one idea, split it.

**Subject**, always. Under 60 characters, imperative, no trailing period, completing *"If applied,
this commit will …"*. Say what the change does, in plain English rather than in this project's
internal shorthand. Use no category label: `fix(proxy):`, `bugfix:` and `[docs]` each name a class
of change rather than the change itself, which the diff already shows. The gate fails on one, so
amend before you push.

**Body**, rarely. Most commits need none. Add one only when the subject leaves a question a reader
would otherwise have to open the diff to answer, and then answer that question. A sentence or two
does it. Wrap it at 72 columns.

Leave out how the work was scheduled, how you tested it, and what led you to it, and stop once the
question is answered. A second paragraph usually means the message has turned into a report of the
session. A rule's reason belongs beside the rule in [`SPEC.md`](SPEC.md), and the investigation that
found it belongs in the pull request.

```
Reject a draft that carries an id
```

```
Re-render the page when the renderer version moves

A ledger records the renderer that wrote its page, so two
binaries claiming one version would render different bytes.
```

Keep the `Co-Authored-By:` trailer when an agent wrote the change. Drop any trailer linking to the
agent's session or transcript. Such a link is private to whoever ran it and dead to everyone else,
and it cannot be fixed after publication.

## Writing

Six principles do most of the work. Read them before you write a document, and apply them when you
edit one:

1. **Introduce a term where you first use it**, in the same sentence, or link to the page that
   defines it. A reader should never meet a word the docs have not explained.
2. **State the point first, then qualify it.** Opening with the qualifier makes the reader decode
   the sentence backwards.
3. **Give every sentence a subject and a verb.** "Two version numbers, one verdict, one remedy"
   reads as knowing rather than clear. Say what the thing is.
4. **A how-to is steps that work.** Put the reasons somewhere else. A reader working through
   one wants commands that run.
5. **Describe what the software does, not how it came to do it.** Leave out what the project used
   to do, what was tried and dropped, and numbers from a run somebody did once.
6. **Do not explain a design by contrast with a worse one.** Say what it is and what you get,
   rather than asking the reader to picture a design nobody proposed.

The mechanical rules are enforced rather than restated here.
[`cs-lint`](https://github.com/codesweep-ai/lint) carries them, and `make check` runs it over this
repository. To read what a rule wants and the guidance behind it:

```bash
cs-lint prose --explain
```

That listing is the authority. Where this section and the linter disagree, the linter is right.
Turning a check off is a waiver: write it under `allow` in [`.cs-lint.yaml`](.cs-lint.yaml) with the
reason, which is printed with the finding.

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
- You ran `make ci` and it passed.
- You cut what the tool added to fill space. A model pads a commit body to the shape it was shown,
  and comments that restate the code around them. Both read as noise to a maintainer, and both are
  yours to remove.

Keep the `Co-Authored-By:` trailer, which is how the work is disclosed. An unattended agent must not
open pull requests or comment on this repository.
