package ledger

// A closed record cites the commit that resolved it, and a citation nobody can
// resolve is not evidence: a fabricated sha reads exactly like a real one. This
// file answers, for the shas one ledger names, which of them are commits in the
// repository that holds it (SPEC R15, R28-R30).

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/codesweep-ai/ledger/internal/ojson"
)

// shaRe is the shape a citation has to have: lower-case hexadecimal, at least
// git's seven-character abbreviation, at most a sha-256 object name. Every sha
// git prints is in this form, and anything else in an evidence array is a
// branch name, a URL or a guess.
var shaRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// hexRunRe finds a hexadecimal run standing on its own inside prose. Whether
// such a run is a sha at all is decided by looksLikeSha.
var hexRunRe = regexp.MustCompile(`[0-9a-f]{7,40}`)

// looksLikeSha reports whether a hexadecimal run reads as a commit sha rather
// than as something else that happens to be hexadecimal. It has to carry a
// digit, because a run of letters is a word (`defaced` is seven characters of
// hexadecimal), and a letter, because a run of digits is a count or a
// timestamp. A real sha fails this test about once in thirty at seven
// characters and never at forty, and a mention is only ever a warning.
func looksLikeSha(s string) bool {
	return strings.ContainsAny(s, "0123456789") && strings.ContainsAny(s, "abcdef")
}

// shasIn returns the distinct sha-shaped tokens of a prose field, in the order
// they appear. A token inside a longer word or number is not one, and neither
// is a `#`-prefixed run, which is how a colour is written.
func shasIn(text string) []string {
	var out []string
	for _, loc := range hexRunRe.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && (isWordByte(text[start-1]) || text[start-1] == '#') {
			continue
		}
		if end < len(text) && isWordByte(text[end]) {
			continue
		}
		tok := text[start:end]
		if looksLikeSha(tok) && !slices.Contains(out, tok) {
			out = append(out, tok)
		}
	}
	return out
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// mention is one sha named in a narrative field, as opposed to one cited in an
// evidence array.
type mention struct{ field, sha string }

// commitIndex holds git's verdict on every sha a ledger names. Skipped says why
// no verdict was reached, and an index that skipped answers about nothing: the
// citations are unchecked rather than wrong.
type commitIndex struct {
	Skipped string
	objects map[string]string // sha as written -> object type, "missing" or "ambiguous"
}

// live reports whether the index carries verdicts. A nil index does not, which
// is what a ledger with verifyCommits turned off has.
func (ci *commitIndex) live() bool { return ci != nil && ci.Skipped == "" }

// fault names what is wrong with a sha, and returns "" when it resolves to a
// commit. A sha the index was never asked about resolves by default: the caller
// decides what to ask, and citedShas gathers exactly what validation checks.
func (ci *commitIndex) fault(sha string) string {
	switch kind := ci.objects[sha]; kind {
	case "commit", "":
		return ""
	case "missing":
		return "is not a commit in this repository"
	case "ambiguous":
		return "matches more than one object — cite more characters"
	default:
		return "names a " + kind + ", not a commit"
	}
}

// resolveShas asks git which of shas name a commit in the repository holding
// dir. It reads that repository's object database and nothing else, runs
// nothing that writes, and never fetches, so an object that is not already on
// disk reads as missing rather than as a network round trip (SPEC R3).
func resolveShas(dir string, shas []string) *commitIndex {
	ci := &commitIndex{objects: make(map[string]string, len(shas))}
	git, err := exec.LookPath("git")
	if err != nil {
		ci.Skipped = "git is not on the PATH"
		return ci
	}
	run := func(stdin string, args ...string) (string, string, error) {
		cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_NO_LAZY_FETCH=1",   // a partial clone must not reach for what it lacks
			"GIT_TERMINAL_PROMPT=0", // and must not stop to ask for credentials
			"GIT_OPTIONAL_LOCKS=0")  // reading a ledger must not touch the index
		cmd.Stdin = strings.NewReader(stdin)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		err := cmd.Run()
		return out.String(), errOut.String(), err
	}

	out, errOut, err := run("", "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(out) != "true" {
		ci.Skipped = gitReason(errOut)
		return ci
	}
	// A shallow clone holds a slice of the history, so most shas are absent for
	// a reason that says nothing about the record citing them. CI checks out
	// this way by default, and a gate that fails there fails on everything.
	if out, _, err := run("", "rev-parse", "--is-shallow-repository"); err == nil && strings.TrimSpace(out) == "true" {
		ci.Skipped = "the clone is shallow and holds only part of the history"
		return ci
	}

	out, errOut, err = run(strings.Join(shas, "\n")+"\n", "cat-file", "--batch-check")
	if err != nil {
		ci.Skipped = gitReason(errOut)
		return ci
	}
	// One answer per line, in the order asked, each `<object> <type> <size>` or
	// `<input> missing|ambiguous`. Zipping by position rather than by name is
	// what reads an abbreviation back: git answers a full sha for one it found.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != len(shas) {
		ci.Skipped = "git answered for " + itoa(len(lines)) + " of " + itoa(len(shas)) + " shas"
		return ci
	}
	for i, line := range lines {
		kind := "missing"
		if fields := strings.Fields(line); len(fields) >= 2 {
			kind = fields[1]
		}
		ci.objects[shas[i]] = kind
	}
	return ci
}

// gitReason turns what git said into the half-sentence that follows "not
// checked". The common failure is a ledger outside any repository, which git
// reports across two lines of its own.
func gitReason(stderr string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	line = strings.TrimPrefix(strings.TrimSpace(line), "fatal: ")
	if line == "" || strings.HasPrefix(line, "not a git repository") {
		return "the ledger is not inside a git repository"
	}
	return line
}

// verifyCommits reports whether this ledger's citations resolve against the
// repository holding it. A ledger that travels apart from the code it
// describes, such as a corpus copied into another project, says so by setting
// the key to false, and its citations are then checked for shape alone.
func verifyCommits(cfg *ojson.Value) bool {
	v := cfg.Get("verifyCommits")
	return v == nil || v.Kind != ojson.Bool || v.B
}

// evidenceShas returns the well-formed shas a record cites, which is what the
// evidence arrays are for. A malformed one is left out: it is already an error
// on its shape, and passing it to git would ask a second question about the
// same mistake.
func evidenceShas(d *ojson.Value) []string {
	var out []string
	ev := d.Get("evidence")
	for _, field := range []string{"commits", "integrated"} {
		arr := ev.Get(field)
		if arr == nil || arr.Kind != ojson.Array {
			continue
		}
		for _, e := range arr.Arr {
			if s, ok := e.StrVal(); ok && shaRe.MatchString(s) && !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	return out
}

// proseMentions returns the shas a record names in its narrative rather than in
// its evidence. A record that cites one commit and discusses another has made
// two claims about the history, and both are checkable.
func proseMentions(d *ojson.Value) []mention {
	if d == nil || d.Kind != ojson.Object {
		return nil
	}
	cited := evidenceShas(d)
	var out []mention
	add := func(field string, v *ojson.Value) {
		s, ok := v.StrVal()
		if !ok {
			return
		}
		for _, sha := range shasIn(s) {
			if slices.Contains(cited, sha) {
				continue // already checked as a citation
			}
			if !slices.ContainsFunc(out, func(m mention) bool { return m.sha == sha }) {
				out = append(out, mention{field, sha})
			}
		}
	}
	add("title", d.Get("title"))
	add("details", d.Get("details"))
	add("resolution", d.Get("resolution"))
	add("evidence.verified", d.Get("evidence").Get("verified"))
	if notes := d.Get("notes"); notes != nil && notes.Kind == ojson.Array {
		for i, n := range notes.Arr {
			add("notes["+itoa(i)+"].text", n.Get("text"))
		}
	}
	return out
}

// queueMentions returns the shas the queue's rationales name.
func queueMentions(q *ojson.Value) []mention {
	items := q.Get("items")
	if items == nil || items.Kind != ojson.Array {
		return nil
	}
	var out []mention
	for i, it := range items.Arr {
		s, ok := it.Get("why").StrVal()
		if !ok {
			continue
		}
		for _, sha := range shasIn(s) {
			out = append(out, mention{"items[" + itoa(i) + "].why", sha})
		}
	}
	return out
}

// citedShas gathers every sha a ledger names, so that one git process answers
// for the whole directory rather than one per record.
func citedShas(tr *Tracker) []string {
	var out []string
	add := func(sha string) {
		if !slices.Contains(out, sha) {
			out = append(out, sha)
		}
	}
	for _, e := range slices.Concat(tr.Records, tr.Drafts) {
		for _, sha := range evidenceShas(e.Data) {
			add(sha)
		}
		for _, m := range proseMentions(e.Data) {
			add(m.sha)
		}
	}
	for _, m := range queueMentions(tr.Queue) {
		add(m.sha)
	}
	return out
}
