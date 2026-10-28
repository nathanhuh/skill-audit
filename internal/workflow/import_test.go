package workflow

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type importTransport func(*http.Request) (*http.Response, error)

func (f importTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func importFixture(t *testing.T, entries []treeEntry, blobs map[string][]byte) (*Client, map[string]int) {
	t.Helper()
	calls := map[string]int{}
	c := NewClient("")
	c.HTTP.Transport = importTransport(func(r *http.Request) (*http.Response, error) {
		calls[r.URL.Path]++
		var payload any
		switch {
		case r.URL.Path == "/repos/owner/repo":
			payload = map[string]any{"private": false}
		case strings.Contains(r.URL.Path, "/commits/"):
			payload = map[string]any{"sha": strings.Repeat("a", 40), "commit": map[string]any{"tree": map[string]any{"sha": strings.Repeat("b", 40)}}}
		case strings.Contains(r.URL.Path, "/git/trees/"):
			payload = map[string]any{"truncated": false, "tree": entries}
		case strings.Contains(r.URL.Path, "/git/blobs/"):
			sha := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			b, ok := blobs[sha]
			if !ok {
				return nil, fmt.Errorf("unexpected blob")
			}
			payload = map[string]any{"sha": sha, "size": len(b), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(b)}
		default:
			return nil, fmt.Errorf("unexpected request")
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b))}, nil
	})
	return c, calls
}
func TestCachedBlobStillValidatesEachTreeEntry(t *testing.T) {
	body := []byte("fixture manifest and reused bytes\n")
	sha := gitBlob(body)
	for _, tc := range []struct {
		name, mode, kind string
		size             int64
		valid            bool
	}{
		{"symlink", "120000", "blob", int64(len(body)), false},
		{"submodule", "160000", "commit", int64(len(body)), false},
		{"inconsistent size", "100644", "blob", int64(len(body) + 1), false},
		{"regular duplicate bytes", "100644", "blob", int64(len(body)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := []treeEntry{{"LICENSE", "100644", "blob", sha, int64(len(body))}, {"skills/pkg/SKILL.md", "100644", "blob", sha, int64(len(body))}, {"skills/pkg/z-file", tc.mode, tc.kind, sha, tc.size}}
			c, _ := importFixture(t, entries, map[string][]byte{sha: body})
			store, err := c.Sync(context.Background(), "owner/repo", "HEAD", 10)
			if err != nil {
				t.Fatal(err)
			}
			if tc.valid && len(store.Entries) != 1 || !tc.valid && len(store.Entries) != 0 {
				t.Fatal("cache bypassed tree metadata", store)
			}
		})
	}
}
func TestRepositoryAndBlobsReusedAcrossTargets(t *testing.T) {
	body := []byte("shared fixture\n")
	sha := gitBlob(body)
	entries := []treeEntry{{"skills/a/SKILL.md", "100644", "blob", sha, int64(len(body))}, {"skills/b/SKILL.md", "100644", "blob", sha, int64(len(body))}}
	c, calls := importFixture(t, entries, map[string][]byte{sha: body})
	c.MaxRequests = 4
	for _, name := range []string{"a", "b", "a"} {
		s, err := c.SyncTarget(context.Background(), Hit{Repository: "owner/repo", Path: "skills/" + name + "/SKILL.md"}, "HEAD", 1)
		if err != nil || len(s.Entries) != 1 || s.Entries[0].Reference.UpstreamPath != "skills/"+name {
			t.Fatal("repeated target spent repository request budget", s, err)
		}
	}
	if c.Requests != 4 {
		t.Fatal(c.Requests, calls)
	}
	for _, n := range calls {
		if n != 1 {
			t.Fatal("duplicate request", calls)
		}
	}
	// A different ref is a different immutable observation, never a silent cache hit.
	if _, err := c.Sync(context.Background(), "owner/repo", "other-ref", 1); err == nil {
		t.Fatal("cache ignored ref/request budget")
	}
}

func TestFailedRepositoryResolutionIsMemoized(t *testing.T) {
	for _, kind := range []string{"private", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			c := NewClient("")
			requests := 0
			c.HTTP.Transport = importTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				var payload any
				switch {
				case strings.Contains(r.URL.Path, "/commits/"):
					payload = map[string]any{"sha": strings.Repeat("a", 40), "commit": map[string]any{"tree": map[string]any{"sha": strings.Repeat("b", 40)}}}
				case strings.Contains(r.URL.Path, "/git/trees/"):
					payload = map[string]any{"truncated": true, "tree": []any{}}
				default:
					payload = map[string]any{"private": kind == "private"}
				}
				b, _ := json.Marshal(payload)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b))}, nil
			})
			for i := 0; i < 4; i++ {
				if _, err := c.SyncTarget(context.Background(), Hit{Repository: "owner/repo", Hint: fmt.Sprint(i)}, "HEAD", 1); err == nil {
					t.Fatal("failure accepted")
				}
			}
			expected := 1
			if kind == "truncated" {
				expected = 3
			}
			if requests != expected {
				t.Fatal("same failed repository repeatedly requested", requests, expected)
			}
		})
	}
}

func TestOnlyOneRepositoryContentCacheIsRetained(t *testing.T) {
	body := []byte("fixture")
	sha := gitBlob(body)
	c, _ := importFixture(t, []treeEntry{{"skills/a/SKILL.md", "100644", "blob", sha, int64(len(body))}}, map[string][]byte{sha: body})
	transport := c.HTTP.Transport
	// Reuse the fixture protocol for distinct public repository names.
	c.HTTP.Transport = importTransport(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		u := *r.URL
		copy.URL = &u
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		copy.URL.Path = "/repos/owner/repo" + strings.TrimPrefix(r.URL.Path, "/repos/"+parts[1]+"/"+parts[2])
		return transport.RoundTrip(copy)
	})
	for _, repo := range []string{"owner/one", "owner/two", "owner/three", "owner/one"} {
		s, err := c.Sync(context.Background(), repo, "HEAD", 1)
		if err != nil || len(s.Entries) != 1 {
			t.Fatal(s, err)
		}
		retained := 0
		for _, snapshot := range c.repositories {
			if snapshot.blobs != nil {
				retained++
			}
		}
		if retained != 1 {
			t.Fatal("content retained for inactive repositories", retained)
		}
	}
}
