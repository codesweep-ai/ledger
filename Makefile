# cs-ledger — build/test/install.
# `make build` produces bin/cs-ledger (version-stamped, CGO_ENABLED=0).
# `make check` is the full local gate: formatting, vet, the Go suite, the
# viewer's JavaScript suite, the coverage gate and the linters. Building needs Go
# alone; the gate also needs node (test-js) and cs-lint (prose, refs, oss, surface).

GORELEASER ?= goreleaser
CS_LINT  ?= cs-lint
BIN      := bin/cs-ledger
PKG      := ./cmd/cs-ledger
PREFIX   ?= $(HOME)/.local
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)
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

.PHONY: help build install uninstall test test-js coverage coverage-check coverage-baseline vet fmt fmt-check check prose refs oss surface cs-lint-installed ledger lint deadcode snapshot release release-check clean

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
prose: cs-lint-installed
	$(CS_LINT) prose

## refs: check that everything the documents point at is there
refs: cs-lint-installed
	$(CS_LINT) refs

## oss: the rules this repo has to satisfy as a published project
oss: cs-lint-installed
	$(CS_LINT) oss

## surface: check the docs against the binary, the code and the build
surface: build cs-lint-installed
	$(CS_LINT) surface

# The four targets above are one shared tool: github.com/codesweep-ai/lint.
# prose and refs ask for no binary and run first; surface reads the one
# build makes.
# Its knobs for this repo live in .cs-lint.yaml, and `cs-lint <linter> --explain`
# says what each rule wants.
cs-lint-installed:
	@command -v $(CS_LINT) >/dev/null 2>&1 || { \
		echo "cs-lint is not installed: go install github.com/codesweep-ai/lint/cmd/cs-lint@latest" >&2; \
		exit 2; \
	}

## ledger: validate this repo's own records and prove ledger.html is current
ledger: build
	./bin/cs-ledger check ledger
	./bin/cs-ledger check fixtures/sandbox/ledger

## check: the full local gate — fmt-check, vet, the linters, and the tests
check: fmt-check vet lint deadcode test coverage-check prose refs oss surface

## lint: the Go rules from .golangci.yml (see that file for what is on and why)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed; see https://golangci-lint.run/welcome/install/" >&2; \
		exit 2; \
	}
	golangci-lint run

## deadcode: functions no entry point reaches. golangci-lint's `unused` cannot
## see this — it reasons one package at a time, so a function whose only caller
## lives in another package looks used. Drop -test and it answers a second,
## softer thing: what only a test keeps alive.
deadcode:
	@command -v deadcode >/dev/null 2>&1 || { \
		echo "deadcode is not installed: go install golang.org/x/tools/cmd/deadcode@latest" >&2; \
		exit 2; \
	}
	@out="$$(deadcode -test ./...)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

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
