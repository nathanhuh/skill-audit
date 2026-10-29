package audit

// These boundary representations preserve the published formats independently
// of the measurement and orchestration model. Older evidence is not migrated.
import (
	"cmp"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type wireRoot struct {
	Scope    string `json:"scope"`
	Path     string `json:"path"`
	Explicit bool   `json:"explicit"`
	Status   string `json:"status"`
}
type wireDiscovery struct {
	BudgetStop           string         `json:"budget_stop,omitempty"`
	DiagnosticsTruncated bool           `json:"diagnostics_truncated,omitempty"`
	OmittedDiagnostics   map[string]int `json:"omitted_diagnostics,omitempty"`
	Complete             *bool          `json:"complete,omitempty"`
	StatePath            string         `json:"state_path,omitempty"`
	Roots                []string       `json:"roots"`
	Home                 string         `json:"home,omitempty"`
	Policy               string         `json:"policy"`
	Limits               Limits         `json:"limits"`
	Exclusions           []Exclusion    `json:"exclusions"`
}
type WireChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
type WireLineage struct {
	Scope   string       `json:"scope"`
	Path    string       `json:"path"`
	Status  string       `json:"status"`
	Origin  []Candidate  `json:"origin"`
	Changes []WireChange `json:"changes"`
}
type wireReport struct {
	Complete    *bool          `json:"complete,omitempty"`
	CreatedAt   *time.Time     `json:"created_at,omitempty"`
	Discovery   *wireDiscovery `json:"discovery,omitempty"`
	Recording   *Recording     `json:"recording,omitempty"`
	Schema      string         `json:"schema"`
	CatalogID   string         `json:"catalog_version"`
	Roots       []wireRoot     `json:"roots"`
	Skills      []Skill        `json:"skills"`
	Lineage     []WireLineage  `json:"lineage"`
	Diagnostics []Diagnostic   `json:"diagnostics"`
}

func (r Report) MarshalJSON() ([]byte, error) {
	w := wireReport{Complete: &r.Complete, Discovery: &wireDiscovery{Roots: r.Roots, Home: r.Home, Policy: r.Policy, Limits: r.Limits, Exclusions: nonNil(r.Exclusions), Complete: &r.DiscoveryComplete, StatePath: r.StatePath, BudgetStop: r.BudgetStop, DiagnosticsTruncated: r.DiagnosticsTruncated, OmittedDiagnostics: r.OmittedDiagnostics}, Recording: r.Recording, Schema: r.Schema, CatalogID: r.CatalogID, Roots: []wireRoot{}, Skills: nonNil(r.Skills), Lineage: append([]WireLineage{}, r.Missing...), Diagnostics: nonNil(r.Diagnostics)}
	if !r.CreatedAt.IsZero() {
		at := r.CreatedAt.UTC()
		w.CreatedAt = &at
	}
	for _, s := range r.Skills {
		w.Roots = append(w.Roots, wireRoot{s.Scope, s.Path, true, "inventoried"})
		if s.Lineage.Status == "" || s.Lineage.Status == "not_compared" {
			continue
		}
		l := WireLineage{Scope: s.Scope, Path: s.Path, Status: s.Lineage.Status, Origin: []Candidate{}, Changes: []WireChange{}}
		if s.Lineage.BaselineOrigin != nil {
			l.Origin = nonNil(s.Lineage.BaselineOrigin.Candidates)
		}
		for _, x := range s.Lineage.Added {
			l.Changes = append(l.Changes, WireChange{x, "added"})
		}
		for _, x := range s.Lineage.Removed {
			l.Changes = append(l.Changes, WireChange{x, "removed"})
		}
		for _, x := range s.Lineage.Modified {
			l.Changes = append(l.Changes, WireChange{x, "modified"})
		}
		sort.Slice(l.Changes, func(i, j int) bool { return l.Changes[i].Path < l.Changes[j].Path })
		w.Lineage = append(w.Lineage, l)
	}
	sort.Slice(w.Lineage, func(i, j int) bool {
		a, b := w.Lineage[i], w.Lineage[j]
		return cmp.Or(cmp.Compare(a.Scope, b.Scope), cmp.Compare(a.Path, b.Path), cmp.Compare(a.Status, b.Status)) < 0
	})
	return json.Marshal(w)
}
func (r *Report) UnmarshalJSON(b []byte) error {
	var w wireReport
	if e := json.Unmarshal(b, &w); e != nil {
		return e
	}
	*r = Report{Schema: w.Schema, CatalogID: w.CatalogID, Skills: w.Skills, Diagnostics: w.Diagnostics, Recording: w.Recording, Complete: true, DiscoveryComplete: true}
	if w.CreatedAt != nil {
		r.CreatedAt = *w.CreatedAt
	}
	if w.Complete != nil && !*w.Complete {
		r.Complete = false
	}
	if w.Discovery != nil {
		r.Roots = w.Discovery.Roots
		r.Home = w.Discovery.Home
		r.Policy = w.Discovery.Policy
		r.Limits = w.Discovery.Limits
		r.Exclusions = w.Discovery.Exclusions
		r.StatePath = w.Discovery.StatePath
		r.BudgetStop = w.Discovery.BudgetStop
		r.DiagnosticsTruncated = w.Discovery.DiagnosticsTruncated
		r.OmittedDiagnostics = w.Discovery.OmittedDiagnostics
	}
	if anyIncomplete(r.Diagnostics) {
		r.Complete = false
		r.DiscoveryComplete = false
	}
	if w.Discovery != nil && w.Discovery.Complete != nil {
		r.DiscoveryComplete = *w.Discovery.Complete
	}
	if !r.DiscoveryComplete || r.DiagnosticsTruncated || len(r.OmittedDiagnostics) > 0 || r.BudgetStop != "" {
		r.DiscoveryComplete = false
		r.Complete = false
	}
	for _, s := range r.Skills {
		if !s.Complete || anyIncomplete(s.Diagnostics) {
			r.Complete = false
		}
		for _, e := range s.Evaluations {
			if !Completed(e) && e.Status != "disabled" {
				r.Complete = false
			}
		}
	}
	return nil
}
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
func (i *Inventory) UnmarshalJSON(b []byte) error {
	type plain Inventory
	var p plain
	if e := json.Unmarshal(b, &p); e != nil {
		return e
	}
	*i = Inventory(p)
	i.Complete = true
	var present map[string]json.RawMessage
	_ = json.Unmarshal(b, &present)
	if _, ok := present["complete"]; ok {
		i.Complete = p.Complete
	}
	if anyIncomplete(i.Diagnostics) {
		i.Complete = false
	}
	return nil
}
func (s State) MarshalJSON() ([]byte, error) {
	w := struct {
		Schema       string            `json:"schema"`
		Observations []json.RawMessage `json:"observations"`
	}{Schema: s.Schema, Observations: []json.RawMessage{}}
	for i, r := range s.Observations {
		if i < len(s.RawObservations) {
			w.Observations = append(w.Observations, s.RawObservations[i])
			continue
		}
		if r.CreatedAt.IsZero() {
			return nil, fmt.Errorf("missing observation time")
		}
		r.Missing = nil
		r.Skills = append([]Skill(nil), r.Skills...)
		for j := range r.Skills {
			r.Skills[j].Lineage = Lineage{}
		}
		observedAt := r.CreatedAt.UTC().Format(time.RFC3339Nano)
		r.CreatedAt = time.Time{}
		b, e := json.Marshal(struct {
			ObservedAt string `json:"observed_at"`
			Report     Report `json:"report"`
		}{observedAt, r})
		if e != nil {
			return nil, e
		}
		w.Observations = append(w.Observations, b)
	}
	return json.Marshal(w)
}
func (s *State) UnmarshalJSON(b []byte) error {
	var w struct {
		Schema       string            `json:"schema"`
		Anchors      json.RawMessage   `json:"baseline_anchors"`
		Observations []json.RawMessage `json:"observations"`
	}
	if e := json.Unmarshal(b, &w); e != nil {
		return e
	}
	if w.Schema == StateSchema && len(w.Anchors) > 0 {
		return fmt.Errorf("state/v3 derives anchors; explicit anchors are unsupported")
	}
	s.Schema = w.Schema
	s.Observations = nil
	s.RawObservations = w.Observations
	for _, raw := range w.Observations {
		var o struct {
			ObservedAt string `json:"observed_at"`
			Report     Report `json:"report"`
		}
		if e := json.Unmarshal(raw, &o); e != nil {
			return e
		}
		at, e := time.Parse(time.RFC3339Nano, o.ObservedAt)
		if e != nil {
			return fmt.Errorf("invalid observation time")
		}
		if !o.Report.CreatedAt.IsZero() && !o.Report.CreatedAt.Equal(at) {
			return fmt.Errorf("conflicting observation times")
		}
		o.Report.CreatedAt = at
		s.Observations = append(s.Observations, o.Report)
	}
	return nil
}

type wireEntry struct {
	Reference Reference         `json:"reference"`
	Data      map[string][]byte `json:"data"`
}
type wireStore struct {
	Schema      string            `json:"schema"`
	Entries     []wireEntry       `json:"entries"`
	Attribution map[string][]byte `json:"attribution"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}

func (c Catalog) MarshalJSON() ([]byte, error) {
	if c.Schema == CatalogSchema {
		type plain Catalog
		return json.Marshal(plain(c))
	}
	w := wireStore{Schema: c.Schema, Entries: []wireEntry{}, Attribution: map[string][]byte{}, Diagnostics: nonNil(c.Diagnostics)}
	for _, r := range c.Skills {
		w.Entries = append(w.Entries, wireEntry{r, r.Data})
	}
	for _, a := range c.Attribution {
		w.Attribution[attributionKey(a)] = a.Data
	}
	return json.Marshal(w)
}
func (c *Catalog) UnmarshalJSON(b []byte) error {
	var schema struct{ Schema string }
	if e := json.Unmarshal(b, &schema); e != nil {
		return e
	}
	if schema.Schema == CatalogSchema {
		type plain Catalog
		var p plain
		e := json.Unmarshal(b, &p)
		*c = Catalog(p)
		return e
	}
	var w wireStore
	if e := json.Unmarshal(b, &w); e != nil {
		return e
	}
	*c = Catalog{Schema: w.Schema, Skills: []Reference{}, Complete: true, Diagnostics: w.Diagnostics}
	for _, e := range w.Entries {
		r := e.Reference
		r.Data = e.Data
		c.Skills = append(c.Skills, r)
	}
	keys := []string{}
	for k := range w.Attribution {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts := strings.SplitN(k, "@", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid attribution identity")
		}
		tail := strings.SplitN(parts[1], "/", 2)
		if len(tail) != 2 {
			return fmt.Errorf("invalid attribution path")
		}
		c.Attribution = append(c.Attribution, Attribution{"https://github.com/" + parts[0], tail[0], tail[1], w.Attribution[k]})
	}
	if anyIncomplete(c.Diagnostics) {
		c.Complete = false
	}
	return nil
}
func (s Skill) MarshalJSON() ([]byte, error) {
	type plain Skill
	s.Files = nonNil(s.Files)
	s.Diagnostics = nonNil(s.Diagnostics)
	s.Origins = nonNil(s.Origins)
	s.Evaluations = nonNil(s.Evaluations)
	s.Findings = nonNil(s.Findings)
	s.Correlation.Candidates = nonNil(s.Correlation.Candidates)
	return json.Marshal(plain(s))
}
func (e Evaluation) MarshalJSON() ([]byte, error) {
	type plain Evaluation
	e.Analyzed = nonNil(e.Analyzed)
	e.OutsideScope = nonNil(e.OutsideScope)
	return json.Marshal(plain(e))
}
