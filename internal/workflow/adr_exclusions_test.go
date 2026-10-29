package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"skill-audit/internal/audit"
)

func TestADRRoutineExclusionsBoundedAndPreservedB1(t *testing.T) {
	root := temp(t)
	put(t, filepath.Join(root, "project", "bar", "SKILL.md"), "fixture")
	const count = 1000
	for i := 0; i < count; i++ {
		put(t, filepath.Join(root, "project", fmt.Sprintf("noise-%04d", i), ".DS_Store"), "metadata")
	}
	put(t, filepath.Join(root, "project", ".git", "ignored"), "fixture")
	put(t, filepath.Join(root, "project", "node_modules", "ignored"), "fixture")
	inv := Discover(context.Background(), []string{filepath.Join(root, "project")}, "", DefaultLimits())
	if len(inv.Diagnostics) != 3 || len(inv.Exclusions) != 3 {
		t.Fatal("per-entry noise", len(inv.Diagnostics), len(inv.Exclusions))
	}
	found := false
	for _, e := range inv.Exclusions {
		if e.Rule == "metadata/.DS_Store" {
			found = true
			if e.Count != count || len(e.Examples) != 3 || e.OmittedExamples != count-3 {
				t.Fatal(e)
			}
		}
	}
	if !found {
		t.Fatal("missing exclusion evidence")
	}
	if again := Discover(context.Background(), inv.Roots, "", DefaultLimits()); !reflect.DeepEqual(inv, again) {
		t.Fatal("nondeterministic summaries")
	}
	store, err := LoadStore("")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Audit(context.Background(), inv, store, "", time.Second)
	if err != nil || r.HasCoverageGaps() {
		t.Fatal(r, err)
	}
	p := filepath.Join(root, "state.json")
	if err = audit.SaveObservation(context.Background(), p, r, true); err != nil {
		t.Fatal(err)
	}
	st, err := audit.LoadState(p)
	if err != nil || !reflect.DeepEqual(st.Observations[0].Report.Discovery.Exclusions, inv.Exclusions) {
		t.Fatal(st, err)
	}
	b, err := json.Marshal(inv)
	if err != nil || len(b) > 16000 {
		t.Fatal("inventory evidence unexpectedly large", len(b), err)
	}
}

func TestCaseAliasedRootsDoNotDuplicatePackages(t *testing.T) {
	root := temp(t)
	projects := filepath.Join(root, "Projects")
	put(t, filepath.Join(projects, "app", ".claude", "skills", "x", "SKILL.md"), "fixture")
	alias := filepath.Join(root, "projects")
	a, err := os.Lstat(projects)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Lstat(alias)
	if os.IsNotExist(err) {
		t.Skip("case-sensitive fixture filesystem; separate case-sensitive test covers non-merging")
	}
	if err != nil || !os.SameFile(a, b) {
		t.Fatal("unexpected alias setup", err)
	}
	for _, roots := range [][]string{
		{projects, alias},
		{projects, filepath.Join(alias, "app")},
		{projects, alias, filepath.Join(alias, "APP")},
	} {
		inv := Discover(context.Background(), roots, "", DefaultLimits())
		one := Discover(context.Background(), []string{projects}, "", DefaultLimits())
		if len(inv.Packages) != 1 || inv.Entries != one.Entries {
			t.Fatal("duplicate traversal", roots, inv, one)
		}
		store, err := LoadStore("")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Audit(context.Background(), inv, store, "", time.Second); err != nil {
			t.Fatal("duplicate identity", err)
		}
	}
	// The selected home hint also accepts a verified case alias.
	put(t, filepath.Join(projects, ".claude", "skills", "global", "SKILL.md"), "fixture")
	scope, _ := classify(filepath.Join(projects, ".claude", "skills", "global"), alias)
	if scope != "global" {
		t.Fatal("case alias changed ownership", scope)
	}
}

func TestDistinctCaseSensitiveRootsStayDistinct(t *testing.T) {
	root := temp(t)
	upper, lower := filepath.Join(root, "Projects"), filepath.Join(root, "projects")
	put(t, filepath.Join(upper, "one", "SKILL.md"), "fixture")
	if _, err := os.Lstat(lower); err == nil {
		t.Skip("case-insensitive fixture filesystem; alias test covers merging")
	}
	put(t, filepath.Join(lower, "two", "SKILL.md"), "fixture")
	if sameDirectoryPath(upper, lower) {
		t.Fatal("case names alone merged distinct directories")
	}
	inv := Discover(context.Background(), []string{upper, lower}, "", DefaultLimits())
	if len(inv.Packages) != 2 {
		t.Fatal(inv)
	}
}

func TestSkippedLinksRemainActionableWithoutDiagnosticRewrite(t *testing.T) {
	root := temp(t)
	pkg := filepath.Join(root, "bar")
	put(t, filepath.Join(pkg, "SKILL.md"), "fixture")
	if err := os.Symlink(pkg, filepath.Join(root, "foo")); err != nil {
		t.Skip("fixture links unavailable:", err)
	}
	inv := Discover(context.Background(), []string{root}, "", DefaultLimits())
	found := false
	for _, d := range inv.Diagnostics {
		if d.Code == "policy_symlink" {
			found = d.Incomplete && d.Path == filepath.Join(root, "foo")
		}
	}
	if !found {
		t.Fatal("actionable link gap was summarized away", inv)
	}
	// Audit preserves supplied coverage flags; there is no unreleased-code shim.
	inv.Diagnostics = []audit.Diagnostic{{Code: "policy_symlink", Path: "fixture", Incomplete: false}}
	store, err := LoadStore("")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Audit(context.Background(), inv, store, "", time.Second)
	if err != nil || r.Diagnostics[0].Incomplete {
		t.Fatal(r, err)
	}
}
