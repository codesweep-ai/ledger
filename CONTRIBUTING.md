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
`cs-lint` pin to the last commit its CI built, and leaves it where lint names none. `make versions`
says which builds the gates used.

Moving a linter pin is an edit to `go.mod`, or to `go.golangci.mod` for `golangci-lint`. A linter
release reaches you when you ask for it, not on an unrelated pull request.

Two programs are expected on the PATH, and only `make ci` requires them. `goreleaser` validates
the release manifest, and `make build` falls back to `go build` where it is absent. Rebuilding the
viewer needs npm, and `make ci` fails without it rather than pass on a bundle it could not check.
`make check` needs neither.

The viewer is a React application under `viewer/app/`, built against the pinned
`@codesweep-ai/ui` package into the single self-contained `viewer/index.html` the Go binary embeds.
That file is committed, so building the binary needs Go alone. Changing the viewer needs Node
22.13 or newer with npm, which is the floor `@codesweep-ai/ui` sets, so `node` and `npm` both on
your PATH. The viewer's `package.json`
sits beside its source in `viewer/`, so npm runs there, and `make` does that for you:

```bash
make viewer-build
```

The install runs through `scripts/with-npmrevs.sh`, which puts cs-npmrevs in front of npmjs.com.
`@codesweep-ai/ui` publishes an image of every build, and a version that has not been released
reaches npm only that way. The tool is pinned in `go.mod`, the images are public, and every other
package still comes from npmjs.com. It also serves cs-npmrevs's shared data directory on port 4875,
so a build packed on this machine (`make npm-pack` in npmrevs, lint or ledger,
`npm run registry:pack` in ui) installs without being pushed.

`make build` rebuilds the viewer on its own when its sources have moved, so this is the explicit
form rather than an extra step. Re-render the page with `bin/cs-ledger render ledger` afterwards:
the viewer's bytes are part of what `toolVersion` pins.

This repository keeps a **ledger** of open issues in `ledger/`. Read
[`ledger/AGENTS.md`](ledger/AGENTS.md) before you start work, and follow it as you go. A commit
that touches `ledger/` needs `cs-ledger render && cs-ledger check` to pass first, and
`make ledger` runs the check half.

A push to main that changes only `ledger/` builds nothing and publishes nothing to npm or as images.
`ci` does not run for it: the `ledger` workflow runs `make ledger`, `make prose`, `make refs` and
`make oss` instead, and the site republishes the ledger's page when it finishes. Such a commit is
never a build a sibling pins.

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
this commit will …"*. Say what the change does in plain English. The test: would this subject make
sense to someone who has not read the diff and does not know this codebase? Use no category label:
`fix(proxy):`, `bugfix:` and `[docs]` each name a class of change rather than the change itself,
which the diff already shows. The gate fails on one, so amend before you push.

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

## Publishing to npm

Every release also goes to npm as five packages: four carry the binary, one per
platform goreleaser builds, and the wrapper picks the right one at run time.
Only the wrapper is written by hand, under `npm/ledger/`. The other four are
generated from goreleaser's output, and nothing under `npm/dist/` is committed.

```bash
make npm-snapshot   # build every target, package it, and show what would publish
make npm-build      # package whatever dist/ already holds
make npm-pack       # package a dev build into cs-npmrevs's data directory
make npm-local      # the same, then serve it, and print how to install it
make npm-publish    # platform packages first, then the wrapper
```

Keep that order in `npm/publish.sh`. The wrapper depends on packages that must
already exist when it is published. Publish it first, and every install between
the two commands resolves a binary the registry does not have.

Running `npm/publish.sh` again is safe. It skips each package the registry
already has from this commit, and stops on one it has from another commit.

The `npm` workflow publishes each commit on main that passes `ci` to the `dev`
channel. It builds the commit `ci` tested, and skips it once main's head changes
more than `ledger/` after it. Every publish also writes an `npm` commit status
to its commit. In a fork, or a copy under another owner, it publishes nothing on its
own, because the packages there take that owner's scope. That owner runs it by
hand, once each package names it as a trusted publisher. A trusted publisher can
only be added to a package that exists, so the first publish runs
`npm/publish.sh` from a machine logged in to npm.

`make npm-local` is how to try a package before publishing it. It packs the five
packages into cs-npmrevs's default data directory and serves them with
[cs-npmrevs](https://github.com/codesweep-ai/npmrevs), which makes every
revision of an npm package installable without publishing it. It runs the
cs-npmrevs that `go.mod` pins, and takes every other package from npmjs.com. It
prints the install command, with the exact version it built. Run it after every
change: a rebuild of the same commit replaces the last run's tarballs.
`npm/local-registry.sh stop` stops the server. Every project's build shares
that directory, so `make npm-pack` alone leaves a build for a later one to
install through cs-npmrevs on port 4875. `make install` runs it too.

These variables belong to the packaging rather than to the tool, which is why
[`MANUAL.md`](MANUAL.md) does not carry them:

| Variable | Effect |
|---|---|
| `CS_LEDGER_BINARY` | The binary the npm wrapper runs, so the packaging can be tried against a local build. |
| `CS_LEDGER_NPM_VERSION` | The version the generated packages carry. A tagged release supplies its own. |
| `CS_LEDGER_NPM_TAG` | The channel a prerelease is published to, `next` unless it says otherwise. |
| `CS_NPMREVS_PORT` | The port `make npm-local` serves on, 4875 unless it says otherwise. |
| `CS_NPMREVS_IMAGES`, `CS_NPMREVS_SCOPE` | The images registry and the scope `make npm-local` serves beside the data directory, `ghcr.io` and `@codesweep-ai` unless they say otherwise. |
| `CS_NPMREVS_DATA` | The data directory `make npm-pack` and `make npm-local` pack into, cs-npmrevs's own default unless it says otherwise. |
| `NPMREVS` | The command `npm/local-registry.sh` and `npm/publish-images.sh` run as cs-npmrevs. `npm/local-registry.sh`, `make images-snapshot` and the workflow use the pinned one, and `npm/publish-images.sh` run by hand uses `cs-npmrevs` from the PATH. |
| `REGISTRY` | The registry `npm/publish-images.sh` publishes to, `ghcr.io` unless it says otherwise. |

### Images of the packages

The `publish images` workflow pushes each commit on main that passes `ci` as
five images, one per package, such as
`ghcr.io/codesweep-ai/npm/ledger:<version>`. It starts when `ci` finishes, and
pull requests get no image. The `prune-images` workflow keeps the newest 20
versions of each package. Those images let a team of AI coding agents install
builds that are not yet meant for people:
[cs-npmrevs](https://github.com/codesweep-ai/npmrevs) serves them to npm, or
copies them into a directory.

Each run posts a `publish images` commit status on the commit it published: a
success once the wrapper is pushed, a failure otherwise. GitHub lists the run
under main's head when `ci` finished, which can be a later commit. The CI status
file reads the registry instead, and lists a commit as built, one a sibling can
pin, once its wrapper's version is there.

`npm/publish-images.sh` builds each image with cs-npmrevs and pushes it with
podman. The workflow is what runs it, and `make images-snapshot` builds the
images of whatever `npm/dist/` holds, pushing nothing. The script is shared with
npmrevs, lint and ui, so change all four together.

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

## The viewer oracle

`make fixtures` drives the rendered viewer in a headless Chromium and compares what it measures
with `viewer/fixtures/expectations.json`. It renders two ledgers: this repository's own `ledger/`,
and the frozen copy in `fixtures/sandbox/ledger`.

The rows against the own ledger compute the record counts, ids, lanes and orders from its records.
Filing a record, or starting work on one, therefore leaves the oracle green. The controls and the
behaviour stay frozen. The model in `viewer/fixtures/derive.mjs` does the computing, and each own
row holds it to the sandbox row that measures the same thing. `make fixtures-model` runs that check
without a browser, and `make ci` runs it.

`make fixtures` itself stays out of `make ci`, because it needs a Chromium. Run it with `CHROME_BIN`
pointing at one when a change moves the viewer, the `@codesweep-ai/ui` pin or the renderer. A frozen
value moves only through `make record-fixtures` with `APPROVE` and `REASON`, and
[`viewer/fixtures/README.md`](viewer/fixtures/README.md) says what each row freezes. The approval
is per row: `FIXTURES_ARGS="--only LF-02,LF-03"` names the rows it covers, and the runner refuses a
change to any other, a `must-change` row's baseline included.

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
