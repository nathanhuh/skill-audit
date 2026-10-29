package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviewE8ImportedInterruptionNamesDoNotRefuseRecording(t *testing.T) {
	for _, source := range []string{"inventory", "catalog"} {
		for _, code := range []string{"audit_interrupted", "inventory_canceled", "inventory_timeout", "audit_canceled"} {
			t.Run(source+"/"+code, func(t *testing.T) {
				root := tempDir(t)
				fixture(t, root, "p/SKILL.md", "# fixture")
				inv := InventoryRun(context.Background(), fixtureOptions(root))
				catalog := Catalog{Schema: StoreSchema, Complete: true}
				d := diag(code, "input", "untrusted prior diagnostic")
				if source == "inventory" {
					inv.Diagnostics = append(inv.Diagnostics, d)
					inv.Complete = false
				} else {
					catalog.Diagnostics = append(catalog.Diagnostics, d)
				}
				state := filepath.Join(tempDir(t), "state.json")
				r := AuditRun(context.Background(), inv, catalog, AuditOptions{StatePath: state, Record: "baseline"})
				if r.Recording.Status != "recorded" || Exit(r) != 2 || !hasCode(r.Diagnostics, code) {
					t.Fatal(r.Recording, Exit(r), r.Diagnostics)
				}
			})
		}
	}
}
func TestReviewD10ExplicitMetadataRootIsRejectedVisibly(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, ".git/p/SKILL.md", "# fixture")
	inv := InventoryRun(context.Background(), fixtureOptions(filepath.Join(root, ".git")))
	if inv.Complete || len(inv.Packages) != 0 || !hasCode(inv.Diagnostics, "excluded_root") {
		t.Fatal(inv)
	}
}
func TestReviewD3D6RejectedAliasCannotHideValidRoot(t *testing.T) {
	root := tempDir(t)
	actual := filepath.Join(root, "actual")
	fixture(t, actual, "x/SKILL.md", "# fixture")
	alias := filepath.Join(root, "alias")
	if e := os.Symlink(actual, alias); e != nil {
		t.Skip(e)
	}
	o := fixtureOptions(actual)
	o.Roots = append(o.Roots, filepath.Join(alias, "x"))
	inv := InventoryRun(context.Background(), o)
	if inv.Complete || len(inv.Packages) != 1 || !hasCode(inv.Diagnostics, "link_path") {
		t.Fatal(inv)
	}
}
func TestReviewX3ConsoleLimitIsPerPackage(t *testing.T) {
	src, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(src)
	if e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(tempDir(t), "scan-flood.exe")
	if e = os.WriteFile(exe, data, 0700); e != nil {
		t.Fatal(e)
	}
	p := measuredFixture(t)
	s := NewScanner(exe, 10*time.Second, 2*time.Second, p)
	defer s.Close()
	for i := 0; i < 2; i++ {
		evaluation, _ := s.Scan(context.Background(), p)
		if evaluation.Status != "partial" || !strings.Contains(evaluation.Message, "console output exceeded") || strings.Contains(evaluation.Message, "launch") {
			t.Fatal(evaluation)
		}
	}
	log, _ := os.ReadFile(exe + ".log")
	if strings.Count(string(log), "scan\n") != 2 || strings.Count(string(log), "version\n") != 1 {
		t.Fatal(string(log))
	}
}
func TestReviewB10DifferentStatePathKeepsActualDiscoveryEvidence(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "p/SKILL.md", "# fixture")
	o := fixtureOptions(root)
	o.StatePath = filepath.Join(root, "original-state.json")
	inv := InventoryRun(context.Background(), o)
	target := filepath.Join(root, "other-state.json")
	r := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: target, Record: "baseline"})
	if r.Recording.Status != "not_recorded" || !hasCode(r.Diagnostics, "state_exclusion_unconfirmed") || r.StatePath != o.StatePath {
		t.Fatal(r)
	}
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	o.StatePath = target
	inv = InventoryRun(context.Background(), o)
	r = AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: target, Record: "baseline"})
	if r.Recording.Status != "recorded" {
		t.Fatal(r.Recording)
	}
}
func TestReviewScannerTempCannotBeInsideAnyPackage(t *testing.T) {
	p := measuredFixture(t)
	other := measuredFixture(t)
	base := filepath.Join(other.Path, "tmp")
	if e := os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, base)
	}
	exe := fixture(t, tempDir(t), "trusted-executable", "data")
	s := NewScanner(exe, time.Second, time.Second, p, other)
	defer s.Close()
	launched := false
	s.process = func(context.Context, string, []string) (string, int, error) {
		launched = true
		return "skill-xray 0.1.0", 0, nil
	}
	evaluation, _ := s.Scan(context.Background(), p)
	if launched || evaluation.Status != "failed" || !strings.Contains(evaluation.Message, "inside audited package") {
		t.Fatal(launched, evaluation)
	}
	entries, e := os.ReadDir(base)
	if e != nil || len(entries) != 0 {
		t.Fatal(entries, e)
	}
}
func TestReviewC5GitHubCaseVariantsShareRepository(t *testing.T) {
	g := newGitFixture()
	g.add("a/SKILL.md", "# fixture")
	c := g.client(t).Sync([]string{"Anthropics/Skills"}, "HEAD", Search{Schema: SearchSchema, Complete: true, Hits: []Hit{{Repository: "anthropics/skills", Path: "a/SKILL.md"}}}, 20)
	if len(c.Skills) != 1 || !c.Complete || g.calls["/repos/anthropics/skills/git/trees/"+strings.Repeat("b", 40)] != 1 || g.calls["/repos/Anthropics/Skills"] != 0 {
		t.Fatal(c, g.calls)
	}
}
func TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity(t *testing.T) {
	root := tempDir(t)
	upper := fixture(t, root, "Report.json", "upper")
	lower := filepath.Join(root, "report.json")
	if i, e := os.Stat(lower); e == nil {
		j, _ := os.Stat(upper)
		if os.SameFile(i, j) {
			t.Skip("requires a case-sensitive native filesystem")
		}
	}
	fixture(t, root, "report.json", "lower")
	if e := OutputSafe(upper, []string{lower}, nil); e != nil {
		t.Fatal(e)
	}
}
func TestO2ConfirmedAliasesStillRefused(t *testing.T) {
	root := tempDir(t)
	input := fixture(t, root, "report.json", "input")
	alias := filepath.Join(root, "hardlink.json")
	if e := os.Link(input, alias); e != nil {
		t.Skip(e)
	}
	if e := OutputSafe(alias, []string{input}, nil); e == nil {
		t.Fatal("hardlink alias admitted")
	}
}
func TestReviewB6RuntimeInterruptionSurvivesChangedRecordingContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := AuditRun(ctx, Inventory{Schema: InventorySchema, Complete: true}, Catalog{}, AuditOptions{})
	if !r.interrupted {
		t.Fatal("runtime interruption missing")
	}
	rec := Record(context.Background(), filepath.Join(tempDir(t), "state.json"), "baseline", r, nil, nil)
	if rec.Status != "not_recorded" {
		t.Fatal(rec)
	}
}
func TestReviewB10CaseAliasScopeStillRequiresExclusion(t *testing.T) {
	root := tempDir(t)
	selected := filepath.Join(root, "Selected")
	fixture(t, selected, "p/SKILL.md", "# fixture")
	alias := filepath.Join(root, "selected")
	if _, e := os.Stat(alias); e != nil {
		t.Skip("requires case aliases on this filesystem")
	}
	inv := InventoryRun(context.Background(), fixtureOptions(selected))
	r := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: filepath.Join(alias, "state.json"), Record: "baseline"})
	if r.Recording.Status != "not_recorded" || !hasCode(r.Diagnostics, "state_exclusion_unconfirmed") {
		t.Fatal(r)
	}
}
func TestReviewX3ExitedProcessIsNotALaunchFailure(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	scans := 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", -1, &exec.ExitError{}
	}
	for i := 0; i < 2; i++ {
		evaluation, _ := s.Scan(context.Background(), measuredFixture(t))
		if evaluation.Status != "partial" {
			t.Fatal(evaluation)
		}
	}
	if scans != 2 {
		t.Fatal(scans)
	}
}
