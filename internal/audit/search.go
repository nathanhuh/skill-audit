package audit

import (
	"cmp"
	"encoding/json"
	"fmt"
	"sort"
)

// SearchInput preserves one search invocation's provenance, including empty or
// failed results. A legacy joined query is kept opaque instead of guessed apart.
type SearchInput struct {
	Source      string       `json:"source"`
	Queries     []string     `json:"queries"`
	Hits        []Hit        `json:"hits"`
	Complete    bool         `json:"complete"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Search struct {
	Schema string
	Inputs []SearchInput
	// Derived views are used by acquisition and text output, never serialized as
	// one source/query claim over a merged set of hits.
	Source      string
	Queries     []string
	Hits        []Hit
	Complete    bool
	Diagnostics []Diagnostic
}

func hitLess(a, b Hit) bool {
	return cmp.Or(cmp.Compare(a.Repository, b.Repository), cmp.Compare(a.Path, b.Path), cmp.Compare(a.Slug, b.Slug), cmp.Compare(a.URL, b.URL)) < 0
}
func searchInputs(s Search) []SearchInput {
	if s.Inputs != nil {
		return s.Inputs
	}
	if s.Schema == "" {
		return nil
	}
	return []SearchInput{{Source: s.Source, Queries: s.Queries, Hits: s.Hits, Complete: s.Complete, Diagnostics: s.Diagnostics}}
}

// MergeSearch retains inputs in supplied order, flattening prior merges. Sync
// deduplicates targets independently; hits here retain their input membership.
func MergeSearch(searches ...Search) Search {
	out := Search{Schema: SearchSchema, Inputs: []SearchInput{}, Complete: true, Hits: []Hit{}}
	for _, s := range searches {
		for _, input := range searchInputs(s) {
			input.Queries = append([]string{}, input.Queries...)
			input.Hits = append([]Hit{}, input.Hits...)
			input.Diagnostics = append([]Diagnostic{}, input.Diagnostics...)
			sort.Slice(input.Hits, func(i, j int) bool { return hitLess(input.Hits[i], input.Hits[j]) })
			out.Inputs = append(out.Inputs, input)
			out.Queries = append(out.Queries, input.Queries...)
			out.Hits = append(out.Hits, input.Hits...)
			out.Diagnostics = append(out.Diagnostics, input.Diagnostics...)
			out.Complete = out.Complete && input.Complete
		}
	}
	if len(out.Inputs) == 1 {
		out.Source = out.Inputs[0].Source
	}
	sort.Slice(out.Hits, func(i, j int) bool { return hitLess(out.Hits[i], out.Hits[j]) })
	return out
}
func (s Search) MarshalJSON() ([]byte, error) {
	current := MergeSearch(s)
	return json.Marshal(struct {
		Schema string        `json:"schema"`
		Inputs []SearchInput `json:"inputs"`
	}{SearchSchema, current.Inputs})
}
func (s *Search) UnmarshalJSON(b []byte) error {
	var w struct {
		Schema      string         `json:"schema"`
		Inputs      *[]SearchInput `json:"inputs"`
		Source      string         `json:"source"`
		Query       string         `json:"query"`
		Hits        []Hit          `json:"hits"`
		Incomplete  bool           `json:"incomplete"`
		Diagnostics []Diagnostic   `json:"diagnostics"`
	}
	if e := json.Unmarshal(b, &w); e != nil {
		return e
	}
	switch w.Schema {
	case LegacySearchSchema:
		*s = MergeSearch(Search{Schema: w.Schema, Source: w.Source, Queries: []string{w.Query}, Hits: w.Hits, Complete: !w.Incomplete, Diagnostics: w.Diagnostics})
	case SearchSchema:
		if w.Inputs == nil {
			return fmt.Errorf("search/v3 requires input batches")
		}
		*s = MergeSearch(Search{Schema: w.Schema, Inputs: *w.Inputs})
	default:
		return fmt.Errorf("unsupported search schema %q", w.Schema)
	}
	// Remember the input version for callers; writers always emit the current one.
	s.Schema = w.Schema
	return nil
}
