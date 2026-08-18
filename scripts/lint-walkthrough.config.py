# Project-specific knobs for scripts/lint-walkthrough.py.
#
# The linter beside this file carries no project knowledge and is meant to stay
# byte-identical everywhere it lands. This file is the half you edit.

TOOL = "cs-ledger"
TOOL_PATH = "bin/cs-ledger"

DOCS = ["README.md", "INSTALL.md", "MANUAL.md", "SPEC.md", "CONTRIBUTING.md",
        "GUIDE.md"]
EXTRA_DOCS = []

ENV_PREFIX = "CS_LEDGER_"

ENV_INTERNAL = {}

SOURCE_SKIP = {}

# Read-only, offline, and safe to run on every gate. `render` is deliberately
# absent: it writes the page, and a checker that writes can hide the very
# staleness the ledger's own gate exists to catch.
SAFE_VERBS = ["version", "check", "check ledger", "check fixtures/sandbox/ledger"]

SAMPLE_SKIP = {
    "cs-ledger version": "the first field is a git describe of the build, which"
                         " the manual says will read differently for every one."
                         " WALK-402 still holds the renderer version",
    "cs-ledger render && cs-ledger check": "render writes the page, and this"
                                           " check must not",
    "cs-ledger check": "the sample is the failure a stale page produces, which a"
                       " healthy tree cannot reproduce",
    "cs-ledger check ledger": "the sample is a freshly scaffolded ledger in"
                              " another repository, not this one",
}

# INSTALL.md and the README both say to change into your own repository first.
PLACEHOLDER_OK = ["~/my-service"]

PREREQ_OK = []

AGENT_SECTION = "Notes for agents"

ALLOW = {}
