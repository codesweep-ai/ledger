# cs-ledger — build/test/install.
# `make build` produces bin/cs-ledger (version-stamped, CGO_ENABLED=0).
# `make check` is the full local gate: formatting, vet, the Go suite, the
# viewer's JavaScript suite and both linters. Building needs Go alone; the
# gate also needs node (test-js) and python3 (docs, oss).

GORELEASER ?= goreleaser
BIN      := bin/cs-ledger
PKG      := ./cmd/cs-ledger
PREFIX   ?= $(HOME)/.local
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)
GO_FILES := $(shell git ls-files '*.go')

.PHONY: help build install uninstall test test-js vet fmt fmt-check check docs oss ledger lint deadcode snapshot release release-check clean

.DEFAULT_GOAL := help

## help: list available targets (this menu)
help:
	@echo "cs-ledger make targets:"
	@grep -E '^## [a-z][a-z0-9-]*: ' $(MAKEFILE_LIST) | sed -E 's/^## ([^:]+): (.*)/  \1|\2/' | column -t -s '|'

## build: host binary at bin/cs-ledger
build:
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
	go test ./... -count=1
	$(MAKE) test-js

## test-js: the viewer-asset suite (markdown renderer in viewer/viewer.js)
test-js:
	node test/run.mjs

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
## docs: the prose rules from CONTRIBUTING.md
docs:
	python3 scripts/lint-docs.py

## oss: the rules this repo has to satisfy as a published project
oss:
	python3 scripts/lint-oss.py

## ledger: validate this repo's own records and prove ledger.html is current
ledger: build
	./bin/cs-ledger check ledger
	./bin/cs-ledger check fixtures/sandbox/ledger

## check: the full local gate — fmt-check, vet, the linters, and the tests
check: fmt-check vet lint deadcode test docs oss

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
	$(GORELEASER) release --snapshot --clean --skip=sbom,sign

## release: tagged release (needs a pushed git tag + credentials). For a full
## signed+SBOM release install: go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest and cosign.
release:
	$(GORELEASER) release --clean

## release-check: validate .goreleaser.yaml
release-check:
	$(GORELEASER) check

## clean: remove build output
clean:
	rm -rf bin dist
