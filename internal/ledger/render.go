package ledger

import (
	"io/fs"
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
// embedded verbatim and so change the page's bytes. 0.4.0 replaces the
// hand-written viewer with the built @codesweep-ai/ui React application.
// 0.5.0 adopts the 0.2.0 component set and its compact Markdown entry.
// 0.6.0 takes @codesweep-ai/ui from the registry rather than a committed
// tarball. 0.6.1 re-pins it to the build that stamps its own version in UTC:
// the package's src/ did not move, so the bundle is byte-identical and only
// the version this page reports about itself changes.
const RendererVersion = "0.6.1"
const UITokensVersion = "0.2.1-dev.20260901200135.3160175"

func escAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// RenderHTML builds the whole page: the viewer assets, the record data and
// the derived block, in one deterministic pass over the tracker.
func RenderHTML(tr *Tracker, assets fs.FS) string {
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
	var links []*ojson.Value
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
			links = append(links, ojson.O(ojson.Kv("label", ojson.S(label)), ojson.Kv("url", ojson.S(url))))
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
		ojson.Kv("links", &ojson.Value{Kind: ojson.Array, Arr: links}),
		ojson.Kv("commitUrlTemplate", commitTpl),
		ojson.Kv("rendererVersion", ojson.S(RendererVersion)),
		ojson.Kv("uiVersion", ojson.S(UITokensVersion)),
		ojson.Kv("lastActivity", ojson.S(lastDate)),
		ojson.Kv("derived", buildDerived(recordEntries, draftEntries, queue)),
	)
	data := payload.String()
	data = strings.ReplaceAll(data, "<", `\u003c`)
	template, err := fs.ReadFile(assets, "viewer/index.html")
	if err != nil {
		panic("embedded viewer missing: " + err.Error())
	}
	html := strings.ReplaceAll(string(template), "__LEDGER_TITLE__", escAttr(project))
	return strings.ReplaceAll(html, "__LEDGER_DATA__", data)
}

func itoa(n int) string { return strconv.Itoa(n) }
