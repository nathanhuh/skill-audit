package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const StateSchema = "skill-audit/state/v1"
const MaxState = 32 << 20

type Observation struct {
	ObservedAt string `json:"observed_at"`
	Report     Report `json:"report"`
}
type State struct {
	Schema       string        `json:"schema"`
	Observations []Observation `json:"observations"`
}

func LoadState(path string) (State, error) {
	var st State
	b, err := ReadBounded(path, MaxState)
	if err != nil {
		return st, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&st); err != nil {
		return st, fmt.Errorf("corrupt state: %w", err)
	}
	if dec.Decode(new(any)) != io.EOF {
		return st, fmt.Errorf("corrupt state: trailing data")
	}
	if st.Schema != StateSchema || len(st.Observations) == 0 {
		return st, fmt.Errorf("unsupported/empty state")
	}
	for _, o := range st.Observations {
		if _, err = time.Parse(time.RFC3339Nano, o.ObservedAt); err != nil {
			return st, fmt.Errorf("corrupt observation time")
		}
		if o.Report.Schema != ReportSchema || o.Report.CatalogVersion == "" {
			return st, fmt.Errorf("corrupt observation schema/catalog")
		}
		ids := map[string]bool{}
		for _, s := range o.Report.Skills {
			if ids[s.identity()] || !filepath.IsAbs(s.Path) || (s.Scope != "global" && s.Scope != "project" && s.Scope != "machine") {
				return st, fmt.Errorf("corrupt/duplicate skill identity")
			}
			ids[s.identity()] = true
			paths := map[string]bool{}
			for _, f := range s.Files {
				if paths[f.Path] || f.Path == "" || filepath.IsAbs(f.Path) || strings.Contains(f.Path, "\\") || f.Path == ".." || strings.HasPrefix(f.Path, "../") || strings.Contains(f.Path, "/../") || !validHash(f.SHA256) || f.Size < 0 {
					return st, fmt.Errorf("corrupt file manifest")
				}
				paths[f.Path] = true
			}
			if s.Complete && (s.Digest != Digest(s.Files) || !paths["SKILL.md"]) {
				return st, fmt.Errorf("corrupt snapshot fingerprint")
			}
		}
	}
	return st, nil
}
func validHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// OutputOutside checks placement only. ReadBounded and WriteAtomic enforce
// no-follow policy at their I/O boundaries.
func OutputOutside(path string, roots []Root) error {
	absolute, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	for dir := filepath.Dir(absolute); ; {
		_, err := os.Lstat(filepath.Join(dir, "SKILL.md"))
		if err == nil {
			return fmt.Errorf("tool-owned files must be outside skill packages")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot verify output placement: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, r := range roots {
		if Within(absolute, r.Path) {
			return fmt.Errorf("tool-owned files must be outside skill root %s", r.Path)
		}
	}
	return nil
}

// SaveObservation serializes writers using an exclusive lock and re-reads state
// under it. Baseline creation cannot overwrite earlier evidence.
func SaveObservation(ctx context.Context, path string, r Report, baseline bool) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("state recording interrupted: %w", err)
	}
	if baseline && r.HasCoverageGaps() {
		return fmt.Errorf("baseline requires a complete audit within the declared policy; no state written")
	}
	// Parent creation and the lock are writes too; guard them before the
	// separate final no-follow check in WriteAtomic.
	if err := NoSymlinks(path); err != nil {
		return err
	}
	if err := OutputOutside(path, r.Roots); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("state lock unavailable (inspect stale locks manually): %w", err)
	}
	lock.Close()
	defer os.Remove(path + ".lock")
	st := State{Schema: StateSchema, Observations: []Observation{}}
	if baseline {
		if _, err = os.Lstat(path); err == nil {
			return fmt.Errorf("baseline already exists; use snapshot")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		st, err = LoadState(path)
		if err != nil {
			return err
		}
	}
	r.Lineage = []Lineage{} // Derived comparisons need not be duplicated inside every observation.
	st.Observations = append(st.Observations, Observation{time.Now().UTC().Format(time.RFC3339Nano), r})
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if len(b) > MaxState {
		return fmt.Errorf("state exceeds 32 MiB; archive state and start a separate baseline")
	}
	return WriteAtomic(ctx, path, b)
}
func Compare(r *Report, st State) {
	baseline := st.Observations[0].Report
	if baseline.CatalogVersion != r.CatalogVersion {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Path: "state", Code: "catalog_changed", Message: "catalog version changed; upstream observations remain separate; local baseline comparison is available", Incomplete: false})
	}
	current := map[string]*Skill{}
	for i := range r.Skills {
		current[r.Skills[i].identity()] = &r.Skills[i]
	}
	baselineIDs := map[string]bool{}
	for _, b := range baseline.Skills {
		baselineIDs[b.identity()] = true
		l := Lineage{Scope: b.Scope, Path: b.Path, Status: "unchanged", Origin: b.Origins, Changes: []Change{}}
		s, ok := current[b.identity()]
		if !ok {
			l.Status = "missing_or_undiscovered"
			r.Diagnostics = append(r.Diagnostics, Diagnostic{b.Scope, b.Path, "tracked_missing", "tracked package missing or outside current discovery; no deletions inferred", true})
		} else if !b.Complete || !s.Complete {
			l.Status = "comparison_unavailable"
			for i := range s.Findings {
				s.Findings[i].Change = "not_compared"
			}
		} else {
			old := map[string]string{}
			now := map[string]string{}
			for _, f := range b.Files {
				old[f.Path] = f.SHA256
			}
			for _, f := range s.Files {
				now[f.Path] = f.SHA256
			}
			for p, h := range now {
				kind := ""
				if prev, ok := old[p]; !ok {
					kind = "added"
				} else if h != prev {
					kind = "modified"
				}
				if kind != "" {
					l.Changes = append(l.Changes, Change{p, kind})
				}
			}
			for p := range old {
				if _, ok := now[p]; !ok {
					l.Changes = append(l.Changes, Change{p, "removed"})
				}
			}
			sort.Slice(l.Changes, func(i, j int) bool { return l.Changes[i].Path < l.Changes[j].Path })
			if len(l.Changes) > 0 {
				l.Status = "changed"
			}
			markFindings(s, b, l.Changes)
		}
		r.Lineage = append(r.Lineage, l)
	}
	for i := range r.Skills {
		s := &r.Skills[i]
		if baselineIDs[s.identity()] {
			continue
		}
		// Absence from a prior inventory does not establish installation time.
		// Current origin candidates remain on the skill, not as baseline origin.
		r.Lineage = append(r.Lineage, Lineage{Scope: s.Scope, Path: s.Path, Status: "not_in_baseline", Origin: []Candidate{}, Changes: []Change{}})
		r.Diagnostics = append(r.Diagnostics, Diagnostic{s.Scope, s.Path, "not_in_baseline", "package identity absent from baseline; installation time and prior findings are unknown", false})
		for j := range s.Findings {
			s.Findings[j].Change = "not_compared"
		}
	}
	sort.Slice(r.Lineage, func(i, j int) bool {
		a, b := r.Lineage[i], r.Lineage[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		return a.Path < b.Path
	})
}
func markFindings(s *Skill, b Skill, changes []Change) {
	comparable := false
	for _, before := range b.Evaluations {
		for _, after := range s.Evaluations {
			if before.Scanner == "skill-audit" && after.Scanner == before.Scanner && before.Version == after.Version && completed(before.Status) && completed(after.Status) {
				comparable = true
			}
		}
	}
	old := map[string]bool{}
	for _, f := range b.Findings {
		old[ruleKey(f)] = true
	}
	changed := map[string]bool{}
	for _, c := range changes {
		changed[c.Path] = true
	}
	for i := range s.Findings {
		f := &s.Findings[i]
		if f.Scanner == "skill-audit" && comparable {
			if old[ruleKey(*f)] {
				f.Change = "existing"
			} else {
				f.Change = "new"
			}
		} else if changed[f.Path] {
			f.Change = "present_in_changed_file"
		} else {
			f.Change = "not_compared"
		}
	}
}

func ruleKey(f Finding) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", f.Scanner, f.RuleID, f.RuleVersion, f.Path, f.Evidence)
}
