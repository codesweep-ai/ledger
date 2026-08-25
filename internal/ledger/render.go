package ledger

import (
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/codesweep-ai/ledger/internal/ojson"
)

// RendererVersion changes WHENEVER rendered output can change — render code
// or any embedded viewer asset. A same-version binary pair that renders
// different bytes makes the toolVersion pin lie, and makes check misreport
// STALE). 0.3.0: first cs-ledger release — derived-data block, the earlier
// JavaScript renderer retired. 0.3.1: evidence-sha links via
// commitUrlTemplate. 0.3.2: viewer stylesheet comments, which are
// embedded verbatim and so change the page's bytes. 0.3.3: viewer
// JavaScript comments, embedded the same way.
const RendererVersion = "0.3.3"
const UITokensVersion = "1.12.0"

// DevStamp marks HTML rendered from --assets (dev mode); check refuses it.
const DevStamp = "v" + RendererVersion + "-dev"

// ViewerAssets holds the viewer files inlined into the rendered page.
// Dev marks assets loaded from disk (--assets): the output gets dev-stamped
// so it can never satisfy the freshness gate.
type ViewerAssets struct {
	Tokens    string
	Base      string
	CSS       string
	ThemeInit string
	JS        string
	Dev       bool
}

// LoadAssets reads the five viewer files from fsys under root (e.g. the
// embedded copy with root "viewer", or an --assets dev directory with ".").
func LoadAssets(fsys fs.FS, root string) (*ViewerAssets, error) {
	read := func(name string) (string, error) {
		b, err := fs.ReadFile(fsys, path.Join(root, name))
		return string(b), err
	}
	a := &ViewerAssets{}
	var err error
	if a.Tokens, err = read("tokens.css"); err != nil {
		return nil, err
	}
	if a.Base, err = read("base.css"); err != nil {
		return nil, err
	}
	if a.CSS, err = read("viewer.css"); err != nil {
		return nil, err
	}
	if a.ThemeInit, err = read("theme-init.js"); err != nil {
		return nil, err
	}
	if a.JS, err = read("viewer.js"); err != nil {
		return nil, err
	}
	return a, nil
}

func escAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// RenderHTML builds the whole page: the viewer assets, the record data and
// the derived block, in one deterministic pass over the tracker.
func RenderHTML(tr *Tracker, assets *ViewerAssets) string {
	cfg := tr.Config
	project := "unknown-project"
	if p, ok := cfg.Get("project").StrVal(); ok {
		project = p
	}
	staleAfterDays := ojson.N(14)
	if sad := cfg.Get("staleAfterDays"); sad != nil && sad.Kind == ojson.Number {
		staleAfterDays = sad
	}

	var records []*ojson.Value
	var recordEntries []*Entry
	for _, r := range tr.Records {
		if r.Err == "" && r.Data != nil && r.Data.Kind == ojson.Object {
			if _, ok := r.Data.Get("id").StrVal(); ok {
				records = append(records, r.Data)
				recordEntries = append(recordEntries, r)
			}
		}
	}
	// sort by id ascending (byte order, matching JS string comparison on ASCII ids)
	for i := 1; i < len(records); i++ {
		for j := i; j > 0; j-- {
			a, _ := records[j-1].Get("id").StrVal()
			b, _ := records[j].Get("id").StrVal()
			if a <= b {
				break
			}
			records[j-1], records[j] = records[j], records[j-1]
		}
	}

	var drafts []*ojson.Value
	var draftEntries []*Entry
	for _, d := range tr.Drafts {
		if d.Err != "" || d.Data == nil || d.Data.Kind != ojson.Object {
			continue
		}
		draftEntries = append(draftEntries, d)
		merged := &ojson.Value{Kind: ojson.Object}
		merged.Obj = append(merged.Obj,
			ojson.Kv("member", ojson.S(d.Member)),
			ojson.Kv("slug", ojson.S(d.Slug)))
		for _, kv := range d.Data.Obj {
			switch kv.Key {
			case "member":
				merged.Obj[0].Val = kv.Val
			case "slug":
				merged.Obj[1].Val = kv.Val
			default:
				merged.Obj = append(merged.Obj, kv)
			}
		}
		drafts = append(drafts, merged)
	}

	lastDate := ""
	seeDate := func(v *ojson.Value) {
		if s, ok := v.StrVal(); ok && s > lastDate {
			lastDate = s
		}
	}
	for _, r := range append(append([]*ojson.Value{}, records...), drafts...) {
		seeDate(r.Get("opened"))
		seeDate(r.Get("resolved"))
		if notes := r.Get("notes"); notes != nil && notes.Kind == ojson.Array {
			for _, n := range notes.Arr {
				if n != nil {
					seeDate(n.Get("date"))
				}
			}
		}
	}

	queue := ojson.Nil()
	if tr.Queue != nil && tr.Queue.Kind == ojson.Object {
		queue = tr.Queue
	}
	description := ojson.Nil()
	if d, ok := cfg.Get("description").StrVal(); ok && strings.TrimSpace(d) != "" {
		description = ojson.S(strings.TrimSpace(d))
	}
	type link struct{ label, url string }
	var links []link
	if ls := cfg.Get("links"); ls != nil && ls.Kind == ojson.Array {
		for _, l := range ls.Arr {
			if l == nil || l.Kind != ojson.Object {
				continue
			}
			label, lok := l.Get("label").StrVal()
			url, uok := l.Get("url").StrVal()
			if !lok || !uok || strings.TrimSpace(label) == "" || strings.TrimSpace(url) == "" {
				continue
			}
			if schemeRe.MatchString(url) && !httpRe.MatchString(url) {
				continue
			}
			links = append(links, link{label, url})
		}
	}

	commitTpl := ojson.Nil()
	if tpl, ok := cfg.Get("commitUrlTemplate").StrVal(); ok && strings.TrimSpace(tpl) != "" {
		commitTpl = ojson.S(tpl)
	}
	payload := ojson.O(
		ojson.Kv("project", ojson.S(project)),
		ojson.Kv("staleAfterDays", staleAfterDays),
		ojson.Kv("records", &ojson.Value{Kind: ojson.Array, Arr: records}),
		ojson.Kv("drafts", &ojson.Value{Kind: ojson.Array, Arr: drafts}),
		ojson.Kv("queue", queue),
		ojson.Kv("description", description),
		ojson.Kv("commitUrlTemplate", commitTpl),
		ojson.Kv("derived", buildDerived(recordEntries, draftEntries, queue)),
	)
	json := strings.ReplaceAll(payload.String(), "<", `\u003c`)

	masthead := ""
	if description.Kind == ojson.String || len(links) > 0 {
		masthead = `<div class="masthead">`
		if description.Kind == ojson.String {
			masthead += `<div class="bdesc">` + escAttr(description.Str) + `</div>`
		}
		if len(links) > 0 {
			masthead += `<div class="mastlinks">`
			var mastheadSb196 strings.Builder
			for _, l := range links {
				mastheadSb196.WriteString(`<a class="plink" href="` + escAttr(l.url) + `">` + escAttr(l.label) + `</a>`)
			}
			masthead += mastheadSb196.String()
			masthead += `</div>`
		}
		masthead += `</div>`
	}

	lastActivity := ""
	if lastDate != "" {
		lastActivity = " · last activity " + lastDate
	}
	versionWord := "v" + RendererVersion
	if assets.Dev {
		versionWord = DevStamp
	}

	lines := []string{
		`<!doctype html>`,
		`<html lang="en" data-theme="dark">`,
		`<head>`,
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<title>` + escAttr(project) + ` · ledger</title>`,
		`<script>` + assets.ThemeInit + `</scr` + `ipt>`,
		`<style>`,
		assets.Tokens,
		assets.Base,
		assets.CSS,
		`</style>`,
		`</head>`,
		`<body>`,
		`<header class="hdr">`,
		`<span class="brandbar"></span>`,
		`<span class="htitle"><span class="proj">` + escAttr(project) + `</span> · ledger</span>`,
		`<span class="hspace"></span>`,
		`<button id="themeBtn" class="tbtn">theme</button>`,
		`</header>`,
		`<main>`,
		`<div class="controls viewbar"><span class="chipset" id="viewChips"><span class="cl">view</span>`,
		`<button class="chip on" id="viewBrief">brief</button>`,
		`<button class="chip" id="viewBoard">board</button>`,
		`<button class="chip" id="viewList">list</button>`,
		`<button class="chip" id="viewActivity">activity</button></span>`,
		`<button id="helpBtn" class="chip aux" title="how to read this ledger">?</button></div>`,
		masthead,
		`<div class="controls filterbar">`,
		`<input id="q" class="search" type="search" placeholder="search id, title, details&hellip;">`,
		`<button id="cycStatus" class="chip cyc" title="click to cycle">status · all</button>`,
		`<button id="cycType" class="chip cyc" title="click to cycle">type · all</button>`,
		`<button id="cycSev" class="chip cyc" title="click to cycle">sev · all</button>`,
		`<button id="staleBtn" class="chip aux">stale only</button>`,
		`<button id="resetChip" class="chip aux" disabled>reset</button>`,
		`<span class="arrange listonly">`,
		`<button id="cycGroup" class="chip cyc2" title="click to cycle">group · status</button>`,
		`<button id="cycSort" class="chip cyc2" title="click to cycle">sort · id</button></span>`,
		`<button id="newBtn" class="chip aux" hidden>new since last visit</button>`,
		`</div>`,
		`<div id="groups"></div>`,
		`<footer class="foot">rendered by cs-ledger ` + versionWord +
			` · @codesweep-ai/ui tokens v` + UITokensVersion +
			` · ` + itoa(len(records)) + ` issues · ` + itoa(len(drafts)) + ` drafts` +
			lastActivity + `</footer>`,
		`</main>`,
		`<div id="modal" class="modalwrap" hidden><div class="modalcard"><button id="modalx" class="modalx" title="close (Esc)">&times;</button><div id="modalbody"></div></div></div>`,
		`<div id="ctip"></div>`,
		`<script id="ledger-data" type="application/json">` + json + `</scr` + `ipt>`,
		`<script>`,
		assets.JS,
		`</scr` + `ipt>`,
		`</body>`,
		`</html>`,
		``,
	}
	return strings.Join(lines, "\n")
}

func itoa(n int) string { return strconv.Itoa(n) }
