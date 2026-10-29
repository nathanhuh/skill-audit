package audit

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type gitFixture struct {
	entries   []treeEntry
	blobs     map[string][]byte
	calls     map[string]int
	order     []string
	status    map[string][]int
	private   bool
	truncated bool
}

func gitHash(b []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func newGitFixture() *gitFixture {
	return &gitFixture{blobs: map[string][]byte{}, calls: map[string]int{}, status: map[string][]int{}}
}
func (g *gitFixture) add(p, content string) {
	b := []byte(content)
	sha := gitHash(b)
	g.entries = append(g.entries, treeEntry{p, "100644", "blob", sha, len(b)})
	g.blobs[sha] = b
}
func (g *gitFixture) client(t *testing.T) *Acquirer {
	t.Helper()
	a := NewAcquirer(context.Background())
	a.Base = "https://fixture.invalid"
	a.Token = "fixture-github-token"
	a.RegistryToken = "fixture-registry-token"
	a.Client = &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Fatalf("unexpected pinned request: %+v", r)
		}
		p := r.URL.Path
		g.calls[p]++
		g.order = append(g.order, p)
		status := 200
		if codes := g.status[p]; len(codes) > 0 {
			status = codes[0]
			g.status[p] = codes[1:]
		}
		var v any
		if strings.Contains(p, "/git/blobs/") {
			sha := p[strings.LastIndex(p, "/")+1:]
			b, ok := g.blobs[sha]
			if !ok {
				status = 404
			}
			v = map[string]any{"sha": sha, "encoding": "base64", "content": base64.StdEncoding.EncodeToString(b), "size": len(b)}
		} else if strings.Contains(p, "/git/trees/") {
			if r.URL.Query().Get("recursive") != "1" {
				t.Fatal("non-recursive tree")
			}
			v = map[string]any{"truncated": g.truncated, "tree": g.entries}
		} else if strings.Contains(p, "/commits/") {
			v = map[string]any{"sha": strings.Repeat("a", 40), "commit": map[string]any{"tree": map[string]string{"sha": strings.Repeat("b", 40)}}}
		} else {
			v = map[string]any{"private": g.private}
		}
		b, _ := json.Marshal(v)
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})}
	return a
}
func TestC1C2C5GroupingCacheAdmissionAndReload(t *testing.T) {
	for _, cache := range []int{0, 16 << 20} {
		g := newGitFixture()
		g.add("skills/a/SKILL.md", "# shared manifest")
		g.add("skills/b/SKILL.md", "# shared manifest")
		g.add("LICENSE", "license bytes")
		a := g.client(t)
		a.CacheLimit = cache
		a.RawLimit = 2*len("# shared manifest") + len("license bytes")
		s := Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "skills/a/SKILL.md"}, {Repository: "b/y", Path: "skills/a/SKILL.md"}, {Repository: "a/x", Path: "skills/b/SKILL.md"}}}}}
		c := a.Sync([]string{"a/x"}, "HEAD", s, 10)
		if len(c.Skills) != 2 || c.Complete || !hasCode(c.Diagnostics, "sync_budget") {
			t.Fatal(len(c.Skills), c.Diagnostics)
		}
		if g.calls["/repos/a/x/git/trees/"+strings.Repeat("b", 40)] != 1 {
			t.Fatal(g.calls)
		}
		b, _ := Encode(c)
		if strings.Contains(string(b), a.Token) || strings.Contains(string(b), a.RegistryToken) {
			t.Fatal("credential serialized")
		}
		if e := ValidateCatalog(c); e != nil {
			t.Fatal(e)
		}
	}
}
func TestC3C4ErrorClasses(t *testing.T) {
	for _, status := range []int{502, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			g := newGitFixture()
			g.add("a/SKILL.md", "# a")
			g.status["/repos/a/x"] = []int{status}
			a := g.client(t)
			s := Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "a/SKILL.md"}, {Repository: "a/x", Slug: "a"}}}}}
			c := a.Sync(nil, "HEAD", s, 10)
			want := 1
			calls := 2
			if status == 404 {
				want = 0
				calls = 1
			}
			if len(c.Skills) != want || g.calls["/repos/a/x"] != calls {
				t.Fatal(c, g.calls)
			}
		})
	}
}
func TestC6C7C8CompleteAdmissionAndSharedAttribution(t *testing.T) {
	g := newGitFixture()
	g.add("a/SKILL.md", strings.Repeat("A", 500))
	g.add("b/SKILL.md", strings.Repeat("B", 500))
	g.add("LICENSE", "required license")
	for _, kind := range []string{"raw", "serialized", "shared"} {
		t.Run(kind, func(t *testing.T) {
			a := g.client(t)
			switch kind {
			case "raw":
				a.RawLimit = 600
			case "serialized":
				one := g.client(t).Sync(nil, "HEAD", Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "a/SKILL.md"}}}}}, 10)
				b, _ := Encode(one)
				a.OutputLimit = len(b) + 2048 + 10
			case "shared":
				a.RawLimit = 1000 + len("required license")
			}
			c := a.Sync([]string{"a/x"}, "HEAD", Search{}, 10)
			want := 1
			if kind == "shared" {
				want = 2
			}
			if len(c.Skills) != want || len(c.Attribution) != 1 {
				t.Fatal(kind, len(c.Skills), c.Diagnostics)
			}
			if kind != "shared" && (!hasCode(c.Diagnostics, "sync_budget") || c.Complete) {
				t.Fatal(c)
			}
			b, e := Encode(c)
			if e != nil || len(b) > a.OutputLimit {
				t.Fatal(len(b), a.OutputLimit, e)
			}
			var roundtrip Catalog
			if e = json.Unmarshal(b, &roundtrip); e != nil || ValidateCatalog(roundtrip) != nil {
				t.Fatal(e)
			}
			if len(roundtrip.Attribution) != 1 {
				t.Fatal(roundtrip)
			}
		})
	}
}
func TestC7DiagnosticAccumulationBound(t *testing.T) {
	a := newGitFixture().client(t)
	a.OutputLimit = 4096
	var repos []string
	for i := 0; i < 100; i++ {
		repos = append(repos, fmt.Sprintf("invalid-%d", i))
	}
	c := a.Sync(repos, "HEAD", Search{}, 10)
	b, _ := Encode(c)
	if len(b) > a.OutputLimit || !hasCode(c.Diagnostics, "sync_budget") {
		t.Fatal(len(b), c.Diagnostics)
	}
}
func TestT1T4AcquisitionRejectsUnsafeAndAttributionFailure(t *testing.T) {
	for _, kind := range []string{"private", "truncated", "unsafe", "symlink", "attribution"} {
		t.Run(kind, func(t *testing.T) {
			g := newGitFixture()
			g.add("a/SKILL.md", "# a")
			switch kind {
			case "private":
				g.private = true
			case "truncated":
				g.truncated = true
			case "unsafe":
				g.add("../SKILL.md", "unsafe")
			case "symlink":
				g.entries[0].Mode = "120000"
			case "attribution":
				g.add("LICENSE", "license")
				sha := g.entries[1].SHA
				g.status["/repos/a/x/git/blobs/"+sha] = []int{502}
			}
			c := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 10)
			if len(c.Skills) != 0 || c.Complete {
				t.Fatal(c)
			}
		})
	}
}
func TestT5NameOptInBeforeRequests(t *testing.T) {
	root := tempDir(t)
	for i := 0; i < 11; i++ {
		fixture(t, root, fmt.Sprintf("p%d/SKILL.md", i), fmt.Sprintf("---\nname: name%d\n---\n", i))
	}
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	p := fixture(t, tempDir(t), "inventory.json", "")
	if e := WriteJSON(context.Background(), p, inv); e != nil {
		t.Fatal(e)
	}
	if _, e := Queries("", p, false); e == nil {
		t.Fatal("missing opt-in")
	}
	if _, e := Queries("", p, true); e == nil {
		t.Fatal("more than ten names accepted")
	}
}
func TestAcquisitionRequestLimitsAndPinnedRetry(t *testing.T) {
	g := newGitFixture()
	g.add("a/SKILL.md", "# a")
	treePath := "/repos/a/x/git/trees/" + strings.Repeat("b", 40)
	g.status[treePath] = []int{502}
	a := g.client(t)
	search := Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "a/SKILL.md"}, {Repository: "a/x", Slug: "a"}}}}}
	c := a.Sync(nil, "HEAD", search, 10)
	if len(c.Skills) != 1 || g.calls["/repos/a/x/commits/HEAD"] != 1 || g.calls[treePath] != 2 {
		t.Fatal(c, g.calls)
	}
	g = newGitFixture()
	g.add("a/SKILL.md", "# a")
	a = g.client(t)
	a.Requests = 0
	c = a.Sync([]string{"a/x"}, "HEAD", Search{}, 10)
	if c.Complete || len(g.calls) != 0 {
		t.Fatal(c, g.calls)
	}
}
func TestPinnedSearchBoundaries(t *testing.T) {
	a := NewAcquirer(context.Background())
	a.Token = "github-secret"
	a.RegistryToken = "registry-secret"
	requests := 0
	a.Client = &http.Client{Transport: fakeTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		var body string
		if r.URL.Host == "api.github.com" {
			if r.Header.Get("Authorization") != "Bearer github-secret" || r.URL.Query().Get("q") != "fixture filename:SKILL.md" {
				t.Fatal(r)
			}
			body = `{"incomplete_results":false,"total_count":1,"items":[{"path":"skills/a/SKILL.md","html_url":"https://untrusted.invalid/not-fetched","repository":{"full_name":"a/x","private":false}}]}`
		} else {
			if r.Header.Get("Authorization") != "Bearer registry-secret" {
				t.Fatal("credential crossed boundary")
			}
			body = `{"data":[{"source":"a/x","sourceType":"github","slug":"a","url":"https://untrusted.invalid/not-fetched"}],"count":1}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	s := a.Search([]string{"fixture"}, "", 20)
	if !s.Complete() || len(s.Hits()) != 1 || s.Hits()[0].Path != "skills/a/SKILL.md" {
		t.Fatal(s)
	}
	s = a.Search([]string{"fixture"}, "https://registry.invalid/v1", 20)
	if !s.Complete() || s.Hits()[0].Slug != "a" || requests != 2 {
		t.Fatal(s)
	}
	before := requests
	s = a.Search([]string{"fixture"}, "http://registry.invalid/v1", 20)
	if s.Complete() || requests != before {
		t.Fatal("insecure registry contacted")
	}
}
func TestC1CachedBlobStillValidatesEntryMetadata(t *testing.T) {
	g := newGitFixture()
	g.add("a/SKILL.md", "# fixture")
	a := g.client(t)
	repo, e := a.resolve("a/x", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	entry := repo.entries[0]
	if _, e = a.blob(repo, entry); e != nil {
		t.Fatal(e)
	}
	entry.Size++
	if _, e = a.blob(repo, entry); e == nil {
		t.Fatal("cached blob bypassed declared-size validation")
	}
	entry = repo.entries[0]
	entry.Mode = "120000"
	if _, e = a.blob(repo, entry); e == nil {
		t.Fatal("cached blob bypassed mode validation")
	}
}

func TestReview2MetadataUsesSameReferenceFingerprint(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	g.add("p/.DS_Store", "metadata")
	g.add("p/.git/config", "metadata")
	g.add("p/nested/.svn/entries", "metadata")
	g.add("LICENSE", "attribution")
	c := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 20)
	if len(c.Skills) != 1 {
		t.Fatal(c)
	}
	r := c.Skills[0]
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	fixture(t, root, ".DS_Store", "metadata")
	fixture(t, root, ".git/config", "metadata")
	fixture(t, root, "nested/.svn/entries", "metadata")
	p := Measure(context.Background(), root, "")
	if p.Digest != r.Digest || NewMatcher(c).Match(p).Status != "exact" {
		t.Fatal(p.Digest, r.Digest, c)
	}
	if len(r.Files) != 1 || len(r.Data) != 1 || len(c.Attribution) != 1 {
		t.Fatal(r, c.Attribution)
	}
}

func TestReview2FinalSearchDiagnosticCanExhaustSyncBudget(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	first := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 20)
	b, _ := Encode(first)
	a := g.client(t)
	a.OutputLimit = len(b) + 2048 + 1
	c := a.Sync([]string{"a/x"}, "HEAD", Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: false}}}, 20)
	if len(c.Skills) != 1 || c.Complete || !hasCode(c.Diagnostics, "sync_budget") {
		t.Fatal(c)
	}
	b, _ = Encode(c)
	if len(b) > a.OutputLimit {
		t.Fatal(len(b), a.OutputLimit)
	}
}

func TestReview3AttributionRequestsSurviveEmptyCache(t *testing.T) {
	g := newGitFixture()
	for i := 0; i < 40; i++ {
		g.add(fmt.Sprintf("p%02d/SKILL.md", i), fmt.Sprintf("# fixture %d", i))
	}
	for _, p := range []string{"LICENSE", "NOTICE", "COPYING"} {
		g.add(p, p+" fixture attribution")
	}
	a := g.client(t)
	a.CacheLimit = 0
	c := a.Sync([]string{"a/x"}, "HEAD", Search{}, 100)
	if !c.Complete || len(c.Skills) != 40 || len(c.Attribution) != 3 {
		t.Fatal(len(c.Skills), len(c.Attribution), c.Diagnostics)
	}
	for _, p := range []string{"LICENSE", "NOTICE", "COPYING"} {
		key := "/repos/a/x/git/blobs/" + gitHash([]byte(p+" fixture attribution"))
		if g.calls[key] != 1 {
			t.Fatal(key, g.calls[key])
		}
	}
	if a.Requests != 54 {
		t.Fatal("unexpected request use", a.Requests)
	}
}

func TestReview3ResolutionCachePinsTransientTreeRetry(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	tree := "/repos/a/x/git/trees/" + strings.Repeat("b", 40)
	g.status[tree] = []int{502, 200}
	c := g.client(t).Sync(nil, "HEAD", Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "p/SKILL.md"}, {Repository: "a/x", Slug: "p"}}}}}, 20)
	if len(c.Skills) != 1 || g.calls[tree] != 2 || g.calls["/repos/a/x"] != 1 || g.calls["/repos/a/x/commits/HEAD"] != 1 {
		t.Fatal(c, g.calls)
	}
}

func TestQ1Q2Q3SearchProvenanceRoundtrip(t *testing.T) {
	legacy := `{"schema":"skill-audit/search/v2","source":"skills-sh-v1","query":"prior; opaque query","hits":[{"repository":"a/x","hint":"p","url":"https://untrusted.invalid/not-fetched"}],"incomplete":true,"diagnostics":[{"code":"prior_gap","path":"","message":"fixture","incomplete":true}]}`
	var saved Search
	if e := json.Unmarshal([]byte(legacy), &saved); e != nil {
		t.Fatal(e)
	}
	next := Search{Schema: SearchSchema, Inputs: []SearchInput{{Source: "github-code-search", Queries: []string{"fresh"}, Complete: true, Hits: []Hit{{Repository: "a/x", Path: "p/SKILL.md"}}}}}
	empty := Search{Schema: SearchSchema, Inputs: []SearchInput{{Source: "skills-sh-v1", Queries: []string{"empty"}, Complete: false, Diagnostics: []Diagnostic{diag("search_failed", "", "fixture")}}}}
	merged := MergeSearch(MergeSearch(saved, next), empty)
	b, e := Encode(merged)
	if e != nil {
		t.Fatal(e)
	}
	var read Search
	if e = json.Unmarshal(b, &read); e != nil {
		t.Fatal(e)
	}
	if read.Schema != SearchSchema || len(read.Inputs) != 3 || len(read.Hits()) != 2 || read.Complete() || len(read.Diagnostics()) != 2 {
		t.Fatal(read, string(b))
	}
	if read.Inputs[0].Source != "skills-sh-v1" || len(read.Inputs[0].Queries) != 1 || read.Inputs[0].Queries[0] != "prior; opaque query" || len(read.Inputs[0].Hits) != 1 || read.Inputs[1].Source != "github-code-search" || read.Inputs[1].Queries[0] != "fresh" || len(read.Inputs[2].Hits) != 0 {
		t.Fatal(read.Inputs)
	}
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	c := g.client(t).Sync(nil, "HEAD", read, 20)
	if len(c.Skills) != 1 || !hasCode(c.Diagnostics, "search_partial") || g.calls["/repos/a/x/git/trees/"+strings.Repeat("b", 40)] != 1 {
		t.Fatal(c, g.calls)
	}
	for _, bad := range []string{`{"schema":"skill-audit/search/v99"}`, `{"schema":"skill-audit/search/v3"}`} {
		if json.Unmarshal([]byte(bad), &read) == nil {
			t.Fatal("accepted", bad)
		}
	}
}

func TestReview3SearchTupleOrdering(t *testing.T) {
	hits := []Hit{{Repository: "a/b", Path: "c", URL: "second"}, {Repository: "a/bc", URL: "third"}, {Repository: "a/b", Path: "c", URL: "first"}}
	var want []byte
	for _, reverse := range []bool{false, true} {
		if reverse {
			for i, j := 0, len(hits)-1; i < j; i, j = i+1, j-1 {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
		s := MergeSearch(Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: hits}}})
		if s.Hits()[0].URL != "first" || s.Hits()[1].URL != "second" || s.Hits()[2].Repository != "a/bc" {
			t.Fatal(s.Hits())
		}
		b, _ := Encode(s)
		if want == nil {
			want = b
		} else if !bytes.Equal(want, b) {
			t.Fatal("unstable search", string(b))
		}
	}
}

func TestReview3ImportTargetTupleOrderingUnderBudget(t *testing.T) {
	g := newGitFixture()
	g.add("a/SKILL.md", "# a")
	g.add("a2/SKILL.md", "# a2")
	a := g.client(t)
	a.RawLimit = len("# a")
	c := a.Sync(nil, "HEAD", Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "a", Slug: "z"}, {Repository: "a/x", Path: "a2"}}}}}, 20)
	if len(c.Skills) != 1 || c.Skills[0].UpstreamPath != "a" || !hasCode(c.Diagnostics, "sync_budget") {
		t.Fatal(c)
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

func TestReviewC5GitHubCaseVariantsShareRepository(t *testing.T) {
	g := newGitFixture()
	g.add("a/SKILL.md", "# fixture")
	c := g.client(t).Sync([]string{"Anthropics/Skills"}, "HEAD", Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "anthropics/skills", Path: "a/SKILL.md"}}}}}, 20)
	if len(c.Skills) != 1 || !c.Complete || g.calls["/repos/anthropics/skills/git/trees/"+strings.Repeat("b", 40)] != 1 || g.calls["/repos/Anthropics/Skills"] != 0 {
		t.Fatal(c, g.calls)
	}
}

func TestT1T4OfflineAuditWithoutRequestsOrCredentials(t *testing.T) {
	prior := http.DefaultTransport
	http.DefaultTransport = fakeTransport(func(*http.Request) (*http.Response, error) { t.Fatal("offline audit attempted HTTP"); return nil, nil })
	t.Cleanup(func() { http.DefaultTransport = prior })
	token := "fixture-token-must-not-be-recorded"
	t.Setenv("GH_TOKEN", token)
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	catalog, e := BuiltinCatalog()
	if e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(tempDir(t), "state.json")
	r := AuditRun(context.Background(), InventoryRun(context.Background(), fixtureOptions(root)), catalog, AuditOptions{StatePath: state, Record: "baseline"})
	if Exit(r) != 0 || r.Recording.Status != "recorded" || r.Skills[0].Evaluations[1].Status != "disabled" {
		t.Fatal(r)
	}
	report, _ := Encode(r)
	stored, e := os.ReadFile(state)
	if e != nil || bytes.Contains(report, []byte(token)) || bytes.Contains(stored, []byte(token)) {
		t.Fatal("credential reached evidence", e)
	}
}

func TestC2EvictedRepositoryReloadKeepsIdentity(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	a := g.client(t)
	a.CacheLimit = 0
	load := func(repo string) Reference {
		t.Helper()
		r, e := a.resolve(repo, "HEAD")
		if e != nil {
			t.Fatal(e)
		}
		ref, _, e := a.importPackage(r, "p", nil)
		if e != nil {
			t.Fatal(e)
		}
		return ref
	}
	first := load("a/x")
	_ = load("b/y")
	reloaded := load("a/x")
	if first.ID != reloaded.ID || first.Digest != reloaded.Digest {
		t.Fatal("reload changed logical import", first, reloaded)
	}
	a.RawLimit = 2 * len("# fixture")
	search := Search{Schema: SearchSchema, Inputs: []SearchInput{{Complete: true, Hits: []Hit{{Repository: "a/x", Path: "p/SKILL.md"}, {Repository: "b/y", Path: "p/SKILL.md"}, {Repository: "a/x", Slug: "p"}}}}}
	c := a.Sync(nil, "HEAD", search, 20)
	if !c.Complete || len(c.Skills) != 2 {
		t.Fatal("reload/duplicate charged as extra content", c)
	}
}
