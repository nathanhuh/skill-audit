package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestI1GapDetailCapPreservesReadablePartialInventory(t *testing.T) {
	root := tempDir(t)
	o := fixtureOptions(root)
	for i := 0; i < 12; i++ {
		o.Roots = append(o.Roots, filepath.Join(root, fmt.Sprintf("missing-%d", i)))
	}
	o.Budget = &InventoryBudget{OutputBytes: 16000, DiagnosticBytes: 600, Diagnostics: 2}
	inv := InventoryRun(context.Background(), o)
	if inv.outputFailure != nil || inv.Complete || !inv.DiagnosticsTruncated || len(inv.OmittedDiagnostics) == 0 {
		t.Fatal(inv, inv.outputFailure)
	}
	if !hasCode(inv.Diagnostics, "inventory_diagnostics_truncated") {
		t.Fatal(inv.Diagnostics)
	}
	total := 0
	for _, n := range inv.OmittedDiagnostics {
		total += n
	}
	retained := 0
	for _, d := range inv.Diagnostics {
		if d.Code == "root_error" {
			retained++
		}
	}
	if total+retained != 12 {
		t.Fatal(total, retained)
	}
	b, _ := Encode(inv)
	if len(b) > o.Budget.OutputBytes {
		t.Fatal(len(b))
	}
	file := filepath.Join(tempDir(t), "inventory.json")
	if e := WriteJSON(context.Background(), file, inv); e != nil {
		t.Fatal(e)
	}
	read, e := ReadInventory(file)
	if e != nil || read.Complete || !read.DiagnosticsTruncated {
		t.Fatal(read, e)
	}
}
func TestI2I4WholePackageAdmissionStopsWithinFormattedBound(t *testing.T) {
	root := tempDir(t)
	for i := 0; i < 10; i++ {
		fixture(t, root, fmt.Sprintf("p%02d/SKILL.md", i), "---\nname: "+strings.Repeat("n", 500)+"\n---\n# fixture")
	}
	o := fixtureOptions(root)
	o.Budget = &InventoryBudget{OutputBytes: 7000, DiagnosticBytes: 1000, Diagnostics: 5}
	inv := InventoryRun(context.Background(), o)
	if inv.outputFailure != nil || inv.Complete || len(inv.Packages) == 0 || len(inv.Packages) >= 10 || inv.BudgetStop == "" {
		t.Fatal(len(inv.Packages), inv, inv.outputFailure)
	}
	for _, p := range inv.Packages {
		if !p.Complete || p.Digest != Digest(p.Files) || len(p.Files) != 1 {
			t.Fatal(p)
		}
	}
	b, _ := Encode(inv)
	if len(b) > o.Budget.OutputBytes {
		t.Fatal(len(b))
	}
}
func TestI3MandatoryEnvelopeFailureNeverWrites(t *testing.T) {
	root := tempDir(t)
	o := fixtureOptions(root)
	o.Budget = &InventoryBudget{OutputBytes: 100, DiagnosticBytes: 1, Diagnostics: 1}
	inv := InventoryRun(context.Background(), o)
	path := filepath.Join(tempDir(t), "inventory.json")
	if e := WriteJSON(context.Background(), path, inv); e == nil {
		t.Fatal("oversized scope was saved")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal(e)
	}
}
func TestInventoryAccountingMatchesActualFormattedBytes(t *testing.T) {
	inv := Inventory{Schema: InventorySchema, Complete: true, Packages: []Package{}, Roots: []string{"/fixture"}}
	s := newInventoryBudget(&inv, nil)
	check := func() {
		t.Helper()
		actual, _ := Encode(inv)
		if len(actual) != s.used {
			t.Fatalf("counted %d actual %d: %s", s.used, len(actual), actual)
		}
	}
	check()
	s.entry()
	s.entry()
	check()
	for i := 0; i < 4; i++ {
		p := Package{Name: fmt.Sprintf("name-%d<&", i), Files: []File{}, Path: "unicode한글", Complete: true}
		if !s.packageMeasurement(p) {
			t.Fatal("unexpected budget")
		}
		check()
	}
	s.diagnostic(diag("error", "path", "message\n<&"))
	check()
	s.omit("error")
	check()
	s.omit("error")
	check()
	s.omit("other")
	check()
	x := Exclusion{Rule: "metadata", Count: 1, Examples: []string{"x"}}
	if !s.exclusion(nil, x) {
		t.Fatal("unexpected budget")
	}
	inv.Exclusions = []Exclusion{x}
	check()
	next := x
	next.Count = 2
	next.Examples = []string{"x", "y"}
	s.exclusion(&x, next)
	inv.Exclusions[0] = next
	check()
	s.finish()
	check()
	var read Inventory
	b, _ := Encode(inv)
	if e := json.Unmarshal(b, &read); e != nil {
		t.Fatal(e)
	}
}
func TestI2FileManifestIsNeverPartiallyAdmitted(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "a/SKILL.md", "# small")
	fixture(t, root, "b/SKILL.md", "# large")
	for i := 0; i < 100; i++ {
		fixture(t, root, "b/"+fmt.Sprintf("%03d-", i)+strings.Repeat("n", 180), "data")
	}
	o := fixtureOptions(root)
	o.Budget = &InventoryBudget{OutputBytes: 20000, DiagnosticBytes: 4000, Diagnostics: 20}
	inv := InventoryRun(context.Background(), o)
	if inv.outputFailure != nil || len(inv.Packages) != 1 || !inv.Packages[0].Complete || inv.BudgetStop != filepath.Join(root, "b") {
		t.Fatal(inv, inv.outputFailure)
	}
	b, _ := Encode(inv)
	if len(b) > o.Budget.OutputBytes {
		t.Fatal(len(b))
	}
}
func TestI1TruncationEvidenceSurvivesAuditAndStoredObservation(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "p/SKILL.md", "# fixture")
	o := fixtureOptions(root)
	for i := 0; i < 10; i++ {
		o.Roots = append(o.Roots, filepath.Join(root, fmt.Sprint(i)))
	}
	o.Budget = &InventoryBudget{OutputBytes: 16000, DiagnosticBytes: 500, Diagnostics: 1}
	inv := InventoryRun(context.Background(), o)
	state := filepath.Join(tempDir(t), "state.json")
	r := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: state, Record: "baseline"})
	if r.Recording.Status != "recorded" || !r.DiagnosticsTruncated || r.OmittedDiagnostics["root_error"] == 0 {
		t.Fatal(r.Recording, r)
	}
	history, e := ReadState(state)
	if e != nil || !history.Observations[0].DiagnosticsTruncated || history.Observations[0].OmittedDiagnostics["root_error"] != r.OmittedDiagnostics["root_error"] {
		t.Fatal(history, e)
	}
}

func TestReview2FinalBudgetFailureIsIncomplete(t *testing.T) {
	inv := Inventory{Schema: InventorySchema, Complete: true, Packages: []Package{}, Roots: []string{"/fixture"}}
	budget := newInventoryBudget(&inv, nil)
	budget.limit.OutputBytes = budget.used - 1
	budget.finish()
	b, _ := Encode(inv)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	if inv.outputFailure == nil || inv.Complete || raw["complete"] != false || !hasCode(inv.Diagnostics, "inventory_output_budget") {
		t.Fatal(inv, string(b))
	}
}
