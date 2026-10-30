package audit

import (
	"cmp"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type AuditOptions struct {
	StatePath, Record, Xray string
	Timeout, ScanTimeout    time.Duration
}

func AuditRun(ctx context.Context, inv Inventory, c Catalog, o AuditOptions) Report {
	r := Report{BudgetStop: inv.BudgetStop, DiagnosticsTruncated: inv.DiagnosticsTruncated, OmittedDiagnostics: inv.OmittedDiagnostics, Schema: ReportSchema, StatePath: inv.StatePath, Home: inv.Home, Limits: inv.Limits, CreatedAt: time.Now().UTC(), CatalogID: CatalogID(c), Roots: inv.Roots, Policy: inv.Policy, DiscoveryComplete: inv.Complete, Complete: inv.Complete, Exclusions: inv.Exclusions, Diagnostics: append([]Diagnostic(nil), inv.Diagnostics...), Skills: []Skill{}, Recording: &Recording{Status: "not_requested"}}
	r.Diagnostics = append(r.Diagnostics, c.Diagnostics...)
	if anyIncomplete(c.Diagnostics) {
		r.Complete = false
	}
	if o.StatePath != "" {
		if e := stateExclusionError(o.StatePath, r.Roots, r.StatePath); e != nil {
			r.Complete = false
			r.Diagnostics = append(r.Diagnostics, diag("state_exclusion_unconfirmed", Abs(o.StatePath), e.Error()))
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
	scanner := NewScanner(o.Xray, o.Timeout, o.ScanTimeout, inv.Packages...)
	defer scanner.Close()
	packages := append([]Package(nil), inv.Packages...)
	sort.Slice(packages, func(i, j int) bool { return Identity(packages[i]) < Identity(packages[j]) })
	interrupted := false
	for _, prior := range packages {
		if ctx.Err() != nil {
			interrupted = true
			r.interrupted = true
			r.Complete = false
			r.Diagnostics = append(r.Diagnostics, diag("audit_interrupted", prior.Path, ctx.Err().Error()))
			break
		}
		p := Measure(ctx, prior.Path, inv.Home)
		p.Scope, p.Agent = prior.Scope, prior.Agent
		if !strings.EqualFold(p.Digest, prior.Digest) || !prior.Complete {
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
	if ctx.Err() != nil && !interrupted {
		r.interrupted = true
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
		s.Lineage.Added, s.Lineage.Removed, s.Lineage.Modified = diffFiles(b.Files, s.Files)
		bf, cf := fileHashes(b.Files), fileHashes(s.Files)
		if s.Digest != b.Digest {
			s.Lineage.Status = "changed"
		}
		// Only skill-audit evaluations are pinned well enough to label new findings.
		comparable := false
		for _, a := range b.Evaluations {
			for _, c := range s.Evaluations {
				if a.Scanner == "skill-audit" && c.Scanner == a.Scanner && a.Version == c.Version && Completed(a) && Completed(c) {
					comparable = true
				}
			}
		}
		prior := map[string]bool{}
		for _, f := range b.Findings {
			prior[findingKey(f)] = true
		}
		for j := range s.Findings {
			f := &s.Findings[j]
			f.Change = "not_compared"
			changed := bf[f.File] != cf[f.File]
			if changed {
				f.Change = "present_in_changed_file"
			}
			if f.Scanner != "skill-audit" || !comparable {
				continue
			}
			f.Change = "new"
			if prior[findingKey(*f)] {
				f.Change = "existing"
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
	sort.Slice(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if n := cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Code, b.Code), cmp.Compare(a.Scope, b.Scope), cmp.Compare(a.Message, b.Message)); n != 0 {
			return n < 0
		}
		return !a.Incomplete && b.Incomplete
	})
}
func fileHashes(fs []File) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.Path] = strings.ToLower(f.SHA256)
	}
	return m
}

// diffFiles compares two manifests by relative path and SHA-256. Each result is
// sorted and nil when empty.
func diffFiles(base, current []File) (added, removed, modified []string) {
	bf, cf := fileHashes(base), fileHashes(current)
	for p, h := range cf {
		if v, ok := bf[p]; !ok {
			added = append(added, p)
		} else if v != h {
			modified = append(modified, p)
		}
	}
	for p := range bf {
		if _, ok := cf[p]; !ok {
			removed = append(removed, p)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(modified)
	return added, removed, modified
}

// upstreamChanges lists local differences from a pinned reference (ADR-0014).
func upstreamChanges(ref, local []File) []WireChange {
	added, removed, modified := diffFiles(ref, local)
	out := []WireChange{}
	for _, p := range added {
		out = append(out, WireChange{p, "added"})
	}
	for _, p := range removed {
		out = append(out, WireChange{p, "removed"})
	}
	for _, p := range modified {
		out = append(out, WireChange{p, "modified"})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// findingKey identifies a finding by the full matched text (ADR-0017); the
// display evidence is truncated and only stands in when no fingerprint exists.
func findingKey(f Finding) string {
	match := f.MatchSHA256
	if match == "" {
		match = f.Evidence
	}
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", f.Scanner, f.RuleID, f.RuleVersion, f.File, match)
}
