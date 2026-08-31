package ledger

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/codesweep-ai/ledger/internal/ojson"
)

var Types = []string{"defect", "improvement", "feature-idea"}
var Severities = []string{"low", "med", "high", "critical"}
var Statuses = []string{"open", "in-progress", "closed", "wont-fix", "moved-to-roadmap"}
var Terminal = []string{"closed", "wont-fix", "moved-to-roadmap"}
var RecordKeys = []string{"id", "title", "type", "severity", "status", "foundBy", "opened",
	"resolved", "stint", "evidence", "resolution", "details", "notes", "links"}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var anyIDRe = regexp.MustCompile(`^[A-Z]{2,6}-\d{3,}$`)
var prefixRe = regexp.MustCompile(`^[A-Z]{2,6}$`)
var schemeRe = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*:`)
var httpRe = regexp.MustCompile(`(?i)^https?:`)

// ValidPrefix reports whether s is a well-formed idPrefix. init checks a
// candidate with it before scaffolding anything, so the rule has one definition
// rather than one per caller.
func ValidPrefix(s string) bool { return prefixRe.MatchString(s) }

func isDateStr(v *ojson.Value) bool {
	s, ok := v.StrVal()
	if !ok || !dateRe.MatchString(s) {
		return false
	}
	t, err := time.Parse("2006-01-02", s)
	return err == nil && t.Format("2006-01-02") == s
}

func isStr(v *ojson.Value) bool { return v != nil && v.Kind == ojson.String }

func nonEmpty(v *ojson.Value) bool {
	s, ok := v.StrVal()
	return ok && strings.TrimSpace(s) != ""
}

func strArr(v *ojson.Value) bool {
	if v == nil || v.Kind != ojson.Array {
		return false
	}
	for _, e := range v.Arr {
		if !nonEmpty(e) {
			return false
		}
	}
	return true
}

type Result struct {
	Errors   []string
	Warnings []string
}

func ValidateAll(tr *Tracker) *Result {
	res := &Result{}
	E := func(f, m string) { res.Errors = append(res.Errors, f+": "+m) }
	W := func(f, m string) { res.Warnings = append(res.Warnings, f+": "+m) }

	if tr.ConfigError != "" {
		E("ledger.json", strings.TrimPrefix(tr.ConfigError, "ledger.json: "))
	}
	cfg := tr.Config
	prefix := "XXX"
	if cfg != nil {
		if !nonEmpty(cfg.Get("project")) {
			E("ledger.json", "project must be a non-empty string")
		}
		if p, ok := cfg.Get("idPrefix").StrVal(); !ok || !prefixRe.MatchString(p) {
			E("ledger.json", "idPrefix must match ^[A-Z]{2,6}$")
		} else {
			prefix = p
		}
		if sv, _ := cfg.Get("schemaVersion").StrVal(); sv != "issue.v1" {
			E("ledger.json", `schemaVersion must be "issue.v1"`)
		}
		if sad := cfg.Get("staleAfterDays"); sad != nil && (sad.Kind != ojson.Number || sad.Num < 0) {
			E("ledger.json", "staleAfterDays must be a number >= 0")
		}
		if d := cfg.Get("description"); cfg.Has("description") && !nonEmpty(d) {
			E("ledger.json", "description, when present, must be a non-empty string")
		}
		if cfg.Has("commitUrlTemplate") {
			tpl, ok := cfg.Get("commitUrlTemplate").StrVal()
			switch {
			case !ok || strings.TrimSpace(tpl) == "":
				E("ledger.json", "commitUrlTemplate, when present, must be a non-empty string")
			case !httpRe.MatchString(tpl):
				E("ledger.json", "commitUrlTemplate must be an http(s) url")
			case !strings.Contains(tpl, "{sha}"):
				E("ledger.json", "commitUrlTemplate must contain the {sha} placeholder")
			}
		}
		if v := cfg.Get("verifyCommits"); cfg.Has("verifyCommits") && (v == nil || v.Kind != ojson.Bool) {
			E("ledger.json", "verifyCommits, when present, must be true or false")
		}
		if cfg.Has("links") {
			links := cfg.Get("links")
			if links == nil || links.Kind != ojson.Array {
				E("ledger.json", "links must be an array of {label, url}")
			} else {
				for i, l := range links.Arr {
					if l == nil || l.Kind != ojson.Object || !nonEmpty(l.Get("label")) || !nonEmpty(l.Get("url")) {
						E("ledger.json", fmt.Sprintf("links[%d] must be {label, url} with non-empty strings", i))
					} else if u, _ := l.Get("url").StrVal(); schemeRe.MatchString(u) && !httpRe.MatchString(u) {
						E("ledger.json", fmt.Sprintf("links[%d] url must be http(s) or a relative path", i))
					}
				}
			}
		}
	}
	idRe := regexp.MustCompile("^" + prefix + `-\d{3,}$`)

	// Every sha the ledger names, resolved against the repository holding it in
	// one git process for the whole directory (SPEC R28). A ledger that travels
	// apart from the code it describes sets verifyCommits to false and keeps the
	// shape check alone; an environment that cannot answer says so and fails
	// nothing (SPEC R29).
	var commits *commitIndex
	if cited := citedShas(tr); len(cited) > 0 && verifyCommits(cfg) {
		commits = resolveShas(tr.Dir, cited)
		if !commits.live() {
			W("evidence", "commit citations not checked against the repository — "+commits.Skipped)
		}
	}

	// A citation has to be a sha, and it has to be one this repository holds.
	// The shape is a property of the record and is checked always; resolution
	// needs the repository and is checked when there is one to ask.
	checkCitations := func(file, field string, arr *ojson.Value) {
		for i, e := range arr.Arr {
			sha, _ := e.StrVal()
			switch {
			case !shaRe.MatchString(sha):
				E(file, fmt.Sprintf("%s[%d] %q is not a commit sha (lower-case hexadecimal, at least 7 characters)", field, i, sha))
			case commits.live():
				if fault := commits.fault(sha); fault != "" {
					E(file, fmt.Sprintf("%s[%d] %s %s", field, i, sha, fault))
				}
			}
		}
	}

	ids := map[string]bool{}
	for _, r := range tr.Records {
		if r.Data != nil {
			if id, ok := r.Data.Get("id").StrVal(); ok {
				if ids[id] {
					E(r.File, "duplicate id "+id)
				}
				ids[id] = true
			}
		}
	}

	checkShape := func(file string, d *ojson.Value, isDraft bool) {
		if d == nil || d.Kind != ojson.Object {
			E(file, "record must be a JSON object")
			return
		}
		for _, k := range RecordKeys {
			if k == "id" && isDraft {
				continue
			}
			if !d.Has(k) {
				E(file, `missing required field "`+k+`"`)
			}
		}
		for _, k := range d.Keys() {
			if !slices.Contains(RecordKeys, k) {
				W(file, `unknown field "`+k+`" (additive fields are allowed; check for typos)`)
			}
		}
		if isDraft && d.Has("id") {
			E(file, "drafts must not carry an id — ids are minted on the integration branch (SPEC R33)")
		}
		if !isDraft && d.Has("id") {
			id, ok := d.Get("id").StrVal()
			if !ok || !idRe.MatchString(id) {
				E(file, "id must match "+jsRegExpString(prefix))
			} else if base := strings.TrimSuffix(file[strings.LastIndex(file, "/")+1:], ".json"); base != id {
				E(file, "filename must equal id ("+id+")")
			}
		}
		if d.Has("title") && !nonEmpty(d.Get("title")) {
			E(file, "title must be a non-empty string")
		}
		if d.Has("type") {
			if t, _ := d.Get("type").StrVal(); !slices.Contains(Types, t) {
				E(file, "type must be one of "+strings.Join(Types, "|"))
			}
		}
		if d.Has("severity") {
			if s, _ := d.Get("severity").StrVal(); !slices.Contains(Severities, s) {
				E(file, "severity must be one of "+strings.Join(Severities, "|"))
			}
		}
		if d.Has("status") {
			if s, _ := d.Get("status").StrVal(); !slices.Contains(Statuses, s) {
				E(file, "status must be one of "+strings.Join(Statuses, "|"))
			}
		}
		if d.Has("foundBy") && !nonEmpty(d.Get("foundBy")) {
			E(file, "foundBy must be a non-empty string")
		}
		if d.Has("opened") && !isDateStr(d.Get("opened")) {
			E(file, "opened must be YYYY-MM-DD")
		}
		if r := d.Get("resolved"); d.Has("resolved") && !r.IsNull() && !isDateStr(r) {
			E(file, "resolved must be null or YYYY-MM-DD")
		}
		if s := d.Get("stint"); d.Has("stint") && !s.IsNull() && !nonEmpty(s) {
			E(file, "stint must be null or a non-empty string")
		}
		if r := d.Get("resolution"); d.Has("resolution") && !r.IsNull() && !isStr(r) {
			E(file, "resolution must be null or a string")
		}
		if d.Has("details") {
			det := d.Get("details")
			if !isStr(det) {
				E(file, "details must be a string")
			} else if strings.TrimSpace(det.Str) == "" {
				W(file, "details is empty — records deserve a narrative")
			}
		}
		if d.Has("evidence") {
			ev := d.Get("evidence")
			if ev == nil || ev.Kind != ojson.Object {
				E(file, "evidence must be an object {commits, integrated, verified}")
			} else {
				for _, field := range []string{"commits", "integrated"} {
					if !strArr(ev.Get(field)) {
						E(file, "evidence."+field+" must be an array of non-empty strings")
						continue
					}
					checkCitations(file, "evidence."+field, ev.Get(field))
				}
				if v := ev.Get("verified"); !v.IsNull() && !isStr(v) {
					E(file, "evidence.verified must be null or a string")
				}
			}
		}
		// A sha named in the narrative is a claim about the history too, but
		// reading one out of prose is a guess about what the words meant, so an
		// unresolvable mention is a warning (SPEC R30).
		if commits.live() {
			for _, m := range proseMentions(d) {
				if fault := commits.fault(m.sha); fault != "" {
					W(file, m.field+" mentions "+m.sha+", which "+fault)
				}
			}
		}
		if d.Has("notes") {
			notes := d.Get("notes")
			if notes == nil || notes.Kind != ojson.Array {
				E(file, "notes must be an array")
			} else {
				prev := ""
				for idx, n := range notes.Arr {
					if n == nil || n.Kind != ojson.Object || !isDateStr(n.Get("date")) || !nonEmpty(n.Get("text")) {
						E(file, fmt.Sprintf("notes[%d] must be {date: YYYY-MM-DD, text: non-empty string}", idx))
						continue
					}
					date, _ := n.Get("date").StrVal()
					if prev != "" && date < prev {
						E(file, fmt.Sprintf("notes[%d] date decreases (notes are an append-only, dated timeline)", idx))
					}
					prev = date
				}
			}
		}
		if d.Has("links") {
			links := d.Get("links")
			ok := links != nil && links.Kind == ojson.Array
			if ok {
				for _, l := range links.Arr {
					if !isStr(l) {
						ok = false
						break
					}
				}
			}
			if !ok {
				E(file, "links must be an array of id strings")
			} else {
				for _, l := range links.Arr {
					s, _ := l.StrVal()
					if !anyIDRe.MatchString(s) {
						E(file, `links entry "`+s+`" is not a well-formed id`)
					} else if !ids[s] {
						W(file, `link "`+s+`" does not resolve to a record in this tracker (allowed during partial migration)`)
					}
				}
			}
		}
		// lifecycle (SPEC §6)
		st, _ := d.Get("status").StrVal()
		terminal := slices.Contains(Terminal, st)
		if slices.Contains(Statuses, st) {
			if terminal && d.Get("resolved").IsNull() {
				E(file, `status "`+st+`" requires a resolved date`)
			}
			if !terminal && isStr(d.Get("resolved")) {
				E(file, `status "`+st+`" must have resolved: null`)
			}
			if st == "in-progress" && d.Has("stint") && d.Get("stint").IsNull() {
				E(file, `status "in-progress" requires a stint reference`)
			}
			if ev := d.Get("evidence"); st == "closed" && ev != nil && ev.Kind == ojson.Object {
				if !nonEmpty(ev.Get("verified")) {
					E(file, `status "closed" requires non-empty evidence.verified`)
				}
				commits := ev.Get("commits")
				links := d.Get("links")
				hasCommits := commits != nil && commits.Kind == ojson.Array && len(commits.Arr) > 0
				hasLinks := links != nil && links.Kind == ojson.Array && len(links.Arr) > 0
				if !hasCommits && !hasLinks {
					E(file, `status "closed" requires evidence.commits, or links to the closing issues (delegation closure)`)
				}
			}
			if (st == "wont-fix" || st == "moved-to-roadmap") && !nonEmpty(d.Get("resolution")) {
				E(file, `status "`+st+`" requires a resolution explaining why`)
			}
			if isDraft && terminal {
				W(file, "a draft with a terminal status is unusual — promote it to a numbered issue instead")
			}
		}
	}

	for _, r := range tr.Records {
		if r.Err != "" {
			E(r.File, "JSON parse error: "+r.Err)
			continue
		}
		checkShape(r.File, r.Data, false)
	}
	for _, dft := range tr.Drafts {
		if dft.Err != "" {
			E(dft.File, "JSON parse error: "+dft.Err)
			continue
		}
		checkShape(dft.File, dft.Data, true)
	}

	// queue.json (optional): the ordered "fix next" recommendation (SPEC §5)
	if tr.QueueError != "" {
		E("queue.json", tr.QueueError)
	} else if tr.QueuePresent {
		q := tr.Queue
		if q == nil || q.Kind != ojson.Object {
			E("queue.json", "must be an object {recommendedBy, updated, items}")
		} else {
			if !nonEmpty(q.Get("recommendedBy")) {
				E("queue.json", "recommendedBy must be a non-empty string")
			}
			if !isDateStr(q.Get("updated")) {
				E("queue.json", "updated must be YYYY-MM-DD")
			}
			for _, k := range q.Keys() {
				if k != "recommendedBy" && k != "updated" && k != "items" {
					W("queue.json", `unknown field "`+k+`"`)
				}
			}
			items := q.Get("items")
			if items == nil || items.Kind != ojson.Array {
				E("queue.json", "items must be an array")
			} else {
				statusOf := map[string]string{}
				for _, r := range tr.Records {
					if r.Data != nil {
						if id, ok := r.Data.Get("id").StrVal(); ok {
							st, _ := r.Data.Get("status").StrVal()
							statusOf[id] = st
						}
					}
				}
				seen := map[string]bool{}
				for i, it := range items.Arr {
					id, idOK := it.Get("id").StrVal()
					if it == nil || it.Kind != ojson.Object || !idOK || !nonEmpty(it.Get("why")) {
						E("queue.json", fmt.Sprintf("items[%d] must be {id, why: non-empty string}", i))
						continue
					}
					if seen[id] {
						E("queue.json", fmt.Sprintf("items[%d] duplicate id %s", i, id))
					}
					seen[id] = true
					st, known := statusOf[id]
					switch {
					case !known:
						E("queue.json", fmt.Sprintf("items[%d] unknown id %s", i, id))
					case slices.Contains(Terminal, st):
						E("queue.json", fmt.Sprintf("items[%d] %s is %s — cannot recommend fixing a terminal issue", i, id, st))
					case st == "in-progress":
						W("queue.json", fmt.Sprintf("items[%d] %s is already in progress — recommendation is redundant", i, id))
					}
				}
				if commits.live() {
					for _, m := range queueMentions(q) {
						if fault := commits.fault(m.sha); fault != "" {
							W("queue.json", m.field+" mentions "+m.sha+", which "+fault)
						}
					}
				}
			}
		}
	}

	// Derived triage signal (agent-facing mirror of the brief's needs-you block):
	// an OPEN critical absent from the queue means nobody scheduled it and nobody
	// is working it (SPEC R22).
	queuedIDs := map[string]bool{}
	if tr.Queue != nil {
		if items := tr.Queue.Get("items"); items != nil && items.Kind == ojson.Array {
			for _, it := range items.Arr {
				if id, ok := it.Get("id").StrVal(); ok {
					queuedIDs[id] = true
				}
			}
		}
	}
	for _, r := range tr.Records {
		if r.Data == nil {
			continue
		}
		id, ok := r.Data.Get("id").StrVal()
		if !ok {
			continue
		}
		st, _ := r.Data.Get("status").StrVal()
		sev, _ := r.Data.Get("severity").StrVal()
		typ, _ := r.Data.Get("type").StrVal()
		if st == "open" && sev == "critical" && typ != "feature-idea" && !queuedIDs[id] {
			W(r.File, id+" is an open critical missing from the queue — needs triage")
		}
	}

	return res
}

// jsRegExpString reproduces how the JS error message stringifies the id RegExp.
func jsRegExpString(prefix string) string {
	return "/^" + prefix + `-\d{3,}$/`
}
