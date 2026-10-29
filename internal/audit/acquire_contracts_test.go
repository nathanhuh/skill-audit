package audit

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
