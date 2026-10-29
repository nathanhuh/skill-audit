package audit

import (
	"cmp"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Hit struct {
	Repository string `json:"repository"`
	Path       string `json:"path,omitempty"`
	Slug       string `json:"hint,omitempty"`
	URL        string `json:"url,omitempty"`
}

func Queries(query, inventory string, names bool) ([]string, error) {
	if inventory != "" && !names {
		return nil, fmt.Errorf("--inventory requires --search-names before transmitting names")
	}
	if names && inventory == "" {
		return nil, fmt.Errorf("--search-names requires --inventory")
	}
	var q []string
	if query != "" {
		q = append(q, query)
	}
	if names {
		i, e := ReadInventory(inventory)
		if e != nil {
			return nil, e
		}
		m := map[string]bool{}
		for _, p := range i.Packages {
			if p.Name != "" {
				m[p.Name] = true
			}
		}
		if len(m) > 10 {
			return nil, fmt.Errorf("at most ten distinct inventory names may be searched")
		}
		for s := range m {
			q = append(q, s)
		}
	}
	sort.Strings(q)
	return unique(q), nil
}

type Acquirer struct {
	Context                           context.Context
	Client                            *http.Client
	Base                              string
	Requests                          int
	RawLimit, OutputLimit, CacheLimit int
	Token, RegistryToken              string
	AllowHTTP                         bool
	failures                          map[string]error
	// Pins survive transient tree-fetch failures before an active repository exists.
	resolutions map[string]*repository
}

func NewAcquirer(ctx context.Context) *Acquirer {
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &Acquirer{Context: ctx, Client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Base: "https://api.github.com", Requests: 100, RawLimit: 40 << 20, OutputLimit: StoreLimit, CacheLimit: 16 << 20, Token: token, RegistryToken: os.Getenv("SKILL_AUDIT_REGISTRY_TOKEN")}
}

type requestError struct {
	status  int
	message string
}

func (e *requestError) Error() string { return e.message }
func permanent(e error) bool {
	x, ok := e.(*requestError)
	return ok && (x.status == 404 || x.status == -1)
}
func (a *Acquirer) get(endpoint, token string, v any) error {
	if e := a.failures[endpoint]; e != nil {
		return e
	}
	if a.Requests <= 0 {
		return fmt.Errorf("request budget exhausted")
	}
	if e := a.Context.Err(); e != nil {
		return e
	}
	a.Requests--
	req, e := http.NewRequestWithContext(a.Context, "GET", endpoint, nil)
	if e != nil {
		return fmt.Errorf("invalid acquisition endpoint")
	}
	responseLimit := int64(8 << 20)
	if strings.HasPrefix(endpoint, a.Base+"/") {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	} else {
		req.Header.Set("Accept", "application/json")
		responseLimit = 2 << 20
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := *a.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("acquisition transport failure")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		e := &requestError{resp.StatusCode, fmt.Sprintf("acquisition HTTP %d", resp.StatusCode)}
		if resp.StatusCode == 404 {
			if a.failures == nil {
				a.failures = map[string]error{}
			}
			a.failures[endpoint] = e
		}
		return e
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if e != nil || int64(len(b)) > responseLimit {
		return fmt.Errorf("bounded acquisition response unavailable")
	}
	if json.Unmarshal(b, v) != nil {
		return fmt.Errorf("invalid acquisition JSON")
	}
	return nil
}

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func validRepo(repo string) bool {
	if !repoName.MatchString(repo) {
		return false
	}
	for _, p := range strings.Split(repo, "/") {
		if p == "." || p == ".." {
			return false
		}
	}
	return true
}
func (a *Acquirer) Search(queries []string, registry string, limit int) Search {
	s := Search{Schema: SearchSchema, Source: "github-code-search", Queries: queries, Hits: []Hit{}, Complete: true}
	fail := func(e error) {
		s.Complete = false
		s.Diagnostics = append(s.Diagnostics, diag("search_failed", "", e.Error()))
	}
	for _, q := range queries {
		if strings.TrimSpace(q) == "" || len(q) > 180 || limit < 1 || limit > 100 {
			fail(fmt.Errorf("query must be 1–180 bytes and limit 1–100"))
			continue
		}
		if registry != "" {
			s.Source = "skills-sh-v1"
			u, e := url.Parse(registry)
			if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(a.AllowHTTP && u.Scheme == "http")) {
				fail(fmt.Errorf("invalid registry endpoint"))
				continue
			}
			params := u.Query()
			params.Set("q", q)
			params.Set("limit", fmt.Sprint(limit))
			u.RawQuery = params.Encode()
			var response struct {
				Data  []struct{ Source, SourceType, Slug, URL string }
				Count *int
			}
			if e = a.get(u.String(), a.RegistryToken, &response); e != nil {
				fail(e)
				continue
			}
			if response.Data == nil || response.Count == nil || *response.Count < 0 {
				fail(fmt.Errorf("unsupported registry data/count response"))
				continue
			}
			for i, r := range response.Data {
				if i >= limit {
					s.Complete = false
					break
				}
				repo := r.Source
				if r.SourceType != "github" || !validRepo(repo) {
					s.Complete = false
					s.Diagnostics = append(s.Diagnostics, diag("registry_source", "", "unsupported registry source"))
					continue
				}
				s.Hits = append(s.Hits, Hit{Repository: repo, Slug: r.Slug, URL: r.URL})
			}
			if *response.Count > len(response.Data) || len(response.Data) >= limit {
				s.Complete = false
			}
			continue
		}
		if a.Token == "" {
			fail(fmt.Errorf("GitHub code search requires GH_TOKEN or GITHUB_TOKEN"))
			continue
		}
		var response struct {
			Incomplete *bool `json:"incomplete_results"`
			Total      *int  `json:"total_count"`
			Items      []struct {
				Path       string
				HTMLURL    string `json:"html_url"`
				Repository struct {
					FullName string `json:"full_name"`
					Private  *bool
				}
			}
		}
		endpoint := a.Base + "/search/code?q=" + url.QueryEscape(q+" filename:SKILL.md") + "&per_page=" + fmt.Sprint(min(limit, 100))
		if e := a.get(endpoint, a.Token, &response); e != nil {
			fail(e)
			continue
		}
		if response.Incomplete == nil || response.Total == nil || response.Items == nil || *response.Total < 0 {
			fail(fmt.Errorf("malformed GitHub search response"))
			continue
		}
		if *response.Incomplete || *response.Total > len(response.Items) {
			s.Complete = false
		}
		for i, r := range response.Items {
			if i >= limit {
				s.Complete = false
				break
			}
			if r.Repository.Private == nil || *r.Repository.Private || !validRepo(r.Repository.FullName) || !SafeRelative(r.Path) || path.Base(r.Path) != "SKILL.md" {
				s.Complete = false
				continue
			}
			s.Hits = append(s.Hits, Hit{Repository: r.Repository.FullName, Path: r.Path, URL: r.HTMLURL})
		}
	}
	return MergeSearch(s)
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}
type repository struct {
	repo, commit, tree string
	entries            []treeEntry
	blobs              map[string][]byte
	cached             int
}

func (a *Acquirer) resolve(repo, ref string) (*repository, error) {
	if !validRepo(repo) {
		return nil, &requestError{-1, "invalid repository name"}
	}
	prefix := a.Base + "/repos/" + repo
	key := repo + "\x00" + ref
	snapshot := a.resolutions[key]
	if snapshot == nil {
		var meta struct{ Private *bool }
		if e := a.get(prefix, a.Token, &meta); e != nil {
			return nil, e
		}
		if meta.Private == nil || *meta.Private {
			return nil, &requestError{-1, "repository is not explicitly public"}
		}
		var commit struct {
			SHA    string
			Commit struct{ Tree struct{ SHA string } }
		}
		if e := a.get(prefix+"/commits/"+url.PathEscape(ref), a.Token, &commit); e != nil {
			return nil, e
		}
		if !gitID(commit.SHA) || !gitID(commit.Commit.Tree.SHA) {
			return nil, &requestError{-1, "invalid commit or tree identity"}
		}
		snapshot = &repository{repo: repo, commit: commit.SHA, tree: commit.Commit.Tree.SHA}
		if a.resolutions == nil {
			a.resolutions = map[string]*repository{}
		}
		a.resolutions[key] = snapshot
	}
	var tree struct {
		Truncated bool
		Tree      []treeEntry
	}
	if e := a.get(prefix+"/git/trees/"+snapshot.tree+"?recursive=1", a.Token, &tree); e != nil {
		return nil, e
	}
	if tree.Truncated {
		return nil, &requestError{-1, "truncated repository tree"}
	}
	seen := map[string]bool{}
	for _, e := range tree.Tree {
		if !SafeRelative(e.Path) || seen[e.Path] || !gitID(e.SHA) {
			return nil, &requestError{-1, "unsafe repository tree"}
		}
		seen[e.Path] = true
	}
	sort.Slice(tree.Tree, func(i, j int) bool { return tree.Tree[i].Path < tree.Tree[j].Path })
	return &repository{repo: repo, commit: snapshot.commit, tree: snapshot.tree, entries: tree.Tree, blobs: map[string][]byte{}}, nil
}
func gitID(s string) bool {
	if len(s) != 40 {
		return false
	}
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 20
}
func (a *Acquirer) blob(r *repository, t treeEntry) ([]byte, error) {
	if t.Type != "blob" || (t.Mode != "100644" && t.Mode != "100755") || t.Size < 0 || t.Size > FileLimit {
		return nil, fmt.Errorf("unsupported or oversized Git blob")
	}
	if b, ok := r.blobs[t.SHA]; ok {
		if len(b) != t.Size {
			return nil, fmt.Errorf("cached Git blob size disagrees with tree entry")
		}
		return b, nil
	}
	var response struct {
		SHA, Encoding, Content string
		Size                   int
	}
	if e := a.get(a.Base+"/repos/"+r.repo+"/git/blobs/"+t.SHA, a.Token, &response); e != nil {
		return nil, e
	}
	if response.Encoding != "base64" || response.SHA != t.SHA {
		return nil, fmt.Errorf("invalid Git blob metadata")
	}
	b, e := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
	if e != nil || len(b) != t.Size || len(b) != response.Size || len(b) > FileLimit {
		return nil, fmt.Errorf("invalid Git blob content")
	}
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	if hex.EncodeToString(h.Sum(nil)) != t.SHA {
		return nil, fmt.Errorf("Git blob identity mismatch")
	}
	if r.cached+len(b) <= a.CacheLimit {
		r.blobs[t.SHA] = b
		r.cached += len(b)
	}
	return b, nil
}
func (a *Acquirer) importPackage(r *repository, dir string, retained map[string]bool) (Reference, []Attribution, error) {
	out := Reference{ID: r.repo + ":" + dir + "@" + r.commit, Repository: "https://github.com/" + r.repo, Commit: r.commit, UpstreamPath: dir, Data: map[string][]byte{}}
	total, count := 0, 0
	for _, entry := range r.entries {
		if entry.Path != dir && !(dir == "." || strings.HasPrefix(entry.Path, dir+"/")) {
			continue
		}
		rel := entry.Path
		if dir != "." {
			rel = strings.TrimPrefix(rel, dir+"/")
		}
		if fingerprintMetadataPath(rel) {
			continue
		}
		if entry.Type == "tree" {
			if entry.Mode != "040000" {
				return out, nil, fmt.Errorf("invalid Git directory mode")
			}
			continue
		}
		count++
		if count > PackageEntries {
			return out, nil, fmt.Errorf("package entry limit")
		}
		b, e := a.blob(r, entry)
		if e != nil {
			return out, nil, e
		}
		total += len(b)
		if total > PackageLimit {
			return out, nil, fmt.Errorf("package byte limit")
		}
		out.Data[rel] = b
		out.Files = append(out.Files, File{rel, Sum(b), int64(len(b))})
	}
	if _, ok := out.Data["SKILL.md"]; !ok {
		return out, nil, fmt.Errorf("manifest not imported")
	}
	out.Digest = Digest(out.Files)
	var attribution []Attribution
	for _, entry := range r.entries {
		if strings.Contains(entry.Path, "/") {
			continue
		}
		upper := strings.ToUpper(entry.Path)
		if !strings.HasPrefix(upper, "LICENSE") && !strings.HasPrefix(upper, "NOTICE") && !strings.HasPrefix(upper, "COPYING") {
			continue
		}
		if retained[out.Repository+"\x00"+r.commit+"\x00"+entry.Path] {
			continue
		}
		b, e := a.blob(r, entry)
		if e != nil {
			return out, nil, fmt.Errorf("required attribution %q unavailable: %w", entry.Path, e)
		}
		attribution = append(attribution, Attribution{Repository: out.Repository, Commit: r.commit, Path: entry.Path, Data: b})
	}
	return out, attribution, nil
}

type importTarget struct {
	repo, dir, slug string
	full            bool
}

func (a *Acquirer) Sync(explicit []string, ref string, search Search, limit int) Catalog {
	c := Catalog{Schema: StoreSchema, Skills: []Reference{}, Complete: true}
	if ref == "" {
		ref = "HEAD"
	}
	groups := map[string][]importTarget{}
	explicitSet := map[string]bool{}
	for _, r := range explicit {
		r = strings.ToLower(r)
		if !explicitSet[r] {
			groups[r] = append(groups[r], importTarget{repo: r, full: true})
			explicitSet[r] = true
		}
	}
	for _, h := range search.Hits {
		h.Repository = strings.ToLower(h.Repository)
		groups[h.Repository] = append(groups[h.Repository], importTarget{h.Repository, h.Path, h.Slug, false})
	}
	order := []string{}
	for r := range groups {
		order = append(order, r)
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := explicitSet[order[i]], explicitSet[order[j]]
		if a != b {
			return a
		}
		return order[i] < order[j]
	})
	// Headroom is retained for a bounded final sync_budget diagnostic and formatting.
	const reserve = 2048
	raw := 0
	seen := map[string]bool{}
	attrs := map[string]bool{}
	budget := false
	budgetReason := "import admission"
	sizes := newStoreSize(c)
	diagnostic := func(code, p, msg string) {
		c.Complete = false
		d := diag(code, p, msg)
		if len(d.Message) > 240 {
			d.Message = string(utf8Prefix([]byte(d.Message), 220)) + "… [truncated]"
		}
		delta, err := sizes.diagnostic(d)
		if err == nil && sizes.bytes+delta+reserve <= a.OutputLimit {
			c.Diagnostics = append(c.Diagnostics, d)
			sizes.bytes += delta
			sizes.diagnostics++
		} else {
			budget = true
			budgetReason = "diagnostic evidence capacity"
		}
	}
	for _, repo := range order {
		var active *repository
		var permanentErr error
		targets := groups[repo]
		sort.SliceStable(targets, func(i, j int) bool {
			if targets[i].full != targets[j].full {
				return targets[i].full
			}
			a, b := targets[i], targets[j]
			return cmp.Or(cmp.Compare(a.dir, b.dir), cmp.Compare(a.slug, b.slug)) < 0
		})
		for _, target := range targets {
			if budget {
				break
			}
			if permanentErr != nil {
				diagnostic("import_failed", repo, permanentErr.Error())
				continue
			}
			if active == nil {
				var e error
				active, e = a.resolve(repo, ref)
				if e != nil {
					if permanent(e) {
						permanentErr = e
					}
					diagnostic("import_failed", repo, e.Error())
					continue
				}
			}
			dirs := []string{}
			for _, entry := range active.entries {
				if path.Base(entry.Path) != "SKILL.md" || entry.Type != "blob" {
					continue
				}
				dir := path.Dir(entry.Path)
				if target.full || target.dir != "" && (dir == target.dir || dir == path.Dir(target.dir) && path.Base(target.dir) == "SKILL.md") || target.slug != "" && path.Base(dir) == target.slug {
					dirs = append(dirs, dir)
				}
			}
			if len(dirs) == 0 {
				diagnostic("target_missing", repo, "requested package target missing")
				continue
			}
			if len(dirs) > limit {
				diagnostic("package_limit", repo, "packages omitted by per-target package limit")
				dirs = dirs[:limit]
			}
			for _, dir := range dirs {
				key := repo + "\x00" + active.commit + "\x00" + dir
				if seen[key] {
					continue
				}
				r, at, e := a.importPackage(active, dir, attrs)
				if e != nil {
					diagnostic("import_failed", repo+"/"+dir, e.Error())
					continue
				}
				extra := 0
				for _, b := range r.Data {
					extra += len(b)
				}
				delta, e := sizes.entry(r)
				newAttrs := []Attribution{}
				for _, x := range at {
					k := x.Repository + "\x00" + x.Commit + "\x00" + x.Path
					if !attrs[k] {
						extra += len(x.Data)
						n, err := sizes.attribute(x, len(newAttrs))
						if err != nil {
							e = err
						}
						delta += n
						newAttrs = append(newAttrs, x)
					}
				}
				if e != nil || raw+extra > a.RawLimit || sizes.bytes+delta+reserve > a.OutputLimit {
					if raw+extra > a.RawLimit {
						budgetReason = "retained raw content"
					} else {
						budgetReason = "serialized output with diagnostic headroom"
					}
					budget = true
					break
				}
				c.Skills = append(c.Skills, r)
				c.Attribution = append(c.Attribution, newAttrs...)
				sizes.bytes += delta
				sizes.entries++
				sizes.attribution += len(newAttrs)
				raw += extra
				seen[key] = true
				for _, k := range newAttrs {
					attrs[k.Repository+"\x00"+k.Commit+"\x00"+k.Path] = true
				}
			}
		}
	}
	if !search.Complete && search.Schema != "" && !budget {
		diagnostic("search_partial", "", "search coverage incomplete")
	}
	if budget {
		c.Complete = false
		c.Diagnostics = append(c.Diagnostics, diag("sync_budget", "", "acquisition stopped at "+budgetReason+" budget; next import/diagnostic omitted; earlier imports and required attribution retained"))
	}
	return c
}
