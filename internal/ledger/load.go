// Package ledger implements the cs-ledger tool: loading, validation, and
// rendering of ledger/ directories (SPEC.md; schema/issue.v1.json).
package ledger

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codesweep-ai/ledger/internal/ojson"
)

type Entry struct {
	File   string // repo-relative-ish display path (issues/<f> or drafts/<m>/<f>)
	Base   string // filename without .json
	Member string // drafts only
	Slug   string // drafts only
	Data   *ojson.Value
	Err    string // JSON parse error, if any
}

type Tracker struct {
	Dir          string
	Config       *ojson.Value
	ConfigError  string
	Queue        *ojson.Value // nil when file absent or unparseable
	QueuePresent bool
	QueueError   string
	Records      []*Entry
	Drafts       []*Entry
}

func LoadLedger(dir string) *Tracker {
	tr := &Tracker{Dir: dir}
	cfgPath := filepath.Join(dir, "ledger.json")
	if data, err := os.ReadFile(cfgPath); err != nil {
		tr.ConfigError = "ledger.json: missing"
	} else if v, perr := ojson.Parse(data); perr != nil {
		tr.ConfigError = "ledger.json: " + perr.Error()
	} else {
		tr.Config = v
	}
	queuePath := filepath.Join(dir, "queue.json")
	if data, err := os.ReadFile(queuePath); err == nil {
		tr.QueuePresent = true
		if v, perr := ojson.Parse(data); perr != nil {
			tr.QueueError = "JSON parse error: " + perr.Error()
		} else {
			tr.Queue = v
		}
	}
	tr.Records = readDir(filepath.Join(dir, "issues"), "issues/")
	draftsDir := filepath.Join(dir, "drafts")
	if members, err := os.ReadDir(draftsDir); err == nil {
		names := make([]string, 0, len(members))
		for _, m := range members {
			names = append(names, m.Name())
		}
		sort.Strings(names)
		for _, member := range names {
			mDir := filepath.Join(draftsDir, member)
			if st, err := os.Stat(mDir); err != nil || !st.IsDir() {
				continue
			}
			for _, e := range readDir(mDir, "drafts/"+member+"/") {
				e.Member = member
				e.Slug = e.Base
				tr.Drafts = append(tr.Drafts, e)
			}
		}
	}
	return tr
}

func readDir(dir, prefix string) []*Entry {
	var out []*Entry
	files, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		e := &Entry{File: prefix + name, Base: strings.TrimSuffix(name, ".json")}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			e.Err = err.Error()
		} else if v, perr := ojson.Parse(data); perr != nil {
			e.Err = perr.Error()
		} else {
			e.Data = v
		}
		out = append(out, e)
	}
	return out
}
