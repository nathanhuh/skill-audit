package main

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

	"skill-audit/internal/workflow"
)

func TestInventoryExplicitHomeWithoutEnvironmentHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("USERPROFILE"); err != nil {
		t.Fatal(err)
	}
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(d, ".claude", "skills", "example")
	if err = os.MkdirAll(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte("body"), 0600); err != nil {
		t.Fatal(err)
	}
	out, errout := new(bytes.Buffer), new(bytes.Buffer)
	if status := run(context.Background(), []string{"inventory", "--root", d, "--home", d}, out, errout); status != 0 {
		t.Fatal(status, errout)
	}
	var inv workflow.Inventory
	if err = json.Unmarshal(out.Bytes(), &inv); err != nil || len(inv.Packages) != 1 || inv.Packages[0].Scope != "global" {
		t.Fatal(out, err)
	}
	out.Reset()
	errout.Reset()
	if status := run(context.Background(), []string{"inventory", "--root", d}, out, errout); status != 0 {
		t.Fatal("optional home lookup became fatal", status, errout)
	}
}

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise real CLI target merging and the real importer through an in-memory
// GitHub transport. No external URL or skill script is fetched/executed.
func TestExplicitSyncWithSearchHintsAndLimits(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	manifest := []byte("---\nname: example\n---\nfixture data\n")
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(manifest), 0)
	h.Write(manifest)
	blob := hex.EncodeToString(h.Sum(nil))
	commit := strings.Repeat("a", 40)
	tree := strings.Repeat("b", 40)
	var reposCalled []string
	http.DefaultTransport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" {
			return nil, fmt.Errorf("unexpected host")
		}
		var response any
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "repos" {
			return nil, fmt.Errorf("unexpected path: %s", r.URL.Path)
		}
		repo := parts[1] + "/" + parts[2]
		switch {
		case len(parts) == 3:
			reposCalled = append(reposCalled, repo)
			response = map[string]any{"private": false}
		case len(parts) == 5 && parts[3] == "commits":
			response = map[string]any{"sha": commit, "commit": map[string]any{"tree": map[string]any{"sha": tree}}}
		case len(parts) == 6 && parts[4] == "trees":
			entries := []any{}
			for _, name := range []string{"first", "second"} {
				entries = append(entries, map[string]any{"path": "skills/" + name + "/SKILL.md", "type": "blob", "mode": "100644", "sha": blob, "size": len(manifest)})
			}
			response = map[string]any{"truncated": false, "tree": entries}
		case len(parts) == 6 && parts[4] == "blobs":
			response = map[string]any{"sha": blob, "size": len(manifest), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(manifest)}
		default:
			return nil, fmt.Errorf("unexpected path: %s", r.URL.Path)
		}
		b, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b))}, nil
	})
	for _, tc := range []struct {
		name, limit           string
		hits                  []workflow.Hit
		wantCode, wantEntries int
	}{
		{"hint cannot narrow explicit", "10", []workflow.Hit{{Repository: "z/explicit", Path: "skills/second/SKILL.md"}}, 0, 2},
		{"registry hint cannot narrow explicit", "10", []workflow.Hit{{Repository: "z/explicit", Hint: "second"}}, 0, 2},
		{"limit does not drop explicit repo", "1", []workflow.Hit{{Repository: "a/search", Path: "skills/second/SKILL.md"}}, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reposCalled = nil
			d, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			searchFile := filepath.Join(d, "search.json")
			catalog := filepath.Join(d, "catalog.json")
			if err = workflow.WriteJSON(searchFile, workflow.Search{Schema: "skill-audit/search/v2", Hits: tc.hits}); err != nil {
				t.Fatal(err)
			}
			out, errout := new(bytes.Buffer), new(bytes.Buffer)
			code := run(context.Background(), []string{"sync", "--repo", "z/explicit", "--search", searchFile, "--limit", tc.limit, "--out", catalog}, out, errout)
			if code != tc.wantCode {
				t.Fatal(code, out, errout)
			}
			var store workflow.Store
			if err = workflow.ReadJSON(catalog, &store); err != nil || len(store.Entries) != tc.wantEntries {
				t.Fatal(store, err)
			}
			if len(reposCalled) == 0 || reposCalled[0] != "z/explicit" {
				t.Fatal("explicit target lost budget priority", reposCalled)
			}
			hasExplicit := false
			for _, e := range store.Entries {
				if e.Reference.Repository == "https://github.com/z/explicit" && e.Reference.UpstreamPath == "skills/first" {
					hasExplicit = true
				}
			}
			if !hasExplicit {
				t.Fatal("full explicit import narrowed/dropped", store.Entries)
			}
			if tc.limit == "1" {
				limited := false
				for _, d := range store.Diagnostics {
					if d.Code == "package_limit" && d.Incomplete {
						limited = true
					}
				}
				if !limited {
					t.Fatal("repository package limit silently ignored")
				}
			}
		})
	}
}
