package audit

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

//go:embed catalog
var bundled embed.FS

func BuiltinCatalog() (Catalog, error) {
	b, e := bundled.ReadFile("catalog/manifest.json")
	if e != nil {
		return Catalog{}, e
	}
	var c Catalog
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	c.Complete = true
	for i := range c.Skills {
		r := &c.Skills[i]
		r.Data = map[string][]byte{}
		prefix := "catalog/skills/" + r.ID + "/"
		err := fs.WalkDir(bundled, strings.TrimSuffix(prefix, "/"), func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			b, e := bundled.ReadFile(p)
			if e == nil {
				r.Data[strings.TrimPrefix(p, prefix)] = b
			}
			return e
		})
		if err != nil {
			return c, err
		}
	}
	return c, ValidateCatalog(c)
}
func ValidateCatalog(c Catalog) error {
	if c.Schema != CatalogSchema && c.Schema != StoreSchema {
		return fmt.Errorf("unsupported catalog schema %q", c.Schema)
	}
	seenIDs := map[string]bool{}
	for _, r := range c.Skills {
		if r.ID == "" || seenIDs[r.ID] || len(r.Data) != len(r.Files) {
			return fmt.Errorf("duplicate reference identity or unlisted reference content")
		}
		seenIDs[r.ID] = true
		if len(r.Files) > PackageEntries {
			return fmt.Errorf("oversized reference")
		}
		total := int64(0)
		seen := map[string]bool{}
		previous := ""
		for _, f := range r.Files {
			if previous != "" && f.Path <= previous {
				return fmt.Errorf("reference manifest is not sorted")
			}
			previous = f.Path
			if !SafeRelative(f.Path) || seen[f.Path] {
				return fmt.Errorf("unsafe or duplicate reference path")
			}
			seen[f.Path] = true
			b, ok := r.Data[f.Path]
			total += int64(len(b))
			if !ok || !strings.EqualFold(Sum(b), f.SHA256) || int64(len(b)) != f.Size || len(b) > FileLimit || total > PackageLimit {
				return fmt.Errorf("reference bytes do not match manifest")
			}
		}
		if !seen["SKILL.md"] || !strings.EqualFold(Digest(r.Files), r.Digest) {
			return fmt.Errorf("reference digest mismatch for %s", r.ID)
		}
	}
	return nil
}
func SafeRelative(p string) bool {
	if !utf8.ValidString(p) || p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "." || s == ".." || s == "" {
			return false
		}
	}
	return true
}
func CatalogID(c Catalog) string {
	rs := append([]Reference{}, c.Skills...)
	sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
	b, _ := json.Marshal(rs)
	return Sum(append([]byte("skill-audit/reference-catalog/v1\x00"), b...))
}
func lineSet(data map[string][]byte) map[string]bool {
	m := map[string]bool{}
	for p, b := range data {
		upper := strings.ToUpper(path.Base(p))
		if strings.HasPrefix(path.Base(upper), "LICENSE") || strings.Contains(upper, "NOTICE") || !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
			continue
		}
		if p == "SKILL.md" {
			b = Body(b)
		}
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			if len(l) >= 20 {
				m[p+"\x00"+l] = true
			}
		}
	}
	return m
}

type Matcher struct {
	refs  []Reference
	lines []map[string]bool
}

func NewMatcher(c Catalog) Matcher {
	m := Matcher{refs: c.Skills}
	for _, r := range c.Skills {
		m.lines = append(m.lines, lineSet(r.Data))
	}
	return m
}
func (m Matcher) Match(p Package) Correlation {
	c := Correlation{Status: "unknown", Method: "path-qualified-line-jaccard/v1;threshold=0.60;shared>=3;names-excluded", Candidates: []Candidate{}}

	lines := lineSet(p.Data)
	var exact, modified []Candidate
	for i, r := range m.refs {
		r.Data = nil
		if p.Complete && strings.EqualFold(p.Digest, r.Digest) {
			exact = append(exact, Candidate{Reference: r, Basis: "observed_exact_match", Score: 1})
			continue
		}
		shared := 0
		for s := range lines {
			if m.lines[i][s] {
				shared++
			}
		}
		union := len(lines) + len(m.lines[i]) - shared
		if shared >= 3 && union > 0 && float64(shared)/float64(union) >= .60 {
			modified = append(modified, Candidate{Reference: r, Basis: "inferred_candidate", Score: float64(shared) / float64(union), Shared: shared, Union: union})
		}
	}
	if len(exact) > 0 {
		c.Status = "exact"
		c.Candidates = exact
	} else if len(modified) > 0 {
		c.Status = "possible_modified"
		c.Candidates = modified
	}
	if len(c.Candidates) > 1 {
		c.Status = "ambiguous"
	}
	return c
}
