# @codesweep-ai/ledger

> **Structured issue tracking for agent-managed repositories: AI agents write JSON records, humans read a generated ledger.html.**

[![CI](https://github.com/codesweep-ai/ledger/actions/workflows/ci.yml/badge.svg)](https://github.com/codesweep-ai/ledger/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](https://github.com/codesweep-ai/ledger/blob/main/LICENSE)

A ledger is a directory of JSON files recording what is wrong with a repository
and what to do next. An agent writes the records. A person reads the page
generated from them. `cs-ledger` is the tool that keeps the two in step.

- **`cs-ledger init`** scaffolds a new `ledger/` directory.
- **`cs-ledger check`** validates the records and the queue, and fails when the
  page is older than the records it was generated from.
- **`cs-ledger render`** rewrites that page.
- **`cs-ledger guide`** prints the doctrine an agent reads before touching a
  record.

The tool is written in Go, and packaged here for npm projects.

## Quickstart

```bash
npm install --save-dev @codesweep-ai/ledger

cd ~/code/my-project
cs-ledger init --project my-project --prefix MYP
cs-ledger check
```

Then wire the check into the one command a contributor already runs:

```json
{
  "scripts": {
    "ledger": "cs-ledger check",
    "ci": "npm run check && npm run ledger"
  }
}
```

`ledger/ledger.html` is generated and must never be edited by hand. Change a
record, re-render, and let the two travel together in one commit:

```bash
cs-ledger render ledger
cs-ledger check ledger
```

## Exit status

| Code | Meaning |
|---|---|
| 0 | `check` found no errors. |
| 1 | `check` failed, or the run hit an error such as an unreadable config. |
| 2 | No verb, or a verb the tool does not have. |

A gate reads those apart, so this package passes the binary's status through
untouched.

## Docs

The documentation lives in the [codesweep-ai/ledger](https://github.com/codesweep-ai/ledger)
GitHub repository, and none of it ships in this package.

- [INSTALL.md](https://github.com/codesweep-ai/ledger/blob/main/INSTALL.md) · how to get the tool, and the setup it needs once
- [MANUAL.md](https://github.com/codesweep-ai/ledger/blob/main/MANUAL.md) · the full surface: verbs, flags, exit codes
- [GUIDE.md](https://github.com/codesweep-ai/ledger/blob/main/GUIDE.md) · how to keep a ledger honest, which `cs-ledger guide` also prints
- [SPEC.md](https://github.com/codesweep-ai/ledger/blob/main/SPEC.md) · what the behaviour must be, and what is left open
- [CONTRIBUTING.md](https://github.com/codesweep-ai/ledger/blob/main/CONTRIBUTING.md) · conventions, and the rituals a diff does not show
- [AGENTS.md](https://github.com/codesweep-ai/ledger/blob/main/AGENTS.md) · where an agent looks first

## Contributing

Read [CONTRIBUTING.md](https://github.com/codesweep-ai/ledger/blob/main/CONTRIBUTING.md).
It applies to coding agents as well as to people.

## License

[Apache-2.0](https://github.com/codesweep-ai/ledger/blob/main/LICENSE).
