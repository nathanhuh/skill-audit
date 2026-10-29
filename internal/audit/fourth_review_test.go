package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReview4ScannerMessagesPreserveEveryCause(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		partial, measurement bool
		code                 int
		want                 []string
	}{
		{"partial exit 2", true, true, 2, []string{"scanner coverage incomplete", "scanner exit 2"}},
		{"partial exit 75", true, true, 75, []string{"scanner coverage incomplete", "scanner exit 75"}},
		{"clean scan incomplete measurement", false, false, 0, []string{"incomplete measurement"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := scannerFixture(t)
			defer s.Close()
			s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
				if args[0] == "version" {
					return "skill-xray 0.1.0", 0, nil
				}
				report := cleanSARIF
				if tc.partial {
					report = strings.Replace(report, "no-reported-gap", "incomplete", 1)
				}
				if e := os.WriteFile(args[2], []byte(report), 0600); e != nil {
					t.Fatal(e)
				}
				if tc.code != 0 {
					return "", tc.code, &exec.ExitError{}
				}
				return "", 0, nil
			}
			p := measuredFixture(t)
			p.Complete = tc.measurement
			e, _ := s.Scan(context.Background(), p)
			for _, want := range tc.want {
				if !strings.Contains(e.Message, want) {
					t.Fatal(e)
				}
			}
			if strings.HasSuffix(e.Message, "; ") {
				t.Fatal("dangling separator", e)
			}
		})
	}
}
func TestReview4InvalidRegistryQueryKeepsSelectedProvider(t *testing.T) {
	a := NewAcquirer(context.Background())
	a.Client = &http.Client{Transport: fakeTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid query contacted a provider")
		return nil, nil
	})}
	s := a.Search([]string{strings.Repeat("x", 181)}, "https://registry.invalid/v1", 20)
	if len(s.Inputs) != 1 || s.Inputs[0].Source != "skills-sh-v1" || s.Inputs[0].Complete || len(s.Inputs[0].Diagnostics) != 1 {
		t.Fatal(s)
	}
}
func TestReview4DuplicateSelectedRootHasEvidence(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, ".claude/skills/p/SKILL.md", "# fixture")
	actual, alias := filepath.Join(root, ".claude/skills"), filepath.Join(root, ".Claude/skills")
	a, _ := os.Stat(actual)
	b, e := os.Stat(alias)
	if e != nil || !os.SameFile(a, b) {
		t.Skip("requires case-insensitive filesystem")
	}
	o := fixtureOptions(actual)
	o.Roots = append(o.Roots, alias)
	inv := InventoryRun(context.Background(), o)
	if !inv.Complete || len(inv.Packages) != 1 || len(inv.Roots) != 2 {
		t.Fatal(inv)
	}
	found := false
	for _, d := range inv.Diagnostics {
		if d.Code == "root_delegated" && d.Path == inv.Roots[1] && strings.Contains(d.Message, inv.Roots[0]) && !d.Incomplete {
			found = true
		}
	}
	if !found {
		t.Fatal("duplicate selected root has no delegation evidence", inv)
	}
}
func TestReview4UppercaseInventoryAndCatalog(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "p/SKILL.md", "# fixture")
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	b, _ := Encode(inv)
	b = bytes.ReplaceAll(b, []byte(inv.Packages[0].Digest), []byte(strings.ToUpper(inv.Packages[0].Digest)))
	b = bytes.ReplaceAll(b, []byte(inv.Packages[0].Files[0].SHA256), []byte(strings.ToUpper(inv.Packages[0].Files[0].SHA256)))
	path := fixture(t, tempDir(t), "inventory.json", string(b))
	read, e := ReadInventory(path)
	if e != nil {
		t.Fatal(e)
	}
	r := AuditRun(context.Background(), read, Catalog{}, AuditOptions{})
	if !r.Complete || hasCode(r.Skills[0].Diagnostics, "inventory_drift") || Exit(r) != 0 {
		t.Fatal(r)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(b, after) {
		t.Fatal("inventory rewritten")
	}
	c, e := BuiltinCatalog()
	if e != nil {
		t.Fatal(e)
	}
	c.Schema = StoreSchema
	lower, _ := Encode(c)
	upper := append([]byte(nil), lower...)
	for _, ref := range c.Skills {
		upper = bytes.ReplaceAll(upper, []byte(ref.Digest), []byte(strings.ToUpper(ref.Digest)))
		for _, f := range ref.Files {
			upper = bytes.ReplaceAll(upper, []byte(f.SHA256), []byte(strings.ToUpper(f.SHA256)))
		}
	}
	catalog := fixture(t, tempDir(t), "catalog.json", string(upper))
	canon, e := ReadCatalog(catalog)
	if e != nil {
		t.Fatal(e)
	}
	if CatalogID(canon) != CatalogID(c) {
		t.Fatal("hash spelling changed reference-catalog identity")
	}
	ref := canon.Skills[0]
	if NewMatcher(canon).Match(Package{Complete: true, Digest: ref.Digest, Data: ref.Data}).Status != "exact" {
		t.Fatal("exact match lost")
	}
	after, _ = os.ReadFile(catalog)
	if !bytes.Equal(upper, after) {
		t.Fatal("catalog rewritten")
	}
}
func TestReview4DuplicateTargetsCannotExhaustDiagnosticBudget(t *testing.T) {
	for _, kind := range []string{"missing", "permanent"} {
		t.Run(kind, func(t *testing.T) {
			g := newGitFixture()
			g.add("p/SKILL.md", "# fixture")
			if kind == "permanent" {
				g.status["/repos/a/x"] = []int{404}
			}
			hits := []Hit{}
			for i := 0; i < 200; i++ {
				hits = append(hits, Hit{Repository: "a/x", Path: "absent/SKILL.md", URL: fmt.Sprintf("https://untrusted.invalid/%d", i)})
			}
			hits = append(hits, Hit{Repository: "z/y", Path: "p/SKILL.md"})
			search := MergeSearch(Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: hits}}})
			a := g.client(t)
			a.OutputLimit = 8000
			c := a.Sync(nil, "HEAD", search, 20)
			if len(c.Skills) != 1 || !strings.Contains(c.Skills[0].Repository, "z/y") || hasCode(c.Diagnostics, "sync_budget") || len(c.Diagnostics) != 1 {
				t.Fatal(len(c.Skills), c.Diagnostics)
			}
		})
	}
}
func TestReview4SearchInputEditsStayAuthoritative(t *testing.T) {
	s := MergeSearch(Search{Schema: SearchSchema, Inputs: []SearchInput{{Source: "fixture", Queries: []string{"first"}, Complete: true, Hits: []Hit{{Repository: "a/x"}}}}})
	s.Inputs[0].Hits = append(s.Inputs[0].Hits, Hit{Repository: "b/y"})
	s.Inputs[0].Complete = false
	b, _ := Encode(s)
	var read Search
	if e := json.Unmarshal(b, &read); e != nil {
		t.Fatal(e)
	}
	if len(s.Hits()) != len(read.Hits()) || s.Complete() != read.Complete() {
		t.Fatal("in-memory views disagree with saved inputs", s, read)
	}
}
func TestReview4StoredTimeHasOneAuthority(t *testing.T) {
	r := Report{Schema: ReportSchema, CatalogID: CatalogID(Catalog{}), CreatedAt: time.Now().UTC(), Complete: true, DiscoveryComplete: true}
	b, e := Encode(State{Schema: StateSchema, Observations: []Report{r}})
	if e != nil {
		t.Fatal(e)
	}
	var w struct {
		Schema       string `json:"schema"`
		Observations []struct {
			ObservedAt string                     `json:"observed_at"`
			Report     map[string]json.RawMessage `json:"report"`
		} `json:"observations"`
	}
	if e = json.Unmarshal(b, &w); e != nil {
		t.Fatal(e)
	}
	if _, ok := w.Observations[0].Report["created_at"]; ok {
		t.Error("observation time stored twice")
	}
	w.Observations[0].Report["created_at"], _ = json.Marshal(r.CreatedAt.Add(time.Second))
	conflicting, _ := json.Marshal(w)
	path := fixture(t, tempDir(t), "state.json", string(conflicting))
	if _, e = ReadState(path); e == nil {
		t.Error("conflicting observation times silently accepted")
	}
	w.Observations[0].Report["created_at"], _ = json.Marshal(r.CreatedAt)
	matching, _ := json.Marshal(w)
	path = fixture(t, tempDir(t), "state.json", string(matching))
	read, e := ReadState(path)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := Encode(read)
	if !bytes.Contains(encoded, []byte(`"created_at"`)) {
		t.Fatal("existing raw observation rewritten")
	}
	if !read.Observations[0].CreatedAt.Equal(r.CreatedAt) {
		t.Fatal("observation time lost")
	}
}

func TestQ4PerQueryProvenance(t *testing.T) {
	for _, provider := range []string{"github", "registry"} {
		t.Run(provider, func(t *testing.T) {
			a := NewAcquirer(context.Background())
			a.Token = "fixture-only"
			calls := map[string]int{}
			a.Client = &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
				q := strings.TrimSuffix(r.URL.Query().Get("q"), " filename:SKILL.md")
				calls[q]++
				status := 200
				var body any
				if q == "failed" {
					status = 502
					body = map[string]any{"message": "fixture unavailable"}
				} else {
					repos := []string{}
					switch q {
					case "alpha":
						repos = []string{"a/x"}
					case "beta":
						repos = []string{"a/x", "b/y"}
					case "empty":
					default:
						t.Fatal("unexpected query", q)
					}
					if provider == "registry" {
						hits := []any{}
						for _, repo := range repos {
							hits = append(hits, map[string]any{"source": repo, "sourceType": "github", "slug": "p"})
						}
						body = map[string]any{"data": hits, "count": len(hits)}
					} else {
						hits := []any{}
						for _, repo := range repos {
							hits = append(hits, map[string]any{"path": "p/SKILL.md", "repository": map[string]any{"full_name": repo, "private": false}})
						}
						body = map[string]any{"items": hits, "total_count": len(hits), "incomplete_results": false}
					}
				}
				b, _ := json.Marshal(body)
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b))}, nil
			})}
			registry, source := "", "github-code-search"
			if provider == "registry" {
				registry = "https://registry.invalid/v1"
				source = "skills-sh-v1"
			}
			invalid := strings.Repeat("x", 181)
			queries := []string{"alpha", "beta", "failed", "empty", invalid}
			s := a.Search(queries, registry, 20)
			if len(s.Inputs) != len(queries) || s.Complete() || len(s.Hits()) != 3 || a.Requests != 96 {
				t.Fatal(s, a.Requests)
			}
			for i, input := range s.Inputs {
				if input.Source != source || len(input.Queries) != 1 || input.Queries[0] != queries[i] {
					t.Fatal(input)
				}
			}
			if len(s.Inputs[0].Hits) != 1 || len(s.Inputs[1].Hits) != 2 || !s.Inputs[3].Complete || len(s.Inputs[3].Hits) != 0 {
				t.Fatal(s.Inputs)
			}
			for _, i := range []int{2, 4} {
				if s.Inputs[i].Complete || len(s.Inputs[i].Diagnostics) != 1 || s.Inputs[i].Diagnostics[0].Path != queries[i] {
					t.Fatal(s.Inputs[i])
				}
			}
			b, _ := Encode(s)
			var read Search
			if e := json.Unmarshal(b, &read); e != nil {
				t.Fatal(e)
			}
			if len(read.Inputs) != 5 || len(read.Inputs[1].Hits) != 2 || len(calls) != 4 {
				t.Fatal(read, calls)
			}
			// A legacy grouped-v3 input has no recoverable per-query mapping.
			grouped := Search{Schema: SearchSchema, Inputs: []SearchInput{{Source: source, Queries: []string{"old one", "old two"}, Hits: []Hit{{Repository: "c/z", Path: "p/SKILL.md"}}, Complete: true}}}
			older, _ := Encode(grouped)
			var legacy Search
			if e := json.Unmarshal(older, &legacy); e != nil {
				t.Fatal(e)
			}
			merged := MergeSearch(legacy, read)
			if len(merged.Inputs) != 6 || len(merged.Inputs[0].Queries) != 2 || len(merged.Inputs[0].Hits) != 1 {
				t.Fatal(merged)
			}
			g := newGitFixture()
			g.add("p/SKILL.md", "# fixture")
			c := g.client(t).Sync(nil, "HEAD", read, 20)
			if len(c.Skills) != 2 || !hasCode(c.Diagnostics, "search_partial") {
				t.Fatal(c)
			}
		})
	}
}
func TestReview4InvalidHashTextStillRejected(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "p/SKILL.md", "# fixture")
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	inv.Packages[0].Files[0].SHA256 = strings.Repeat("g", 64)
	b, _ := Encode(inv)
	path := fixture(t, tempDir(t), "inventory.json", string(b))
	if _, e := ReadInventory(path); e == nil {
		t.Fatal("invalid inventory hash accepted")
	}
	c, e := BuiltinCatalog()
	if e != nil {
		t.Fatal(e)
	}
	c.Schema = StoreSchema
	c.Skills[0].Files[0].SHA256 = strings.Repeat("g", 64)
	b, _ = Encode(c)
	path = fixture(t, tempDir(t), "catalog.json", string(b))
	if _, e = ReadCatalog(path); e == nil {
		t.Fatal("invalid reference hash accepted")
	}
}

func TestReview4OverlappingFailedImportsShareOneDiagnostic(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	g.entries[0].Mode = "120000"
	search := Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "p/SKILL.md"}, {Repository: "a/x", Slug: "p"}}}}}
	c := g.client(t).Sync([]string{"a/x"}, "HEAD", search, 20)
	if len(c.Skills) != 0 || len(c.Diagnostics) != 1 || c.Diagnostics[0].Code != "import_failed" || c.Complete {
		t.Fatal(c)
	}
	// Distinct missing targets retain their own evidence.
	g = newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	search.Inputs[0].Hits = []Hit{{Repository: "a/x", Path: "missing-one/SKILL.md"}, {Repository: "a/x", Path: "missing-two/SKILL.md"}}
	c = g.client(t).Sync(nil, "HEAD", search, 20)
	if len(c.Diagnostics) != 2 || c.Diagnostics[0].Message == c.Diagnostics[1].Message {
		t.Fatal(c)
	}

	// Distinct full causes must not collapse when their display prefixes truncate equally.
	prefix := strings.Repeat("same-prefix", 30)
	search.Inputs[0].Hits = []Hit{{Repository: "a/x", Path: prefix + "/one/SKILL.md"}, {Repository: "a/x", Path: prefix + "/two/SKILL.md"}}
	c = g.client(t).Sync(nil, "HEAD", search, 20)
	if len(c.Diagnostics) != 2 {
		t.Fatal(c)
	}
}
