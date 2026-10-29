package audit

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type AuditOptions struct {
	StatePath, Record, Xray string
	Timeout, ScanTimeout    time.Duration
}

func AuditRun(ctx context.Context, inv Inventory, c Catalog, o AuditOptions) Report {
	r := Report{Schema: ReportSchema, StatePath: inv.StatePath, Home: inv.Home, Limits: inv.Limits, CreatedAt: time.Now().UTC(), CatalogID: CatalogID(c), Roots: inv.Roots, Policy: inv.Policy, DiscoveryComplete: inv.Complete, Complete: inv.Complete, Exclusions: inv.Exclusions, Diagnostics: append([]Diagnostic(nil), inv.Diagnostics...), Skills: []Skill{}, Recording: &Recording{Status: "not_requested"}}
	r.Diagnostics = append(r.Diagnostics, c.Diagnostics...)
	for _, d := range c.Diagnostics {
		if d.Incomplete {
			r.Complete = false
		}
	}
	var state *State
	var stateErr error
	if o.StatePath != "" && o.Record != "baseline" {
		s, e := ReadState(o.StatePath)
		if e != nil {
			stateErr = e
			r.Complete = false
			r.Diagnostics = append(r.Diagnostics, diag("state_invalid", o.StatePath, e.Error()))
		} else {
			state = &s
		}
	}
	matcher := NewMatcher(c)
	scanner := NewScanner(o.Xray, o.Timeout, o.ScanTimeout)
	defer scanner.Close()
	packages := append([]Package(nil), inv.Packages...)
	sort.Slice(packages, func(i, j int) bool { return Identity(packages[i]) < Identity(packages[j]) })
	for _, prior := range packages {
		if ctx.Err() != nil {
			r.Complete = false
			r.Diagnostics = append(r.Diagnostics, diag("audit_interrupted", prior.Path, ctx.Err().Error()))
			break
		}
		p := Measure(ctx, prior.Path, inv.Home)
		p.Scope, p.Agent = prior.Scope, prior.Agent
		if p.Digest != prior.Digest || !prior.Complete {
			p.Diagnostics = append(p.Diagnostics, diag("inventory_drift", p.Path, "package differs from inventory or prior measurement was incomplete"))
			r.Complete = false
		}
		s := Skill{Package: p, Correlation: matcher.Match(p), Lineage: Lineage{Status: "not_compared"}}
		e, f := LocalScan(p)
		s.Origins = s.Correlation.Candidates
		s.Evaluations = append(s.Evaluations, e)
		s.Findings = append(s.Findings, f...)
		x, xf := scanner.Scan(ctx, p)
		s.Evaluations = append(s.Evaluations, x)
		s.Findings = append(s.Findings, xf...)
		if !p.Complete || len(p.Diagnostics) > 0 || !Completed(e) || (x.Status != "disabled" && !Completed(x)) {
			r.Complete = false
		}
		s.Data = nil
		r.Skills = append(r.Skills, s)
	}
	if ctx.Err() != nil && !hasCode(r.Diagnostics, "audit_interrupted") {
		r.Complete = false
		r.Diagnostics = append(r.Diagnostics, diag("audit_interrupted", "", ctx.Err().Error()))
	}
	if state != nil {
		Compare(&r, state.Observations[0])
	}
	if o.Record != "" {
		out := Recording{Status: "not_recorded", Path: Abs(o.StatePath)}
		if stateErr != nil {
			out.Message = stateErr.Error()
		} else {
			out = Record(ctx, o.StatePath, o.Record, r, state, nil)
		}
		r.Recording = &out
	}
	return r
}
func Compare(r *Report, base Report) {
	if r.Policy != base.Policy {
		r.Diagnostics = append(r.Diagnostics, info("discovery_policy_changed", "", "discovery coverage is not equivalent; complete content remains independently comparable"))
	}
	if r.CatalogID != base.CatalogID {
		r.Diagnostics = append(r.Diagnostics, info("catalog_changed", "", "reference correlation differs; local content remains independently comparable"))
	}
	old := map[string]Skill{}
	for _, s := range base.Skills {
		old[Identity(s.Package)] = s
	}
	seen := map[string]bool{}
	for i := range r.Skills {
		s := &r.Skills[i]
		id := Identity(s.Package)
		seen[id] = true
		b, ok := old[id]
		if !ok {
			s.Lineage.Status = "not_in_baseline"
			r.Diagnostics = append(r.Diagnostics, info("not_in_baseline", s.Path, "identity has no baseline anchor; installation time is unknown"))
			continue
		}
		if !b.Complete || !s.Complete || b.Digest == "" || s.Digest == "" {
			s.Lineage.Status = "comparison_unavailable"
			continue
		}
		s.Lineage.Status = "unchanged"
		origin := b.Correlation
		origin.Candidates = b.Origins
		s.Lineage.BaselineOrigin = &origin
		bf, cf := map[string]string{}, map[string]string{}
		for _, f := range b.Files {
			bf[f.Path] = f.SHA256
		}
		for _, f := range s.Files {
			cf[f.Path] = f.SHA256
			if v, ok := bf[f.Path]; !ok {
				s.Lineage.Added = append(s.Lineage.Added, f.Path)
			} else if v != f.SHA256 {
				s.Lineage.Modified = append(s.Lineage.Modified, f.Path)
			}
		}
		for p := range bf {
			if _, ok := cf[p]; !ok {
				s.Lineage.Removed = append(s.Lineage.Removed, p)
			}
		}
		sort.Strings(s.Lineage.Removed)
		if s.Digest != b.Digest {
			s.Lineage.Status = "changed"
		}
		for j := range s.Findings {
			f := &s.Findings[j]
			f.Change = "not_compared"
			changed := bf[f.File] != cf[f.File]
			if changed {
				f.Change = "present_in_changed_file"
			}
			if f.Scanner != "skill-audit" {
				continue
			}
			comparable := false
			for _, a := range b.Evaluations {
				for _, c := range s.Evaluations {
					if a.Scanner == f.Scanner && c.Scanner == a.Scanner && a.Version == c.Version && Completed(a) && Completed(c) {
						comparable = true
					}
				}
			}
			if comparable {
				found := false
				for _, prior := range b.Findings {
					if findingKey(prior) == findingKey(*f) {
						found = true
					}
				}
				if !found {
					f.Change = "new"
				} else {
					f.Change = "existing"
				}
			}
		}
	}
	for id, b := range old {
		if !seen[id] {
			r.Complete = false
			r.Missing = append(r.Missing, WireLineage{Scope: b.Scope, Path: b.Path, Status: "missing_or_undiscovered", Origin: b.Origins, Changes: []WireChange{}})
			r.Diagnostics = append(r.Diagnostics, diag("tracked_missing", b.Path, "tracked package missing or undiscovered; no deletion or finding-resolution claim"))
		}
	}
	sort.SliceStable(r.Diagnostics, func(i, j int) bool { a, b := r.Diagnostics[i], r.Diagnostics[j]; return a.Path+a.Code < b.Path+b.Code })
}
func findingKey(f Finding) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", f.Scanner, f.RuleID, f.RuleVersion, f.File, f.Evidence)
}
