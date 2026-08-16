# Fixture corpus — the sandbox project's ledger

> **FROZEN SNAPSHOT.** Copied from
> [codesweep-ai/sandbox](https://github.com/codesweep-ai/sandbox) at `81e988a` on 2026-08-16.
> That repository keeps its ledger live, so the two diverge from this date by
> design. This copy is never synced forward.

Fourteen records a real project filed against itself: defects found while
reading its own source, an install path that points at an empty releases page,
and three feature ideas. Two are closed and carry the commit that resolved them,
one is in progress with a stint, and three carry dated notes — so the corpus
exercises the whole lifecycle rather than the open end of it.

## Why it is here

`make check` renders this corpus and compares the result with the committed
`ledger.html`, byte for byte. It is a scale test rather than a rule test: the
lifecycle rules each have their own synthetic corpus in
[`internal/ledger`](../../internal/ledger), and what this one proves is that
the renderer and the freshness gate hold on a ledger somebody actually keeps.

Nothing here is edited. A record that reads oddly reads that way upstream, and
correcting it would make the corpus a fiction rather than a sample.
