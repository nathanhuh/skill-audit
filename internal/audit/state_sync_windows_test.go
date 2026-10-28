package audit

import (
	"path/filepath"
	"testing"
)

// Run on native Windows when an environment is available. Cross-compilation
// checks this test builds but does not establish persistence behavior on NTFS.
func TestWindowsStateBaselineAndReplacement(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	r := Report{Schema: ReportSchema, CatalogVersion: "fixture", Skills: []Skill{}, Roots: []Root{}, Lineage: []Lineage{}, Diagnostics: []Diagnostic{}}
	if err := SaveObservation(p, r, true); err != nil {
		t.Fatal(err)
	}
	if err := SaveObservation(p, r, false); err != nil {
		t.Fatal(err)
	}
	s, err := LoadState(p)
	if err != nil || len(s.Observations) != 2 {
		t.Fatal(s, err)
	}
}
