// Command cs-ledger validates and renders ledger/ directories.
//
// Verbs: check, render, manual, guide, init, version. The viewer
// assets, the man page and the operating guide are embedded at build time;
// --assets <dir> overrides the viewer assets from disk for design iteration
// (dev mode — output is dev-stamped and check refuses it as committable).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	root "github.com/codesweep-ai/ledger"
	"github.com/codesweep-ai/ledger/internal/ledger"
	"github.com/codesweep-ai/ledger/internal/ojson"
)

// devVersion marks a binary that carried no release stamp.
const devVersion = "dev"

// Version is the tool version (set via -ldflags at release).
var Version = devVersion

// buildVersion reports the release stamp when there is one, and otherwise the
// module version the toolchain recorded. A binary installed straight from the
// module path carries no stamp, so without this it would answer "dev" and
// leave you guessing which revision wrote a page.
func buildVersion() string {
	if Version != devVersion {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return Version
	}
	return info.Main.Version
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: cs-ledger <verb> [ledgerDir] [flags]

verbs:
  check    validate records + queue + ledger.html freshness (the standing gate)
  render   (re)write ledger.html, the toolVersion and the ledger's docs
  manual   print the cs-ledger man page (MANUAL.md)
  guide    print the guide to keeping a ledger (GUIDE.md)
  init     scaffold a new ledger/ dir  (--project NAME --prefix AB[2-6])
  version  print tool + renderer versions

flags:
  --assets DIR   dev mode: load viewer assets from DIR instead of the embedded
                 copy; output is dev-stamped and not committable

ledgerDir defaults to "ledger".
Agents: run 'cs-ledger guide' before touching a ledger. It is the operating
doctrine; 'cs-ledger manual' is the command surface.
`)
}

func report(label string, list []string) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%s (%d):\n", label, len(list))
	for _, item := range list {
		fmt.Fprintf(os.Stderr, "  - %s\n", item)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	verb := args[0]
	rest := args[1:]

	switch verb {
	case "version", "--version":
		fmt.Printf("cs-ledger %s (%s/%s, %s, renderer %s, ui tokens %s)\n",
			buildVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version(), ledger.RendererVersion, ledger.UITokensVersion)
		return
	case "manual":
		fmt.Print(root.ManualMD)
		return
	case "guide":
		fmt.Print(root.GuideMD)
		return
	case "help", "--help", "-h":
		usage()
		return
	}

	fs := flag.NewFlagSet("cs-ledger", flag.ExitOnError)
	assetsDir := fs.String("assets", "", "load viewer assets from this directory (dev mode)")
	project := fs.String("project", "", "init: project name")
	prefix := fs.String("prefix", "", "init: record id prefix (^[A-Z]{2,6}$)")
	fs.Usage = usage
	var dir string
	if len(rest) > 0 && rest[0] != "" && rest[0][0] != '-' {
		dir = rest[0]
		rest = rest[1:]
	}
	_ = fs.Parse(rest)
	if dir == "" {
		dir = "ledger"
	}

	switch verb {
	case "check", "render", "init":
	default:
		usage()
		os.Exit(2)
	}

	dev := *assetsDir != ""
	assets, err := loadAssets(*assetsDir)
	if err != nil {
		fail("cannot load viewer assets: " + err.Error())
	}
	assets.Dev = dev

	if verb == "init" {
		doInit(dir, *project, *prefix, assets)
		return
	}

	tr := ledger.LoadLedger(dir)
	res := ledger.ValidateAll(tr)
	outPath := filepath.Join(dir, "ledger.html")
	guidePath := filepath.Join(dir, "GUIDE.md")

	// toolVersion records which renderer wrote the page. It is descriptive
	// rather than a gate: render moves it, and check reports a difference
	// without failing. Absent pin = a ledger written before the pin existed.
	pin, pinned := "", false
	if tr.Config != nil {
		if p, ok := tr.Config.Get("toolVersion").StrVal(); ok {
			pin, pinned = p, true
		}
	}
	skew := pinned && pin != ledger.RendererVersion

	if verb == "render" {
		report("validation errors (rendering anyway — fix before commit; check will fail)", res.Errors)
		report("warnings", res.Warnings)
		if tr.ConfigError != "" {
			fail("cannot render without a valid ledger.json")
		}
		// A dev-stamped page is not committable, so it must not move the pin or
		// rewrite the docs. Dev mode writes the page and nothing else.
		if !dev {
			if skew || !pinned {
				tr.Config.Set("toolVersion", ojson.S(ledger.RendererVersion))
				if err := os.WriteFile(filepath.Join(dir, "ledger.json"), []byte(tr.Config.StringifyIndent()+"\n"), 0o644); err != nil {
					fail("write ledger.json failed: " + err.Error())
				}
				tr = ledger.LoadLedger(dir) // reload so the render sees the new pin
			}
			materializeDocs(dir, guidePath)
		}
		html := ledger.RenderHTML(tr, assets)
		if err := os.WriteFile(outPath, []byte(html), 0o644); err != nil {
			fail("write failed: " + err.Error())
		}
		devNote := ""
		if dev {
			devNote = " [dev-stamped — not committable]"
		}
		fmt.Printf("wrote %s (%d bytes, %d issues, %d drafts)%s\n", outPath, len(html), len(tr.Records), len(tr.Drafts), devNote)
		if !dev && skew {
			fmt.Printf("toolVersion %s -> %s\n", pin, ledger.RendererVersion)
		}
		return
	}

	// check — always gates against the embedded (release) assets. Collect the
	// page and guide failures before reporting anything: they are errors, and an
	// error printed after the warnings block reads as one more warning.
	errors := res.Errors
	warnings := res.Warnings
	// Freshness is only assertable within a renderer version. Across two of
	// them the bytes differ whatever the records say, so a comparison cannot
	// tell a stale page from version skew. Say which one this is and move on.
	if skew {
		warnings = append(warnings, fmt.Sprintf(
			"ledger.html was rendered by %s and this binary renders %s — run: cs-ledger render %s (freshness not checked across versions)",
			pin, ledger.RendererVersion, dir))
	}
	if tr.ConfigError == "" {
		committed, err := os.ReadFile(outPath)
		switch {
		case err != nil:
			errors = append(errors, "ledger.html: missing — run: cs-ledger render")
		case strings.Contains(string(committed), ledger.DevStamp):
			errors = append(errors, "ledger.html: rendered in dev mode (--assets) — re-render with the release binary before committing")
		case skew:
			// Handled as a warning above.
		case string(committed) != ledger.RenderHTML(tr, releaseAssets()):
			errors = append(errors, "ledger.html: STALE — records changed without re-render. Run: cs-ledger render")
		}
		// guide-sync gate: the generated half of a materialized guide must match
		// the binary's copy. What sits below the project marker is the
		// repository's own and is not compared.
		if g, err := os.ReadFile(guidePath); err == nil &&
			root.GeneratedGuide(string(g)) != root.GeneratedGuide(root.GuideMD) {
			errors = append(errors, "GUIDE.md: does not match this binary's embedded guide — run: cs-ledger render")
		}
	}
	report("validation errors", errors)
	report("warnings", warnings)
	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "check FAILED: %d error(s), %d warning(s)\n", len(errors), len(warnings))
		os.Exit(1)
	}
	freshness := "ledger.html fresh"
	if skew {
		freshness = "ledger.html not compared (rendered by " + pin + ")"
	}
	fmt.Printf("check OK: %d issues, %d drafts, %d warning(s), %s\n",
		len(tr.Records), len(tr.Drafts), len(warnings), freshness)
}

// materializeDocs brings the ledger's own documents up to this binary. It is
// idempotent, so render can call it every time: the guide's generated half is
// rewritten, the project's conventions below the marker are carried across, and
// the router is put in place if it is missing.
//
// A ledger scaffolded by an older binary picks up whatever it lacks here. That
// is the only route by which an existing ledger gains a new document.
func materializeDocs(dir, guidePath string) {
	guide := root.GuideMD
	if existing, err := os.ReadFile(guidePath); err == nil {
		guide = root.GeneratedGuide(root.GuideMD) + root.ProjectConventions(string(existing))
	}
	if err := os.WriteFile(guidePath, []byte(guide), 0o644); err != nil {
		fail("write GUIDE.md failed: " + err.Error())
	}
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if err := os.WriteFile(agentsPath, []byte(root.LedgerAgentsMD), 0o644); err != nil {
			fail("write AGENTS.md failed: " + err.Error())
		}
	}
}

func doInit(dir, project, prefix string, assets *ledger.ViewerAssets) {
	if project == "" || prefix == "" {
		fail("init requires --project and --prefix")
	}
	// Before anything is written: a rejected prefix used to leave a half-built
	// ledger behind, which init then refused to overwrite.
	if !ledger.ValidPrefix(prefix) {
		fail("--prefix must match ^[A-Z]{2,6}$, got " + prefix)
	}
	if _, err := os.Stat(filepath.Join(dir, "ledger.json")); err == nil {
		fail(dir + "/ledger.json already exists — refusing to overwrite")
	}
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		fail(err.Error())
	}
	cfg := ojson.O(
		ojson.Kv("project", ojson.S(project)),
		ojson.Kv("idPrefix", ojson.S(prefix)),
		ojson.Kv("schemaVersion", ojson.S("issue.v1")),
		ojson.Kv("staleAfterDays", ojson.N(14)),
		ojson.Kv("toolVersion", ojson.S(ledger.RendererVersion)),
	)
	if err := os.WriteFile(filepath.Join(dir, "ledger.json"), []byte(cfg.StringifyIndent()+"\n"), 0o644); err != nil {
		fail(err.Error())
	}
	materializeDocs(dir, filepath.Join(dir, "GUIDE.md"))
	tr := ledger.LoadLedger(dir)
	res := ledger.ValidateAll(tr)
	if len(res.Errors) > 0 {
		report("validation errors", res.Errors)
		fail("init produced an invalid ledger — this is a bug")
	}
	html := ledger.RenderHTML(tr, assets)
	if err := os.WriteFile(filepath.Join(dir, "ledger.html"), []byte(html), 0o644); err != nil {
		fail(err.Error())
	}
	fmt.Printf("initialized %s (project %s, prefix %s, toolVersion %s)\n", dir, project, prefix, ledger.RendererVersion)
	fmt.Printf("next: read %s/GUIDE.md — %s/AGENTS.md routes agents to it\n", dir, dir)
}

func loadAssets(dir string) (*ledger.ViewerAssets, error) {
	if dir != "" {
		return ledger.LoadAssets(os.DirFS(dir), ".")
	}
	return releaseAssets(), nil
}

func releaseAssets() *ledger.ViewerAssets {
	a, err := ledger.LoadAssets(root.Assets, "viewer")
	if err != nil {
		panic("embedded assets missing: " + err.Error())
	}
	return a
}
