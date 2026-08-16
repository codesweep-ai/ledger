// Package ledger exposes the viewer assets and manual embedded into the
// cs-ledger binary. It is not this module's entry point: cs-ledger is a
// command-line tool rather than a library, and the program is cmd/cs-ledger.
//
//	go install github.com/codesweep-ai/ledger/cmd/cs-ledger@latest
//
// The package sits at the module root only because a //go:embed directive
// cannot reach a parent directory and the trees it embeds, viewer/ and
// MANUAL.md, are there. Everything the tool actually does lives under
// internal/.
//
// The files in viewer/ are the single source of truth for the interactive
// viewer; dev iteration edits them directly (cs-ledger render --assets ./viewer)
// and release builds bake them in here.
package ledger

import (
	"embed"
	"strings"
)

//go:embed viewer
var Assets embed.FS

// ProjectMarker separates the generated part of a materialized guide from the
// project's own conventions. Everything above it belongs to the binary and is
// gated; everything below belongs to the repository and is never touched.
const ProjectMarker = "<!-- LEDGER:PROJECT -->"

// GeneratedGuide returns the part of a guide the binary owns: everything up to
// and including ProjectMarker. A guide with no marker is generated whole.
func GeneratedGuide(s string) string {
	if i := strings.Index(s, ProjectMarker); i >= 0 {
		return s[:i+len(ProjectMarker)]
	}
	return s
}

// ProjectConventions returns the part of a guide the repository owns: whatever
// follows ProjectMarker. It is empty when the guide has no marker.
func ProjectConventions(s string) string {
	if _, after, ok := strings.Cut(s, ProjectMarker); ok {
		return after
	}
	return ""
}

// ManualMD is the cs-ledger man page: verbs, flags, files, exit status and
// diagnostics. `cs-ledger manual` prints it from inside the binary, so a machine
// that has the tool has its command surface with no checkout to read.
//
//go:embed MANUAL.md
var ManualMD string

// GuideMD is the operating guide for keeping a ledger, which is what an agent
// needs rather than the command surface. `cs-ledger guide` prints it, and
// init and render materialize it into target repos as LEDGERDIR/GUIDE.md. Only
// the part above ProjectMarker is generated: check gates that against this copy
// and leaves the project's own conventions below it alone.
//
//go:embed GUIDE.md
var GuideMD string

// LedgerAgentsMD is the router init drops at LEDGERDIR/AGENTS.md. It is how the
// ledger is discovered: agent harnesses read the AGENTS.md nearest the file
// being edited, so an agent touching a record walks up to this one and reaches
// the guide. It is written once and never gated, because a router holds no
// knowledge that could drift.
//
//go:embed templates/ledger-agents.md
var LedgerAgentsMD string
