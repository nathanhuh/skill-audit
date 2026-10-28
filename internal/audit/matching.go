package audit

import (
	"bytes"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

func lineSet(data map[string][]byte) map[string]bool {
	out := map[string]bool{}
	for p, b := range data {
		base := strings.ToLower(path.Base(p))
		if strings.HasPrefix(base, "license") || strings.Contains(base, "notice") || !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
			continue
		}
		text := string(b)
		if p == "SKILL.md" {
			lines := strings.Split(text, "\n")
			if strings.TrimSpace(lines[0]) == "---" {
				for i := 1; i < len(lines); i++ {
					if strings.TrimSpace(lines[i]) == "---" {
						text = strings.Join(lines[i+1:], "\n")
						break
					}
				}
			}
		}
		for _, l := range strings.Split(text, "\n") {
			l = strings.TrimSpace(l)
			if len(l) >= 20 {
				out[p+"\x00"+l] = true
			}
		}
	}
	return out
}

// Matcher prepares immutable reference features once for an audit. Catalog
// contents must not be mutated during its lifetime; no global cache is used.
type Matcher struct {
	references []Reference
	lines      []map[string]bool
}

func NewMatcher(c Catalog) *Matcher {
	m := &Matcher{references: append([]Reference{}, c.Skills...)}
	for _, r := range m.references {
		m.lines = append(m.lines, lineSet(r.Data))
	}
	return m
}
func (m *Matcher) Correlate(s Skill) Correlation {
	out := Correlation{Status: "unknown", Method: "path-qualified-line-jaccard/v1;threshold=0.60;shared>=3;names-excluded", Candidates: []Candidate{}}
	for _, r := range m.references {
		if s.Complete && s.Digest == r.Digest {
			out.Candidates = append(out.Candidates, Candidate{Reference: r, Basis: "observed_exact_match", Score: 1})
		}
	}
	if len(out.Candidates) > 0 {
		out.Status = "exact"
	} else {
		a := lineSet(s.Data)
		for i, r := range m.references {
			b := m.lines[i]
			shared := 0
			for l := range a {
				if b[l] {
					shared++
				}
			}
			union := len(a) + len(b) - shared
			score := 0.0
			if union > 0 {
				score = float64(shared) / float64(union)
			}
			if shared >= 3 && score >= 0.6 {
				out.Candidates = append(out.Candidates, Candidate{r, "inferred_candidate", score, shared, union})
			}
		}
		if len(out.Candidates) > 0 {
			out.Status = "possible_modified"
		}
	}
	if len(out.Candidates) > 1 {
		out.Status = "ambiguous"
	}
	sort.Slice(out.Candidates, func(i, j int) bool { return out.Candidates[i].Reference.ID < out.Candidates[j].Reference.ID })
	return out
}
