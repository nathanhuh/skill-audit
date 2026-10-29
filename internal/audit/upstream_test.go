package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// installedCopy writes a bundled catalog skill into a temporary package root.
func installedCopy(t *testing.T, id string) (string, Catalog) {
	t.Helper()
	c, e := BuiltinCatalog()
	if e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(tempDir(t), "skills", id)
	for _, r := range c.Skills {
		if r.ID == id {
			for rel, b := range r.Data {
				fixture(t, root, rel, string(b))
			}
			return root, c
		}
	}
	t.Fatalf("no bundled reference %s", id)
	return "", c
}
func onlySkill(t *testing.T, r Report) Skill {
	t.Helper()
	if len(r.Skills) != 1 {
		t.Fatalf("skills=%d", len(r.Skills))
	}
	return r.Skills[0]
}

// modifyBrandGuidelines edits SKILL.md, adds one file and deletes the license,
// which similarity ignores, so the copy stays a possible_modified candidate.
func modifyBrandGuidelines(t *testing.T, root string) {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if e != nil {
		t.Fatal(e)
	}
	fixture(t, root, "SKILL.md", string(b)+"\n## Team colours\nUse teal for every heading in internal decks.\n")
	fixture(t, root, "notes/team.md", "local notes")
	if e = os.Remove(filepath.Join(root, "LICENSE.txt")); e != nil {
		t.Fatal(e)
	}
}

func TestU1UpstreamChangesWithoutBaseline(t *testing.T) {
	root, c := installedCopy(t, "brand-guidelines")
	modifyBrandGuidelines(t, root)
	r := AuditRun(context.Background(), InventoryRun(context.Background(), fixtureOptions(root)), c, AuditOptions{})
	s := onlySkill(t, r)
	if s.Correlation.Status != "possible_modified" || len(s.Correlation.Candidates) != 1 || s.Lineage.Status != "not_compared" {
		t.Fatal(s.Correlation, s.Lineage)
	}
	got := s.Correlation.Candidates[0]
	want := []WireChange{{"LICENSE.txt", "removed"}, {"SKILL.md", "modified"}, {"notes/team.md", "added"}}
	if got.UpstreamComparison != "compared" || !reflect.DeepEqual(got.UpstreamChanges, want) {
		t.Fatal(got.UpstreamComparison, got.UpstreamChanges)
	}
	if !reflect.DeepEqual(s.Origins, s.Correlation.Candidates) {
		t.Fatal("stored origins must carry the same comparison")
	}
}

func TestU2ExactCopyHasNoUpstreamComparison(t *testing.T) {
	root, c := installedCopy(t, "brand-guidelines")
	s := onlySkill(t, AuditRun(context.Background(), InventoryRun(context.Background(), fixtureOptions(root)), c, AuditOptions{}))
	if s.Correlation.Status != "exact" {
		t.Fatal(s.Correlation)
	}
	got := s.Correlation.Candidates[0]
	if got.UpstreamComparison != "" || got.UpstreamChanges != nil {
		t.Fatal(got)
	}
	b, _ := json.Marshal(got)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(b, &fields)
	if _, ok := fields["upstream_comparison"]; ok {
		t.Fatal("exact candidate serialized an upstream comparison")
	}
}

func TestU3IncompleteMeasurementMakesNoUpstreamClaim(t *testing.T) {
	root, c := installedCopy(t, "brand-guidelines")
	modifyBrandGuidelines(t, root)
	if e := os.Symlink(filepath.Join(root, "SKILL.md"), filepath.Join(root, "linked.md")); e != nil {
		t.Skip("symlinks unavailable:", e)
	}
	p := Measure(context.Background(), root, "")
	if p.Complete {
		t.Fatal("a skipped link must make the measurement incomplete")
	}
	co := NewMatcher(c).Match(p)
	if co.Status != "possible_modified" {
		t.Fatal(co)
	}
	got := co.Candidates[0]
	if got.UpstreamComparison != "unavailable" || got.UpstreamChanges != nil {
		t.Fatal(got)
	}
}

func TestU4UpstreamComparisonIndependentOfBaseline(t *testing.T) {
	root, c := installedCopy(t, "brand-guidelines")
	modifyBrandGuidelines(t, root)
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	plain := AuditRun(context.Background(), inv, c, AuditOptions{})
	state := filepath.Join(tempDir(t), "state.json")
	base := AuditRun(context.Background(), inv, c, AuditOptions{StatePath: state, Record: "baseline"})
	if base.Recording.Status != "recorded" {
		t.Fatal(base.Recording)
	}
	tracked := AuditRun(context.Background(), inv, c, AuditOptions{StatePath: state})
	p, b, tr := onlySkill(t, plain), onlySkill(t, base), onlySkill(t, tracked)
	if !reflect.DeepEqual(p.Correlation, b.Correlation) || !reflect.DeepEqual(p.Correlation, tr.Correlation) {
		t.Fatal("upstream comparison changed with a baseline")
	}
	if tr.Lineage.Status != "unchanged" || tr.Lineage.BaselineOrigin == nil || !reflect.DeepEqual(tr.Lineage.BaselineOrigin.Candidates, p.Correlation.Candidates) {
		t.Fatal("baseline origin must read back the stored comparison", tr.Lineage)
	}
	if !plain.Complete || !tracked.Complete || Exit(plain) != 0 || Exit(tracked) != 0 {
		t.Fatal(plain.Complete, tracked.Complete, Exit(plain), Exit(tracked))
	}
}
