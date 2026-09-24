//go:build ignore

// renderer-version — carry the viewer's @codesweep-ai/ui pin into the renderer.
//
// Run as:  go run scripts/renderer-version.go
//
// The viewer's bytes are part of every page cs-ledger renders, so a viewer
// built against another ui, or on other npm packages, is another renderer.
// RendererVersion has to move with it (SPEC.md R41), and nothing else catches a
// forgotten bump. UIVersion is the ui version the page's footer reports, so it
// has to be the one the viewer is built against.
//
// This sets UIVersion to the pin in viewer/package.json. Where that pin or the
// built viewer differs from HEAD's, and RendererVersion does not yet, it moves
// RendererVersion up a patch and adds a line to its history saying why. A minor
// move, such as adopting a new component set, is still a person's call: make it
// by hand, and this finds RendererVersion already moved and leaves it. The
// sample of `cs-ledger version` in MANUAL.md follows both.
//
// `make viewer-repin` runs it once the viewer is rebuilt, then rebuilds the
// binary and renders both ledgers again. `make repin` and oss-repin run that
// target after they move the pin.
//
// It is a standalone file, not a package: `//go:build ignore` keeps it out of
// ./... so vet, deadcode, golangci-lint and the coverage baseline ignore it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

const (
	renderGo = "internal/ledger/render.go"
	manifest = "viewer/package.json"
	viewer   = "viewer/index.html"
	manual   = "MANUAL.md"
	uiPkg    = "@codesweep-ai/ui"
)

var (
	rendererRe = regexp.MustCompile(`(?m)^const RendererVersion = "(\d+)\.(\d+)\.(\d+)"$`)
	uiRe       = regexp.MustCompile(`(?m)^const UIVersion = "([^"]*)"$`)
	sampleRe   = regexp.MustCompile(`renderer \d+\.\d+\.\d+, @codesweep-ai/ui [^\s)]+`)
)

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "renderer-version: "+format+"\n", a...)
	os.Exit(1)
}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		die("%v", err)
	}
	return string(b)
}

func write(path, before, after string) {
	if after == before {
		return
	}
	if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
		die("%v", err)
	}
}

// versions finds both constants in one copy of render.go.
func versions(src, where string) (renderer []string, ui string) {
	r, u := rendererRe.FindStringSubmatch(src), uiRe.FindStringSubmatch(src)
	if r == nil || u == nil {
		die("%s names no RendererVersion or no UIVersion", where)
	}
	return r[1:], u[1]
}

// viewerMoved says whether the built viewer differs from HEAD's.
func viewerMoved() bool {
	err := exec.Command("git", "diff", "--quiet", "HEAD", "--", viewer).Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return true
	}
	if err != nil {
		die("git diff %s: %v", viewer, err)
	}
	return false
}

func main() {
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(read(manifest)), &pkg); err != nil {
		die("%s: %v", manifest, err)
	}
	pin := pkg.Dependencies[uiPkg]
	if pin == "" {
		die("%s has no %s dependency", manifest, uiPkg)
	}
	head, err := exec.Command("git", "show", "HEAD:"+renderGo).Output()
	if err != nil {
		die("git show HEAD:%s: %v", renderGo, err)
	}
	src := read(renderGo)
	renderer, _ := versions(src, renderGo)
	headRenderer, headUI := versions(string(head), "HEAD's "+renderGo)
	version := renderer[0] + "." + renderer[1] + "." + renderer[2]

	out := uiRe.ReplaceAllLiteralString(src, fmt.Sprintf("const UIVersion = %q", pin))
	why := ""
	switch {
	case pin != headUI:
		why = "re-pins " + uiPkg + " to " + pin
	case viewerMoved():
		why = "rebuilds the viewer on the npm packages it now pins"
	}
	if why != "" && version == headRenderer[0]+"."+headRenderer[1]+"."+headRenderer[2] {
		patch, _ := strconv.Atoi(renderer[2])
		next := fmt.Sprintf("%s.%s.%d", renderer[0], renderer[1], patch+1)
		out = rendererRe.ReplaceAllLiteralString(out,
			fmt.Sprintf("// %s %s.\nconst RendererVersion = %q", next, why, next))
		fmt.Printf("renderer %s -> %s, as it %s\n", version, next, why)
		version = next
	} else {
		fmt.Printf("renderer %s, %s %s\n", version, uiPkg, pin)
	}
	write(renderGo, src, out)

	doc := read(manual)
	write(manual, doc, sampleRe.ReplaceAllLiteralString(doc, fmt.Sprintf("renderer %s, %s %s", version, uiPkg, pin)))
}
