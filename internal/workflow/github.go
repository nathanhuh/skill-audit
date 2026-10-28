package workflow

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"skill-audit/internal/audit"
)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

const MaxBlobCache = 16 << 20

type githubHTTPError struct {
	status     int
	retryAfter string
}

func (e *githubHTTPError) Error() string {
	return fmt.Sprintf("GitHub HTTP %d (check authorization/rate limits; retry-after=%q)", e.status, e.retryAfter)
}

type permanentRepositoryError struct{ message string }

func (e *permanentRepositoryError) Error() string { return e.message }
func permanentRepositoryFailure(err error) bool {
	var httpErr *githubHTTPError
	var repositoryErr *permanentRepositoryError
	return errors.As(err, &repositoryErr) || errors.As(err, &httpErr) && httpErr.status == http.StatusNotFound
}

type Client struct {
	BaseURL               string
	HTTP                  *http.Client
	Token                 string
	Requests, MaxRequests int
	repositories          map[string]*repositorySnapshot
	activeRepository      string
}

func NewClient(token string) *Client {
	return &Client{BaseURL: "https://api.github.com", Token: token, MaxRequests: 100, HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirect refused") }}}
}
func (c *Client) get(ctx context.Context, endpoint string, out any) error {
	if c.Requests >= c.MaxRequests {
		return fmt.Errorf("GitHub request budget exhausted")
	}
	c.Requests++
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+endpoint, nil)
	if e != nil {
		return e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "skill-audit")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("GitHub transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return &githubHTTPError{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After")}
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
	if e != nil {
		return e
	}
	if len(b) > 8<<20 {
		return fmt.Errorf("GitHub response exceeds 8 MiB")
	}
	return json.Unmarshal(b, out)
}

type Hit struct {
	Hint       string `json:"hint,omitempty"`
	Repository string `json:"repository"`
	Path       string `json:"path,omitempty"`
	URL        string `json:"url,omitempty"`
}
type Search struct {
	Schema      string             `json:"schema"`
	Source      string             `json:"source"`
	Query       string             `json:"query"`
	Hits        []Hit              `json:"hits"`
	Incomplete  bool               `json:"incomplete"`
	Diagnostics []audit.Diagnostic `json:"diagnostics"`
}

func (c *Client) Search(ctx context.Context, query string, limit int) (Search, error) {
	out := Search{Schema: "skill-audit/search/v2", Source: "github-code-search", Query: query, Hits: []Hit{}, Diagnostics: []audit.Diagnostic{}}
	if c.Token == "" {
		return out, fmt.Errorf("GitHub code search requires GH_TOKEN or GITHUB_TOKEN; explicit repository sync can work without auth")
	}
	if strings.TrimSpace(query) == "" || len(query) > 180 || limit < 1 || limit > 100 {
		return out, fmt.Errorf("query must be 1–180 bytes and limit 1–100")
	}
	var raw struct {
		Total      *int  `json:"total_count"`
		Incomplete *bool `json:"incomplete_results"`
		Items      []struct {
			Path       string `json:"path"`
			URL        string `json:"html_url"`
			Repository struct {
				FullName string `json:"full_name"`
				Private  *bool  `json:"private"`
			} `json:"repository"`
		} `json:"items"`
	}
	q := url.Values{"q": {query + " filename:SKILL.md"}, "per_page": {fmt.Sprint(limit)}}
	if e := c.get(ctx, "/search/code?"+q.Encode(), &raw); e != nil {
		return out, e
	}
	if raw.Total == nil || raw.Incomplete == nil || raw.Items == nil || *raw.Total < 0 {
		return out, fmt.Errorf("malformed GitHub search response")
	}
	out.Incomplete = *raw.Incomplete || *raw.Total > len(raw.Items)
	for i, item := range raw.Items {
		if i >= limit {
			out.Incomplete = true
			break
		}
		if item.Repository.Private == nil || *item.Repository.Private || !repoPattern.MatchString(item.Repository.FullName) || !safePath(item.Path) || path.Base(item.Path) != "SKILL.md" {
			out.Incomplete = true
			out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Code: "search_record_skipped", Message: "non-public or malformed result skipped", Incomplete: true})
			continue
		}
		out.Hits = append(out.Hits, Hit{Repository: item.Repository.FullName, Path: item.Path, URL: item.URL})
	}
	sort.Slice(out.Hits, func(i, j int) bool {
		a, b := out.Hits[i], out.Hits[j]
		return a.Repository+"\x00"+a.Path+"\x00"+a.URL < b.Repository+"\x00"+b.Path+"\x00"+b.URL
	})
	return out, nil
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

func (c *Client) Sync(ctx context.Context, repo, ref string, maxPackages int) (Store, error) {
	return c.SyncTarget(ctx, Hit{Repository: repo}, ref, maxPackages)
}

type repositorySnapshot struct {
	treeSHA     string
	err         error
	commit      string
	entries     []treeEntry
	blobs       map[string][]byte
	cachedBytes int
}

// A client represents one acquisition operation. Pin each repo/ref once so
// search hits share requests and immutable content, never a process-global cache.
func (c *Client) repository(ctx context.Context, repo, ref string) (result *repositorySnapshot, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := c.BaseURL + "\x00" + repo + "\x00" + ref
	if c.repositories == nil {
		c.repositories = map[string]*repositorySnapshot{}
	}
	snapshot := c.repositories[key]
	if snapshot != nil && snapshot.err != nil {
		return nil, snapshot.err
	}
	// Pin compact references and failures for the invocation, but retain content
	// for only the active repository. Catalog output owns any bytes it retains.
	if c.activeRepository != key {
		if previous := c.repositories[c.activeRepository]; previous != nil {
			previous.entries, previous.blobs = nil, nil
			previous.cachedBytes = 0
		}
		c.activeRepository = key
	}
	requestsBefore := c.Requests
	defer func() {
		if err != nil && ctx.Err() == nil && c.Requests > requestsBefore && permanentRepositoryFailure(err) {
			c.repositories[key] = &repositorySnapshot{err: err}
		}
	}()
	base := "/repos/" + repo
	if snapshot == nil {
		var meta struct {
			Private *bool `json:"private"`
		}
		if e := c.get(ctx, base, &meta); e != nil {
			return nil, e
		}
		if meta.Private == nil {
			return nil, fmt.Errorf("repository response omitted public/private status")
		}
		if *meta.Private {
			return nil, &permanentRepositoryError{message: "only explicitly public repositories are imported"}
		}
		var commit struct {
			SHA    string `json:"sha"`
			Commit struct {
				Tree struct {
					SHA string `json:"sha"`
				} `json:"tree"`
			} `json:"commit"`
		}
		if e := c.get(ctx, base+"/commits/"+url.PathEscape(ref), &commit); e != nil {
			return nil, e
		}
		if !shaPattern.MatchString(commit.SHA) || !shaPattern.MatchString(commit.Commit.Tree.SHA) {
			return nil, fmt.Errorf("missing immutable commit/tree identity")
		}
		snapshot = &repositorySnapshot{commit: commit.SHA, treeSHA: commit.Commit.Tree.SHA}
		c.repositories[key] = snapshot

	}
	if snapshot.blobs != nil {
		return snapshot, nil
	}
	var tree struct {
		Truncated bool        `json:"truncated"`
		Entries   []treeEntry `json:"tree"`
	}
	if e := c.get(ctx, base+"/git/trees/"+snapshot.treeSHA+"?recursive=1", &tree); e != nil {
		return nil, e
	}
	if tree.Truncated {
		return nil, &permanentRepositoryError{message: "GitHub tree truncated; repository import refused"}
	}
	sort.Slice(tree.Entries, func(i, j int) bool { return tree.Entries[i].Path < tree.Entries[j].Path })
	for _, node := range tree.Entries {
		if !safePath(node.Path) {
			return nil, &permanentRepositoryError{message: "unsafe tree path; import refused"}
		}
	}

	snapshot.entries = tree.Entries
	snapshot.blobs = map[string][]byte{}
	return snapshot, nil
}

// SyncTarget fetches the matching package path (or registry slug hint), never unrelated first-N packages.
func (c *Client) SyncTarget(ctx context.Context, target Hit, ref string, maxPackages int) (Store, error) {
	repo := target.Repository
	out := EmptyStore()
	if !repoPattern.MatchString(repo) || ref == "" || maxPackages < 1 || maxPackages > 100 {
		return out, fmt.Errorf("invalid repository/ref/package limit")
	}
	snapshot, err := c.repository(ctx, repo, ref)
	if err != nil {
		return out, err
	}
	base := "/repos/" + repo
	blob := func(node treeEntry) ([]byte, error) {
		if node.Type != "blob" || (node.Mode != "100644" && node.Mode != "100755") || !shaPattern.MatchString(node.SHA) || node.Size < 0 || node.Size > audit.MaxFile {
			return nil, fmt.Errorf("unsupported or oversized blob: %s", node.Path)
		}
		if b, ok := snapshot.blobs[node.SHA]; ok {
			if int64(len(b)) != node.Size {
				return nil, fmt.Errorf("cached blob size mismatch: %s", node.Path)
			}
			return b, nil
		}
		var raw struct {
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
			SHA      string `json:"sha"`
			Size     int64  `json:"size"`
		}
		if e := c.get(ctx, base+"/git/blobs/"+node.SHA, &raw); e != nil {
			return nil, e
		}
		if raw.Encoding != "base64" {
			return nil, fmt.Errorf("unexpected blob encoding")
		}
		b, e := base64.StdEncoding.DecodeString(strings.ReplaceAll(raw.Content, "\n", ""))
		if e != nil || len(b) > audit.MaxFile || int64(len(b)) != node.Size || raw.Size != node.Size || raw.SHA != node.SHA {
			return nil, fmt.Errorf("blob size/identity mismatch")
		}
		h := sha1.New()
		fmt.Fprintf(h, "blob %d%c", len(b), 0)
		h.Write(b)
		if hex.EncodeToString(h.Sum(nil)) != node.SHA {
			return nil, fmt.Errorf("blob hash mismatch")
		}
		// Cache admission bounds retained memory, never which valid skills
		// may be imported. Per-file/package and operation limits are separate.
		if snapshot.cachedBytes+len(b) <= MaxBlobCache {
			snapshot.cachedBytes += len(b)
			snapshot.blobs[node.SHA] = b
		}
		return b, nil
	}
	for _, n := range snapshot.entries {
		baseName := strings.ToLower(path.Base(n.Path))
		if !strings.Contains(n.Path, "/") && (strings.HasPrefix(baseName, "license") || strings.Contains(baseName, "notice")) {
			b, e := blob(n)
			if e != nil {
				out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Path: n.Path, Code: "attribution_error", Message: e.Error(), Incomplete: true})
				continue
			}
			out.Attribution[repo+"@"+snapshot.commit+"/"+n.Path] = b
		}
	}
	for _, manifest := range snapshot.entries {
		if target.Path != "" && manifest.Path != target.Path {
			continue
		}
		if target.Hint != "" && path.Base(path.Dir(manifest.Path)) != target.Hint {
			continue
		}
		if path.Base(manifest.Path) != "SKILL.md" {
			continue
		}
		if len(out.Entries) >= maxPackages {
			out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Code: "package_limit", Message: "additional repository packages omitted", Incomplete: true})
			break
		}
		dir := path.Dir(manifest.Path)
		prefix := dir + "/"
		if dir == "." {
			prefix = ""
		}
		data := map[string][]byte{}
		files := []audit.File{}
		failed := false
		packageBytes := 0
		for _, node := range snapshot.entries {
			if !strings.HasPrefix(node.Path, prefix) || node.Type == "tree" {
				continue
			}
			relative := strings.TrimPrefix(node.Path, prefix)
			if metadataPath(relative) {
				continue
			}
			if len(files) >= audit.MaxEntries {
				failed = true
				out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Path: dir, Code: "package_incomplete", Message: "package entry limit", Incomplete: true})
				break
			}
			b, e := blob(node)
			if e != nil {
				failed = true
				out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Path: node.Path, Code: "package_incomplete", Message: e.Error(), Incomplete: true})
				break
			}
			packageBytes += len(b)
			if packageBytes > audit.MaxPackage {
				failed = true
				out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Path: dir, Code: "package_incomplete", Message: "package exceeds 16 MiB", Incomplete: true})
				break
			}
			data[relative] = b
			files = append(files, audit.File{Path: relative, SHA256: audit.Hash(b), Size: int64(len(b))})
		}
		if failed || data["SKILL.md"] == nil {
			continue
		}
		r := audit.Reference{ID: repo + ":" + dir + "@" + snapshot.commit, Repository: "https://github.com/" + repo, Commit: snapshot.commit, UpstreamPath: dir, Digest: audit.Digest(files), Files: files}
		out.Entries = append(out.Entries, Entry{r, data})
	}
	if len(out.Entries) == 0 {
		out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Code: "no_complete_packages", Message: "repository yielded no complete packages", Incomplete: true})
	}
	return out, nil
}
func metadataPath(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if audit.ExcludedMetadata(part) {
			return true
		}
	}
	return false
}
