# Installing cs-ledger

`cs-ledger` is a single static binary with no runtime dependencies. Get it, put it on your PATH,
then scaffold a ledger in the repository you want it to track. After that, head for the
[README](README.md).

## 1. Install the binary

### Download a release

From the releases page grab the archive for your OS/arch
(`cs-ledger_<version>_<os>_<arch>.tar.gz`) and `checksums.txt`, verify, then install:

```bash
sha256sum -c --ignore-missing checksums.txt      # releases are checksummed + cosign-signed
tar xzf cs-ledger_*.tar.gz cs-ledger
install -m755 cs-ledger ~/.local/bin/cs-ledger   # anywhere on your PATH
```

To verify the cosign signature as well:

```bash
cosign verify-blob checksums.txt \
  --signature checksums.txt.sig --certificate checksums.txt.pem \
  --certificate-identity-regexp 'https://github.com/codesweep-ai/ledger/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Releases carry Linux and macOS builds for both architectures, each with a software bill of
materials beside its archive.

**No version has been tagged yet, so there is nothing on the releases page today.** This route
starts working at the first tag, which is what cuts the archives, the checksum file and the
signature. Until then, take one of the two below.

### Or with `go install`

```bash
go install github.com/codesweep-ai/ledger/cmd/cs-ledger@latest
```

This route stamps no version, so `cs-ledger version` reports `dev` for the tool. The renderer
version it prints beside that is the one a ledger records, and it is correct either way.

### Or build from source

Needs **Go 1.26+** and nothing else. The version is stamped from `git describe`, so build from a
clone rather than from a source tarball:

```bash
git clone https://github.com/codesweep-ai/ledger && cd ledger
make build         # -> bin/cs-ledger  (static, CGO_ENABLED=0)
make install       # -> ~/.local/bin/cs-ledger  (override with PREFIX=)
```

However you got it, check what you installed and read what it does:

```bash
cs-ledger version       # the build, the renderer version, and the UI token version
cs-ledger manual | less # the full reference, carried inside the binary
```

## 2. Put a ledger in a repository

A **ledger** is a directory of JSON records committed next to the code, holding what is wrong with
the repository and what to do next. `init` scaffolds one and renders it:

```console
$ cd ~/my-service
$ cs-ledger init ledger --project my-service --prefix MYS
initialized ledger (project my-service, prefix MYS, toolVersion 0.3.3)
next: read ledger/GUIDE.md — ledger/AGENTS.md routes agents to it
```

Four files land in `ledger/`, and you commit all four. `ledger.json` holds the project name and the
record ID prefix, `GUIDE.md` is the operating doctrine your agents read, `AGENTS.md` routes them to
it, and `ledger.html` is the rendered page. An agent that opens a record finds `AGENTS.md` beside
it and follows it to the guide. [MANUAL.md](MANUAL.md#files) lists every path the tool reads or
writes.

There is nothing else to configure. `cs-ledger` reads no user config file, no environment variable
and no network: a ledger's own `ledger.json` is the whole settings model.

## 3. Verify the installation

```console
$ cs-ledger check ledger
check OK: 0 issues, 0 drafts, 0 warning(s), ledger.html fresh
```

Open `ledger/ledger.html` in a browser, straight from disk. Then commit the directory, and tell
your agents the repository keeps one. The [README](README.md#quickstart) has the line to paste and
the loop that follows.

## Moving to a newer binary

A ledger records the renderer version that produced its page. After installing a newer
`cs-ledger`, render once and commit:

```bash
cs-ledger render ledger
git add ledger && git commit -m "Re-render the ledger on the current renderer"
```

Nothing refuses to run in the meantime. Until you render, `check` reports which renderer wrote the
page and skips the freshness comparison, because the two versions produce different bytes whatever
the records say. [MANUAL.md](MANUAL.md#render) describes what `render` touches.
