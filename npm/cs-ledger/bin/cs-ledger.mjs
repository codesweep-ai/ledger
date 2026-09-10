#!/usr/bin/env node
// The command `npx cs-ledger` runs. It finds the binary for this machine and
// becomes it: same arguments, same streams, same exit status.
//
// Nothing here interprets the run. cs-ledger separates three exits — 0 for a
// ledger that checks out, 1 for one that does not or a run that hit an error,
// 2 for a verb it does not have — and a gate reads them apart, so a launcher
// that collapsed them would report a broken run as a passing one. The status is
// passed through untouched.
//
// The launcher's own failures exit 2 rather than 1. 2 is the code cs-ledger
// gives a run that never reached a verb, and a launcher that cannot find its
// binary is exactly that: nothing was checked. Exiting 1 would be
// indistinguishable from a ledger that failed its check.

import { spawnSync } from "node:child_process";
import { constants } from "node:os";
import { binaryPath } from "../index.mjs";

// See above: the code for a run that never got as far as doing anything.
const NEVER_RAN = 2;

let bin;
try {
  bin = binaryPath();
} catch (err) {
  console.error(`cs-ledger: ${err.message}`);
  process.exit(NEVER_RAN);
}

const result = spawnSync(bin, process.argv.slice(2), {
  // cs-ledger writes its report for a person and reads nothing, so the streams
  // are the parent's. Piping them would buffer the report until the run ended
  // and drop the colours a terminal would have got.
  stdio: "inherit",
  // `render` prints a line per record, so a large ledger is a large report.
  maxBuffer: Infinity,
});

if (result.error) {
  const { code } = result.error;
  const hint =
    code === "ENOENT"
      ? "the file is missing; reinstall with `rm -rf node_modules && npm install`"
      : code === "EACCES"
        ? "the file is not executable; some archive tools drop the permission bit"
        : result.error.message;
  console.error(`cs-ledger: cannot run ${bin}: ${hint}`);
  process.exit(NEVER_RAN);
}

// A binary killed by a signal has no exit code. Reporting the shell's
// 128 + signal keeps a Ctrl-C from reading as a clean run, which is what a
// bare `process.exit(result.status)` would produce, since status is null here.
if (result.signal) {
  console.error(`cs-ledger: killed by ${result.signal}`);
  process.exit(128 + (constants.signals[result.signal] ?? 0));
}

process.exit(result.status ?? NEVER_RAN);
