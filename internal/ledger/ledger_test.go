package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	root "github.com/codesweep-ai/ledger"
)

// ---- corpus builders (ordered-key record JSON, mirroring test/run.mjs) ----

func record(id string, over map[string]any) string {
	fields := []string{"id", "title", "type", "severity", "status", "foundBy", "opened",
		"resolved", "stint", "evidence", "resolution", "details", "notes", "links"}
	defaults := map[string]any{
		"id": id, "title": "Test issue " + id, "type": "defect", "severity": "med",
		"status": "open", "foundBy": "test", "opened": "2026-08-01", "resolved": nil,
		"stint": nil, "evidence": json.RawMessage(`{"commits":[],"integrated":[],"verified":null}`),
		"resolution": nil, "details": "Some details.", "notes": json.RawMessage(`[]`),
		"links": json.RawMessage(`[]`),
	}
	var sb strings.Builder
	sb.WriteByte('{')
	first := true
	for _, k := range fields {
		v, isOver := over[k]
		if !isOver {
			v = defaults[k]
		}
		if v == deleted {
			continue
		}
		if !first {
			sb.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(v)
		sb.Write(kb)
		sb.WriteByte(':')
		sb.Write(vb)
	}
	// extra (unknown) fields appended after the known ones
	for k, v := range over {
		known := slices.Contains(fields, k)
		if known {
			continue
		}
		sb.WriteByte(',')
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(v)
		sb.Write(kb)
		sb.WriteByte(':')
		sb.Write(vb)
	}
	sb.WriteByte('}')
	return sb.String()
}

type sentinel struct{}

var deleted = sentinel{} // marker: omit this field entirely

// testConfig leaves commit resolution off. A corpus in a temp directory is not
// inside the repository whose shas it cites, so resolving them would say
// nothing; the tests that do exercise resolution build a repository of their
// own with gitCorpus.
const testConfig = `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14,"verifyCommits":false}`

func makeCorpus(t *testing.T, records map[string]string, drafts map[string]string, queueJSON string) string {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, testConfig, records, drafts, queueJSON)
	return dir
}

func writeCorpus(t *testing.T, dir, cfg string, records, drafts map[string]string, queueJSON string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "ledger.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range records {
		if err := os.WriteFile(filepath.Join(dir, "issues", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range drafts { // path like "norm/flicker-on-resize.json"
		full := filepath.Join(dir, "drafts", path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if queueJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "queue.json"), []byte(queueJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func validCorpus(t *testing.T) string {
	return makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", nil),
		"TST-002.json": record("TST-002", map[string]any{
			"status": "closed", "resolved": "2026-08-02", "stint": "rev-1",
			"evidence": json.RawMessage(`{"commits":["abc1234"],"integrated":["def5678"],"verified":"gates green from fresh clone"}`),
			"notes":    json.RawMessage(`[{"date":"2026-08-01","text":"first note"},{"date":"2026-08-02","text":"closed"}]`),
		}),
		"TST-003.json": record("TST-003", map[string]any{"type": "feature-idea", "severity": "low"}),
		"TST-004.json": record("TST-004", map[string]any{"status": "in-progress", "stint": "rev-2", "severity": "critical"}),
		"TST-005.json": record("TST-005", map[string]any{
			"status": "moved-to-roadmap", "resolved": "2026-08-03",
			"resolution": "needs design work first", "links": json.RawMessage(`["TST-001"]`),
		}),
	}, map[string]string{
		"norm/flicker-on-resize.json": draftRecord(),
	}, "")
}

func draftRecord() string {
	r := record("X", map[string]any{"id": deleted})
	return r
}

func checkDir(t *testing.T, dir string) *Result {
	t.Helper()
	return ValidateAll(LoadLedger(dir))
}

func errsOf(t *testing.T, dir string) string  { return strings.Join(checkDir(t, dir).Errors, "\n") }
func warnsOf(t *testing.T, dir string) string { return strings.Join(checkDir(t, dir).Warnings, "\n") }

func mustMatch(t *testing.T, s, pattern string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(s) {
		t.Errorf("expected match %q in:\n%s", pattern, s)
	}
}

func mustNotMatch(t *testing.T, s, pattern string) {
	t.Helper()
	if regexp.MustCompile(pattern).MatchString(s) {
		t.Errorf("expected NO match %q in:\n%s", pattern, s)
	}
}

// ---- validation ----

func TestValidCorpusZeroErrors(t *testing.T) {
	res := checkDir(t, validCorpus(t))
	if len(res.Errors) != 0 {
		t.Fatalf("expected zero errors, got: %s", strings.Join(res.Errors, "; "))
	}
}

func TestDuplicateID(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json":     record("TST-001", nil),
		"TST-001-dup.json": record("TST-001", nil),
	}, nil, "")
	mustMatch(t, errsOf(t, dir), `duplicate id TST-001|filename must equal id`)
}

func TestFilenameMustEqualID(t *testing.T) {
	dir := makeCorpus(t, map[string]string{"TST-099.json": record("TST-001", nil)}, nil, "")
	mustMatch(t, errsOf(t, dir), `filename must equal id`)
}

func TestIDPrefixPattern(t *testing.T) {
	dir := makeCorpus(t, map[string]string{"CTV-001.json": record("CTV-001", nil)}, nil, "")
	mustMatch(t, errsOf(t, dir), `id must match`)
}

func TestMissingRequiredField(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"foundBy": deleted}),
	}, nil, "")
	mustMatch(t, errsOf(t, dir), `missing required field "foundBy"`)
}

func TestBadEnums(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"type": "bug", "severity": "urgent", "status": "wip"}),
	}, nil, "")
	e := errsOf(t, dir)
	mustMatch(t, e, `type must be`)
	mustMatch(t, e, `severity must be`)
	mustMatch(t, e, `status must be`)
}

func TestBadOrMisplacedDates(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"opened": "2026-13-45"}),
		"TST-002.json": record("TST-002", map[string]any{"resolved": "2026-08-02"}), // resolved while open
		"TST-003.json": record("TST-003", map[string]any{
			"status": "closed", "resolved": nil,
			"evidence": json.RawMessage(`{"commits":["abc1234"],"integrated":[],"verified":"v"}`),
		}),
	}, nil, "")
	e := errsOf(t, dir)
	mustMatch(t, e, `opened must be YYYY-MM-DD`)
	mustMatch(t, e, `must have resolved: null`)
	mustMatch(t, e, `requires a resolved date`)
}

func TestOpenCriticalQueueWarning(t *testing.T) {
	recs := map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"severity": "critical"}),
		"TST-002.json": record("TST-002", map[string]any{"severity": "critical"}),
		"TST-003.json": record("TST-003", map[string]any{"severity": "critical", "status": "in-progress", "stint": "rev-1"}),
		"TST-004.json": record("TST-004", map[string]any{"severity": "critical", "type": "feature-idea"}),
	}
	queue := `{"recommendedBy":"test","updated":"2026-08-01","items":[{"id":"TST-002","why":"queued critical"}]}`
	w := warnsOf(t, makeCorpus(t, recs, nil, queue))
	mustMatch(t, w, `TST-001 is an open critical missing from the queue`)
	mustNotMatch(t, w, `TST-002 is an open critical`)
	mustNotMatch(t, w, `TST-003 is an open critical`)
	mustNotMatch(t, w, `TST-004 is an open critical`)
	w2 := warnsOf(t, makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"severity": "critical"}),
	}, nil, ""))
	mustMatch(t, w2, `TST-001 is an open critical missing from the queue`)
}

func TestInProgressRequiresStint(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"status": "in-progress"}),
	}, nil, "")
	mustMatch(t, errsOf(t, dir), `requires a stint`)
}

func TestClosedRequiresVerified(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{
			"status": "closed", "resolved": "2026-08-02",
			"evidence": json.RawMessage(`{"commits":["abc1234"],"integrated":[],"verified":null}`),
		}),
	}, nil, "")
	mustMatch(t, errsOf(t, dir), `requires non-empty evidence.verified`)
}

func TestClosedRequiresCommitsOrLinks(t *testing.T) {
	noEvidence := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{
			"status": "closed", "resolved": "2026-08-02",
			"evidence": json.RawMessage(`{"commits":[],"integrated":[],"verified":"closed by children"}`),
		}),
	}, nil, "")
	mustMatch(t, errsOf(t, noEvidence), `requires evidence.commits, or links`)

	delegation := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{
			"status": "closed", "resolved": "2026-08-02", "links": json.RawMessage(`["TST-002"]`),
			"evidence": json.RawMessage(`{"commits":[],"integrated":[],"verified":"closed via TST-002"}`),
		}),
		"TST-002.json": record("TST-002", map[string]any{
			"status": "closed", "resolved": "2026-08-02",
			"evidence": json.RawMessage(`{"commits":["abc1234"],"integrated":[],"verified":"v"}`),
		}),
	}, nil, "")
	if res := checkDir(t, delegation); len(res.Errors) != 0 {
		t.Fatalf("delegation closure should pass, got: %s", strings.Join(res.Errors, "; "))
	}
}

// ---- commit citations (SPEC R15, R28-R30) ----

// gitTestConfig leaves resolution on, which is the default a real ledger has.
const gitTestConfig = `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14}`

// gitIn runs git in dir with a fixed identity and no user configuration, so a
// global commit.gpgsign or hooks path cannot fail a commit that has nothing to
// do with what is being tested. A git that will not run skips the test rather
// than failing it: the suite has to pass on a machine without one.
func gitIn(t *testing.T, dir string) func(args ...string) string {
	t.Helper()
	return func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Skipf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
}

// gitCorpus builds a repository holding an empty ledger, and returns the ledger
// directory, the sha of the one commit in it and that commit's tree sha.
// Resolution only means anything against a real object database, so these tests
// make one rather than standing in for git.
func gitCorpus(t *testing.T, cfg string) (dir, commit, tree string) {
	t.Helper()
	repo := t.TempDir()
	git := gitIn(t, repo)
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "first")
	dir = filepath.Join(repo, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCorpus(t, dir, cfg, nil, nil, "")
	return dir, git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
}

func writeRecord(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "issues", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// closedWith is a record closed on one cited sha, which is the shape every rule
// in this section is about.
func closedWith(id, sha string) string {
	return record(id, map[string]any{
		"status": "closed", "resolved": "2026-08-02", "stint": "rev-1",
		"evidence": json.RawMessage(`{"commits":["` + sha + `"],"integrated":[],"verified":"gates green"}`),
	})
}

// A citation nobody can follow is not evidence, and its shape is a property of
// the record: it is checked wherever the ledger sits.
func TestEvidenceShaMustBeWellFormed(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{
			"status": "closed", "resolved": "2026-08-02", "stint": "rev-1",
			"evidence": json.RawMessage(`{"commits":["HEAD~1","abc","A1B2C3D"],"integrated":["see the pull request"],"verified":"v"}`),
		}),
		"TST-002.json": closedWith("TST-002", "0123456789abcdef0123456789abcdef01234567"),
	}, nil, "")
	e := errsOf(t, dir)
	mustMatch(t, e, `TST-001\.json: evidence\.commits\[0\] "HEAD~1" is not a commit sha \(lower-case hexadecimal, at least 7 characters\)`)
	mustMatch(t, e, `TST-001\.json: evidence\.commits\[1\] "abc" is not a commit sha`)
	mustMatch(t, e, `TST-001\.json: evidence\.commits\[2\] "A1B2C3D" is not a commit sha`)
	mustMatch(t, e, `TST-001\.json: evidence\.integrated\[0\] "see the pull request" is not a commit sha`)
	// A full sha is as good a citation as an abbreviation.
	mustNotMatch(t, e, `TST-002`)
}

// The rule the format exists for. An invented sha reads exactly like a real
// one, so nothing but resolving it against the repository tells them apart.
func TestEvidenceShasResolveAgainstTheRepository(t *testing.T) {
	dir, commit, tree := gitCorpus(t, gitTestConfig)
	writeRecord(t, dir, "TST-001.json", closedWith("TST-001", commit[:7]))
	writeRecord(t, dir, "TST-002.json", closedWith("TST-002", commit))
	writeRecord(t, dir, "TST-003.json", closedWith("TST-003", "a1b2c3d4"))
	writeRecord(t, dir, "TST-004.json", closedWith("TST-004", tree))

	res := checkDir(t, dir)
	e := strings.Join(res.Errors, "\n")
	mustNotMatch(t, e, `TST-001|TST-002`) // the abbreviation and the full sha both resolve
	mustMatch(t, e, `TST-003\.json: evidence\.commits\[0\] a1b2c3d4 is not a commit in this repository`)
	mustMatch(t, e, `TST-004\.json: evidence\.commits\[0\] `+tree+` names a tree, not a commit`)
	mustNotMatch(t, strings.Join(res.Warnings, "\n"), `not checked`)

	// The gate has to fail on it rather than mention it in passing.
	if out, err := run(t, "render", dir); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	out, err := run(t, "check", dir)
	if err == nil {
		t.Fatalf("check should fail on an invented sha:\n%s", out)
	}
	mustMatch(t, out, `is not a commit in this repository`)
}

// Where the question cannot be asked the citations are unchecked rather than
// wrong, and unchecked never fails the gate: CI checks out shallow by default,
// and a ledger can be read from an unpacked archive.
func TestCitationsOutsideARepositoryAreUnchecked(t *testing.T) {
	dir := t.TempDir()
	// git walks up from the ledger looking for a repository, and a temp
	// directory could sit inside one. The ceiling stops the walk, so the test
	// asserts the same thing wherever it runs.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	writeCorpus(t, dir, gitTestConfig, map[string]string{
		"TST-001.json": closedWith("TST-001", "a1b2c3d4"),
	}, nil, "")

	res := checkDir(t, dir)
	if len(res.Errors) != 0 {
		t.Fatalf("an unanswerable question is not a failure, got: %s", strings.Join(res.Errors, "; "))
	}
	mustMatch(t, strings.Join(res.Warnings, "\n"),
		`commit citations not checked against the repository — the ledger is not inside a git repository`)
}

// A shallow clone holds a slice of the history, so a sha missing from it says
// nothing about the record citing it.
func TestShallowCloneLeavesCitationsUnchecked(t *testing.T) {
	dir, first, _ := gitCorpus(t, gitTestConfig)
	writeRecord(t, dir, "TST-001.json", closedWith("TST-001", first))
	repo := filepath.Dir(dir)
	git := gitIn(t, repo)
	git("add", "-A")
	git("commit", "-q", "-m", "the ledger")
	clone := filepath.Join(t.TempDir(), "clone")
	git("clone", "-q", "--depth", "1", "file://"+repo, clone)

	res := checkDir(t, filepath.Join(clone, "ledger"))
	if len(res.Errors) != 0 {
		t.Fatalf("a shallow clone must not fail the gate, got: %s", strings.Join(res.Errors, "; "))
	}
	mustMatch(t, strings.Join(res.Warnings, "\n"), `not checked against the repository — the clone is shallow`)
}

// A ledger that travels apart from the code it describes says so, and then its
// citations are checked for shape alone. An opt-out is a decision rather than a
// gap, so it carries no warning.
func TestVerifyCommitsFalseSkipsResolution(t *testing.T) {
	dir, _, _ := gitCorpus(t, `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14,"verifyCommits":false}`)
	writeRecord(t, dir, "TST-001.json", closedWith("TST-001", "a1b2c3d4"))
	writeRecord(t, dir, "TST-002.json", closedWith("TST-002", "nope"))

	res := checkDir(t, dir)
	e := strings.Join(res.Errors, "\n")
	mustNotMatch(t, e, `TST-001`)
	mustMatch(t, e, `TST-002\.json: evidence\.commits\[0\] "nope" is not a commit sha`)
	mustNotMatch(t, strings.Join(res.Warnings, "\n"), `not checked`)
}

func TestVerifyCommitsMustBeBoolean(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, `{"project":"p","idPrefix":"TST","schemaVersion":"issue.v1","verifyCommits":"yes"}`, nil, nil, "")
	mustMatch(t, errsOf(t, dir), `verifyCommits, when present, must be true or false`)
}

// A sha named in prose is a claim about the history too, but reading one out of
// a sentence is a guess about what the words meant, so it is a warning. A word
// spelled in hexadecimal and a bare run of digits are not shas.
func TestProseShaMentionsWarn(t *testing.T) {
	dir, commit, _ := gitCorpus(t, gitTestConfig)
	writeRecord(t, dir, "TST-001.json", record("TST-001", map[string]any{
		"details": "Follows on from a1b2c3d4, unlike " + commit[:8] + ", which landed.",
		"notes":   json.RawMessage(`[{"date":"2026-08-01","text":"defaced by 1234567890, and by deadbee1"}]`),
	}))
	queue := `{"recommendedBy":"t","updated":"2026-08-01","items":[{"id":"TST-001","why":"blocked on c0ffee12"}]}`
	if err := os.WriteFile(filepath.Join(dir, "queue.json"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	res := checkDir(t, dir)
	if len(res.Errors) != 0 {
		t.Fatalf("a mention is never an error, got: %s", strings.Join(res.Errors, "; "))
	}
	w := strings.Join(res.Warnings, "\n")
	mustMatch(t, w, `details mentions a1b2c3d4, which is not a commit in this repository`)
	mustMatch(t, w, `notes\[0\]\.text mentions deadbee1, which is not a commit`)
	mustMatch(t, w, `queue\.json: items\[0\]\.why mentions c0ffee12, which is not a commit`)
	mustNotMatch(t, w, `mentions `+commit[:8]) // the real one resolves
	mustNotMatch(t, w, `defaced|1234567890`)   // a word and a count are not shas
}

func TestWontFixRequiresResolution(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"status": "wont-fix", "resolved": "2026-08-02"}),
	}, nil, "")
	mustMatch(t, errsOf(t, dir), `requires a resolution`)
}

func TestNotesDatedWellFormedNonDecreasing(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{
			"notes": json.RawMessage(`[{"date":"2026-08-03","text":"later"},{"date":"2026-08-01","text":"earlier"}]`),
		}),
		"TST-002.json": record("TST-002", map[string]any{
			"notes": json.RawMessage(`[{"date":"yesterday","text":"x"}]`),
		}),
	}, nil, "")
	e := errsOf(t, dir)
	mustMatch(t, e, `date decreases`)
	mustMatch(t, e, `notes\[0\] must be`)
}

func TestDraftWithIDIsError(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", nil),
	}, map[string]string{
		"norm/sneaky.json": record("TST-050", nil),
	}, "")
	mustMatch(t, errsOf(t, dir), `drafts must not carry an id`)
}

func TestUnresolvableLinkIsWarning(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"links": json.RawMessage(`["TST-999"]`)}),
	}, nil, "")
	res := checkDir(t, dir)
	if len(res.Errors) != 0 {
		t.Fatalf("expected zero errors, got: %s", strings.Join(res.Errors, "; "))
	}
	mustMatch(t, strings.Join(res.Warnings, "\n"), `does not resolve`)
}

func TestUnknownFieldIsWarning(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"severty": "oops"}),
	}, nil, "")
	mustMatch(t, warnsOf(t, dir), `unknown field "severty"`)
}

func TestMalformedJSONIsError(t *testing.T) {
	dir := makeCorpus(t, map[string]string{}, nil, "")
	if err := os.WriteFile(filepath.Join(dir, "issues", "TST-001.json"), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustMatch(t, errsOf(t, dir), `JSON parse error`)
}

// ---- queue ----

func TestValidQueuePassesAndRenders(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", nil),
		"TST-004.json": record("TST-004", map[string]any{"status": "in-progress", "stint": "rev-2", "severity": "critical"}),
	}, nil, `{"recommendedBy":"orchestrator","updated":"2026-08-06","items":[{"id":"TST-001","why":"top-leverage fix"},{"id":"TST-004","why":"already selected"}]}`)
	res := checkDir(t, dir)
	if len(res.Errors) != 0 {
		t.Fatalf("expected zero errors, got: %s", strings.Join(res.Errors, "; "))
	}
	mustMatch(t, strings.Join(res.Warnings, "\n"), `TST-004 is already in progress`)
	html := RenderHTML(LoadLedger(dir), root.Assets)
	mustMatch(t, html, `top-leverage fix`)
	mustMatch(t, html, `"queue":\{`)
}

func TestQueueBadIDs(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", nil),
		"TST-002.json": record("TST-002", map[string]any{
			"status": "closed", "resolved": "2026-08-02",
			"evidence": json.RawMessage(`{"commits":["abc1234"],"integrated":[],"verified":"v"}`),
		}),
	}, nil, `{"recommendedBy":"orchestrator","updated":"2026-08-06","items":[{"id":"TST-999","why":"x"},{"id":"TST-002","why":"x"},{"id":"TST-001","why":"x"},{"id":"TST-001","why":"again"}]}`)
	e := errsOf(t, dir)
	mustMatch(t, e, `unknown id TST-999`)
	mustMatch(t, e, `TST-002 is closed — cannot recommend`)
	mustMatch(t, e, `duplicate id TST-001`)
}

func TestQueueMalformed(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", nil),
	}, nil, `{"updated":"soon","items":[{"id":"TST-001"}]}`)
	e := errsOf(t, dir)
	mustMatch(t, e, `recommendedBy must be`)
	mustMatch(t, e, `updated must be YYYY-MM-DD`)
	mustMatch(t, e, `items\[0\] must be`)
}

func TestAbsentQueueRendersNull(t *testing.T) {
	dir := validCorpus(t)
	os.Remove(filepath.Join(dir, "queue.json"))
	if res := checkDir(t, dir); len(res.Errors) != 0 {
		t.Fatalf("expected zero errors, got: %s", strings.Join(res.Errors, "; "))
	}
	mustMatch(t, RenderHTML(LoadLedger(dir), root.Assets), `"queue":null`)
}

// ---- render ----

func TestRenderDeterministic(t *testing.T) {
	dir := validCorpus(t)
	a := RenderHTML(LoadLedger(dir), root.Assets)
	b := RenderHTML(LoadLedger(dir), root.Assets)
	if a != b {
		t.Fatal("render not deterministic")
	}
}

func TestRenderEmbedsDataNoTimestamps(t *testing.T) {
	dir := validCorpus(t)
	html := RenderHTML(LoadLedger(dir), root.Assets)
	mustMatch(t, html, `ledger-data`)
	mustMatch(t, html, `TST-002`)
	mustMatch(t, html, `test-project`)
	mustNotMatch(t, html, `\d{4}-\d\d-\d\dT`) // no ISO timestamps
}

func TestScriptBreakoutEscaped(t *testing.T) {
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{
			"details": "evil </script><script>alert(1)</script> and `backticks` ${too}",
		}),
	}, nil, "")
	html := RenderHTML(LoadLedger(dir), root.Assets)
	mustNotMatch(t, html, `evil </script>`)
	mustMatch(t, html, `evil \\u003c/script`)
}

// ---- CLI (black-box against the built binary) ----

var binPath string

func TestMain(m *testing.M) {
	// os.Exit runs no deferred function, so the temp dir is removed on every
	// path out by hand. A deferred RemoveAll here reads as cleanup and performs
	// none, leaving a built binary behind on each run.
	tmp, err := os.MkdirTemp("", "cs-ledger-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(tmp, "cs-ledger")
	// These tests drive the real binary rather than calling into it, so what it
	// executes counts towards coverage only when it is built instrumented and
	// told where to write. Without this the CLI tier contributes nothing and
	// cmd/cs-ledger reads as uncovered however much of it the tests exercise.
	//
	// CS_COVERDIR is set by the Makefile's test targets. It carries the path
	// rather than GOCOVERDIR because `go test` overwrites GOCOVERDIR in the test
	// process with a directory of its own and does not fold what lands there
	// back into the profile. Setting GOCOVERDIR here, after that, is what points
	// the children at the tier directory: exec.Command passes on the current
	// environment, so every exec.Command(binPath, ...) below inherits it and no
	// call site needs to know about coverage. Under a plain `go test` the
	// variable is unset, the build stays ordinary and nothing is written.
	build := []string{"build", "-o", binPath, "github.com/codesweep-ai/ledger/cmd/cs-ledger"}
	if dir := os.Getenv("CS_COVERDIR"); dir != "" {
		build = append([]string{"build", "-cover", "-covermode=atomic",
			"-coverpkg=github.com/codesweep-ai/ledger/..."}, build[1:]...)
		_ = os.Setenv("GOCOVERDIR", dir)
	}
	cmd := exec.Command("go", build...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building cs-ledger for CLI tests failed:", err)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func TestCLIRenderCheckStale(t *testing.T) {
	dir := validCorpus(t)
	if out, err := exec.Command(binPath, "render", dir).CombinedOutput(); err != nil {
		t.Fatalf("render failed: %v\n%s", err, out)
	}
	if out, err := exec.Command(binPath, "check", dir).CombinedOutput(); err != nil {
		t.Fatalf("check failed: %v\n%s", err, out)
	}
	p := filepath.Join(dir, "issues", "TST-001.json")
	content, _ := os.ReadFile(p)
	edited := strings.Replace(string(content), "Test issue TST-001", "edited after render", 1)
	if err := os.WriteFile(p, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binPath, "check", dir).CombinedOutput()
	if err == nil {
		t.Fatal("check should fail STALE after a record edit")
	}
	mustMatch(t, string(out), `STALE`)
}

// The page and guide gates are errors, and check has to report them as errors.
// They used to print after the warnings block, so a stale page read as one more
// warning and the errors header's count did not match the list beneath it.
func TestCLICheckReportsPageFailuresAsErrors(t *testing.T) {
	// One open critical with no queue, so the run carries a warning as well.
	dir := makeCorpus(t, map[string]string{
		"TST-001.json": record("TST-001", map[string]any{"severity": "critical"}),
	}, nil, "")
	if out, err := exec.Command(binPath, "render", dir).CombinedOutput(); err != nil {
		t.Fatalf("render failed: %v\n%s", err, out)
	}
	p := filepath.Join(dir, "issues", "TST-001.json")
	content, _ := os.ReadFile(p)
	edited := strings.Replace(string(content), "Test issue TST-001", "edited after render", 1)
	if err := os.WriteFile(p, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(binPath, "check", dir).CombinedOutput()
	if err == nil {
		t.Fatal("check should fail on a stale page")
	}
	s := string(out)
	mustMatch(t, s, `validation errors \(1\):\n  - ledger\.html: STALE`)
	mustMatch(t, s, `check FAILED: 1 error\(s\), 1 warning\(s\)`)
	if i, j := strings.Index(s, "STALE"), strings.Index(s, "warnings ("); i < 0 || j < 0 || i > j {
		t.Errorf("the stale error must print above the warnings block:\n%s", s)
	}
}

func TestSelfLedgerFresh(t *testing.T) {
	dir := filepath.Join("..", "..", "ledger")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("self-ledger absent (vendored context)")
	}
	if out, err := exec.Command(binPath, "check", dir).CombinedOutput(); err != nil {
		t.Fatalf("self-ledger check failed: %v\n%s", err, out)
	}
}

// ---- stage 3: pin / manual / init / render / dev-stamp ----

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command(binPath, args...).CombinedOutput()
	return string(out), err
}

// pinAt rewrites the corpus config to claim a renderer that is not this one.
func pinAt(t *testing.T, dir, version string) {
	t.Helper()
	cfgPath := filepath.Join(dir, "ledger.json")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(cfg, &obj); err != nil {
		t.Fatalf("corpus config is not JSON: %v", err)
	}
	obj["toolVersion"] = version
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A page written by another renderer is not a failure. Across two versions the
// bytes differ whatever the records say, so check reports which renderer wrote
// the page and does not pretend to have compared it.
func TestVersionSkewWarnsRatherThanBlocks(t *testing.T) {
	dir := validCorpus(t)
	if out, err := run(t, "render", dir); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	pinAt(t, dir, "9.9.9")

	out, err := run(t, "check", dir)
	if err != nil {
		t.Fatalf("check must not block on version skew: %v\n%s", err, out)
	}
	mustMatch(t, out, `rendered by 9\.9\.9`)
	mustMatch(t, out, `cs-ledger render`)
	mustMatch(t, out, `not compared`)
}

// render is the one write verb, and it moves the recorded version with the
// page. Nothing else has to be run to bring a ledger onto this binary.
func TestRenderMovesThePin(t *testing.T) {
	dir := validCorpus(t)
	pinAt(t, dir, "9.9.9")

	out, err := run(t, "render", dir)
	if err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	mustMatch(t, out, `toolVersion 9\.9\.9 -> `+regexp.QuoteMeta(RendererVersion))
	cfg, _ := os.ReadFile(filepath.Join(dir, "ledger.json"))
	mustMatch(t, string(cfg), `"toolVersion": "`+regexp.QuoteMeta(RendererVersion)+`"`)
	if out, err := run(t, "check", dir); err != nil {
		t.Fatalf("check after render: %v\n%s", err, out)
	}
	mustMatch(t, mustRun(t, "check", dir), `ledger.html fresh`)
}

// A verb the tool does not have fails loudly, so a stale script cannot appear
// to work.
func TestUnknownVerbPrintsUsage(t *testing.T) {
	dir := validCorpus(t)
	out, err := run(t, "reticulate", dir)
	if err == nil {
		t.Fatalf("an unknown verb should be a usage error:\n%s", out)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("want exit 2 on a usage error, got %v", err)
	}
	mustMatch(t, out, `usage: cs-ledger`)
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out
}

func TestInitScaffoldsWorkingLedger(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ledger")
	if out, err := run(t, "init", dir, "--project", "demo", "--prefix", "DMO"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	for _, f := range []string{"ledger.json", "GUIDE.md", "AGENTS.md", "ledger.html"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("init did not create %s", f)
		}
	}
	if out, err := run(t, "check", dir); err != nil {
		t.Fatalf("check on fresh init: %v\n%s", err, out)
	}
	// refuses to overwrite
	if _, err := run(t, "init", dir, "--project", "demo", "--prefix", "DMO"); err == nil {
		t.Fatal("init should refuse an existing ledger.json")
	}
}

func TestInitRejectsBadPrefixWithoutScaffolding(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ledger")
	out, err := run(t, "init", dir, "--project", "demo", "--prefix", "abc")
	if err == nil {
		t.Fatal("init should refuse a prefix that is not ^[A-Z]{2,6}$")
	}
	mustMatch(t, out, `--prefix must match \^\[A-Z\]\{2,6\}\$`)
	// Nothing may survive a rejected init: a stray ledger.json would make the
	// overwrite guard refuse the corrected re-run.
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("rejected init left %s behind", dir)
	}
	if out, err := run(t, "init", dir, "--project", "demo", "--prefix", "DMO"); err != nil {
		t.Fatalf("init with a corrected prefix: %v\n%s", err, out)
	}
}

func TestManualPrintsTheCommandSurface(t *testing.T) {
	out, err := run(t, "manual")
	if err != nil {
		t.Fatalf("manual: %v", err)
	}
	mustMatch(t, out, `The cs-ledger manual`)
	mustMatch(t, out, `evidence\.verified`)
}

// The doctrine has to reach a machine that has the binary and no checkout,
// which is the case `guide` exists for.
func TestGuidePrintsTheDoctrine(t *testing.T) {
	out, err := run(t, "guide")
	if err != nil {
		t.Fatalf("guide: %v", err)
	}
	mustMatch(t, out, `Keeping a ledger`)
	mustMatch(t, out, `The five moves`)
	mustMatch(t, out, `evidence\.verified`)
	// The two documents are not the same text: one is the command surface.
	mustNotMatch(t, out, `The cs-ledger manual`)
}

func TestGuideSyncGate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ledger")
	if out, err := run(t, "init", dir, "--project", "demo", "--prefix", "DMO"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	guidePath := filepath.Join(dir, "GUIDE.md")
	if err := os.WriteFile(guidePath, []byte("# stale guide\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "check", dir)
	if err == nil {
		t.Fatal("check should fail on a drifted GUIDE.md")
	}
	mustMatch(t, out, `does not match this binary's embedded guide`)
	if out, err := run(t, "render", dir); err != nil {
		t.Fatalf("render should rewrite the guide: %v\n%s", err, out)
	}
	if out, err := run(t, "check", dir); err != nil {
		t.Fatalf("check after render: %v\n%s", err, out)
	}
}

// The half of the guide below the marker is the project's, so the gate must
// ignore it and render must carry it across untouched.
func TestProjectConventionsSurviveTheGateAndRender(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ledger")
	if out, err := run(t, "init", dir, "--project", "demo", "--prefix", "DMO"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	guidePath := filepath.Join(dir, "GUIDE.md")
	generated, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatal(err)
	}
	const house = "\n\nVerified cites `go test ./... -count=1` and the test name.\n"
	withHouse := root.GeneratedGuide(string(generated)) + house
	if err := os.WriteFile(guidePath, []byte(withHouse), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "check", dir); err != nil {
		t.Fatalf("check must not gate the project's own conventions: %v\n%s", err, out)
	}
	if out, err := run(t, "render", dir); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	after, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatal(err)
	}
	if root.ProjectConventions(string(after)) != house {
		t.Fatalf("render lost the project's conventions:\n%q", root.ProjectConventions(string(after)))
	}
	mustMatch(t, string(after), `The five moves`)
}

// Discovery rests on the router alone. Harnesses read the AGENTS.md nearest
// the file being edited, so the one beside the records is what an agent
// touching a record finds, and it has to name the guide. Nothing outside the
// ledger directory is written.
func TestInitWritesTheRouterAndNothingOutside(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "ledger")
	rootAgents := filepath.Join(repo, "AGENTS.md")

	if err := os.WriteFile(rootAgents, []byte("# Working in this repo\n\n- README.md — the tour.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(rootAgents)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "init", dir, "--project", "demo", "--prefix", "DMO"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	router, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("init did not write the router: %v", err)
	}
	mustMatch(t, string(router), `GUIDE\.md`)

	after, err := os.ReadFile(rootAgents)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("init wrote outside the ledger directory:\n%s", after)
	}
}

// A ledger scaffolded before the guide existed has to pick one up, or the
// guide only ever reaches ledgers created after it shipped. render is the
// route, since init refuses a ledger that is already there.
func TestRenderMaterializesAMissingGuide(t *testing.T) {
	dir := validCorpus(t)
	guidePath := filepath.Join(dir, "GUIDE.md")
	if _, err := os.Stat(guidePath); !os.IsNotExist(err) {
		t.Fatalf("corpus should start without a guide")
	}
	if out, err := run(t, "render", dir); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	got, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("render did not materialize the guide: %v", err)
	}
	mustMatch(t, string(got), `The five moves`)
	router, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("render did not materialize the router: %v", err)
	}
	mustMatch(t, string(router), `GUIDE\.md`)
	if out, err := run(t, "check", dir); err != nil {
		t.Fatalf("check after render: %v\n%s", err, out)
	}
}

func TestCommitUrlTemplate(t *testing.T) {
	dir := validCorpus(t)
	cfgPath := filepath.Join(dir, "ledger.json")
	write := func(tpl string) {
		cfg := `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14,"verifyCommits":false,"commitUrlTemplate":` + tpl + `}`
		if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// invalid: not http(s)
	write(`"javascript:alert(1)//{sha}"`)
	mustMatch(t, errsOf(t, dir), `commitUrlTemplate must be an http\(s\) url`)
	// invalid: no placeholder
	write(`"https://github.com/org/repo/commit/"`)
	mustMatch(t, errsOf(t, dir), `must contain the \{sha\} placeholder`)
	// invalid: empty
	write(`"  "`)
	mustMatch(t, errsOf(t, dir), `must be a non-empty string`)
	// valid: renders shas as links with the substituted href
	write(`"https://github.com/org/repo/commit/{sha}"`)
	if res := checkDir(t, dir); len(res.Errors) != 0 {
		t.Fatalf("valid template should pass: %s", strings.Join(res.Errors, "; "))
	}
	html := RenderHTML(LoadLedger(dir), root.Assets)
	mustMatch(t, html, `"commitUrlTemplate":"https://github.com/org/repo/commit/\{sha\}"`)
	// absent template: payload carries null, viewer keeps plain text
	cfg := testConfig
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	html = RenderHTML(LoadLedger(dir), root.Assets)
	mustMatch(t, html, `"commitUrlTemplate":null`)
}
