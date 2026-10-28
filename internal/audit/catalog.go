package audit

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed catalog
var catalogFS embed.FS

type Catalog struct {
	Schema  string      `json:"schema"`
	Skills  []Reference `json:"skills"`
	Version string      `json:"version"`
}

func LoadBuiltinCatalog() (Catalog, error) {
	var c Catalog
	b, err := catalogFS.ReadFile("catalog/manifest.json")
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Schema != "skill-audit/catalog/v1" {
		return c, fmt.Errorf("unsupported catalog schema")
	}
	for i := range c.Skills {
		r := &c.Skills[i]
		r.Data = map[string][]byte{}
		actual := []File{}
		root := "catalog/skills/" + r.ID
		err = fs.WalkDir(catalogFS, root, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			b, e := catalogFS.ReadFile(p)
			if e != nil {
				return e
			}
			rel := strings.TrimPrefix(p, root+"/")
			r.Data[rel] = b
			actual = append(actual, File{rel, hash(b), int64(len(b))})
			return nil
		})
		if err != nil {
			return c, err
		}
		x, _ := json.Marshal(actual)
		y, _ := json.Marshal(r.Files)
		if string(x) != string(y) || Digest(actual) != r.Digest {
			return c, fmt.Errorf("catalog integrity mismatch: %s", r.ID)
		}
	}
	c.Version = c.Fingerprint()
	return c, nil
}

// Fingerprint identifies the references used for correlation, independent of
// acquisition status or attribution packaging. It is not a signature.
func (c Catalog) Fingerprint() string {
	refs := append([]Reference{}, c.Skills...)
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	b, _ := json.Marshal(refs) // Reference.Data is excluded; Files carries its hashes.
	return hash(append([]byte("skill-audit/reference-catalog/v1\x00"), b...))
}
