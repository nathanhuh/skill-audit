package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"skill-audit/internal/audit"
)

const CatalogSchema = "skill-audit/catalog-store/v2"
const MaxStore = 64 << 20

type Entry struct {
	Reference audit.Reference   `json:"reference"`
	Data      map[string][]byte `json:"data"`
}
type Store struct {
	Schema      string             `json:"schema"`
	Entries     []Entry            `json:"entries"`
	Attribution map[string][]byte  `json:"attribution"`
	Diagnostics []audit.Diagnostic `json:"diagnostics"`
}

func EmptyStore() Store {
	return Store{Schema: CatalogSchema, Entries: []Entry{}, Attribution: map[string][]byte{}, Diagnostics: []audit.Diagnostic{}}
}

// LoadStore uses the pinned assignment catalog unless the operator selects a file.
// An invalid explicit file is an error, never a silent fallback.
func LoadStore(filename string) (Store, error) {
	s := EmptyStore()
	if filename != "" {
		err := ReadJSON(filename, &s)
		return s, err
	}
	c, err := audit.LoadBuiltinCatalog()
	if err != nil {
		return s, err
	}
	for _, r := range c.Skills {
		s.Entries = append(s.Entries, Entry{Reference: r, Data: r.Data})
	}
	return s, nil
}
func safePath(p string) bool {
	return p != "" && p != "." && path.Clean(p) == p && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") && !strings.Contains(p, ":") && p != ".." && !strings.HasPrefix(p, "../")
}
func (s Store) Catalog() (audit.Catalog, error) {
	c := audit.Catalog{Schema: "skill-audit/catalog/v1", Skills: []audit.Reference{}}
	if s.Schema != CatalogSchema {
		return c, fmt.Errorf("unsupported catalog store schema")
	}
	seen := map[string]bool{}
	for _, entry := range s.Entries {
		r := entry.Reference
		if r.ID == "" || seen[r.ID] {
			return c, fmt.Errorf("duplicate/empty catalog identity")
		}
		seen[r.ID] = true
		r.Data = entry.Data
		files := []audit.File{}
		for p, b := range r.Data {
			if !safePath(p) {
				return c, fmt.Errorf("unsafe catalog path")
			}
			files = append(files, audit.File{Path: p, SHA256: audit.Hash(b), Size: int64(len(b))})
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		a, _ := json.Marshal(files)
		b, _ := json.Marshal(r.Files)
		if string(a) != string(b) || audit.Digest(files) != r.Digest || r.Data["SKILL.md"] == nil {
			return c, fmt.Errorf("catalog fingerprint mismatch: %s", r.ID)
		}
		c.Skills = append(c.Skills, r)
	}
	sort.Slice(c.Skills, func(i, j int) bool { return c.Skills[i].ID < c.Skills[j].ID })
	c.Version = c.Fingerprint()
	return c, nil
}
func WriteJSON(p string, v any) error {
	absolute, e := filepath.Abs(p)
	if e != nil {
		return e
	}
	if e := audit.OutputOutside(absolute, nil); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > MaxStore {
		return fmt.Errorf("output exceeds 64 MiB")
	}
	return audit.WriteAtomic(context.Background(), absolute, b)
}
func ReadJSON(p string, v any) error {
	b, e := audit.ReadBounded(p, MaxStore)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
