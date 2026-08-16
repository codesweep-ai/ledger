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

func makeCorpus(t *testing.T, records map[string]string, drafts map[string]string, queueJSON string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14}`
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
	return dir
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

func testAssets(t *testing.T) *ViewerAssets {
	t.Helper()
	a, err := LoadAssets(root.Assets, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	return a
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
			"evidence": json.RawMessage(`{"commits":["abc"],"integrated":[],"verified":"v"}`),
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
			"evidence": json.RawMessage(`{"commits":["abc"],"integrated":[],"verified":null}`),
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
			"evidence": json.RawMessage(`{"commits":["abc"],"integrated":[],"verified":"v"}`),
		}),
	}, nil, "")
	if res := checkDir(t, delegation); len(res.Errors) != 0 {
		t.Fatalf("delegation closure should pass, got: %s", strings.Join(res.Errors, "; "))
	}
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
	html := RenderHTML(LoadLedger(dir), testAssets(t))
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
	mustMatch(t, RenderHTML(LoadLedger(dir), testAssets(t)), `"queue":null`)
}

// ---- render ----

func TestRenderDeterministic(t *testing.T) {
	dir := validCorpus(t)
	a := RenderHTML(LoadLedger(dir), testAssets(t))
	b := RenderHTML(LoadLedger(dir), testAssets(t))
	if a != b {
		t.Fatal("render not deterministic")
	}
}

func TestRenderEmbedsDataNoTimestamps(t *testing.T) {
	dir := validCorpus(t)
	html := RenderHTML(LoadLedger(dir), testAssets(t))
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
	html := RenderHTML(LoadLedger(dir), testAssets(t))
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
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/codesweep-ai/ledger/cmd/cs-ledger")
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

// A dev-stamped page is not committable, so dev mode must not move the pin or
// rewrite the docs on the strength of it.
func TestDevRenderLeavesThePinAndDocsAlone(t *testing.T) {
	dir := validCorpus(t)
	pinAt(t, dir, "9.9.9")

	if out, err := run(t, "render", dir, "--assets", "../../viewer"); err != nil {
		t.Fatalf("dev render: %v\n%s", err, out)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, "ledger.json"))
	mustMatch(t, string(cfg), `"toolVersion":"9\.9\.9"`)
	if _, err := os.Stat(filepath.Join(dir, "GUIDE.md")); !os.IsNotExist(err) {
		t.Fatal("dev render should not materialize the guide")
	}
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
	mustMatch(t, out, `cs-ledger\(1\)`)
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
	mustNotMatch(t, out, `cs-ledger\(1\)`)
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

func TestDevStampRefusedByCheck(t *testing.T) {
	dir := validCorpus(t)
	// dev-mode render from the checked-out viewer/ directory
	assetsDir, _ := filepath.Abs(filepath.Join("..", "..", "viewer"))
	if out, err := run(t, "render", dir, "--assets", assetsDir); err != nil {
		t.Fatalf("dev render: %v\n%s", err, out)
	}
	html, _ := os.ReadFile(filepath.Join(dir, "ledger.html"))
	mustMatch(t, string(html), regexp.QuoteMeta(DevStamp))
	out, err := run(t, "check", dir)
	if err == nil {
		t.Fatal("check should refuse dev-stamped ledger.html")
	}
	mustMatch(t, out, `rendered in dev mode`)
}

func TestCommitUrlTemplate(t *testing.T) {
	dir := validCorpus(t)
	cfgPath := filepath.Join(dir, "ledger.json")
	write := func(tpl string) {
		cfg := `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14,"commitUrlTemplate":` + tpl + `}`
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
	html := RenderHTML(LoadLedger(dir), testAssets(t))
	mustMatch(t, html, `"commitUrlTemplate":"https://github.com/org/repo/commit/\{sha\}"`)
	// absent template: payload carries null, viewer keeps plain text
	cfg := `{"project":"test-project","idPrefix":"TST","schemaVersion":"issue.v1","staleAfterDays":14}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	html = RenderHTML(LoadLedger(dir), testAssets(t))
	mustMatch(t, html, `"commitUrlTemplate":null`)
}
