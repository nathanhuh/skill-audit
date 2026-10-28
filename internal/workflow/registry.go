package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"skill-audit/internal/audit"
)

// SearchRegistry implements only the documented Skills.sh v1 search response.
// Endpoint and token are explicit. GitHub credentials never enter this client.
func SearchRegistry(ctx context.Context, endpoint, token, query string, limit int) (Search, error) {
	return searchRegistry(ctx, &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("registry redirect refused") }}, endpoint, token, query, limit, false)
}
func searchRegistry(ctx context.Context, client *http.Client, endpoint, token, query string, limit int, allowHTTP bool) (Search, error) {
	out := Search{Schema: "skill-audit/search/v2", Source: "skills-sh-v1", Query: query, Hits: []Hit{}, Diagnostics: []audit.Diagnostic{}}
	u, e := url.Parse(endpoint)
	if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
		return out, fmt.Errorf("registry URL must be an explicit HTTPS endpoint without userinfo/fragment")
	}
	if strings.TrimSpace(query) == "" || len(query) > 180 || limit < 1 || limit > 100 {
		return out, fmt.Errorf("query must be 1–180 bytes and limit 1–100")
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", fmt.Sprint(limit))
	u.RawQuery = q.Encode()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return out, e
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, e := client.Do(req)
	if e != nil {
		return out, fmt.Errorf("registry transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return out, fmt.Errorf("registry HTTP %d; configure its own token if required", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20+1))
	if e != nil || len(b) > 2<<20 {
		return out, fmt.Errorf("registry response unreadable or oversized")
	}
	var raw struct {
		Data []struct {
			Slug       string `json:"slug"`
			Source     string `json:"source"`
			SourceType string `json:"sourceType"`
			URL        string `json:"url"`
		} `json:"data"`
		Count *int `json:"count"`
	}
	if e = json.Unmarshal(b, &raw); e != nil || raw.Data == nil || raw.Count == nil || *raw.Count < 0 {
		return out, fmt.Errorf("unsupported registry schema; expected Skills.sh v1 data/count")
	}
	out.Incomplete = len(raw.Data) >= limit || *raw.Count > len(raw.Data) // API does not prove exhaustive global results.
	for i, record := range raw.Data {
		if i >= limit {
			out.Incomplete = true
			break
		}
		if record.SourceType != "github" || !repoPattern.MatchString(record.Source) {
			out.Incomplete = true
			out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Code: "registry_record_skipped", Message: "non-GitHub or malformed source skipped", Incomplete: true})
			continue
		}
		out.Hits = append(out.Hits, Hit{Repository: record.Source, Hint: record.Slug, URL: record.URL})
	}
	sort.Slice(out.Hits, func(i, j int) bool {
		a, b := out.Hits[i], out.Hits[j]
		return a.Repository+"\x00"+a.Hint+"\x00"+a.URL < b.Repository+"\x00"+b.Hint+"\x00"+b.URL
	})
	return out, nil
}
