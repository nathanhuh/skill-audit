package workflow

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"skill-audit/internal/audit"
)

func sizedImportFixture(t *testing.T, packages, files int) *Client {
	t.Helper()
	entries := []treeEntry{}
	blobs := map[string][]byte{}
	for p := 0; p < packages; p++ {
		dir := fmt.Sprintf("skills/p%d/", p)
		manifest := []byte(fmt.Sprintf("fixture package %d\n", p))
		sha := gitBlob(manifest)
		blobs[sha] = manifest
		entries = append(entries, treeEntry{dir + "SKILL.md", "100644", "blob", sha, int64(len(manifest))})
		for f := 0; f < files; f++ {
			b := bytes.Repeat([]byte{byte(1 + p*files + f)}, audit.MaxFile)
			sha := gitBlob(b)
			blobs[sha] = b
			entries = append(entries, treeEntry{fmt.Sprintf("%sfile-%d", dir, f), "100644", "blob", sha, int64(len(b))})
		}
	}
	c, _ := importFixture(t, entries, blobs)
	original := c.HTTP.Transport
	c.HTTP.Transport = importTransport(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		u := *r.URL
		clone.URL = &u
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		clone.URL.Path = "/repos/owner/repo" + strings.TrimPrefix(r.URL.Path, "/repos/"+parts[1]+"/"+parts[2])
		return original.RoundTrip(clone)
	})
	return c
}
func TestRevisitedRepositoryDoesNotCountReloadedBytesTwice(t *testing.T) {
	c := sizedImportFixture(t, 1, 9)
	for _, repo := range []string{"owner/one", "owner/two", "owner/one"} {
		s, err := c.Sync(context.Background(), repo, "HEAD", 1)
		if err != nil || len(s.Entries) != 1 || len(s.Diagnostics) != 0 {
			t.Fatal("valid package lost after eviction", repo, len(s.Entries), s.Diagnostics, err)
		}
	}
}
func TestMonorepoImportKeepsAllIndividuallyBoundedPackages(t *testing.T) {
	c := sizedImportFixture(t, 4, 5)
	s, err := c.Sync(context.Background(), "owner/repo", "HEAD", 10)
	if err != nil || len(s.Entries) != 4 || len(s.Diagnostics) != 0 {
		t.Fatal("repository-wide byte cap dropped a valid skill", len(s.Entries), s.Diagnostics, err)
	}
	for _, snapshot := range c.repositories {
		if snapshot.cachedBytes > MaxBlobCache {
			t.Fatal("cache admission exceeded memory cap", snapshot.cachedBytes)
		}
	}
}

func TestTransientRepositoryFailureCanRecoverOnNextTarget(t *testing.T) {
	for _, failure := range []string{"502", "403", "transport"} {
		t.Run(failure, func(t *testing.T) {
			body := []byte("fixture")
			sha := gitBlob(body)
			c, _ := importFixture(t, []treeEntry{{"skills/pkg/SKILL.md", "100644", "blob", sha, int64(len(body))}}, map[string][]byte{sha: body})
			transport := c.HTTP.Transport
			calls := 0
			c.HTTP.Transport = importTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					if failure == "transport" {
						return nil, fmt.Errorf("fixture dropped connection")
					}
					code := 502
					if failure == "403" {
						code = 403
					}
					return &http.Response{StatusCode: code, Header: make(http.Header), Body: http.NoBody}, nil
				}
				return transport.RoundTrip(r)
			})
			if _, err := c.Sync(context.Background(), "owner/repo", "HEAD", 1); err == nil {
				t.Fatal("expected first failure")
			}
			s, err := c.Sync(context.Background(), "owner/repo", "HEAD", 1)
			if err != nil || len(s.Entries) != 1 {
				t.Fatal("transient error cached permanently", s, err, calls)
			}
		})
	}
}
