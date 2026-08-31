# ledger

> **Structured issue tracking for agent-managed repositories: AI agents write small JSON records,
> humans read a generated, interactive, self-contained `ledger.html`.**

[![CI](https://github.com/codesweep-ai/ledger/actions/workflows/ci.yml/badge.svg)](https://github.com/codesweep-ai/ledger/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
![Platforms](https://img.shields.io/badge/platform-Linux%20%C2%B7%20macOS-lightgrey)

A **ledger** is a directory of small JSON files, committed next to the code, holding what is wrong
with the repository and what to do next. There is no service to run. A single static Go binary,
`cs-ledger`, validates the records and renders the page, and you commit both.

```
┌──────────────┐          ┌───────────────────┐         ┌──────────────┐
│  agents      │  write   │  ledger/          │ render  │ ledger.html  │
│              ├─────────►│  issues/*.json    ├────────►│              │
│  file, note, │          │  queue.json       │  check  │  one file,   │
│  close       │          │  ledger.json      │◄────────┤  no server   │
└──────────────┘          └───────────────────┘         └──────────────┘
                              all committed
```

## Look at one first

No build and no server. Open either of these in a browser:

- [This repository's own ledger](https://codesweep.ai/ledger/ledger/ledger.html), tracking its own
  work ([source](ledger/ledger.html)).
- [A frozen copy of another project's ledger](https://codesweep.ai/ledger/fixtures/sandbox/ledger/ledger.html),
  kept as a test corpus ([source](fixtures/sandbox/ledger/ledger.html)).

## Why a ledger

An agent that finds a bug halfway through a task has nowhere good to put it. A comment gets lost, a
`TODO` never gets read, and an issue tracker needs credentials the agent does not have. The finding
gets mentioned once in a summary nobody keeps, and then it is gone. A record in the ledger outlives
the session that wrote it, and the next agent starts from it.

## Quickstart

Build the tool from a clone and put a ledger in a repository that has none.
[INSTALL.md](INSTALL.md) has the other ways to get it:

```bash
git clone https://github.com/codesweep-ai/ledger && cd ledger
make install                                        # ~/.local/bin/cs-ledger
cd ~/my-service
cs-ledger init ledger --project my-service --prefix MYS
```

That writes the config, an empty `issues/`, a copy of the guide, a router beside it, and a first
render. An agent that opens a record finds the router and follows it to the guide. To point one
there from the start, a line in `AGENTS.md`, `CLAUDE.md` or a role brief carries the same weight:

> This repo keeps a ledger. Read `ledger/GUIDE.md` and follow it: file records before building,
> close only with verified evidence, keep the queue current, and run `cs-ledger check` green before
> every commit.

From then on, agents write records and run two commands before each commit. Here they are against
this repository's own ledger:

```console
$ cs-ledger render && cs-ledger check
wrote ledger/ledger.html (… bytes, 10 issues, 0 drafts)
check OK: 10 issues, 0 drafts, 0 warning(s), ledger.html fresh
```

If the page no longer matches the records, `check` says so and exits non-zero, so a stale page
cannot reach a commit:

```console
$ cs-ledger check
validation errors (1):
  - ledger.html: STALE — records changed without re-render. Run: cs-ledger render
check FAILED: 1 error(s), 0 warning(s)
```

## What a record looks like

One file per issue, named after its ID:

```json
{
  "id": "MYS-004",
  "title": "Token refresh races on concurrent requests",
  "type": "defect",
  "severity": "high",
  "status": "closed",
  "foundBy": "session agent, while reading auth/refresh.go",
  "opened": "2026-08-11",
  "resolved": "2026-08-13",
  "stint": "rev-5",
  "evidence": {
    "commits": ["9f2c1ab"],
    "integrated": [],
    "verified": "100 concurrent refreshes, 1 token minted (was 7)"
  },
  "resolution": null,
  "details": "Two goroutines that both find an expired token each mint a new one...",
  "notes": [{ "date": "2026-08-12", "text": "Reproduced with a 100-way race." }],
  "links": []
}
```

The rules are few and they are the point. A `closed` record has to say how the fix was proved and
which commit did it. `check` resolves that sha against the repository, so an invented one fails the
gate. Notes append and never rewrite history. IDs are never renumbered or reused.
[SPEC.md](SPEC.md) states all of them.

## What the page shows

`ledger.html` inlines its own data, CSS and JavaScript, so it works from `file://` with no network
request. Four views share one masthead:

- **brief**: the landing view. What changed lately, what the queue says to fix next and why, and
  what needs a human right now.
- **board**: records stacked by status.
- **list**: the full inventory, grouped and sorted on demand.
- **activity**: every dated event, newest first.

The page is a pure function of the records, with no timestamp of its own. That is what lets `check`
prove it is current, by rendering again and comparing bytes. It also makes a merge conflict in
`ledger.html` a non-event: you never hand-merge that file, you render it again.

## Working on ledger itself

```bash
make build                                    # bin/cs-ledger
make check                                    # gofmt, vet, Go suite, viewer suite, docs
bin/cs-ledger check ledger                    # this repo's own ledger
bin/cs-ledger check fixtures/sandbox/ledger
bin/cs-ledger render ledger --assets ./viewer # iterate on the viewer, no recompile
```

`viewer/` holds the viewer's real CSS and JavaScript, embedded into the binary at build time.
`--assets` reads them from disk instead, which is how you iterate on the design. Those renders are
dev-stamped and `check` refuses them, so only a release binary's output is committable.

`ui/codesweep-ui/` is a pinned copy of the design system's tokens, and `viewer/` carries the
adaptation the no-build page ships. The viewer is styled through those tokens alone, with no React,
no Tailwind and no build step.

## Docs

- [INSTALL.md](INSTALL.md) · getting the binary, and scaffolding a ledger with it
- [GUIDE.md](GUIDE.md) · how to keep a ledger: the moves, the evidence rules, the queue
- [MANUAL.md](MANUAL.md) · every verb, flag, file and diagnostic
- [SPEC.md](SPEC.md) · the record schema, the rules, and why they are the rules
- [CONTRIBUTING.md](CONTRIBUTING.md) · working on the tool

## License

[Apache-2.0](LICENSE).
