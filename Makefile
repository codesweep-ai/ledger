# cs-ledger — build/test/install.
# `make build` produces bin/cs-ledger (version-stamped, CGO_ENABLED=0).
# `make check` is the full local gate: formatting, vet, the Go suite, the
# viewer's JavaScript suite, the coverage gate and the linters. Building needs Go
# alone; the gate also needs node (test-js) and cs-lint (prose, refs, oss, surface).

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

.PHONY: help build install uninstall test test-js coverage coverage-check ci coverage-baseline vet fmt fmt-check check prose refs oss surface ledger lint deadcode actionlint snapshot release release-check clean

.DEFAULT_GOAL := help

## help: list available targets (this menu)
help:
	@echo "cs-ledger make targets:"
	@grep -E '^## [a-z][a-z0-9-]*: ' $(MAKEFILE_LIST) | sed -E 's/^## ([^:]+): (.*)/  \1|\2/' | column -t -s '|'

## build: host binary at bin/cs-ledger via goreleaser (single target)
build:
	@mkdir -p $(dir $(BIN))
	@if command -v $(GORELEASER) >/dev/null 2>&1; then \
		VERSION='$(VERSION)' $(GORELEASER) build --single-target --snapshot --clean --output $(BIN); \
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

## test: Go suite (validation, render, black-box CLI) + viewer-asset JS suite
# -count=1 disables the test cache. The suite shells out to a built binary and
# compares against embedded assets, so a change to GUIDE.md or viewer/ leaves the
# cached result green while the gate it stands for has started failing.
test:
	@scripts/coverage.sh reset unit
	CS_COVERDIR=$(COVER_ABS)/unit go test $(COVERFLAGS) ./... -count=1 -args -test.gocoverdir=$(COVER_ABS)/unit
	$(MAKE) test-js

## test-js: the viewer-asset suite (markdown renderer in viewer/viewer.js)
test-js:
	node test/run.mjs

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

## check: the full local gate — fmt-check, vet, the linters, and the tests
check: fmt-check vet lint deadcode test coverage-check prose refs oss surface

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
