# cs-ledger — build/test/install.
# `make build` produces bin/cs-ledger (version-stamped, CGO_ENABLED=0).
# `make check` is the full local gate: formatting, vet, the Go suite, the
# viewer build, the coverage gate and the linters. Building needs Go alone when
# the committed viewer is current; viewer changes need Node and npm, and the
# gate also needs cs-lint (prose, refs, oss, surface).

GORELEASER ?= goreleaser
CS_LINT  ?= go tool cs-lint
# The linters the gates shell out to, all pinned and all built from the module
# cache, so a fresh checkout runs `make check` with nothing installed by hand.
# deadcode and actionlint are `tool` directives in go.mod and run with `go tool`.
# golangci-lint is one in go.golangci.mod, which says at its head why it needs a
# module file of its own.
GOLANGCI := bin/tools/golangci-lint
BIN      := bin/cs-ledger
PKG      := ./cmd/cs-ledger
PREFIX   ?= $(HOME)/.local
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w
GO_FILES := $(shell git ls-files '*.go')

# What $(BIN) is made of. It is a real target rather than a phony one, so make
# skips the build when the binary is already newer than every input — which is
# what stops `make install` from repeating the `make build` that just ran.
#
# `find` rather than $(GO_FILES): a source file that is new and not yet added to
# the index is still an input. $(GIT_DIR)/HEAD is one because the version is the
# VCS stamp Go embeds, so a commit changes the binary even when no source did.
# The embedded files are listed because //go:embed makes them compile-time
# inputs; add to the list when a new one is embedded.
GIT_DIR    := $(shell git rev-parse --git-dir 2>/dev/null)
# The viewer is a Vite/React app built into one self-contained file, and that
# file is the only viewer artifact //go:embed reads. Naming it rather than all
# of viewer/ keeps the fixture suite and its vendored axe-core out of the
# binary's prerequisites, where a change to either rebuilt it for nothing.
VIEWER     := viewer/index.html
VIEWER_SRC := $(shell find viewer/app -type f 2>/dev/null) \
              viewer/vite.config.ts viewer/tsconfig.json viewer/package.json viewer/package-lock.json
EMBED_DEPS := MANUAL.md GUIDE.md templates/ledger-agents.md $(VIEWER)
# //go:embed inputs deliberately left out of $(EMBED_DEPS). Nothing belongs here
# yet; `make embed-check` allows exactly this list and nothing else.
EMBED_EXEMPT :=
BUILD_DEPS := $(shell find . \( -name bin -o -name dist -o -name node_modules -o -name .git \) -prune -o -name '*.go' -print) \
              go.mod go.sum .goreleaser.yaml Makefile $(EMBED_DEPS) $(wildcard $(GIT_DIR)/HEAD)

# Coverage is not a separate mode: every test target below writes Go binary
# coverage data into its own tier directory under $(COVERDIR), and `make
# coverage` merges whichever tiers are present. That is what lets
# `make test test-integration` report one aggregate number instead of the last
# tier overwriting the one before it. scripts/coverage.sh documents the layout.
# -test.gocoverdir must be absolute: `go test` runs each package's test binary
# with that package's directory as its working directory, so a relative path
# would scatter the data one directory per package.
COVERDIR   ?= .coverage
COVER_ABS  := $(abspath $(COVERDIR))
COVERFLAGS := -covermode=atomic -coverpkg=./...
# CS_COVERDIR, passed per tier below, tells a test that builds and execs the
# real binary where the instrumented child should write. It is not GOCOVERDIR
# because `go test` overwrites that one in the test process with a directory of
# its own, and does not fold what lands there back into the profile.

.PHONY: help tidy-check embed-check build build-go install uninstall test viewer viewer-build viewer-check coverage coverage-check ci coverage-baseline vet fmt fmt-check check prose refs oss surface conventions ledger lint deadcode actionlint snapshot release release-check clean

.DEFAULT_GOAL := help

## help: list available targets (this menu)
help:
	@echo "cs-ledger make targets:"
	@grep -E '^## [a-z][a-z0-9-]*: ' $(MAKEFILE_LIST) | sed -E 's/^## ([^:]+): (.*)/  \1|\2/' | column -t -s '|'

## build: host binary at bin/cs-ledger via goreleaser (single target)
##
## A phony alias for $(BIN), so the work sits on a file target and make can skip
## it. `make build install`, and an `install` after a build, then copy what is
## already there instead of building the same binary a second time.
##
## $(VIEWER) is a prerequisite through $(EMBED_DEPS) and has a rule of its own,
## so a viewer source change rebuilds the page and then the binary that embeds
## it, in that order, without either being asked for by name.
##
## --skip=before, because .goreleaser.yaml's before hooks are `go mod tidy`,
## `go vet ./...` and `go test ./...`: release gates that `make check` runs in
## its own right, and that made every build pay for the whole suite and rewrite
## go.mod as a side effect. `make snapshot` and `make release` still run them.
build: $(BIN)

$(BIN): $(BUILD_DEPS)
	@mkdir -p $(dir $@)
	@if command -v $(GORELEASER) >/dev/null 2>&1; then \
		VERSION='$(VERSION)' $(GORELEASER) build --single-target --snapshot --clean --skip=before --output $@; \
	else \
		echo "goreleaser not found; using go build (run 'make build-go' explicitly to force)"; \
		$(MAKE) build-go; \
	fi

## build-go: bin/cs-ledger straight from go build, no goreleaser
build-go:
	@mkdir -p $(dir $(BIN))
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

## versions: what this build is made of — this repo's binary, every pinned tool,
## the Go toolchain, and whether a workspace is overriding the go.mod pins. The
## binary answers for itself; every tool is read out of the module file that
## pins it, which is the one place a `go tool` run can get it from. It
## deliberately depends on nothing and runs from source: reporting a version
## must not trigger a build.
## -buildvcs=true because `go run` leaves out the VCS stamp by default, and that
## stamp is the version now that nothing injects one with -X.
.PHONY: versions
versions:
	@if out="$$(go run -buildvcs=true -ldflags '$(LDFLAGS)' $(PKG) version 2>&1)"; then \
		printf '%-14s %-42s %s\n' '$(notdir $(BIN))' "$$(printf '%s\n' "$$out" | awk 'NR==1{print $$2}')" 'this repo'; \
	else \
		printf '%-14s %s\n' '$(notdir $(BIN))' "FAILED — $$(printf '%s\n' "$$out" | head -1)"; \
	fi
	@ver='{{with .Module}}{{if .Replace}}{{.Replace.Path}}{{else if .Version}}{{.Version}}{{else}}{{.Dir}}{{end}}{{end}}'; \
	for t in $$(go list tool 2>/dev/null); do \
		v="$$(go list -f "$$ver" $$t 2>/dev/null)"; \
		printf '%-14s %s\n' "$$(basename $$t)" "$${v:-FAILED}"; \
	done; \
	for t in $$(GOWORK=off go list -modfile=go.golangci.mod tool 2>/dev/null); do \
		v="$$(GOWORK=off go list -modfile=go.golangci.mod -f "$$ver" $$t 2>/dev/null)"; \
		printf '%-14s %s\n' "$$(basename $$t)" "$${v:-FAILED}"; \
	done
	@printf '%-14s %s\n' 'go' "$$(go env GOVERSION)"
	@w="$$(go env GOWORK)"; \
	case "$$w" in \
		''|off) printf '%-14s %s\n' 'workspace' 'off — versions above are go.mod pins' ;; \
		*)      printf '%-14s %s\n' 'workspace' "$$w — local checkouts override the go.mod pins" ;; \
	esac

## repin: move every codesweep-ai tool pin to its branch tip, then report. Uses
## GOPROXY=direct because the module proxy caches branch resolution and `@main`
## can come back a commit behind origin/main. Uses GOWORK=off so this edits the
## recorded pins even while a workspace is serving local checkouts.
.PHONY: repin
repin:
	@tools="$$(go list tool 2>/dev/null | grep codesweep-ai || true)"; \
	if [ -z "$$tools" ]; then \
		echo "no codesweep-ai tools declared yet — add the first with:" >&2; \
		echo "  GOPROXY=direct go get -tool github.com/codesweep-ai/lint/cmd/cs-lint@main" >&2; \
		exit 1; \
	fi; \
	GOWORK=off GOPROXY=direct go get -tool $$(echo "$$tools" | sed 's|$$|@main|')
	@GOWORK=off go mod tidy
	@$(MAKE) versions

## install: copy bin/cs-ledger into $(PREFIX)/bin (default ~/.local/bin)
install: build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/cs-ledger
	@echo "installed $(PREFIX)/bin/cs-ledger ($(VERSION))"
	@case ":$(PATH):" in *":$(PREFIX)/bin:"*) : ;; *) echo "note: add $(PREFIX)/bin to PATH" ;; esac

## uninstall: remove the installed binary
uninstall:
	rm -f $(PREFIX)/bin/cs-ledger

## test: Go suite (validation, rendering and black-box CLI)
# -count=1 disables the test cache. The suite shells out to a built binary and
# compares against embedded assets, so a change to GUIDE.md or viewer/ leaves the
# cached result green while the gate it stands for has started failing.
test: viewer
	@scripts/coverage.sh reset unit
	CS_COVERDIR=$(COVER_ABS)/unit go test $(COVERFLAGS) ./... -count=1 -args -test.gocoverdir=$(COVER_ABS)/unit

## viewer: rebuild the committed viewer when its sources move
##
## A phony alias for $(VIEWER), so a tree whose viewer is current does no npm
## work at all. Where npm is absent it says so and leaves the committed file
## alone, which is what lets a Go-only clone build: the viewer is committed
## precisely so that building the binary never requires Node.
viewer: $(VIEWER)

$(VIEWER): $(VIEWER_SRC)
	@if command -v npm >/dev/null 2>&1; then \
		$(MAKE) viewer-build; \
	else \
		echo "viewer: SKIP (npm not found; using committed $(VIEWER))"; \
	fi

## viewer-build: restore dependencies, typecheck and build the single-file viewer
# npm runs in viewer/, which is where the manifest and the ESM package scope are.
viewer-build:
	cd viewer && npm ci && npm run typecheck && npm run build

## coverage: merge every tier present under $(COVERDIR) and print the report
coverage:
	@scripts/coverage.sh report

## coverage-check: report, then fail if a package .coverage-baseline records as
## covered has stopped being reached. It checks presence, never a percentage:
## what it exists to catch is a suite that quietly stopped running.
coverage-check: coverage
	@scripts/coverage.sh check

## coverage-baseline: re-record .coverage-baseline. Records every tier present
## by default; pass BASELINE_TIERS to restrict it to the tiers CI actually runs,
## e.g. `make coverage-baseline BASELINE_TIERS="unit race smoke"`. Recording a
## tier CI never runs commits a promise nothing keeps.
coverage-baseline:
	@scripts/coverage.sh baseline $(BASELINE_TIERS)

## vet: go vet
vet:
	go vet ./...
## fmt: gofmt all tracked Go files
fmt:
	gofmt -w $(GO_FILES)
## fmt-check: fail if any Go file is unformatted
fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## tidy-check: go.mod and go.sum are what `go mod tidy` would write
##
## The build no longer runs `go mod tidy`. It used to, as a goreleaser before
## hook, so every `make build` rewrote the module files as a side effect and
## nothing ever reported the drift. This gate replaces it and is the stronger of
## the two: it says what moved instead of quietly absorbing it, and it puts the
## originals back before failing, so a red gate leaves the tree as it found it.
## GOWORK=off, so a workspace serving local checkouts cannot make an untidy
## go.mod look tidy.
tidy-check:
	@t="$$(mktemp -d)"; cp go.mod go.sum "$$t/"; \
	GOWORK=off go mod tidy || { cp "$$t/go.mod" go.mod; cp "$$t/go.sum" go.sum; rm -rf "$$t"; exit 1; }; \
	if cmp -s go.mod "$$t/go.mod" && cmp -s go.sum "$$t/go.sum"; then \
		rm -rf "$$t"; echo "tidy: go.mod and go.sum are what \`go mod tidy\` writes"; \
	else \
		echo "go.mod/go.sum are not tidy; \`go mod tidy\` would apply:" >&2; \
		diff -u "$$t/go.mod" go.mod >&2; diff -u "$$t/go.sum" go.sum >&2; \
		cp "$$t/go.mod" go.mod; cp "$$t/go.sum" go.sum; rm -rf "$$t"; \
		exit 1; \
	fi

## embed-check: every //go:embed input is a prerequisite of the binary
##
## $(EMBED_DEPS) is written by hand, and an embed added without a line there
## leaves make holding a binary it calls current while the bytes inside it have
## moved -- the one kind of staleness no other gate can see. `go list` resolves
## the patterns itself, so this compares against what the toolchain actually
## embeds rather than re-reading the directives and reimplementing their globs.
embed-check:
	@deps="$$(mktemp)"; embeds="$$(mktemp)"; raw="$$(mktemp)"; \
	printf '%s\n' $(patsubst ./%,%,$(BUILD_DEPS)) $(EMBED_EXEMPT) | LC_ALL=C sort -u >"$$deps"; \
	if ! go list -f '{{range .EmbedFiles}}{{$$.Dir}}/{{.}}{{"\n"}}{{end}}' ./... >"$$raw"; then \
		rm -f "$$deps" "$$embeds" "$$raw"; \
		echo "embed-check: go list failed, so the embed set is unknown" >&2; exit 1; \
	fi; \
	grep -v '/node_modules/' "$$raw" | sed "s|^$$PWD/||" | grep . | LC_ALL=C sort -u >"$$embeds"; \
	missing="$$(LC_ALL=C comm -23 "$$embeds" "$$deps")"; n="$$(wc -l <"$$embeds")"; \
	rm -f "$$deps" "$$embeds" "$$raw"; \
	if [ -n "$$missing" ]; then \
		echo "//go:embed reads these, and no prerequisite of $(BIN) covers them:" >&2; \
		printf '  %s\n' $$missing >&2; \
		echo "add each to EMBED_DEPS, or a change to one will not rebuild the binary" >&2; \
		exit 1; \
	fi; \
	echo "embed: all $$n //go:embed inputs are prerequisites of $(notdir $(BIN))"
## prose: check how this repository's documents are written
prose:
	$(CS_LINT) prose

## refs: check that everything the documents point at is there
refs:
	$(CS_LINT) refs

## oss: the rules this repo has to satisfy as a published project
oss:
	$(CS_LINT) oss

## surface: check the docs against the binary, the code and the build
surface: build
	$(CS_LINT) surface

# The four targets above are one shared tool: github.com/codesweep-ai/lint,
# pinned in go.mod and run with `go tool`, so the gates use the version this
# repo records rather than whatever a machine happens to have installed. `make
# repin` moves that pin. prose and refs ask for no binary and run first;
# surface reads the one build makes.
# Its knobs for this repo live in .cs-lint.yaml, and `cs-lint <linter> --explain`
# says what each rule wants.

## ledger: validate this repo's own records and prove ledger.html is current
ledger: build
	./bin/cs-ledger check ledger
	./bin/cs-ledger check fixtures/sandbox/ledger


## conventions: the house rules a @codesweep-ai/ui consumer must keep — the pin
## agrees across the specifiers, the lockfiles and the install, and a target
## that re-records committed files is named record-*. The script is vendored
## byte-identical into ledger, tracer and campaign; keep the copies identical.
conventions:
	@go run scripts/consumer-conventions.go

## viewer-check: the committed viewer is what viewer/app builds
##
## It rebuilds through viewer-build rather than asking anybody to run npm, then
## asserts that the rebuild changed nothing. On the clean checkout CI runs, a
## difference means the commit carries a bundle its own sources do not produce.
## In a tree with viewer edits in flight it means the same thing one commit
## early: the rebuilt file has to go in with them, and it is already rebuilt.
##
## It rebuilds unconditionally rather than depending on $(VIEWER), because a
## clone's timestamps are its checkout order and this gate exists precisely
## where timestamps cannot be trusted. npm is required, unlike in $(VIEWER)'s
## own rule, which skips so a Go-only clone still builds. That skip is what
## this gate stops from reaching a commit.
viewer-check:
	@command -v npm >/dev/null 2>&1 || { echo "viewer-check: npm is required to rebuild the viewer" >&2; exit 1; }
	@$(MAKE) --no-print-directory viewer-build
	@if git diff --quiet -- $(VIEWER); then \
		echo "viewer: $(VIEWER) is what viewer/app builds"; \
	else \
		echo "$(VIEWER) was stale. It has been rebuilt from viewer/app;" >&2; \
		echo "commit it alongside the viewer change that moved it." >&2; \
		exit 1; \
	fi

## check: the full local gate — fmt-check, vet, the linters, and the tests
check: fmt-check tidy-check embed-check vet lint deadcode test coverage-check conventions prose refs oss surface

# say prints a heading above each gate, so a long run reads as a list rather
# than as a wall. Bold where a terminal is reading it and plain where a pipe
# is: `make ci > ci.log` should leave a log somebody can read. The escapes are
# the same ones scripts/check.sh uses in tracer, which is where the shape came
# from.
define say
@if [ -t 1 ]; then printf '\n\033[1m==> %s\033[0m\n' "$(1)"; else printf '\n==> %s\n' "$(1)"; fi
endef

## ci: every gate the CI workflow runs, on this machine
##
## One Linux leg of .github/workflows/ci.yml, in the order CI runs it, so a
## red build is something you can see before you push rather than after. What
## it cannot reproduce it names on the way out: a run that skipped a gate must
## never read as a run that ran them all.
ci:
	$(call say,viewer)
	@$(MAKE) --no-print-directory viewer-check
	$(call say,the gate a contributor runs before pushing)
	@$(MAKE) --no-print-directory check
	$(call say,actionlint)
	@$(MAKE) --no-print-directory actionlint
	$(call say,build)
	@$(MAKE) --no-print-directory build
	$(call say,release manifest)
	@$(MAKE) --no-print-directory release-check
	$(call say,ledger)
	@$(MAKE) --no-print-directory ledger
	@printf '\nci: every gate ran. Not reproduced here: build-test on macOS.\n'

# Built rather than run with `go tool`, because -modfile is refused in workspace
# mode. The build is the only step that reads go.golangci.mod, so only the build
# turns the workspace off; the linter then runs with it back on, against the
# checkouts a workspace is there to serve. A rebuild costs about a fifth of a
# second once the binary is current, which is what lets it be a prerequisite
# rather than a step somebody remembers.
$(GOLANGCI): go.golangci.mod
	@mkdir -p $(@D)
	@GOWORK=off go build -modfile=go.golangci.mod -o $@ \
		github.com/golangci/golangci-lint/v2/cmd/golangci-lint

## lint: the Go rules from .golangci.yml (see that file for what is on and why)
lint: $(GOLANGCI)
	$(GOLANGCI) run

## deadcode: functions no entry point reaches. golangci-lint's `unused` cannot
## see this — it reasons one package at a time, so a function whose only caller
## lives in another package looks used. Drop -test and it answers a second,
## softer thing: what only a test keeps alive.
deadcode:
	@out="$$(go tool deadcode -test ./...)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

## actionlint: the workflow files, which the forge validates only by refusing to
## run them. Extra runner labels it does not know about go in .github/actionlint.yaml.
actionlint:
	go tool actionlint

## snapshot: local release dry-run into dist/ (all platforms, archives, checksums).
## Skips SBOM + cosign signing (those need cyclonedx-gomod + cosign; run in CI/release).
snapshot:
	VERSION='$(VERSION)' $(GORELEASER) release --snapshot --clean --skip=sbom,sign

## release: tagged release (needs a pushed git tag + credentials). For a full
## signed+SBOM release install: go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest and cosign.
release:
	$(GORELEASER) release --clean

## release-check: validate .goreleaser.yaml
release-check:
	$(GORELEASER) check

## clean: remove build output
clean:
	rm -rf bin dist $(COVERDIR)
