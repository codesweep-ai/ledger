package ledger

import (
	"regexp"
	"sort"
	"strings"

	"github.com/codesweep-ai/ledger/internal/ojson"
)

// Derived computes every data-only derivation the viewer displays, so the
// derivation logic lives exactly once, in one place rather than two: the
// browser keeps only now/visitor-dependent computations (stale ages, new
// badges, the trend window anchor), each a one-line comparison over values
// emitted here.

var mdStrip = regexp.MustCompile("[#*`>|]")
var wsRun = regexp.MustCompile(`\s+`)

func keyOf(e *Entry) string {
	if e.Data != nil {
		if id, ok := e.Data.Get("id").StrVal(); ok {
			return id
		}
	}
	return "draft:" + e.Member + "/" + e.Slug
}

func lastActivityOf(d *ojson.Value) string {
	last := ""
	if s, ok := d.Get("opened").StrVal(); ok {
		last = s
	}
	if s, ok := d.Get("resolved").StrVal(); ok && s > last {
		last = s
	}
	if notes := d.Get("notes"); notes != nil && notes.Kind == ojson.Array {
		for _, n := range notes.Arr {
			if s, ok := n.Get("date").StrVal(); ok && s > last {
				last = s
			}
		}
	}
	return last
}

func noteExcerpt(text string) string {
	s := mdStrip.ReplaceAllString(text, "")
	s = strings.TrimSpace(wsRun.ReplaceAllString(s, " "))
	r := []rune(s)
	if len(r) > 120 {
		r = r[:120]
	}
	return string(r)
}

type event struct {
	d, kind, k, x string
}

// buildDerived returns the payload's `derived` member.
func buildDerived(entries []*Entry, draftEntries []*Entry, queue *ojson.Value) *ojson.Value {
	terminal := map[string]bool{}
	for _, s := range Terminal {
		terminal[s] = true
	}

	lastAct := &ojson.Value{Kind: ojson.Object}
	var events []event
	daily := map[string]*struct{ f, x int }{}
	firstDate := ""

	all := append(append([]*Entry{}, entries...), draftEntries...)
	for _, e := range all {
		d := e.Data
		k := keyOf(e)
		lastAct.Obj = append(lastAct.Obj, ojson.Kv(k, ojson.S(lastActivityOf(d))))
		st, _ := d.Get("status").StrVal()
		if opened, ok := d.Get("opened").StrVal(); ok && opened != "" {
			events = append(events, event{opened, "opened", k, ""})
		}
		if notes := d.Get("notes"); notes != nil && notes.Kind == ojson.Array {
			for _, n := range notes.Arr {
				date, dok := n.Get("date").StrVal()
				text, tok := n.Get("text").StrVal()
				if dok && tok {
					events = append(events, event{date, "note", k, noteExcerpt(text)})
				}
			}
		}
		if resolved, ok := d.Get("resolved").StrVal(); ok && terminal[st] && resolved != "" {
			events = append(events, event{resolved, st, k, ""})
		}
	}
	// trend buckets: records only (drafts have no ids yet; mirrors the
	// historical sparkline which iterated recs)
	for _, e := range entries {
		d := e.Data
		st, _ := d.Get("status").StrVal()
		if opened, ok := d.Get("opened").StrVal(); ok && opened != "" {
			if daily[opened] == nil {
				daily[opened] = &struct{ f, x int }{}
			}
			daily[opened].f++
			if firstDate == "" || opened < firstDate {
				firstDate = opened
			}
		}
		if resolved, ok := d.Get("resolved").StrVal(); ok && terminal[st] && resolved != "" {
			if daily[resolved] == nil {
				daily[resolved] = &struct{ f, x int }{}
			}
			daily[resolved].x++
			if firstDate == "" || resolved < firstDate {
				firstDate = resolved
			}
		}
	}

	// newest-first; ties broken by key ascending (the viewer's historical order)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].d != events[j].d {
			return events[i].d > events[j].d
		}
		return events[i].k < events[j].k
	})

	evArr := &ojson.Value{Kind: ojson.Array}
	for _, ev := range events {
		o := ojson.O(
			ojson.Kv("d", ojson.S(ev.d)),
			ojson.Kv("kind", ojson.S(ev.kind)),
			ojson.Kv("k", ojson.S(ev.k)),
			ojson.Kv("x", ojson.S(ev.x)),
		)
		evArr.Arr = append(evArr.Arr, o)
	}

	dailyObj := &ojson.Value{Kind: ojson.Object}
	var dates []string
	for d := range daily {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	for _, d := range dates {
		dailyObj.Obj = append(dailyObj.Obj, ojson.Kv(d, ojson.O(
			ojson.Kv("f", ojson.N(float64(daily[d].f))),
			ojson.Kv("x", ojson.N(float64(daily[d].x))),
		)))
	}

	queuePredates := 0
	queuedIDs := map[string]bool{}
	if queue != nil && queue.Kind == ojson.Object {
		if updated, ok := queue.Get("updated").StrVal(); ok {
			for _, ev := range events {
				if ev.d > updated {
					queuePredates++
				}
			}
		}
		if items := queue.Get("items"); items != nil && items.Kind == ojson.Array {
			for _, it := range items.Arr {
				if id, ok := it.Get("id").StrVal(); ok {
					queuedIDs[id] = true
				}
			}
		}
	}

	// OPEN criticals only, which is what the check warning means: in-progress
	// criticals are claimed work, visible via stint, not a triage gap. The
	// viewer's needs-you block historically included them; single-brain
	// unification resolves the divergence in check's favor.
	unqueued := &ojson.Value{Kind: ojson.Array}
	for _, e := range entries {
		d := e.Data
		id, _ := d.Get("id").StrVal()
		st, _ := d.Get("status").StrVal()
		sev, _ := d.Get("severity").StrVal()
		typ, _ := d.Get("type").StrVal()
		if st == "open" && sev == "critical" && typ != "feature-idea" && !queuedIDs[id] {
			unqueued.Arr = append(unqueued.Arr, ojson.S(id))
		}
	}

	awaiting := &ojson.Value{Kind: ojson.Array}
	for _, e := range draftEntries {
		title, ok := e.Data.Get("title").StrVal()
		if !ok || title == "" {
			title = e.Slug
		}
		awaiting.Arr = append(awaiting.Arr, ojson.O(
			ojson.Kv("k", ojson.S(keyOf(e))),
			ojson.Kv("title", ojson.S(title)),
		))
	}

	terminalArr := &ojson.Value{Kind: ojson.Array}
	for _, s := range Terminal {
		terminalArr.Arr = append(terminalArr.Arr, ojson.S(s))
	}

	first := ojson.Nil()
	if firstDate != "" {
		first = ojson.S(firstDate)
	}
	return ojson.O(
		ojson.Kv("terminalStatuses", terminalArr),
		ojson.Kv("lastActivity", lastAct),
		ojson.Kv("events", evArr),
		ojson.Kv("daily", dailyObj),
		ojson.Kv("firstEventDate", first),
		ojson.Kv("queuePredates", ojson.N(float64(queuePredates))),
		ojson.Kv("unqueuedCriticals", unqueued),
		ojson.Kv("draftsAwaiting", awaiting),
	)
}
