package audit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
func TestD4ImplicitMacAliasRuleKey(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS alias scenario")
	}
	info, e := os.Stat(tempDir(t))
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"/tmp", "/etc", "/var"} {
		rule := exclusionRule(p, info, false)
		if rule != "system"+p || strings.Contains(rule, "//") {
			t.Fatal(p, rule)
		}
		if rule = exclusionRule(p, info, true); rule != "" {
			t.Fatal("explicit alias bypasses the no-follow gate", p, rule)
		}
	}
}
func TestD8FiftyThousandMetadataExclusions(t *testing.T) {
	if os.Getenv("SKILL_AUDIT_LARGE_FIXTURES") != "1" {
		t.Skip("set SKILL_AUDIT_LARGE_FIXTURES=1 for the 50,000-entry fixture")
	}
	root := tempDir(t)
	for i := 0; i < 50000; i++ {
		fixture(t, root, fmt.Sprintf("d%05d/.DS_Store", i), "fixture")
	}
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	if !inv.Complete || len(inv.Exclusions) != 1 {
		t.Fatal(inv.Complete, len(inv.Exclusions))
	}
	x := inv.Exclusions[0]
	if x.Count != 50000 || len(x.Examples) != 3 || x.OmittedExamples != 49997 {
		t.Fatal(x)
	}
	b, e := Encode(inv)
	if e != nil || len(b) > 10000 {
		t.Fatal("unbounded exclusion evidence", len(b), e)
	}
}
func TestX4DefaultProbeAllowsColdStart(t *testing.T) {
	exe := fixture(t, tempDir(t), "scanner", "trusted fake data")
	s := NewScanner(exe, time.Minute, 30*time.Second)
	defer s.Close()
	s.process = func(ctx context.Context, _ string, args []string) (string, int, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 6*time.Second {
			t.Fatal("cold probe would be capped below six seconds")
		}
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	e, _ := s.Scan(context.Background(), measuredFixture(t))
	if !Completed(e) {
		t.Fatal(e)
	}
}

func TestB7CompletedEmptyScopeRecords(t *testing.T) {
	inv := InventoryRun(context.Background(), fixtureOptions(tempDir(t)))
	state := filepath.Join(tempDir(t), "state.json")
	r := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: state, Record: "baseline"})
	if !r.Complete || len(r.Skills) != 0 || r.Recording.Status != "recorded" || Exit(r) != 0 {
		t.Fatal(r)
	}
	loaded, e := ReadState(state)
	if e != nil || len(loaded.Observations) != 1 || len(loaded.Observations[0].Skills) != 0 {
		t.Fatal(loaded, e)
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
