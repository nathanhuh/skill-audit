package audit

import (
	"cmp"
	"encoding/json"
	"fmt"
	"sort"
)

// SearchInput retains one new query or an original legacy group. Legacy hit
// membership is never split speculatively when a grouped query is read.
type SearchInput struct {
	Source      string       `json:"source"`
	Queries     []string     `json:"queries"`
	Hits        []Hit        `json:"hits"`
	Complete    bool         `json:"complete"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (in SearchInput) MarshalJSON() ([]byte, error) {
	type plain SearchInput
	in.Queries = nonNil(in.Queries)
	in.Hits = nonNil(in.Hits)
	in.Diagnostics = nonNil(in.Diagnostics)
	return json.Marshal(plain(in))
}

type Search struct {
	Schema string
	Inputs []SearchInput
}

func (s Search) Hits() []Hit {
	var out []Hit
	for _, input := range s.Inputs {
		out = append(out, input.Hits...)
	}
	return out
}
func (s Search) Complete() bool {
	for _, input := range s.Inputs {
		if !input.Complete {
			return false
		}
	}
	return true
}
func (s Search) Diagnostics() []Diagnostic {
	var out []Diagnostic
	for _, input := range s.Inputs {
		out = append(out, input.Diagnostics...)
	}
	return out
}
func hitLess(a, b Hit) bool {
	return cmp.Or(cmp.Compare(a.Repository, b.Repository), cmp.Compare(a.Path, b.Path), cmp.Compare(a.Slug, b.Slug), cmp.Compare(a.URL, b.URL)) < 0
}

// MergeSearch flattens inputs in supplied order, copying and ordering each input
// once. Serialization reads these inputs directly, without derived mutable fields.
func MergeSearch(searches ...Search) Search {
	out := Search{Schema: SearchSchema, Inputs: []SearchInput{}}
	for _, s := range searches {
		for _, input := range s.Inputs {
			input.Queries = append([]string{}, input.Queries...)
			input.Hits = append([]Hit{}, input.Hits...)
			input.Diagnostics = append([]Diagnostic{}, input.Diagnostics...)
			sort.Slice(input.Hits, func(i, j int) bool { return hitLess(input.Hits[i], input.Hits[j]) })
			out.Inputs = append(out.Inputs, input)
		}
	}
	return out
}
func (s Search) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Schema string        `json:"schema"`
		Inputs []SearchInput `json:"inputs"`
	}{SearchSchema, nonNil(s.Inputs)})
}
func (s *Search) UnmarshalJSON(b []byte) error {
	var w struct {
		Schema        string         `json:"schema"`
		Inputs        *[]SearchInput `json:"inputs"`
		Source, Query string
		Hits          []Hit
		Incomplete    bool
		Diagnostics   []Diagnostic
	}
	if e := json.Unmarshal(b, &w); e != nil {
		return e
	}
	switch w.Schema {
	case LegacySearchSchema:
		*s = MergeSearch(Search{Inputs: []SearchInput{{Source: w.Source, Queries: []string{w.Query}, Hits: w.Hits, Complete: !w.Incomplete, Diagnostics: w.Diagnostics}}})
	case SearchSchema:
		if w.Inputs == nil {
			return fmt.Errorf("search/v3 requires input batches")
		}
		*s = MergeSearch(Search{Inputs: *w.Inputs})
	default:
		return fmt.Errorf("unsupported search schema %q", w.Schema)
	}
	s.Schema = w.Schema
	return nil
}
